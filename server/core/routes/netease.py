import time
import os
import threading
import subprocess
import datetime
import requests
from flask import Blueprint, redirect, request, Response
from core.config import app_config
from core.models.db import get_db
from core.services.downloader import (
    call_netease_api,
    _extract_song_level,
    _extract_song_size,
    _format_netease_songs,
    _resolve_netease_input,
    _fetch_playlist_songs,
    _fetch_song_detail,
    run_download_task,
    get_active_download_tasks_count,
    NETEASE_MAX_CONCURRENT,
    get_download_task_status,
    update_download_task,
    clear_download_task,
    clear_all_download_tasks
)
from core.utils.logger import logger
from core.utils.common import normalize_cookie_string
from core.routes.ws import broadcast_ws_message

netease_bp = Blueprint('netease', __name__)

# Docker API 服务自动安装任务的状态缓存
INSTALL_STATUS = {
    'status': 'idle',  # idle, running, success, error
    'progress': 0,
    'step': '',
    'error': None
}

# 网易云登录用户状态缓存，避免频繁串行请求网易云官网
_user_status_cache = None
_user_status_cache_time = 0
USER_STATUS_CACHE_TTL = 43200  # 缓存有效期 12 小时 (43200秒)

# 网易云每日推荐缓存，缓存格式为列表 [song, ...]
_daily_recommend_cache = None
_daily_recommend_cache_time = 0

def clear_netease_user_cache():
    """手动清空网易云用户状态缓存及每日推荐缓存，以在下一次请求时强制向官网重新拉取"""
    global _user_status_cache, _user_status_cache_time, _daily_recommend_cache, _daily_recommend_cache_time
    _user_status_cache = None
    _user_status_cache_time = 0
    _daily_recommend_cache = None
    _daily_recommend_cache_time = 0

def save_netease_cookie(cookie_str: str):
    """保存网易云 Cookie，并更新内存缓存"""
    app_config.NETEASE_COOKIE = normalize_cookie_string(cookie_str or '')
    try:
        with get_db() as conn:
            conn.execute("INSERT OR REPLACE INTO system_settings (key, value) VALUES (?, ?)", ('netease_cookie', app_config.NETEASE_COOKIE))
            conn.commit()
    except Exception as e:
        logger.warning(f"保存网易云 cookie 失败: {e}")

def save_netease_config(download_dir: str = None, api_base: str = None):
    """保存网易云下载路径及 API 基础地址配置"""
    if download_dir:
        app_config.NETEASE_DOWNLOAD_DIR = download_dir
    if api_base is not None:
        app_config.NETEASE_API_BASE = api_base.strip().rstrip('/')
        
    try:
        with get_db() as conn:
            if download_dir:
                conn.execute("INSERT OR REPLACE INTO system_settings (key, value) VALUES (?, ?)", ('netease_download_dir', download_dir))
            if api_base is not None:
                conn.execute("INSERT OR REPLACE INTO system_settings (key, value) VALUES (?, ?)", ('netease_api_base', app_config.NETEASE_API_BASE))
            conn.commit()
    except Exception as e:
        logger.warning(f"保存网易云配置失败: {e}")

# --- WebSocket 处理逻辑实现 ---

def handle_search_netease_music(keywords: str, limit: any) -> tuple:
    """网易云云端检索"""
    keywords = (keywords or '').strip()
    if not keywords:
        return False, None, "请输入搜索关键词"
    try:
        limit = max(1, min(int(limit or 20), 50))
    except Exception:
        limit = 20

    try:
        api_resp = call_netease_api('/cloudsearch', {'keywords': keywords, 'type': 1, 'limit': limit})
        songs = []
        for item in api_resp.get('result', {}).get('songs', []):
            song_id = item.get('id')
            if not song_id: 
                continue
            artists = ' / '.join([a.get('name') for a in item.get('ar', []) if a.get('name')]) or '未知艺术家'
            album_info = item.get('al') or {}
            privilege = item.get('privilege') or {}
            fee = item.get('fee')
            privilege_fee = privilege.get('fee')
            is_vip = (fee == 1) or (privilege_fee == 1)
            user_level, max_level = _extract_song_level(privilege)
            songs.append({
                'id': song_id,
                'title': item.get('name') or f"未命名 {song_id}",
                'artist': artists,
                'album': album_info.get('name') or '',
                'cover': (album_info.get('picUrl') or '').replace('http://', 'https://'),
                'duration': (item.get('dt') or 0) / 1000,
                'level': user_level,
                'max_level': max_level,
                'size': _extract_song_size(item),
                'is_vip': is_vip
            })
        return True, songs, None
    except Exception as e:
        logger.warning(f"网易云搜索失败: {e}")
        return False, None, "搜索失败，请检查网易云 API 服务"

def handle_netease_daily_recommend() -> tuple:
    """获取网易云每日推荐 (需登录账号，带每日推荐缓存机制)"""
    global _daily_recommend_cache, _daily_recommend_cache_time
    
    # 检查推荐缓存是否有效
    now = time.time()
    if _daily_recommend_cache is not None:
        # 减去 6 小时以符合网易云每日推荐 6:00 AM 更新的时间
        def get_recommend_day(ts):
            return (datetime.datetime.fromtimestamp(ts) - datetime.timedelta(hours=6)).strftime('%Y-%m-%d')
            
        if get_recommend_day(now) == get_recommend_day(_daily_recommend_cache_time):
            logger.info("网易云每日推荐：命中内存缓存，直接返回")
            return True, _daily_recommend_cache, None

    try:
        api_resp = call_netease_api('/recommend/songs', {'timestamp': int(now * 1000)}, need_cookie=True)
        if isinstance(api_resp, dict) and api_resp.get('code') == 301:
            return False, None, "需要登录以获取每日推荐"
        daily = (api_resp.get('data') or {}).get('dailySongs', []) if isinstance(api_resp, dict) else []
        songs = _format_netease_songs(daily)
        
        # 成功获取后，记录在内存缓存中
        if songs:
            _daily_recommend_cache = songs
            _daily_recommend_cache_time = now
            
        return True, songs, None
    except Exception as e:
        logger.warning(f"获取每日推荐失败: {e}")
        return False, None, "获取每日推荐失败，请检查登录状态或 API 服务"

def handle_netease_login_status() -> tuple:
    """查询 Cookie 登录有效性 (带有内存缓存加速机制)"""
    global _user_status_cache, _user_status_cache_time
    now = time.time()
    
    # 若缓存未过期且有效，直接返回
    if _user_status_cache is not None and (now - _user_status_cache_time) < USER_STATUS_CACHE_TTL:
        logger.debug("网易云登录状态：命中内存缓存，直接返回")
        return True, _user_status_cache, None

    try:
        if not app_config.NETEASE_COOKIE:
            logger.info("网易云登录状态检查：当前未加载 cookie")
            return True, {'logged_in': False, 'error': '未登录'}, None
            
        api_resp = call_netease_api('/login/status', {'timestamp': int(now * 1000)}, need_cookie=True)
        profile = api_resp.get('data', {}).get('profile') if isinstance(api_resp, dict) else None
        if profile:
            is_vip = False
            vip_info = {}
            try:
                vip_resp = call_netease_api('/vip/info', {'uid': profile.get('userId')})
                if isinstance(vip_resp, dict):
                    vip_info = vip_resp.get('data') or vip_resp
                    data = vip_info or {}
                    now_ms = int(now * 1000)

                    def _active(pkg: dict):
                        if not pkg:
                            return False
                        code = pkg.get('vipCode') or 0
                        exp = pkg.get('expireTime') or pkg.get('expiretime')
                        if code <= 0:
                            return False
                        if exp is None:
                            return False
                        try:
                            return int(exp) > now_ms
                        except Exception:
                            return False

                    is_vip = bool(data.get('isVip'))
                    if not is_vip:
                        is_vip = any([
                            _active(data.get('associator')),
                            _active(data.get('musicPackage')),
                            _active(data.get('redplus')),
                            _active(data.get('familyVip'))
                        ])
            except Exception as e:
                logger.warning(f"获取VIP信息失败: {e}")
                
            res_data = {
                'logged_in': True,
                'nickname': profile.get('nickname'),
                'user_id': profile.get('userId'),
                'avatar': profile.get('avatarUrl'),
                'is_vip': is_vip,
                'vip_info': vip_info
            }
            # 更新缓存
            _user_status_cache = res_data
            _user_status_cache_time = now
            return True, res_data, None
            
        res_data = {'logged_in': False, 'error': '未登录'}
        _user_status_cache = res_data
        _user_status_cache_time = now
        return True, res_data, None
    except Exception as e:
        logger.warning(f"检查网易云登录状态失败: {e}")
        return False, None, "状态检查失败"

def handle_netease_logout() -> tuple:
    """退出账号并清除 Cookie"""
    try:
        if app_config.NETEASE_COOKIE:
            try:
                call_netease_api('/logout', {'timestamp': int(time.time() * 1000)}, need_cookie=True)
            except Exception as e:
                logger.info(f"网易云 API 注销调用失败，继续清理本地 cookie: {e}")
        save_netease_cookie('')
        clear_netease_user_cache()
        return True, None, None
    except Exception as e:
        logger.warning(f"网易云退出登录失败: {e}")
        return False, None, "退出失败"

def poll_netease_qr_status(key: str):
    """后台线程：轮询检测网易云扫码状态并进行 WebSocket 实时推送"""
    logger.info(f"启动网易云扫码后台监听线程: {key}")
    last_status = None
    start_time = time.time()
    max_duration = 300  # 最多轮询 5 分钟
    
    while time.time() - start_time < max_duration:
        try:
            resp = call_netease_api('/login/qr/check', {'key': key, 'timestamp': int(time.time() * 1000)}, need_cookie=False)
            code = resp.get('code')
            message = resp.get('message')
            cookie_str = resp.get('cookie')
            if not cookie_str and isinstance(resp.get('cookies'), list):
                cookie_str = '; '.join(resp.get('cookies'))
                
            status_map = {
                800: 'expired',
                801: 'waiting',
                802: 'scanned',
                803: 'authorized'
            }
            status = status_map.get(code, 'unknown')
            
            # 如果状态发生变化，广播推送给前端
            if status != last_status:
                logger.info(f"网易云扫码状态变化: {last_status} -> {status} (key: {key})")
                last_status = status
                
                broadcast_ws_message('netease_login_status', {
                    'key': key,
                    'status': status,
                    'message': message
                })
                
            if status == 'authorized' and cookie_str:
                save_netease_cookie(cookie_str)
                clear_netease_user_cache()
                logger.info(f"网易云扫码登录成功，已保存 Cookie，线程优雅退出 (key: {key})")
                break
                
            if status in ('expired', 'unknown'):
                logger.info(f"网易云扫码状态为 {status}，线程优雅退出 (key: {key})")
                break
                
        except Exception as e:
            logger.warning(f"网易云扫码后台轮询异常: {e}")
            
        time.sleep(2)
    else:
        logger.info(f"网易云扫码后台轮询超时退出 (key: {key})")

def handle_netease_login_qrcode() -> tuple:
    """获取扫码登录二维码"""
    try:
        key_resp = call_netease_api('/login/qr/key', {'timestamp': int(time.time() * 1000)}, need_cookie=False)
        unikey = key_resp.get('data', {}).get('unikey')
        if not unikey:
            return False, None, "获取登录 key 失败"
        qr_resp = call_netease_api('/login/qr/create', {'key': unikey, 'qrimg': 1, 'timestamp': int(time.time() * 1000)}, need_cookie=False)
        qrimg = qr_resp.get('data', {}).get('qrimg')
        if not qrimg:
            return False, None, "获取二维码失败"
            
        # 启动后台监听线程
        threading.Thread(target=poll_netease_qr_status, args=(unikey,), daemon=True).start()
        
        return True, {'unikey': unikey, 'qrimg': qrimg}, None
    except Exception as e:
        logger.warning(f"生成网易云二维码失败: {e}")
        return False, None, "二维码生成失败"

def handle_netease_config(method: str, download_dir: str = None, api_base: str = None) -> tuple:
    """读写网易云下载位置及 API Base 根路径设置"""
    try:
        if method == 'GET':
            return True, {
                'download_dir': app_config.NETEASE_DOWNLOAD_DIR, 
                'api_base': app_config.NETEASE_API_BASE, 
                'max_concurrent': NETEASE_MAX_CONCURRENT,
                'quality': app_config.NETEASE_QUALITY_DEFAULT
            }, None
            
        if download_dir:
            download_dir = os.path.abspath(download_dir)
            os.makedirs(download_dir, exist_ok=True)
            
        if api_base is not None:
            api_base = api_base.strip().rstrip('/')
            if api_base:
                try:
                    # 校验 API 是否可用：请求 /login/status 接口，超时为 3 秒
                    test_url = f"{api_base}/login/status"
                    resp = requests.get(test_url, timeout=3.0)
                    resp.raise_for_status()
                except Exception as e:
                    logger.warning(f"网易云 API 连通性测试失败 (url: {api_base}): {e}")
                    return False, None, "API 连接测试失败：请检查服务是否已启动且地址正确"

                try:
                    data = resp.json()
                    # 校验返回数据的网易云 API 格式特征
                    if not isinstance(data, dict) or ('code' not in data and 'data' not in data):
                        return False, None, "API 校验失败：该地址未返回符合网易云 API 特征的数据"
                except Exception as e:
                    logger.warning(f"网易云 API 数据解析失败 (url: {api_base}): {e}")
                    return False, None, "API 校验失败：该地址不是合法的网易云 API 服务"
            
        if download_dir is None and api_base is None:
            return False, None, "未提供任何配置项"
            
        save_netease_config(download_dir, api_base)
        return True, {
            'download_dir': app_config.NETEASE_DOWNLOAD_DIR, 
            'api_base': app_config.NETEASE_API_BASE, 
            'max_concurrent': NETEASE_MAX_CONCURRENT,
            'quality': app_config.NETEASE_QUALITY_DEFAULT
        }, None
    except Exception as e:
        logger.warning(f"更新网易云配置失败: {e}")
        return False, None, f"保存失败: {str(e)}"

def handle_netease_resolve(raw_input: str) -> tuple:
    """短链跳转及特征解析"""
    parsed_input = _resolve_netease_input(raw_input)
    if not parsed_input:
        return False, None, "请粘贴网易云分享链接或输入ID"
    try:
        if parsed_input['type'] == 'playlist':
            songs = _fetch_playlist_songs(parsed_input['id'])
            return True, {'type': 'playlist', 'id': parsed_input['id'], 'name': '', 'data': songs}, None
        songs = _fetch_song_detail(parsed_input['id'])
        return True, {'type': 'song', 'id': parsed_input['id'], 'data': songs}, None
    except Exception as e:
        logger.warning(f"解析网易云链接失败: {e}")
        return False, None, "解析失败，请确认链接有效且 API 服务正常"



def handle_download_netease_music(payload: dict) -> tuple:
    """异步下载单曲"""
    song_id = payload.get('id')
    if not song_id:
        return False, None, "缺少歌曲ID"

    active = get_active_download_tasks_count()
    if active >= NETEASE_MAX_CONCURRENT:
        return False, None, f"并发下载已达上限 ({NETEASE_MAX_CONCURRENT})，请稍后再试"
    
    task_id = f"task_{int(time.time()*1000)}_{os.urandom(4).hex()}"
    update_download_task(
        task_id,
        status='pending',
        progress=0,
        title=payload.get('title', '未知'),
        artist=payload.get('artist', '未知')
    )
    
    # 异步开始任务，避免请求挂起
    threading.Thread(target=run_download_task, args=(task_id, payload), daemon=True).start()
    return True, {'task_id': task_id}, None

def handle_get_netease_task_detail(task_id: str) -> tuple:
    """查询指定下载任务的执行状态与进度"""
    task = get_download_task_status(task_id)
    if not task:
        return False, None, "任务不存在"
    return True, task, None

def handle_clear_netease_task(task_id: str) -> tuple:
    """清除指定下载任务的状态记录，并向所有客户端广播被删除的状态"""
    if not task_id:
        return False, None, "缺少任务ID"
    success = clear_download_task(task_id)
    if success:
        # 向前端广播以同步移除本地任务
        broadcast_ws_message('download_status', {'task_id': task_id, 'status': 'deleted'})
    return True, {'task_id': task_id}, None

def handle_clear_all_netease_tasks() -> tuple:
    """清除所有已完成或失败的下载任务记录，并向所有客户端广播删除的消息"""
    cleared_ids = clear_all_download_tasks()
    for tid in cleared_ids:
        broadcast_ws_message('download_status', {'task_id': tid, 'status': 'deleted'})
    return True, {'cleared_count': len(cleared_ids)}, None

def handle_check_docker_container() -> tuple:
    """检查 Docker 环境和目标容器状态（不触发安装）"""
    result = {
        'docker_installed': False,
        'container_exists': False,
        'container_running': False,
    }
    container_name = "2fmusic-ncm-api"
    try:
        subprocess.run(["docker", "--version"], check=True, capture_output=True)
        result['docker_installed'] = True

        check_proc = subprocess.run(
            ["docker", "ps", "-a", "--filter", f"name={container_name}", "--format", "{{.Names}}"],
            capture_output=True, text=True
        )
        if container_name in check_proc.stdout.strip():
            result['container_exists'] = True
            running_proc = subprocess.run(
                ["docker", "ps", "--filter", f"name={container_name}", "--filter", "status=running", "--format", "{{.Names}}"],
                capture_output=True, text=True
            )
            if container_name in running_proc.stdout.strip():
                result['container_running'] = True
    except FileNotFoundError:
        pass  # Docker 未安装
    except Exception as e:
        logger.warning(f"检查 Docker 容器状态时出现异常: {e}")
    return True, result, None

def handle_get_install_status() -> tuple:
    """查询 Docker API 安装任务的运行进度"""
    return True, INSTALL_STATUS, None

def handle_install_netease_service() -> tuple:
    """自动部署网易云 API 镜像服务进程"""
    global INSTALL_STATUS
    if INSTALL_STATUS['status'] == 'running':
         return False, None, "安装部署任务正在执行中"

    INSTALL_STATUS = {'status': 'running', 'progress': 0, 'step': '准备安装...', 'error': None}
    logger.info("开始执行部署网易云服务")
    
    def run_install():
        global INSTALL_STATUS
        try:
            INSTALL_STATUS.update({'progress': 10, 'step': '检查 Docker 环境...'})
            subprocess.run(["docker", "--version"], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            
            container_name = "2fmusic-ncm-api"
            INSTALL_STATUS.update({'progress': 20, 'step': f'检查容器 {container_name}...'})
            
            check_proc = subprocess.run(
                ["docker", "ps", "-a", "--filter", f"name={container_name}", "--format", "{{.Names}}"],
                capture_output=True, text=True
            )
            
            if container_name in check_proc.stdout.strip():
                INSTALL_STATUS.update({'progress': 60, 'step': '容器已存在，正在启动...'})
                logger.info("容器已存在，尝试启动...")
                subprocess.run(["docker", "start", container_name], check=True)
            else:
                INSTALL_STATUS.update({'progress': 30, 'step': '正在拉取镜像 (耗时较长)...'})
                logger.info("正在拉取镜像 moefurina/ncm-api...")
                subprocess.run(["docker", "pull", "moefurina/ncm-api:latest"], check=True)
                
                INSTALL_STATUS.update({'progress': 70, 'step': '镜像拉取完成，正在启动容器...'})
                logger.info("正在启动容器...")
                subprocess.run([
                    "docker", "run", "-d", 
                    "-p", "23236:3000", 
                    "--name", container_name, 
                    "--restart", "always",
                    "moefurina/ncm-api"
                ], check=True)
            
            INSTALL_STATUS.update({'status': 'success', 'progress': 100, 'step': '服务启动成功！'})
            # 保存默认 API 地址，使前端自动建立连接
            try:
                save_netease_config(api_base=app_config.NETEASE_API_BASE_DEFAULT)
            except Exception as e:
                logger.warning(f"保存默认 API 地址失败: {e}")
            logger.info("网易云服务安装/启动指令执行完成")
            
        except subprocess.CalledProcessError as e:
            msg = f"操作失败: {e}"
            logger.error(msg)
            INSTALL_STATUS.update({'status': 'error', 'error': msg, 'step': '发生错误'})
        except FileNotFoundError:
            msg = "未找到 Docker，请确保已安装 Docker Desktop"
            logger.error(msg)
            INSTALL_STATUS.update({'status': 'error', 'error': msg, 'step': '环境缺失'})
        except Exception as e:
            msg = f"未知错误: {str(e)}"
            logger.exception(msg)
            INSTALL_STATUS.update({'status': 'error', 'error': msg, 'step': '系统异常'})

    threading.Thread(target=run_install, daemon=True).start()
    return True, "安装部署任务已挂起后台启动", None

# --- 仅保留下载落地页跳转的 HTTP 接口 ---

@netease_bp.route('/api/netease/download_page')
def netease_download_page():
    """网易云下载落地页跳转"""
    return redirect("https://music.163.com/client")

@netease_bp.route('/api/netease/<path:subpath>', methods=['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'])
def proxy_netease_api(subpath):
    """代理网易云 API 请求"""
    if not app_config.NETEASE_API_BASE:
        return {"success": False, "error": "网易云 API 服务未配置"}, 400
    
    url = f"{app_config.NETEASE_API_BASE}/{subpath}"
    if request.query_string:
        url += f"?{request.query_string.decode('utf-8')}"
        
    try:
        resp = requests.request(
            method=request.method,
            url=url,
            headers={key: value for key, value in request.headers if key.lower() not in ('host', 'content-length')},
            data=request.get_data(),
            cookies=request.cookies,
            allow_redirects=False,
            timeout=10.0
        )
        
        excluded_headers = ['content-encoding', 'content-length', 'transfer-encoding', 'connection']
        headers = [
            (name, value) for name, value in resp.raw.headers.items()
            if name.lower() not in excluded_headers
        ]
        
        return Response(resp.content, resp.status_code, headers)
    except Exception as e:
        logger.warning(f"代理网易云 API 请求失败 (url: {url}): {e}")
        return {"success": False, "error": f"代理失败: {str(e)}"}, 500
