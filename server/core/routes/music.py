import os
import time
import shutil
from urllib.parse import unquote, quote
import requests
from flask import Blueprint, request, jsonify, send_file
from core.config import app_config
from core.models.db import get_db, AUDIO_EXTS
from core.models.song import (
    get_all_songs_deduplicated,
    get_song_by_id,
    delete_song_by_path,
    get_song_by_path
)
from core.services.metadata import (    extract_embedded_lyrics,
    extract_embedded_cover
)
from core.services.scanner import index_single_file, notify_library_changed
from core.utils.logger import logger
from core.utils.common import COMMON_HEADERS
from core.utils.hasher import generate_song_id
import mod

music_bp = Blueprint('music', __name__)

# --- 以下为供 WebSocket 路由器调用的业务处理函数 ---

def handle_get_music_list() -> tuple:
    """获取所有去重后的音乐曲目列表"""
    try:
        songs = get_all_songs_deduplicated()
        return True, songs, None
    except Exception as e:
        logger.exception(f"获取音乐列表失败: {e}")
        return False, None, str(e)

def handle_delete_file(song_id: str) -> tuple:
    """物理删除歌曲以及关联的所有缓存文件"""
    if not song_id:
        return False, None, "缺少歌曲ID"
    try:
        target_path = None
        song = get_song_by_id(song_id)
        if song: 
            target_path = song['path']
        
        if not target_path or not os.path.exists(target_path):
            return False, None, "文件未找到"

        # 安全防线：仅允许删除指定音频格式后缀的文件，防止物理遍历误删系统文件
        _, ext = os.path.splitext(target_path)
        if ext.lower() not in AUDIO_EXTS:
             return False, None, f"安全性约束：禁止物理删除 {ext} 类型的文件"

        # 重载重试机制应对 Windows 文件锁定占用
        for i in range(10):
            try:
                os.remove(target_path)
                break
            except PermissionError:
                if i < 9: 
                    time.sleep(0.2)
                else: 
                    return False, None, "文件正被其他进程锁定，无法物理删除"
        
        # 清理同级关联附属文件 (封面/歌词/逐字歌词)
        base = os.path.splitext(target_path)[0]
        for ext in ['.lrc', '.yrc', '.jpg', '.webp']:
            try:
                if os.path.exists(base + ext): 
                    os.remove(base + ext)
            except Exception: 
                pass
            
        # 清理以 song_id 命名的集中化缓存 covers/lyrics 目录
        try:
             cv_path = os.path.join(app_config.COVERS_DIR, song_id + '.webp')
             if os.path.exists(cv_path): 
                 os.remove(cv_path)
        except Exception: 
            pass

        for lext in ['.lrc', '.yrc']:
            try:
                ly_path = os.path.join(app_config.LYRICS_DIR, song_id + lext)
                if os.path.exists(ly_path): 
                    os.remove(ly_path)
            except Exception: 
                pass
        
        # 从数据库中移除歌曲
        delete_song_by_path(target_path)
        notify_library_changed()
            
        return True, None, None
    except Exception as e: 
        logger.exception(f"物理删除文件失败: {e}")
        return False, None, str(e)

def handle_clear_metadata(song_id: str = None, path: str = None) -> tuple:
    """清除指定歌曲或指定路径文件的封面与歌词缓存"""
    try:
        target_path = None
        song = None
        if song_id:
            song = get_song_by_id(song_id)
        elif path:
            song = get_song_by_path(path)

        if song:
            target_path = song['path']
            song_id = song['id']
        elif path:
            target_path = path

        if not target_path:
            return False, None, "未找到对应文件路径"

        # 安全边界过滤：仅能处理挂载点内的文件
        target_path = os.path.abspath(target_path)
        allowed_roots = [os.path.abspath(app_config.MUSIC_LIBRARY_PATH)]
        try:
            with get_db() as conn:
                rows = conn.execute("SELECT path FROM mount_points").fetchall()
                allowed_roots.extend([os.path.abspath(r['path']) for r in rows])
        except Exception: 
            pass
        
        if not any(target_path.startswith(root) for root in allowed_roots):
            return False, None, "越界限制：仅能清理库内曲目的缓存数据"

        deleted_count = 0
        if song_id:
            # 删除封面缓存
            cv_path = os.path.join(app_config.COVERS_DIR, f"{song_id}.webp")
            try:
                if os.path.exists(cv_path):
                    os.remove(cv_path)
                    deleted_count += 1
            except Exception:
                pass
            
            # 删除歌词缓存
            for ext in ['.lrc', '.yrc']:
                ly_path = os.path.join(app_config.LYRICS_DIR, f"{song_id}{ext}")
                try:
                    if os.path.exists(ly_path):
                        os.remove(ly_path)
                        deleted_count += 1
                except Exception:
                    pass

            # 清除数据库标记
            with get_db() as conn:
                conn.execute("UPDATE songs SET has_cover=0, has_lyrics=0 WHERE id=?", (song_id,))
                conn.commit()
            
            logger.info(f"元数据已清除: ID={song_id}, 物理删除文件数: {deleted_count}")
            notify_library_changed()
            return True, None, None
        else:
            return False, None, "无法识别的歌曲ID"
    except Exception as e: 
        logger.exception(f"清理元数据失败: {e}")
        return False, None, str(e)

def handle_get_lyrics(title: str, artist: str, filename: str, song_id: str = None) -> tuple:
    """获取单曲歌词 (优先本地/内嵌，后聚合搜索)"""
    if not title:
        return False, None, "歌词请求缺少 title 参数"
        
    filename = unquote(filename) if filename else None
    
    # 1. 查找歌曲的实际本地物理路径和 song_id
    actual_path = None
    if not song_id and filename:
        if os.path.isabs(filename) and os.path.exists(filename):
            actual_path = filename
            try:
                song = get_song_by_path(filename)
                if song:
                    song_id = song['id']
            except Exception:
                pass
        else:
            try:
                song = get_song_by_path(filename)
                if song and os.path.exists(song['path']):
                    actual_path = song['path']
                    song_id = song['id']
                else:
                    # 尝试按文件名在库内粗略查找
                    with get_db() as conn:
                        row = conn.execute("SELECT id, path FROM songs WHERE filename=?", (os.path.basename(filename),)).fetchone()
                        if row and os.path.exists(row['path']):
                            actual_path = row['path']
                            song_id = row['id']
            except Exception as e:
                logger.warning(f"查询歌曲路径和ID失败: {e}")

    if song_id and not actual_path:
        song = get_song_by_id(song_id)
        if song:
            actual_path = song['path']

    if not song_id:
        if actual_path:
            sid = generate_song_id(actual_path)
        else:
            return False, None, "无法识别该曲目的ID或路径"
    else:
        sid = song_id

    # 2. 检查缓存是否有 lrc
    lrc_cache_path = os.path.join(app_config.LYRICS_DIR, f"{sid}.lrc")
    
    if os.path.exists(lrc_cache_path):
        try:
            with open(lrc_cache_path, 'r', encoding='utf-8') as f:
                logger.info(f"缓存歌词命中: {lrc_cache_path}")
                return True, {'lyrics': f.read()}, None
        except Exception as e:
            logger.warning(f"读取缓存歌词失败: {e}")

    # 3. 检查歌曲同级目录下是否有同名外部 .lrc 文件，有的话复制并返回
    if actual_path:
        local_dir = os.path.dirname(actual_path)
        base_name = os.path.splitext(os.path.basename(actual_path))[0]
        adj_lrc = os.path.join(local_dir, f"{base_name}.lrc")
        if os.path.exists(adj_lrc):
            try:
                shutil.copy(adj_lrc, lrc_cache_path)
                if song_id:
                    with get_db() as conn:
                        conn.execute("UPDATE songs SET has_lyrics=1 WHERE id=?", (sid,))
                        conn.commit()
                with open(lrc_cache_path, 'r', encoding='utf-8') as f:
                    logger.info(f"本地同级歌词复制并命中: {adj_lrc}")
                    return True, {'lyrics': f.read()}, None
            except Exception as e:
                logger.warning(f"读取同级歌词失败: {e}")

    # 4. 根据用户偏好决定内嵌歌词与网络刮削的优先级
    from core.models.preferences import get_preference
    lyrics_pref = get_preference('lyrics_source_preference', 'embedded')

    def try_embedded():
        """尝试从音频源文件中提取内嵌歌词"""
        if not actual_path:
            return None
        embedded_lrc = extract_embedded_lyrics(actual_path)
        if embedded_lrc:
            try:
                os.makedirs(app_config.LYRICS_DIR, exist_ok=True)
                with open(lrc_cache_path, 'w', encoding='utf-8') as f:
                    f.write(embedded_lrc)
                if song_id:
                    with get_db() as conn:
                        conn.execute("UPDATE songs SET has_lyrics=1 WHERE id=?", (sid,))
                        conn.commit()
                logger.info(f"内嵌歌词提取并保存: {lrc_cache_path}")
                return embedded_lrc
            except Exception as e:
                logger.warning(f"保存内嵌歌词失败: {e}")
        return None

    def try_network():
        """聚合网络检索 - LrcApi 搜索并写入缓存"""
        try:
            logger.info(f"网络刮削搜索歌词: title={title}, artist={artist}")
            result = mod.search_all(title=title, artist=artist, album='')
            best_lrc = result.get('lyrics') if result and result.get('lyrics') else None
            if best_lrc:
                try:
                    os.makedirs(app_config.LYRICS_DIR, exist_ok=True)
                    with open(lrc_cache_path, 'wb') as f:
                        f.write(best_lrc.encode('utf-8'))
                    if song_id:
                        with get_db() as conn:
                            conn.execute("UPDATE songs SET has_lyrics=1 WHERE id=?", (sid,))
                            conn.commit()
                        try:
                            notify_library_changed()
                        except Exception as notify_err:
                            logger.warning(f"广播歌词变更失败: {notify_err}")
                    logger.info(f"网络歌词下载保存成功: {lrc_cache_path}")
                    return best_lrc
                except Exception as e:
                    logger.warning(f"保存网络歌词失败: {e}")
            else:
                logger.warning(f"网络检索未找到匹配歌词: {title}")
        except Exception as e:
            logger.warning(f"网络刮削搜索歌词发生异常: {e}")
        return None

    if lyrics_pref == 'network':
        # 优先网络刮削，次选内嵌
        lrc = try_network()
        if lrc:
            return True, {'lyrics': lrc}, None
        lrc = try_embedded()
        if lrc:
            return True, {'lyrics': lrc}, None
    else:
        # 默认优先内嵌，次选网络刮削
        lrc = try_embedded()
        if lrc:
            return True, {'lyrics': lrc}, None
        lrc = try_network()
        if lrc:
            return True, {'lyrics': lrc}, None

    logger.warning(f"歌词获取失败: {title} - {artist}")
    return False, None, "未找到匹配的歌词"

def handle_get_album_art(title: str, artist: str, filename: str, song_id: str = None) -> tuple:
    """获取单曲封面图片链接"""
    if not title or not filename: 
        return False, None, "缺少必要参数"
        
    filename = unquote(filename)
    actual_path = None
    
    # 1. 查找歌曲的实际本地物理路径和 song_id
    if not song_id and filename:
        if os.path.isabs(filename) and os.path.exists(filename):
            actual_path = filename
            try:
                song = get_song_by_path(filename)
                if song:
                    song_id = song['id']
            except Exception:
                pass
        else:
            try:
                song = get_song_by_path(filename)
                if song and os.path.exists(song['path']):
                    actual_path = song['path']
                    song_id = song['id']
                else:
                    with get_db() as conn:
                        row = conn.execute("SELECT id, path FROM songs WHERE filename=?", (os.path.basename(filename),)).fetchone()
                        if row and os.path.exists(row['path']):
                            actual_path = row['path']
                            song_id = row['id']
            except Exception as e:
                logger.warning(f"查询歌曲路径和ID失败: {e}")

    if song_id and not actual_path:
        song = get_song_by_id(song_id)
        if song:
            actual_path = song['path']

    if not song_id:
        if actual_path:
            sid = generate_song_id(actual_path)
        else:
            return False, None, "无法识别该曲目的ID或路径"
    else:
        sid = song_id

    cover_cache_path = os.path.join(app_config.COVERS_DIR, f"{sid}.webp")
    
    # 2. 检查缓存是否存在，若存在直接返回
    if os.path.exists(cover_cache_path):
        return True, {'album_art': f"/api/music/covers/{sid}.webp"}, None

    # 3. 检查歌曲同级目录下是否有外部图片，有的话压缩转换并返回
    if actual_path:
        local_dir = os.path.dirname(actual_path)
        base_name = os.path.splitext(os.path.basename(actual_path))[0]
        
        cover_file_path = None
        for img_ext in ('.jpg', '.jpeg', '.png'):
            test_path = os.path.join(local_dir, f"{base_name}{img_ext}")
            if os.path.exists(test_path):
                cover_file_path = test_path
                break
                
        if cover_file_path:
            try:
                from core.services.metadata import save_cover_file
                with open(cover_file_path, 'rb') as rf:
                    save_cover_file(rf.read(), sid)
                if song_id:
                    with get_db() as conn:
                        conn.execute("UPDATE songs SET has_cover=1 WHERE id=?", (sid,))
                        conn.commit()
                return True, {'album_art': f"/api/music/covers/{sid}.webp"}, None
            except Exception as e:
                logger.warning(f"处理同级封面失败: {e}")

    # 4. 优先尝试从音频内嵌提取封面
    if actual_path and extract_embedded_cover(actual_path, sid):
        if song_id:
            try:
                with get_db() as conn:
                    conn.execute("UPDATE songs SET has_cover=1 WHERE id=?", (sid,))
                    conn.commit()
            except Exception:
                pass
        return True, {'album_art': f"/api/music/covers/{sid}.webp"}, None

    # 5. 网络刮削
    try:
        logger.info(f"本地调用 LrcApi 搜索封面: title={title}, artist={artist}")
        result = mod.search_all(title=title, artist=artist, album='')
        cover_url = result.get('cover') if result and result.get('cover') else None
        if cover_url:
            logger.info(f"LrcApi 找到封面 URL: {cover_url}")
            try:
                resp = requests.get(cover_url, timeout=10, headers=COMMON_HEADERS)
                if resp.status_code == 200 and resp.headers.get('content-type', '').startswith('image/'):
                    from core.services.metadata import save_cover_file
                    save_cover_file(resp.content, sid)
                    if song_id:
                        try:
                            with get_db() as conn:
                                conn.execute("UPDATE songs SET has_cover=1 WHERE id=?", (sid,))
                                conn.commit()
                            notify_library_changed()
                        except Exception as db_err:
                            logger.warning(f"保存网络封面并更新数据库状态失败: {db_err}")
                    return True, {'album_art': f"/api/music/covers/{sid}.webp"}, None
                else:
                    logger.warning(f"网络封面图片下载失败: HTTP {resp.status_code}")
            except Exception as dl_err:
                logger.warning(f"封面下载异常: {dl_err}")
        else:
            logger.warning("LrcApi 未找到该歌曲的网络封面")
    except Exception as e:
        logger.warning(f"LrcApi 搜索封面发生异常: {e}")
        
    return False, None, "未找到关联的封面"

# --- 以下为保留的 HTTP 静态/流/上传接口 ---

@music_bp.route('/api/music/play/<song_id>')
def play_music(song_id):
    """音频流式播放服务 (支持 Byte-Range 分片下载)"""
    try:
        song = get_song_by_id(song_id)
        if song:
            title = song.get('title') or '未知'
            artist = song.get('artist') or '未知'
            logger.info(f"API请求: 播放音乐 ID={song_id} ({title} - {artist})")
            path = song.get('path')
            if path and os.path.exists(path):
                return send_file(path, conditional=True)
        else:
            logger.info(f"API请求: 播放音乐 ID={song_id}")
            
    except Exception as e:
        logger.error(f"音频点播播放失败: {e}")

    logger.warning(f"文件未找到或ID无效: {song_id}")
    return jsonify({'error': 'Not Found'}), 404

@music_bp.route('/api/music/covers/<cover_name>')
def get_cover_image(cover_name):
    """读取并托管 covers/ 缓存目录下的封面图片资源"""
    cover_name = unquote(cover_name)
    base_name, _ = os.path.splitext(cover_name)
    path = os.path.join(app_config.COVERS_DIR, f"{base_name}.webp")
    if os.path.exists(path): 
        resp = send_file(path, mimetype='image/webp')
        resp.headers['Cache-Control'] = 'public, max-age=2592000'
        return resp
    return jsonify({'error': 'Not found'}), 404

@music_bp.route('/api/music/upload', methods=['POST'])
def upload_file():
    """上传本地歌曲文件到音乐库目录中"""
    if 'file' not in request.files: 
        return jsonify({'success': False, 'error': '未收到文件'})
    file = request.files['file']
    if file.filename == '': 
        return jsonify({'success': False, 'error': '文件名为空'})
        
    if file:
        filename = file.filename
        target_dir = request.form.get('target_dir') or app_config.MUSIC_LIBRARY_PATH
        target_dir = os.path.abspath(target_dir)
        
        # 安全验证：只能保存到允许的根目录
        allowed_roots = [os.path.abspath(app_config.MUSIC_LIBRARY_PATH)]
        try:
            with get_db() as conn:
                rows = conn.execute("SELECT path FROM mount_points").fetchall()
                allowed_roots.extend([os.path.abspath(r['path']) for r in rows])
        except Exception:
            pass
            
        if not any(target_dir.startswith(root) for root in allowed_roots):
            return jsonify({'success': False, 'error': '越界限制：指定的保存路径非法，请先添加挂载点'})
            
        os.makedirs(target_dir, exist_ok=True)
        save_path = os.path.join(target_dir, filename)

        # 数据去重校验 (全路径校验 + 全局大小/名称查重)
        try:
            with get_db() as conn:
                exists = conn.execute("SELECT 1 FROM songs WHERE path=?", (save_path,)).fetchone()
                if exists:
                    return jsonify({'success': False, 'error': '当前目标目录下已存在同名音频文件'})
                
                file.seek(0, os.SEEK_END)
                file_size = file.tell()
                file.seek(0)
                
                dup = conn.execute("SELECT path FROM songs WHERE filename=? AND size=?", (filename, file_size)).fetchone()
                if dup:
                    return jsonify({'success': False, 'error': f'音乐库中已存在相同文件，路径在: {dup["path"]}'})
        except Exception as e:
            logger.error(f"查重操作异常: {e}")

        try:
            file.save(save_path)
            # 文件保存后由 Watchdog 自动扫描入库
            return jsonify({'success': True})
        except Exception as e:
            return jsonify({'success': False, 'error': str(e)})
            
    return jsonify({'success': False, 'error': '未知错误'})

@music_bp.route('/api/music/external/play')
def play_external_file():
    """音频点播：播放已允许挂载路径下的音频文件"""
    path = request.args.get('path')
    if not path:
        return jsonify({'error': '缺少必要参数'}), 400
        
    path = os.path.abspath(unquote(path))
    
    # 安全验证：限制只能播放允许的音乐库或挂载点路径下的音频文件
    allowed_roots = [os.path.abspath(app_config.MUSIC_LIBRARY_PATH)]
    try:
        with get_db() as conn:
            rows = conn.execute("SELECT path FROM mount_points").fetchall()
            allowed_roots.extend([os.path.abspath(r['path']) for r in rows])
    except Exception:
        pass

    if not any(path.startswith(root) for root in allowed_roots):
        return jsonify({'error': '越权限制：禁止播放挂载目录外的文件'}), 403

    if os.path.exists(path): 
        return send_file(path, conditional=True)
    return jsonify({'error': '文件未找到'}), 404
