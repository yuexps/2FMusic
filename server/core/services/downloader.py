import os
import re
import time
import requests
from urllib.parse import urlparse, parse_qs
import shutil

from core.config import app_config
from core.services.metadata import (
    fetch_cover_bytes,
    embed_cover_to_file,
    save_cover_file,
    embed_lyrics_to_file
)
# 这里由于 scan_library_incremental 等方法可能发生循环依赖，故 index_single_file 延迟导入

from core.utils.logger import logger
from core.utils.common import COMMON_HEADERS, parse_cookie_string

# WebSocket 广播回调
_ws_broadcast_callback = None

def register_downloader_ws_broadcast_callback(cb):
    """注册用于下载任务状态的 WebSocket 广播回调"""
    global _ws_broadcast_callback
    _ws_broadcast_callback = cb

# 全局任务池与并发限制
DOWNLOAD_TASKS = {}
NETEASE_MAX_CONCURRENT = 5

def clean_expired_tasks():
    """清除超过 10 分钟已完成或失败的任务状态"""
    global DOWNLOAD_TASKS
    now = time.time()
    to_delete = []
    for tid, task in list(DOWNLOAD_TASKS.items()):
        status = task.get('status')
        if status in ('success', 'error'):
            completed_at = task.get('completed_at', 0)
            if completed_at and (now - completed_at) > 600:
                to_delete.append(tid)
    for tid in to_delete:
        DOWNLOAD_TASKS.pop(tid, None)

def update_download_task(task_id: str, **kwargs):
    """更新下载任务状态，并通过 WebSocket 实时推送给前端"""
    global DOWNLOAD_TASKS
    
    try:
        clean_expired_tasks()
    except Exception:
        pass

    if task_id not in DOWNLOAD_TASKS:
        DOWNLOAD_TASKS[task_id] = {}
        
    for k, v in kwargs.items():
        DOWNLOAD_TASKS[task_id][k] = v

    if 'status' in kwargs and kwargs['status'] in ('success', 'error'):
        DOWNLOAD_TASKS[task_id]['completed_at'] = time.time()
        
    task_info = dict(DOWNLOAD_TASKS[task_id])
    task_info['task_id'] = task_id
    
    if _ws_broadcast_callback:
        try:
            _ws_broadcast_callback('download_status', task_info)
        except Exception as e:
            logger.debug(f"广播下载进度失败: {e}")
            
    return task_info

def get_download_task_status(task_id: str) -> dict:
    """获取下载任务状态"""
    return DOWNLOAD_TASKS.get(task_id)

def sanitize_filename(name: str) -> str:
    """清理文件名中的非法字符，避免写入失败"""
    cleaned = re.sub(r'[\\/:*?"<>|]+', '_', name).strip().strip('.')
    return cleaned or 'netease_song'

def call_netease_api(path: str, params: dict, method: str = 'GET', need_cookie: bool = True):
    """调用网易云本地 API 服务，统一拦截处理 Cookie"""
    api_base = app_config.NETEASE_API_BASE or app_config.NETEASE_API_BASE_DEFAULT
    base = api_base.rstrip('/')
    url = f"{base}{path}"
    
    headers = dict(COMMON_HEADERS)
    params = dict(params or {})
    cookies = {}
    
    cookie_str = app_config.NETEASE_COOKIE
    if need_cookie and cookie_str:
        # 透传 Cookie 字符串
        headers['Cookie'] = cookie_str
        params.setdefault('cookie', cookie_str)
        cookies = parse_cookie_string(cookie_str)
        
    if method.upper() == 'POST':
        resp = requests.post(url, data=params, timeout=10, headers=headers, cookies=cookies)
    else:
        resp = requests.get(url, params=params, timeout=10, headers=headers, cookies=cookies)
        
    resp.raise_for_status()
    return resp.json()

def _extract_song_level(privilege: dict):
    """返回 (用户可下载的最高音质, 曲目最高音质)"""
    privilege = privilege or {}
    def _norm(val):
        if not val:
            return 'standard'
        v = str(val).lower()
        if v == 'none': 
            return 'standard'
        if v.isdigit():
            br = int(v)
            if br >= 999000: return 'lossless'
            if br >= 320000: return 'exhigh'
            if br >= 192000: return 'higher'
            return 'standard'
        return v
    
    max_br = privilege.get('maxBrLevel') or privilege.get('maxbr') or privilege.get('maxLevel')
    max_level = _norm(max_br)
    user_level = _norm(privilege.get('dlLevel') or privilege.get('plLevel') or max_level)
    return (user_level or 'standard', max_level or user_level or 'standard')

def _extract_song_size(track: dict) -> int:
    """根据期望音质优先取对应大小，找不到再按从低到高回退"""
    if not track:
        return None
    level = 'exhigh'
    prefer_map = {
        'standard': ('l', 'm', 'h', 'sq', 'hr'),
        'higher': ('m', 'h', 'sq', 'hr'),
        'exhigh': ('h', 'sq', 'hr', 'm'),
        'lossless': ('sq', 'hr', 'h', 'm'),
        'hires': ('hr', 'sq', 'h', 'm'),
        'jyeffect': ('sq', 'h', 'm'),
        'sky': ('hr', 'sq', 'h', 'm'),
        'dolby': ('hr', 'sq', 'h', 'm'),
        'jymaster': ('hr', 'sq', 'h', 'm')
    }
    orders = prefer_map.get(level) or ('l', 'm', 'h', 'sq', 'hr')
    for key in orders:
        data = track.get(key) or {}
        size = data.get('size')
        if size:
            try:
                return int(size)
            except Exception:
                continue
    return None

def _format_netease_songs(source_tracks: list) -> list:
    """格式化网易云搜索或歌单返回的曲目数据"""
    songs = []
    for item in source_tracks or []:
        sid = item.get('id')
        if not sid:
            continue
        fee = item.get('fee')
        privilege = item.get('privilege') or {}
        privilege_fee = privilege.get('fee')
        is_vip = (fee == 1) or (privilege_fee == 1)
        user_level, max_level = _extract_song_level(privilege)
        artists = ' / '.join([a.get('name') for a in item.get('ar', []) if a.get('name')]) or '未知艺术家'
        album_info = item.get('al') or {}
        size_bytes = _extract_song_size(item)
        songs.append({
            'id': sid,
            'title': item.get('name') or f"未命名 {sid}",
            'artist': artists,
            'album': album_info.get('name') or '',
            'cover': (album_info.get('picUrl') or '').replace('http://', 'https://'),
            'duration': (item.get('dt') or 0) / 1000,
            'is_vip': is_vip,
            'level': user_level,
            'max_level': max_level,
            'size': size_bytes
        })
    return songs

def _resolve_netease_input(raw: str, prefer: str = None) -> dict:
    """根据链接或纯数字ID解析网易云曲目/歌单"""
    if not raw:
        return None
    prefer = prefer if prefer in ('song', 'playlist') else None
    text = str(raw).strip()

    if text.isdigit():
        return {'type': prefer or 'song', 'id': text}

    candidate = text
    if candidate.startswith(('music.163.com', 'y.music.163.com', '163cn.tv')):
        candidate = f"https://{candidate}"
        
    if re.match(r'^https?://', candidate, re.I):
        def _follow(url):
            try:
                resp = requests.get(url, allow_redirects=True, timeout=8, headers=COMMON_HEADERS)
                return resp.url or url
            except Exception as e:
                logger.warning(f"网易云链接跳转跟随失败: {e}")
                return None

        followed = _follow(candidate)
        if not followed and '163cn.tv' in candidate:
            try:
                resp = requests.head(candidate, allow_redirects=True, timeout=6, headers=COMMON_HEADERS)
                followed = resp.url or resp.headers.get('Location')
            except Exception as e:
                logger.warning(f"网易云短链 HEAD 跳转获取失败: {e}")
        if followed:
            candidate = followed

    def extract_from_url(url_str: str):
        parsed = urlparse(url_str)
        path = parsed.path or ''
        fragment = parsed.fragment or ''
        frag_path, frag_query = '', {}
        if fragment:
            if '?' in fragment:
                frag_path, frag_qs = fragment.split('?', 1)
                frag_query = parse_qs(frag_qs)
            else:
                frag_path = fragment
        query = parse_qs(parsed.query or '')

        def pick_id(qs):
            for key in ('id', 'songId', 'playlistId'):
                if qs.get(key):
                    return str(qs[key][0])
            return None

        rid = pick_id(query) or pick_id(frag_query)
        route_hint = None
        for seg in (path, frag_path):
            if 'playlist' in seg:
                route_hint = 'playlist'
                break
            if 'song' in seg:
                route_hint = 'song'
        if not rid:
            m = re.search(r'/(song|playlist)/(\d+)', path)
            if not m and frag_path:
                m = re.search(r'(song|playlist)[^0-9]*(\d+)', frag_path)
            if m:
                route_hint = route_hint or m.group(1)
                rid = m.group(2)
        if not rid:
            m = re.search(r'id=(\d+)', url_str)
            if m:
                rid = m.group(1)
        if rid:
            return {'type': route_hint or prefer or 'song', 'id': rid}
        return None

    parsed = extract_from_url(candidate)
    if parsed:
        return parsed

    m = re.search(r'(playlist|song)[^0-9]*(\d+)', text, re.IGNORECASE)
    if m:
        return {'type': m.group(1).lower(), 'id': m.group(2)}
    m = re.search(r'(\d{5,})', text)
    if m:
        return {'type': prefer or 'song', 'id': m.group(1)}
    return None

def _fetch_playlist_songs(playlist_id: str) -> list:
    """获取网易云歌单下所有曲目的详细列表"""
    detail_resp = call_netease_api('/playlist/detail', {'id': playlist_id})
    playlist = detail_resp.get('playlist') if isinstance(detail_resp, dict) else None
    
    if not playlist:
        return []
    track_ids = [str(t.get('id')) for t in playlist.get('trackIds', [])]
    if not track_ids:
        return []
        
    songs = []
    # 接口通常上限限制批量请求数量，每次最多获取 500 首
    chunk_size = 500
    for i in range(0, len(track_ids), chunk_size):
        chunk = track_ids[i:i+chunk_size]
        try:
            songs_resp = call_netease_api('/song/detail', {'ids': ','.join(chunk)})
            if isinstance(songs_resp, dict) and songs_resp.get('songs'):
                songs.extend(_format_netease_songs(songs_resp['songs']))
        except Exception as e:
            logger.warning(f"批量抓取歌单歌曲详情分片出错 (索引 {i}): {e}")
            
    return songs

def _fetch_song_detail(song_id: str) -> list:
    """获取网易云单曲的详细信息"""
    detail_resp = call_netease_api('/song/detail', {'ids': song_id})
    songs = detail_resp.get('songs', []) if isinstance(detail_resp, dict) else []
    parsed = _format_netease_songs(songs)
    if not parsed:
        raise Exception('未获取到歌曲信息')
    return parsed

def _normalize_cover_url(url: str) -> str:
    """规范化网易云图片分辨率参数"""
    if not url:
        return None
    u = url.replace('http://', 'https://')
    if '//' not in u:
        return None
    if 'param=' not in u and '?param=' not in u:
        sep = '&' if '?' in u else '?'
        u = f"{u}{sep}param=1024y1024"
    return u

def fetch_netease_lyrics(song_id: str) -> tuple:
    """拉取网易云音乐歌词与逐字歌词"""
    if not song_id:
        return None, None
    lrc_text = None
    yrc_text = None
    try:
        lyr_resp = call_netease_api('/lyric/new', {'id': song_id}, need_cookie=False)
        if isinstance(lyr_resp, dict):
            yrc_text = (lyr_resp.get('yrc') or {}).get('lyric')
            lrc_text = (lyr_resp.get('lrc') or {}).get('lyric')
        if not lrc_text:
            old_resp = call_netease_api('/lyric', {'id': song_id}, need_cookie=False)
            if isinstance(old_resp, dict):
                lrc_text = (old_resp.get('lrc') or {}).get('lyric') or lrc_text
                if not yrc_text:
                    yrc_text = (old_resp.get('yrc') or {}).get('lyric')
    except Exception as e:
        logger.warning(f"获取网易歌词失败: {e}")
    return lrc_text, yrc_text

def run_download_task(task_id: str, payload: dict):
    """单独执行下载任务的主线程逻辑"""
    song_id = payload.get('id')
    title = (payload.get('title') or '').strip()
    artist = (payload.get('artist') or '').strip()
    album = (payload.get('album') or '').strip()
    level = payload.get('level') or 'exhigh'
    cover_url = _normalize_cover_url(payload.get('cover') or payload.get('album_art'))
    cover_bytes = fetch_cover_bytes(cover_url) if cover_url else None
    
    target_dir = payload.get('target_dir') or app_config.NETEASE_DOWNLOAD_DIR
    target_dir = os.path.abspath(target_dir)
    
    update_download_task(task_id, status='preparing', progress=0)

    try:
        os.makedirs(target_dir, exist_ok=True)
        need_detail_for_level = not payload.get('level')
        need_detail_for_cover = cover_bytes is None
        
        # 1. 尝试从详情接口补充丢失的元数据
        if not title or need_detail_for_level or need_detail_for_cover:
            meta_resp = call_netease_api('/song/detail', {'ids': song_id})
            songs = meta_resp.get('songs', []) if isinstance(meta_resp, dict) else []
            if songs:
                info = songs[0]
                if need_detail_for_level:
                    level, _ = _extract_song_level(info.get('privilege') or {})
                title = info.get('name') or title or f"未命名 {song_id}"
                artist = ' / '.join([a.get('name') for a in info.get('ar', []) if a.get('name')]) or artist
                album = (info.get('al') or {}).get('name') or album
                if need_detail_for_cover and not cover_bytes:
                    pic_url = _normalize_cover_url((info.get('al') or {}).get('picUrl'))
                    if pic_url:
                        cover_bytes = fetch_cover_bytes(pic_url)
                        
        if not title:
            title = f"未命名 {song_id}"
        if not artist:
            artist = '未知艺术家'
            
        base_filename = sanitize_filename(f"{artist} - {title}")
            
        update_download_task(task_id, title=title, artist=artist)

        # 2. 调用 /song/url/v1 抓取曲目音乐地址
        api_resp = call_netease_api('/song/url/v1', {'id': song_id, 'level': level}, need_cookie=bool(app_config.NETEASE_COOKIE))
        data_list = api_resp.get('data') if isinstance(api_resp, dict) else None
        track_info = None
        if isinstance(data_list, list) and data_list:
            track_info = data_list[0]
        elif isinstance(data_list, dict):
            track_info = data_list

        # 如果没有高音质，回退标准音质再试一次
        if not track_info or (not track_info.get('url') and not track_info.get('proxyUrl')):
            if level != 'standard':
                try:
                    api_resp_std = call_netease_api('/song/url/v1', {'id': song_id, 'level': 'standard'}, need_cookie=bool(app_config.NETEASE_COOKIE))
                    data_list = api_resp_std.get('data') if isinstance(api_resp_std, dict) else None
                    if isinstance(data_list, list) and data_list:
                        track_info = data_list[0]
                    elif isinstance(data_list, dict):
                        track_info = data_list
                except Exception:
                    pass
                    
            if not track_info or (not track_info.get('url') and not track_info.get('proxyUrl')):
                raise RuntimeError('暂无可用下载地址，可能需要登录或属于付费/无版权歌曲')

        download_url = track_info.get('url') or track_info.get('proxyUrl')
        ext = (track_info.get('type') or track_info.get('encodeType') or 'mp3').lower()
        filename = f"{base_filename}.{ext}"
        target_path = os.path.join(target_dir, filename)

        # 解决文件名重名冲突
        counter = 1
        while os.path.exists(target_path):
            filename = f"{base_filename} ({counter}).{ext}"
            target_path = os.path.join(target_dir, filename)
            counter += 1

        tmp_path = target_path + ".part"
        update_download_task(task_id, status='downloading')
        
        # 3. 开始执行网络流式下载
        try:
            with requests.get(download_url, stream=True, timeout=20, headers=COMMON_HEADERS) as resp:
                resp.raise_for_status()
                total_size = int(resp.headers.get('content-length', 0))
                downloaded = 0
                
                with open(tmp_path, 'wb') as f:
                    for chunk in resp.iter_content(chunk_size=8192):
                        if chunk:
                            f.write(chunk)
                            downloaded += len(chunk)
                            if total_size > 0:
                                progress = int((downloaded / total_size) * 100)
                                update_download_task(task_id, progress=progress)
                                
            shutil.move(tmp_path, target_path)
        finally:
            if os.path.exists(tmp_path):
                try: 
                    os.remove(tmp_path)
                except Exception: 
                    pass
            
        # 4. 计算内容 MD5，获得歌曲的真正 song_id
        from core.utils.hasher import get_file_md5
        new_sid = get_file_md5(target_path)

        # 5. 内嵌与保存封面及歌词
        if cover_bytes:
            embed_cover_to_file(target_path, cover_bytes)
            save_cover_file(cover_bytes, new_sid)
            
        lrc_text, yrc_text = fetch_netease_lyrics(song_id)
        if lrc_text:
            try:
                lrc_path = os.path.join(app_config.LYRICS_DIR, f"{new_sid}.lrc")
                with open(lrc_path, 'w', encoding='utf-8') as f:
                    f.write(lrc_text)
            except Exception as e:
                logger.warning(f"保存本地歌词失败: {e}")
            embed_lyrics_to_file(target_path, lrc_text)
            
        if yrc_text:
            try:
                yrc_path = os.path.join(app_config.LYRICS_DIR, f"{new_sid}.yrc")
                with open(yrc_path, 'w', encoding='utf-8') as f:
                    f.write(yrc_text)
            except Exception as e:
                logger.warning(f"保存本地逐字歌词失败: {e}")
                
        # 6. 调用单文件索引并广播通知
        try:
            from core.services.scanner import index_single_file, notify_library_changed
            index_single_file(target_path)
            notify_library_changed()
        except ImportError:
            pass
        
        update_download_task(task_id, status='success', progress=100)
        logger.info(f"网易云歌曲已下载: {filename} | {title} - {artist}")
        
    except Exception as e:
        logger.warning(f"网易云下载失败: {e}")
        update_download_task(task_id, status='error', message=str(e))
    finally:
        pass
