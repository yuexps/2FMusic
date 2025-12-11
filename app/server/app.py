#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import os
import sys
import time
import re
import sqlite3
import threading
import shutil
import logging
import argparse
import locale
import concurrent.futures
from urllib.parse import quote, unquote, urlparse, parse_qs
import hashlib

if getattr(sys, 'frozen', False):
    # 【打包模式】基准目录是二进制文件所在位置
    BASE_DIR = os.path.dirname(sys.executable)
else:
    # 【源码模式】基准目录是脚本所在位置
    BASE_DIR = os.path.dirname(os.path.abspath(__file__))
    # 仅在源码模式下加载 lib
    sys.path.insert(0, os.path.join(BASE_DIR, 'lib'))

try:
    from flask import Flask, render_template, request, jsonify, send_file, redirect
    import requests
    from mutagen import File
    from mutagen.easyid3 import EasyID3
    from mutagen.id3 import ID3, APIC, USLT
    from mutagen.flac import FLAC, Picture
    from mutagen.mp4 import MP4, MP4Cover
    from watchdog.observers import Observer
    from watchdog.events import FileSystemEventHandler
    from werkzeug.middleware.proxy_fix import ProxyFix
except ImportError as e:
    print(f"错误：无法导入依赖库。\n详情: {e}")
    sys.exit(1)

# 计算 www 的绝对路径
TEMPLATE_DIR = os.path.abspath(os.path.join(BASE_DIR, '../www/templates'))
STATIC_DIR = os.path.abspath(os.path.join(BASE_DIR, '../www/static'))

# --- 环境配置 ---
os.environ['PYTHONIOENCODING'] = 'utf-8'
if hasattr(sys.stdout, 'reconfigure'):
    sys.stdout.reconfigure(encoding='utf-8')
    sys.stderr.reconfigure(encoding='utf-8')

for encoding in ['UTF-8', 'utf-8', 'en_US.UTF-8', 'zh_CN.UTF-8']:
    try:
        locale.setlocale(locale.LC_ALL, f'en_US.{encoding}')
        break
    except:
        continue

# --- 参数解析 ---
parser = argparse.ArgumentParser(description='2FMusic Server')
parser.add_argument('--music-library-path', type=str, default=os.environ.get('MUSIC_LIBRARY_PATH'), help='Path to music library')
parser.add_argument('--log-path', type=str, default=os.environ.get('LOG_PATH'), help='Path to log file')
parser.add_argument('--port', type=int, default=int(os.environ.get('PORT', 23237)), help='Server port')
args = parser.parse_args()

# --- 路径初始化 ---
MUSIC_LIBRARY_PATH = args.music_library_path or os.getcwd()
os.makedirs(MUSIC_LIBRARY_PATH, exist_ok=True)
os.makedirs(os.path.join(MUSIC_LIBRARY_PATH, 'lyrics'), exist_ok=True)
os.makedirs(os.path.join(MUSIC_LIBRARY_PATH, 'covers'), exist_ok=True)

log_file = args.log_path or os.path.join(os.getcwd(), 'app.log')
os.makedirs(os.path.dirname(log_file), exist_ok=True)
DB_PATH = os.path.join(MUSIC_LIBRARY_PATH, 'data.db')

# --- 日志配置 ---
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)
logger.handlers.clear()
file_handler = logging.FileHandler(log_file, mode='w', encoding='utf-8')
file_handler.setFormatter(logging.Formatter('%(asctime)s - %(levelname)s - %(message)s'))
console_handler = logging.StreamHandler()
console_handler.setFormatter(logging.Formatter('%(levelname)s: %(message)s'))
logger.addHandler(file_handler)
logger.addHandler(console_handler)

# 过滤 Werkzeug 访问日志，隐藏心跳检测的 200 响应
class AccessLogFilter(logging.Filter):
    def filter(self, record):
        msg = record.getMessage()
        return not ('/api/system/status' in msg and '" 200 ' in msg)

logging.getLogger('werkzeug').addFilter(AccessLogFilter())

logger.info(f"Music Library Path: {MUSIC_LIBRARY_PATH}")

# --- 全局状态变量 ---
SCAN_STATUS = {
    'scanning': False,
    'total': 0,
    'processed': 0,
    'current_file': ''
}

# 库版本戳，用于前端检测变更
LIBRARY_VERSION = time.time()

# 辅助: 生成ID
def generate_song_id(path):
    return hashlib.md5(path.encode('utf-8')).hexdigest()

# --- 文件监听器 ---
class MusicFileEventHandler(FileSystemEventHandler):
    """监听音乐库文件变动"""
    def on_created(self, event):
        if event.is_directory: return
        self._process(event.src_path, 'created')

    def on_deleted(self, event):
        if event.is_directory: return
        self._process(event.src_path, 'deleted')

    def on_moved(self, event):
        if event.is_directory: return
        # 视为删除旧文件，添加新文件
        self._process(event.src_path, 'deleted')
        self._process(event.dest_path, 'created')

    def _process(self, path, action):
        global LIBRARY_VERSION
        filename = os.path.basename(path)
        ext = os.path.splitext(filename)[1].lower()
        
        is_audio = ext in AUDIO_EXTS
        is_misc = ext in ('.lrc', '.jpg', '.jpeg', '.png')
        
        if not is_audio and not is_misc:
            return

        logger.info(f"检测到文件变更 [{action}]: {filename}")
        
        try:
            if action == 'created':
                time.sleep(0.5)
                if is_audio:
                    index_single_file(path)
                elif is_misc:
                    # 如果是附件，尝试重新索引同名音频文件以更新状态
                    base = os.path.splitext(path)[0]
                    for aud in AUDIO_EXTS:
                        aud_path = base + aud
                        if os.path.exists(aud_path):
                            index_single_file(aud_path)
                            
            elif action == 'deleted':
                if is_audio:
                    with get_db() as conn:
                        conn.execute("DELETE FROM songs WHERE path=?", (path,))
                        conn.commit()
                elif is_misc:
                    # 附件删除，同样反向更新音频状态
                    base = os.path.splitext(path)[0]
                    for aud in AUDIO_EXTS:
                        aud_path = base + aud
                        if os.path.exists(aud_path):
                            index_single_file(aud_path)
            
            LIBRARY_VERSION = time.time()
            
        except Exception as e:
            logger.error(f"处理文件变更失败: {e}")

# 全局 Observer 实例
global_observer = None

def init_watchdog():
    global global_observer
    if not Observer: return
    
    if global_observer:
        global_observer.stop()
        global_observer.join()
        
    global_observer = Observer()
    refresh_watchdog_paths()
    global_observer.start()
    logger.info("文件监听服务已启动")
    try:
        while True:
            time.sleep(1)
    except:
        global_observer.stop()
    global_observer.join()

def refresh_watchdog_paths():
    """根据数据库刷新监听目录"""
    global global_observer
    if not global_observer: return
    
    # 1. 移除现有所有 schedule
    global_observer.unschedule_all()
    
    # 2. 获取目标路径
    targets = {MUSIC_LIBRARY_PATH}
    try:
        with get_db() as conn:
            rows = conn.execute("SELECT path FROM mount_points").fetchall()
            for r in rows: targets.add(r['path'])
    except: pass
    
    # 3. 重新添加 schedule
    event_handler = MusicFileEventHandler()
    for path in targets:
        if os.path.exists(path):
            try:
                global_observer.schedule(event_handler, path, recursive=True)
                logger.info(f"监听目录: {path}")
            except Exception as e:
                logger.warning(f"无法监听目录 {path}: {e}")

DOWNLOAD_TASKS = {} # task_id -> {status, progress, message, filename}

# 修复路径问题
app = Flask(__name__, static_folder=STATIC_DIR, template_folder=TEMPLATE_DIR)
app.wsgi_app = ProxyFix(app.wsgi_app, x_for=1, x_proto=1, x_host=1, x_prefix=1)
# 配置静态文件缓存过期时间为 1 年 (31536000 秒)
app.config['SEND_FILE_MAX_AGE_DEFAULT'] = 31536000

@app.after_request
def add_cors_headers(response):
    response.headers['Access-Control-Allow-Origin'] = '*'
    response.headers['Access-Control-Allow-Headers'] = 'Content-Type,Authorization'
    response.headers['Access-Control-Allow-Methods'] = 'GET,PUT,POST,DELETE,OPTIONS'
    return response

# --- 数据库管理 ---
def get_db():
    conn = sqlite3.connect(DB_PATH, timeout=30.0, check_same_thread=False)
    conn.row_factory = sqlite3.Row
    return conn

def init_db():
    try:
        with get_db() as conn:
            # 检查旧模式并迁移（直接重建更简单）
            # 如果 songs 表没有 'path' 列，视为旧表，删除重建
            try:
                cursor = conn.execute("SELECT path FROM songs LIMIT 1")
            except Exception:
                # 抛出异常说明列不存在，或者是旧表
                conn.execute("DROP TABLE IF EXISTS songs")
                # mount_files 也不再需要
                conn.execute("DROP TABLE IF EXISTS mount_files")

            conn.execute('''
                CREATE TABLE IF NOT EXISTS songs (
                    id TEXT PRIMARY KEY,
                    path TEXT UNIQUE,
                    filename TEXT,
                    title TEXT,
                    artist TEXT,
                    album TEXT,
                    mtime REAL,
                    size INTEGER,
                    has_cover INTEGER DEFAULT 0
                )
            ''')
            conn.execute('''
                CREATE TABLE IF NOT EXISTS mount_points (
                    path TEXT PRIMARY KEY,
                    created_at REAL
                )
            ''')
            
            conn.execute('''
                CREATE TABLE IF NOT EXISTS system_settings (
                    key TEXT PRIMARY KEY,
                    value TEXT
                )
            ''')
            

            
            # 清理错误索引的非音频文件
            try:
                placeholders = ' OR '.join([f"filename NOT LIKE '%{ext}'" for ext in AUDIO_EXTS])
                conn.execute(f"DELETE FROM songs WHERE {placeholders}")
            except: pass
            
            conn.commit()
        logger.info("数据库初始化完成。")
    except Exception as e:
        logger.exception(f"数据库初始化失败: {e}")

# --- 元数据提取 ---
def get_metadata(file_path):
    metadata = {'title': None, 'artist': None, 'album': None}
    try:
        audio = None
        try:
            audio = EasyID3(file_path)
        except Exception as e1:
            try:
                audio = File(file_path, easy=True)
            except Exception as e2:
                audio = File(file_path)
                logger.warning(f"文件 {file_path} 元数据解析异常: {e2}")
        if audio:
            def get_tag(key):
                if hasattr(audio, 'get'):
                    val = audio.get(key)
                    if isinstance(val, list): return val[0]
                    return val
                return None
            metadata['title'] = get_tag('title')
            metadata['artist'] = get_tag('artist')
            metadata['album'] = get_tag('album')
    except Exception as e:
        logger.error(f"提取元数据失败: {file_path}, 错误: {e}")
    filename = os.path.splitext(os.path.basename(file_path))[0]
    if not metadata['title']:
        if ' - ' in filename:
            parts = filename.split(' - ', 1)
            if not metadata['artist']: metadata['artist'] = parts[0].strip()
            metadata['title'] = parts[1].strip()
        else:
            metadata['title'] = filename
    if not metadata['artist']: metadata['artist'] = "未知艺术家"
    logger.debug(f"文件 {file_path} 元数据: {metadata}")
    return metadata

def extract_embedded_cover(file_path: str, base_name: str = None):
    """提取音频内嵌封面并保存为 covers/<base_name>.jpg，成功返回 True。"""
    try:
        if not os.path.exists(file_path):
            return False
        base_name = base_name or os.path.splitext(os.path.basename(file_path))[0]
        cover_dir = os.path.join(MUSIC_LIBRARY_PATH, 'covers')
        os.makedirs(cover_dir, exist_ok=True)
        target_path = os.path.join(cover_dir, f"{base_name}.jpg")
        if os.path.exists(target_path):
            return True

        audio = File(file_path)
        if not audio:
            return False

        data = None

        # MP3 / ID3
        if hasattr(audio, 'tags') and audio.tags:
            if hasattr(audio.tags, 'getall'):
                for tag in audio.tags.getall('APIC'):
                    if getattr(tag, 'data', None):
                        data = tag.data
                        break
            if not data:
                covr = audio.tags.get('covr')
                if covr:
                    val = covr[0] if isinstance(covr, (list, tuple)) else covr
                    try:
                        data = bytes(val)
                    except Exception:
                        pass

        # FLAC / 其他
        if not data and hasattr(audio, 'pictures'):
            pics = getattr(audio, 'pictures') or []
            if pics:
                data = pics[0].data

        if not data:
            return False

        with open(target_path, 'wb') as f:
            f.write(data)
        return True
    except Exception as e:
        logger.warning(f"提取内嵌封面失败: {file_path}, 错误: {e}")
        return False

def fetch_cover_bytes(url: str):
    if not url:
        return None
    try:
        resp = requests.get(url, timeout=8, headers=COMMON_HEADERS)
        if resp.status_code == 200 and resp.content:
            return resp.content
    except Exception as e:
        logger.warning(f"封面下载失败: {url}, 错误: {e}")
    return None

def embed_cover_to_file(audio_path: str, cover_bytes: bytes):
    """将封面嵌入音频文件（支持 mp3/flac/m4a）。"""
    if not cover_bytes or not os.path.exists(audio_path):
        return
    ext = os.path.splitext(audio_path)[1].lower()
    try:
        if ext == '.mp3':
            audio = None
            try:
                audio = ID3(audio_path)
            except Exception:
                audio = File(audio_path)
                audio.add_tags()
                audio.save()
                audio = ID3(audio_path)
            if audio:
                audio.delall('APIC')
                audio.add(APIC(mime='image/jpeg', type=3, desc='Cover', data=cover_bytes))
                audio.save()
        elif ext == '.flac':
            audio = FLAC(audio_path)
            pic = Picture()
            pic.data = cover_bytes
            pic.type = 3
            pic.mime = 'image/jpeg'
            audio.clear_pictures()
            audio.add_picture(pic)
            audio.save()
        elif ext in ('.m4a', '.m4b', '.m4p'):
            audio = MP4(audio_path)
            fmt = MP4Cover.FORMAT_JPEG
            if cover_bytes.startswith(b'\x89PNG'):
                fmt = MP4Cover.FORMAT_PNG
            audio['covr'] = [MP4Cover(cover_bytes, fmt)]
            audio.save()
    except Exception as e:
        logger.warning(f"内嵌封面失败: {audio_path}, 错误: {e}")

def save_cover_file(cover_bytes: bytes, base_name: str):
    if not cover_bytes or not base_name:
        return None
    try:
        cover_dir = os.path.join(MUSIC_LIBRARY_PATH, 'covers')
        os.makedirs(cover_dir, exist_ok=True)
        cover_path = os.path.join(cover_dir, f"{base_name}.jpg")
        with open(cover_path, 'wb') as f:
            f.write(cover_bytes)
        return cover_path
    except Exception as e:
        logger.warning(f"封面保存失败: {base_name}, 错误: {e}")
        return None

def fetch_netease_lyrics(song_id: str):
    """返回 (lrc, yrc) 字符串；若无则为 None。"""
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

def embed_lyrics_to_file(audio_path: str, lrc_text: str):
    """将歌词嵌入音频（行级歌词）。"""
    if not lrc_text or not os.path.exists(audio_path):
        return
    ext = os.path.splitext(audio_path)[1].lower()
    try:
        if ext == '.mp3':
            try:
                tags = ID3(audio_path)
            except Exception:
                tags = File(audio_path)
                tags.add_tags()
                tags.save()
                tags = ID3(audio_path)
            tags.delall('USLT')
            tags.add(USLT(encoding=3, lang='chi', desc='Lyric', text=lrc_text))
            tags.save()
        elif ext == '.flac':
            audio = FLAC(audio_path)
            audio['LYRICS'] = lrc_text
            audio.save()
        elif ext in ('.m4a', '.m4b', '.m4p'):
            audio = MP4(audio_path)
            audio['\xa9lyr'] = lrc_text
            audio.save()
        elif ext in ('.ogg', '.oga'):
            audio = File(audio_path)
            audio['LYRICS'] = lrc_text
            audio.save()
    except Exception as e:
        logger.warning(f"内嵌歌词失败: {audio_path}, 错误: {e}")

AUDIO_EXTS = ('.mp3', '.wav', '.ogg', '.flac', '.aac', '.m4a')

def index_single_file(file_path):
    """单独索引一个文件。"""
    try:
        if not os.path.exists(file_path): return
        # 严格限制只能索引音频文件
        ext = os.path.splitext(file_path)[1].lower()
        if ext not in AUDIO_EXTS: return
        
        stat = os.stat(file_path)
        meta = get_metadata(file_path)
        sid = generate_song_id(file_path)
        base_name = os.path.splitext(os.path.basename(file_path))[0]
        base_path = os.path.splitext(file_path)[0]
        cover_path = os.path.join(MUSIC_LIBRARY_PATH, 'covers', f"{base_name}.jpg")
        has_cover = 0
        if os.path.exists(base_path + ".jpg") or os.path.exists(cover_path):
            has_cover = 1
        else:
            # 尝试提取内嵌封面
            if extract_embedded_cover(file_path, base_name):
                has_cover = 1
        
        with get_db() as conn:
            conn.execute('''
                INSERT OR REPLACE INTO songs (id, path, filename, title, artist, album, mtime, size, has_cover)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            ''', (sid, file_path, os.path.basename(file_path), meta['title'], meta['artist'], meta['album'], stat.st_mtime, stat.st_size, has_cover))
            conn.commit()
        logger.info(f"单文件索引完成: {file_path}")
    except Exception as e:
        logger.error(f"单文件索引失败: {e}")

# --- 优化后的并发扫描逻辑 ---
def scan_library_incremental():
    global SCAN_STATUS
    
    lock_file = os.path.join(MUSIC_LIBRARY_PATH, '.scan_lock')
    if os.path.exists(lock_file):
        if time.time() - os.path.getmtime(lock_file) > 300:
            try:
                os.remove(lock_file)
                logger.info("过期扫描锁文件已移除。")
            except Exception as e:
                logger.warning(f"移除扫描锁文件失败: {e}")
        else:
            return 

    try:
        # 更新状态：开始
        SCAN_STATUS.update({'scanning': True, 'total': 0, 'processed': 0, 'current_file': '正在遍历文件...'})
        
        with open(lock_file, 'w') as f: f.write(str(time.time()))
        logger.info("开始增量扫描...")
        
        # 1. 获取所有扫描根目录
        scan_roots = [MUSIC_LIBRARY_PATH]
        try:
            with get_db() as conn:
                rows = conn.execute("SELECT path FROM mount_points").fetchall()
                scan_roots.extend([r['path'] for r in rows])
        except Exception: pass
        
        disk_files = {} # path -> info
        supported_exts = AUDIO_EXTS
        
        # 2. 遍历所有目录
        for root_dir in scan_roots:
            if not os.path.exists(root_dir): continue
            for root, dirs, files in os.walk(root_dir):
                # 排除自动生成的目录
                dirs[:] = [d for d in dirs if d not in ('lyrics', 'covers')]
                for f in files:
                    if f.lower().endswith(supported_exts):
                        path = os.path.join(root, f)
                        try:
                            stat = os.stat(path)
                            info = {'mtime': stat.st_mtime, 'size': stat.st_size, 'path': path, 'filename': f}
                            disk_files[path] = info
                        except: pass

        with get_db() as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT id, path, mtime, size FROM songs")
            db_rows = {row['path']: row for row in cursor.fetchall()}
            
            # 删除不存在的文件
            # 注意：如果某个挂载点被临时拔出，这里会删除其歌曲。
            # 为了稳健，如果根目录都不存在，应该跳过删除该目录下的歌曲？
            # 暂时简化处理：只删除那些“在其父目录仍然存在但文件消失”的情况？
            # 简单起见：全量比对，消失即删除。
            to_delete_paths = set(db_rows.keys()) - set(disk_files.keys())
            if to_delete_paths:
                cursor.executemany("DELETE FROM songs WHERE path=?", [(p,) for p in to_delete_paths])
                conn.commit()

            # 筛选需要更新的文件
            files_to_process_list = []
            for path, info in disk_files.items():
                db_rec = db_rows.get(path)
                if not db_rec or db_rec['mtime'] != info['mtime'] or db_rec['size'] != info['size']:
                    files_to_process_list.append(info)

            # 更新状态
            total_files = len(files_to_process_list)
            SCAN_STATUS.update({'total': total_files, 'processed': 0})
            
            to_update_db = []
            
            # 3. 多线程处理
            if total_files > 0:
                logger.info(f"使用线程池处理 {total_files} 个文件...")
                
                def process_file_metadata(info):
                    meta = get_metadata(info['path'])
                    sid = generate_song_id(info['path'])
                    # 封面逻辑
                    base_path = os.path.splitext(info['path'])[0]
                    has_cover = 1 if os.path.exists(base_path + ".jpg") or os.path.exists(os.path.join(MUSIC_LIBRARY_PATH, 'covers', f"{os.path.basename(base_path)}.jpg")) else 0
                    return (sid, info['path'], info['filename'], meta['title'], meta['artist'], meta['album'], info['mtime'], info['size'], has_cover)

                with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
                    futures = {executor.submit(process_file_metadata, item): item for item in files_to_process_list}
                    for future in concurrent.futures.as_completed(futures):
                        try:
                            res = future.result()
                            to_update_db.append(res)
                        except Exception: pass
                        
                        SCAN_STATUS['processed'] += 1
                        if SCAN_STATUS['processed'] % 10 == 0:
                            SCAN_STATUS['current_file'] = f"处理中... {int((SCAN_STATUS['processed']/total_files)*100)}%"

                if to_update_db:
                    cursor.executemany('''
                        INSERT OR REPLACE INTO songs (id, path, filename, title, artist, album, mtime, size, has_cover)
                        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                    ''', to_update_db)
                    conn.commit()

        logger.info("扫描完成。")
        global LIBRARY_VERSION; LIBRARY_VERSION = time.time()
        
    except Exception as e:
        logger.error(f"扫描失败: {e}")
    finally:
        SCAN_STATUS['scanning'] = False
        SCAN_STATUS['current_file'] = ''
        if os.path.exists(lock_file): 
            try: os.remove(lock_file)
            except: pass

threading.Thread(target=lambda: (init_db(), scan_library_incremental()), daemon=True).start()
threading.Thread(target=init_watchdog, daemon=True).start()

# --- 路由定义 ---
@app.route('/')
@app.route('/import_mode')
@app.route('/preview')
def index(): return render_template('index.html')

# --- 系统状态接口 ---
@app.route('/api/system/status')
def get_system_status():
    """返回当前扫描状态和进度"""
    status = dict(SCAN_STATUS)
    status['library_version'] = LIBRARY_VERSION
    return jsonify(status)

@app.route('/api/music', methods=['GET'])
def get_music_list():
    logger.info("API请求: 获取音乐列表")
    try:
        with get_db() as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM songs ORDER BY title")
            songs = []
            seen = set()
            
            for row in cursor.fetchall():
                # 去重逻辑：如果 标题+歌手+大小 完全一致，视为重复文件，仅保留第一个
                # 这样可以解决不同目录下存放相同文件导致的列表重复问题
                unique_key = (row['title'], row['artist'], row['size'])
                if unique_key in seen:
                    continue
                seen.add(unique_key)
                
                album_art = None
                if row['has_cover']:
                    base_name = os.path.splitext(row['filename'])[0]
                    # 封面图链接带上 filename 参数仅作缓存区分，实际通过 scan 查找
                    album_art = f"/api/music/covers/{quote(base_name)}.jpg?filename={quote(row['filename'])}"
                songs.append({
                    'id': row['id'], # 新增 ID
                    'filename': row['filename'], 'title': row['title'],
                    'artist': row['artist'], 'album': row['album'], 'album_art': album_art
                })
        logger.info(f"返回音乐数量: {len(songs)}")
        return jsonify({'success': True, 'data': songs})
    except Exception as e:
        logger.exception(f"获取音乐列表失败: {e}")
        return jsonify({'success': False, 'error': str(e)})

@app.route('/api/music/play/<song_id>')
def play_music(song_id):
    logger.info(f"API请求: 播放音乐 ID={song_id}")
    try:
        with get_db() as conn:
            row = conn.execute("SELECT path FROM songs WHERE id=?", (song_id,)).fetchone()
            if row and os.path.exists(row['path']):
                return send_file(row['path'], conditional=True)
            
    except Exception as e:
        logger.error(f"播放失败: {e}")

    logger.warning(f"文件未找到或ID无效: {song_id}")
    return jsonify({'error': 'Not Found'}), 404

# --- 挂载相关 ---
@app.route('/api/mount_points', methods=['GET'])
def list_mount_points():
    try:
        with get_db() as conn:
            rows = conn.execute("SELECT path FROM mount_points ORDER BY created_at DESC").fetchall()
            return jsonify({'success': True, 'data': [row['path'] for row in rows]})
    except Exception as e: return jsonify({'success': False, 'error': str(e)})

@app.route('/api/mount_points', methods=['POST'])
def add_mount_point():
    logger.info("API请求: 添加挂载点")
    try:
        path = request.json.get('path')
        if not path or not os.path.exists(path):
            return jsonify({'success': False, 'error': '路径不存在'})
            
        path = os.path.abspath(path)
        
        with get_db() as conn:
            if conn.execute("SELECT 1 FROM mount_points WHERE path=?", (path,)).fetchone():
                return jsonify({'success': False, 'error': '已挂载'})
            conn.execute("INSERT INTO mount_points (path, created_at) VALUES (?, ?)", (path, time.time()))
            conn.commit()

        # 刷新监听并触发扫描
        refresh_watchdog_paths()
        threading.Thread(target=scan_library_incremental, daemon=True).start()
        
        return jsonify({'success': True, 'message': '挂载点已添加，正在后台处理...'})
    except Exception as e:
        logger.exception(f"添加挂载点失败: {e}")
        return jsonify({'success': False, 'error': str(e)})

@app.route('/api/mount_points', methods=['DELETE'])
def remove_mount_point():
    try:
        path = request.json.get('path')
        with get_db() as conn:
            # 清理该路径下的歌曲
            conn.execute("DELETE FROM songs WHERE path LIKE ? || '%'", (path,))
            conn.execute("DELETE FROM mount_points WHERE path=?", (path,))
            conn.commit()
            
        refresh_watchdog_paths()
        
        # 触发一次库版本更新
        global LIBRARY_VERSION; LIBRARY_VERSION = time.time()
            
        return jsonify({'success': True, 'message': '已移除'})
    except Exception as e: return jsonify({'success': False, 'error': str(e)})

# --- 资源获取 ---
COMMON_HEADERS = {
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36',
    'Authorization': '2FMusic'
}
NETEASE_API_BASE_DEFAULT = os.environ.get('NETEASE_API_BASE', 'http://localhost:23236')
NETEASE_API_BASE = NETEASE_API_BASE_DEFAULT
NETEASE_DOWNLOAD_DIR = os.environ.get('NETEASE_DOWNLOAD_PATH', MUSIC_LIBRARY_PATH)
NETEASE_COOKIE = None
NETEASE_MAX_CONCURRENT = 20
LYRICS_DIR = os.path.join(MUSIC_LIBRARY_PATH, 'lyrics')

os.makedirs(LYRICS_DIR, exist_ok=True)

def parse_cookie_string(cookie_str: str):
    """将 Set-Cookie 字符串解析为 requests 兼容的字典。"""
    if not cookie_str: 
        return {}
    cookies = {}
    # 只取 key=value 形式，忽略 Path/Expires 等属性
    for part in cookie_str.split(';'):
        if '=' in part:
            k, v = part.strip().split('=', 1)
            if k.lower() in ('path', 'expires', 'max-age', 'domain', 'samesite', 'secure'): 
                continue
            cookies[k] = v
    return cookies

def normalize_cookie_string(raw: str) -> str:
    """规范化 cookie 字符串，移除换行并过滤非关键属性。"""
    if not raw: 
        return ''
    parts = []
    # 常见的 Set-Cookie 属性，不应出现在请求头 Cookie 中
    skip_keys = ('path', 'expires', 'max-age', 'domain', 'samesite', 'secure', 'httponly')
    
    for part in raw.replace('\n', ';').split(';'):
        part = part.strip()
        if not part: continue
        
        # 忽略没有等号的属性 (如 Secure, HttpOnly)
        if '=' not in part: 
            # 但有些 cookie 值可能就是没有等号？不太可能，标准cookie都是 k=v
            # 如果是 Secure/HttpOnly 这种flag，肯定要忽略
            continue
            
        k, v = part.split('=', 1)
        if k.strip().lower() in skip_keys:
            continue
            
        parts.append(part)
        
    return '; '.join(parts)

def load_netease_cookie():
    global NETEASE_COOKIE
    try:
        with get_db() as conn:
            row = conn.execute("SELECT value FROM system_settings WHERE key='netease_cookie'").fetchone()
            if row and row['value']:
                NETEASE_COOKIE = normalize_cookie_string(row['value'])
    except Exception as e:
        logger.warning(f"读取网易云 cookie 失败: {e}")

def save_netease_cookie(cookie_str: str):
    global NETEASE_COOKIE
    NETEASE_COOKIE = normalize_cookie_string(cookie_str or '')
    try:
        with get_db() as conn:
            conn.execute("INSERT OR REPLACE INTO system_settings (key, value) VALUES (?, ?)", ('netease_cookie', NETEASE_COOKIE))
            conn.commit()
    except Exception as e:
        logger.warning(f"保存网易云 cookie 失败: {e}")

def load_netease_config():
    global NETEASE_DOWNLOAD_DIR, NETEASE_API_BASE
    try:
        with get_db() as conn:
            # Download Dir
            row = conn.execute("SELECT value FROM system_settings WHERE key='netease_download_dir'").fetchone()
            if row and row['value']: NETEASE_DOWNLOAD_DIR = row['value']
            
            # API Base
            row = conn.execute("SELECT value FROM system_settings WHERE key='netease_api_base'").fetchone()
            if row and row['value']: NETEASE_API_BASE = row['value']
            
    except Exception as e:
        logger.warning(f"读取网易云配置失败: {e}")

def save_netease_config(download_dir: str = None, api_base: str = None):
    global NETEASE_DOWNLOAD_DIR, NETEASE_API_BASE
    if download_dir: NETEASE_DOWNLOAD_DIR = download_dir
    if api_base: NETEASE_API_BASE = api_base.rstrip('/') or NETEASE_API_BASE_DEFAULT
    
    try:
        with get_db() as conn:
            if download_dir:
                conn.execute("INSERT OR REPLACE INTO system_settings (key, value) VALUES (?, ?)", ('netease_download_dir', NETEASE_DOWNLOAD_DIR))
            if api_base:
                conn.execute("INSERT OR REPLACE INTO system_settings (key, value) VALUES (?, ?)", ('netease_api_base', NETEASE_API_BASE))
            conn.commit()
    except Exception as e:
        logger.warning(f"保存网易云配置失败: {e}")

def sanitize_filename(name: str) -> str:
    """移除非法字符，避免文件名错误。"""
    cleaned = re.sub(r'[\\/:*?"<>|]+', '_', name).strip().strip('.')
    return cleaned or 'netease_song'

def call_netease_api(path: str, params: dict, method: str = 'GET', need_cookie: bool = True):
    """调用本地网易云 API，统一处理错误。"""
    base = (NETEASE_API_BASE or NETEASE_API_BASE_DEFAULT).rstrip('/')
    url = f"{base}{path}"
    headers = dict(COMMON_HEADERS)
    params = dict(params or {})
    cookies = {}
    if need_cookie and NETEASE_COOKIE:
        # 直接透传原始 cookie 字符串，保证完整性
        headers['Cookie'] = NETEASE_COOKIE
        # 部分接口（如 login/status）需要 cookie 字符串参数
        params.setdefault('cookie', NETEASE_COOKIE)
        cookies = parse_cookie_string(NETEASE_COOKIE)
    if method.upper() == 'POST':
        resp = requests.post(url, data=params, timeout=10, headers=headers, cookies=cookies)
    else:
        resp = requests.get(url, params=params, timeout=10, headers=headers, cookies=cookies)
    resp.raise_for_status()
    return resp.json()

def _extract_song_level(privilege: dict):
    """返回(用户可下载的最高音质, 曲目最高音质)。"""
    privilege = privilege or {}
    def _norm(val):
        if not val:
            return 'standard'
        v = str(val).lower()
        return 'standard' if v == 'none' else val
    max_level = _norm(privilege.get('maxBrLevel') or privilege.get('maxbr') or privilege.get('maxLevel'))
    user_level = _norm(privilege.get('dlLevel') or privilege.get('plLevel') or max_level)
    return (user_level or 'standard', max_level or user_level or 'standard')

def _extract_song_size(track: dict, preferred: str = None):
    """根据期望音质优先取对应大小（字节），找不到再按从低到高回退。"""
    if not track:
        return None
    level = (preferred or '').lower()
    # 映射期望音质到字段优先级（标准优先用 l）
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

def _format_netease_songs(source_tracks):
    """将网易云接口返回的曲目统一格式化。"""
    songs = []
    for item in source_tracks or []:
        sid = item.get('id')
        if not sid:
            continue
        fee = item.get('fee')
        privilege = item.get('privilege') or {}
        privilege_fee = privilege.get('fee')
        # 仅在明确 fee==1（VIP 曲目）时标记 VIP，避免 fee=8 的“会员高音质”误标
        is_vip = (fee == 1) or (privilege_fee == 1)
        user_level, max_level = _extract_song_level(privilege)
        artists = ' / '.join([a.get('name') for a in item.get('ar', []) if a.get('name')]) or '未知艺术家'
        album_info = item.get('al') or {}
        size_bytes = _extract_song_size(item, user_level)
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

def _resolve_netease_input(raw: str, prefer: str = None):
    """支持短链/长链/纯数字的资源解析，返回 {'type': 'song'|'playlist', 'id': '123'}。"""
    if not raw:
        return None
    prefer = prefer if prefer in ('song', 'playlist') else None
    text = str(raw).strip()

    # 处理纯数字直接返回
    if text.isdigit():
        return {'type': prefer or 'song', 'id': text}

    candidate = text
    # 链接补全 scheme
    if candidate.startswith(('music.163.com', 'y.music.163.com', '163cn.tv')):
        candidate = f"https://{candidate}"
    # 跟随短链跳转获取真实地址，兼容 163cn.tv
    if re.match(r'^https?://', candidate, re.I):
        def _follow(url):
            try:
                resp = requests.get(url, allow_redirects=True, timeout=8, headers=COMMON_HEADERS)
                return resp.url or url
            except Exception as e:
                logger.warning(f"网易云链接解析失败: {e}")
                return None

        followed = _follow(candidate)
        # 针对 163cn.tv 短链再尝试一次 HEAD，避免部分环境 GET 被拦截
        if not followed and '163cn.tv' in candidate:
            try:
                resp = requests.head(candidate, allow_redirects=True, timeout=6, headers=COMMON_HEADERS)
                followed = resp.url or resp.headers.get('Location')
            except Exception as e:
                logger.warning(f"网易云短链 HEAD 解析失败: {e}")
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
                route_hint = 'playlist'; break
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

    # 回退：直接在文本中寻找
    m = re.search(r'(playlist|song)[^0-9]*(\d+)', text, re.IGNORECASE)
    if m:
        return {'type': m.group(1).lower(), 'id': m.group(2)}
    m = re.search(r'(\d{5,})', text)
    if m:
        return {'type': prefer or 'song', 'id': m.group(1)}
    return None

def _fetch_playlist_songs(playlist_id: str):
    detail_resp = call_netease_api('/playlist/detail', {'id': playlist_id})
    playlist = detail_resp.get('playlist') if isinstance(detail_resp, dict) else None
    if not playlist:
        raise Exception('无法获取歌单信息')
    track_ids = [t.get('id') for t in playlist.get('trackIds', []) if t.get('id')]
    tracks = playlist.get('tracks') or []
    if not tracks and track_ids:
        ids_str = ','.join(map(str, track_ids[:300]))  # protect from huge lists
        song_detail = call_netease_api('/song/detail', {'ids': ids_str})
        tracks = song_detail.get('songs', []) if isinstance(song_detail, dict) else []
    songs = _format_netease_songs(tracks)
    return songs, playlist.get('name')

def _fetch_song_detail(song_id: str):
    detail_resp = call_netease_api('/song/detail', {'ids': song_id})
    songs = detail_resp.get('songs', []) if isinstance(detail_resp, dict) else []
    parsed = _format_netease_songs(songs)
    if not parsed:
        raise Exception('未获取到歌曲信息')
    return parsed

# 预加载网易云 cookie
load_netease_config()
load_netease_cookie()

@app.route('/api/music/lyrics')
def get_lyrics_api():
    title = request.args.get('title')
    logger.info("API请求: 获取歌词")
    title = request.args.get('title')
    artist = request.args.get('artist')
    filename = request.args.get('filename')
    if not title:
        logger.warning("歌词请求缺少title参数")
        return jsonify({'success': False})
    filename = unquote(filename) if filename else None
    # 1. 优先读取本地
    if filename:
        base_name = os.path.splitext(os.path.basename(filename))[0]
        lrc_path = os.path.join(MUSIC_LIBRARY_PATH, 'lyrics', f"{base_name}.lrc")
        if os.path.exists(lrc_path):
            try:
                with open(lrc_path, 'r', encoding='utf-8') as f:
                    logger.info(f"本地歌词命中: {lrc_path}")
                    return jsonify({'success': True, 'lyrics': f.read()})
            except Exception as e:
                logger.warning(f"读取本地歌词失败: {lrc_path}, 错误: {e}")
    # 2. 网络获取
    api_urls = [
        f"https://lrcapi.msfxp.top/lyrics?artist={quote(artist or '')}&title={quote(title)}",
        f"https://api.lrc.cx/lyrics?artist={quote(artist or '')}&title={quote(title)}"
    ]
    for api_url in api_urls:
        try:
            logger.info(f"请求歌词API: {api_url}")
            resp = requests.get(api_url, timeout=3, headers=COMMON_HEADERS)
            if resp.status_code == 200:
                with open(lrc_path, 'wb') as f:
                    f.write(resp.text.encode('utf-8'))
                logger.info(f"网络歌词保存: {lrc_path}")
                return jsonify({'success': True, 'lyrics': resp.text})
            else:
                logger.warning(f"歌词API响应异常: {api_url}, 状态码: {resp.status_code}")
        except Exception as e:
            logger.warning(f"歌词API请求失败: {api_url}, 错误: {e}")
    logger.warning(f"歌词获取失败: {title} - {artist}")
    return jsonify({'success': False})

@app.route('/api/music/album-art')
def get_album_art_api():
    title = request.args.get('title')
    artist = request.args.get('artist') or ''
    filename = request.args.get('filename')
    
    if not title or not filename: return jsonify({'success': False})
    filename = unquote(filename)
    base_name = os.path.splitext(os.path.basename(filename))[0]
    
    local_path = os.path.join(MUSIC_LIBRARY_PATH, 'covers', f"{base_name}.jpg")
    if os.path.exists(local_path):
        return jsonify({'success': True, 'album_art': f"/api/music/covers/{quote(base_name)}.jpg?filename={quote(base_name)}"})

    # 优先尝试从音频内嵌封面提取
    actual_path = None
    if os.path.isabs(filename) and os.path.exists(filename):
        actual_path = filename
    else:
        try:
            with get_db() as conn:
                row = conn.execute("SELECT path FROM songs WHERE filename=?", (os.path.basename(filename),)).fetchone()
                if row and os.path.exists(row['path']):
                    actual_path = row['path']
        except Exception as e:
            logger.warning(f"查询歌曲路径失败: {e}")

    if actual_path and extract_embedded_cover(actual_path, base_name):
        try:
            if not os.path.isabs(filename):
                with get_db() as conn:
                    conn.execute("UPDATE songs SET has_cover=1 WHERE filename=?", (os.path.basename(filename),))
                    conn.commit()
        except Exception:
            pass
        return jsonify({'success': True, 'album_art': f"/api/music/covers/{quote(base_name)}.jpg?filename={quote(base_name)}"})

    # 网络获取并保存
    api_urls = [
        f"https://api.lrc.cx/cover?artist={quote(artist)}&title={quote(title)}",
        f"https://lrcapi.msfxp.top/cover?artist={quote(artist)}&title={quote(title)}"
    ]
    
    for api_url in api_urls:
        try:
            resp = requests.get(api_url, timeout=3, headers=COMMON_HEADERS)
            if resp.status_code == 200 and resp.headers.get('content-type', '').startswith('image/'):
                with open(local_path, 'wb') as f: 
                    f.write(resp.content)
                
                # 更新数据库标识
                if not os.path.isabs(filename):
                    with get_db() as conn: 
                        conn.execute("UPDATE songs SET has_cover=1 WHERE filename=?", (filename,))
                        conn.commit()
                        
                return jsonify({'success': True, 'album_art': f"/api/music/covers/{quote(base_name)}.jpg?filename={quote(base_name)}"})
        except: pass
        
    return jsonify({'success': False})

@app.route('/api/music/delete/<song_id>', methods=['DELETE'])
def delete_file(song_id):
    try:
        # 1. 查询路径
        target_path = None
        with get_db() as conn:
            row = conn.execute("SELECT path FROM songs WHERE id=?", (song_id,)).fetchone()
            if row: target_path = row['path']
        
        if not target_path or not os.path.exists(target_path):
            return jsonify({'success': False, 'error': '文件未找到'})

        # 2. 执行删除
        # 重试机制应对 Windows 文件锁
        for i in range(10):
            try:
                os.remove(target_path)
                break
            except PermissionError:
                if i < 9: time.sleep(0.2)
                else: return jsonify({'success': False, 'error': '文件正被占用，无法删除'})
        
        # 3. 清理关联资源 (封面/歌词)
        # 注意：多目录下，关联资源可能在同级目录
        base = os.path.splitext(target_path)[0]
        for ext in ['.lrc', '.jpg']:
            try:
                if os.path.exists(base + ext): os.remove(base + ext)
            except: pass
            
        # 尝试清理主库下的 covers/lyrics (如果是主库文件)
        filename = os.path.basename(target_path)
        base_name = os.path.splitext(filename)[0]
        for sub in ['lyrics', 'covers']:
            ext = '.lrc' if sub == 'lyrics' else '.jpg'
            sub_path = os.path.join(MUSIC_LIBRARY_PATH, sub, base_name + ext)
            try: 
                if os.path.exists(sub_path): os.remove(sub_path)
            except: pass
        
        # 4. 数据库清理 (Watchdog 也会做，但双重保障)
        with get_db() as conn:
            conn.execute("DELETE FROM songs WHERE path=?", (target_path,))
            conn.commit()
            
        return jsonify({'success': True})
    except Exception as e: 
        return jsonify({'success': False, 'error': str(e)})

# --- 辅助接口 ---
@app.route('/api/music/covers/<cover_name>')
def get_cover(cover_name):
    cover_name = unquote(cover_name)
    path = os.path.join(MUSIC_LIBRARY_PATH, 'covers', cover_name)
    if os.path.exists(path): return send_file(path, mimetype='image/jpeg')
    return jsonify({'error': 'Not found'}), 404

@app.route('/api/music/upload', methods=['POST'])
def upload_file():
    if 'file' not in request.files: return jsonify({'success': False, 'error': '未收到文件'})
    file = request.files['file']
    if file.filename == '': return jsonify({'success': False, 'error': '文件名为空'})
    if file:
        filename = file.filename
        save_path = os.path.join(MUSIC_LIBRARY_PATH, filename)
        try:
            file.save(save_path)
            # 使用单文件索引，不再全量扫描
            threading.Thread(target=index_single_file, args=(save_path,), daemon=True).start()
            return jsonify({'success': True})
        except Exception as e: return jsonify({'success': False, 'error': str(e)})
    return jsonify({'success': False, 'error': '未知错误'})

@app.route('/api/music/import_path', methods=['POST'])
def import_music_by_path():
    try:
        data = request.json
        src_path = data.get('path')
        if not src_path or not os.path.exists(src_path): return jsonify({'success': False, 'error': '无效路径'})
        filename = os.path.basename(src_path)
        dst_path = os.path.join(MUSIC_LIBRARY_PATH, filename)
        if not os.path.exists(dst_path):
            shutil.copy2(src_path, dst_path)
            threading.Thread(target=index_single_file, args=(dst_path,), daemon=True).start()
        return jsonify({'success': True, 'filename': filename})
    except Exception as e: return jsonify({'success': False, 'error': str(e)})

@app.route('/api/netease/search')
def search_netease_music():
    """通过本地网易云 API 搜索歌曲。"""
    keywords = (request.args.get('keywords') or '').strip()
    if not keywords:
        return jsonify({'success': False, 'error': '请输入搜索关键词'})
    limit = request.args.get('limit', 20)
    try:
        limit = max(1, min(int(limit), 50))
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
            # 仅 fee==1 视为 VIP；fee=8 只代表会员可享高音质，不强制标 VIP
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
                'size': _extract_song_size(item, user_level),
                'is_vip': is_vip
            })
        return jsonify({'success': True, 'data': songs})
    except Exception as e:
        logger.warning(f"网易云搜索失败: {e}")
        return jsonify({'success': False, 'error': '搜索失败，请检查网易云 API 服务'})

@app.route('/api/netease/recommend')
def netease_daily_recommend():
    """获取每日推荐歌曲，需要已登录网易云账号。"""
    try:
        api_resp = call_netease_api('/recommend/songs', {'timestamp': int(time.time() * 1000)}, need_cookie=True)
        if isinstance(api_resp, dict) and api_resp.get('code') == 301:
            return jsonify({'success': False, 'error': '需要登录以获取每日推荐'})
        daily = (api_resp.get('data') or {}).get('dailySongs', []) if isinstance(api_resp, dict) else []
        songs = _format_netease_songs(daily)
        return jsonify({'success': True, 'data': songs})
    except Exception as e:
        logger.warning(f"获取每日推荐失败: {e}")
        return jsonify({'success': False, 'error': '获取每日推荐失败，请检查登录状态或 API 服务'})

@app.route('/api/netease/login/status')
def netease_login_status():
    """检测当前 cookie 是否已登录。"""
    try:
        if not NETEASE_COOKIE:
            logger.info("网易云登录状态检查：当前未加载 cookie")
        api_resp = call_netease_api('/login/status', {'timestamp': int(time.time() * 1000)}, need_cookie=True)
        profile = api_resp.get('data', {}).get('profile') if isinstance(api_resp, dict) else None
        if profile:
            is_vip = False
            vip_info = {}
            try:
                vip_resp = call_netease_api('/vip/info', {'uid': profile.get('userId')})
                if isinstance(vip_resp, dict):
                    vip_info = vip_resp.get('data') or vip_resp
                    data = vip_info or {}
                    now_ms = int(time.time() * 1000)

                    def _active(pkg: dict):
                        """vipCode>0 且未过期的套餐视为有效；expireTime 为空默认为有效。"""
                        if not pkg:
                            return False
                        code = pkg.get('vipCode') or 0
                        exp = pkg.get('expireTime') or pkg.get('expiretime')
                        if code <= 0:
                            return False
                        if exp is None:
                            return True
                        try:
                            return int(exp) > now_ms
                        except Exception:
                            return False

                    # 综合判断：isVip 明确标记 > 任一未过期套餐/标识 > redVipLevel>0
                    is_vip = bool(data.get('isVip'))
                    if not is_vip:
                        is_vip = any([
                            _active(data.get('associator')),
                            _active(data.get('musicPackage')),
                            _active(data.get('redplus')),
                            _active(data.get('familyVip')),
                            (data.get('redVipLevel') or 0) > 0
                        ])
            except Exception as e:
                logger.warning(f"获取VIP信息失败: {e}")
            return jsonify({
                'success': True,
                'logged_in': True,
                'nickname': profile.get('nickname'),
                'user_id': profile.get('userId'),
                'avatar': profile.get('avatarUrl'),
                'is_vip': is_vip,
                'vip_info': vip_info
            })
        return jsonify({'success': True, 'logged_in': False, 'error': '未登录'})
    except Exception as e:
        logger.warning(f"检查网易云登录状态失败: {e}")
        return jsonify({'success': False, 'error': '状态检查失败'})

@app.route('/api/netease/logout', methods=['POST'])
def netease_logout():
    """退出登录并清空本地保存的网易云 cookie。"""
    try:
        if NETEASE_COOKIE:
            try:
                call_netease_api('/logout', {'timestamp': int(time.time() * 1000)}, need_cookie=True)
            except Exception as e:
                logger.info(f"网易云 API 注销调用失败，继续清理本地 cookie: {e}")
        save_netease_cookie('')
        return jsonify({'success': True})
    except Exception as e:
        logger.warning(f"网易云退出登录失败: {e}")
        return jsonify({'success': False, 'error': '退出失败'})

@app.route('/api/netease/login/qrcode')
def netease_login_qrcode():
    """生成扫码登录二维码。"""
    try:
        key_resp = call_netease_api('/login/qr/key', {'timestamp': int(time.time() * 1000)}, need_cookie=False)
        unikey = key_resp.get('data', {}).get('unikey')
        if not unikey:
            return jsonify({'success': False, 'error': '获取登录 key 失败'})
        qr_resp = call_netease_api('/login/qr/create', {'key': unikey, 'qrimg': 1, 'timestamp': int(time.time() * 1000)}, need_cookie=False)
        qrimg = qr_resp.get('data', {}).get('qrimg')
        if not qrimg:
            return jsonify({'success': False, 'error': '获取二维码失败'})
        return jsonify({'success': True, 'unikey': unikey, 'qrimg': qrimg})
    except Exception as e:
        logger.warning(f"生成网易云二维码失败: {e}")
        return jsonify({'success': False, 'error': '二维码生成失败'})

@app.route('/api/netease/login/check')
def netease_login_check():
    """轮询扫码状态，成功后保存 cookie。"""
    key = request.args.get('key')
    if not key:
        return jsonify({'success': False, 'error': '缺少 key'})
    try:
        resp = call_netease_api('/login/qr/check', {'key': key, 'timestamp': int(time.time() * 1000)}, need_cookie=False)
        code = resp.get('code')
        message = resp.get('message')
        cookie_str = resp.get('cookie')
        if not cookie_str and isinstance(resp.get('cookies'), list):
            cookie_str = '; '.join(resp.get('cookies'))
        
        # Debug Log
        if code == 803:
            logger.info(f"扫码成功 (803). Raw cookie: {bool(cookie_str)}, Length: {len(cookie_str) if cookie_str else 0}")
            
        if code == 803 and cookie_str:
            save_netease_cookie(cookie_str)
            return jsonify({'success': True, 'status': 'authorized', 'message': message})
        status_map = {
            800: 'expired',
            801: 'waiting',
            802: 'scanned'
        }
        return jsonify({'success': True, 'status': status_map.get(code, 'unknown'), 'message': message})
    except Exception as e:
        logger.warning(f"扫码检查失败: {e}")
        return jsonify({'success': False, 'error': '扫码轮询失败'})

@app.route('/api/netease/download_page')
def netease_download_page():
    """重定向到网易云音乐客户端下载页面。"""
    return redirect("https://music.163.com/client")

@app.route('/api/netease/config', methods=['GET', 'POST'])
def netease_config():
    """获取或更新网易云下载配置。"""
    try:
        if request.method == 'GET':
            return jsonify({'success': True, 'download_dir': NETEASE_DOWNLOAD_DIR, 'api_base': NETEASE_API_BASE, 'max_concurrent': NETEASE_MAX_CONCURRENT})
        data = request.json or {}
        target_dir = data.get('download_dir')
        api_base = (data.get('api_base') or '').strip()
        if target_dir:
            target_dir = os.path.abspath(target_dir)
            os.makedirs(target_dir, exist_ok=True)
        else:
            target_dir = None
        if api_base:
            api_base = api_base.rstrip('/')
        if not target_dir and not api_base:
            return jsonify({'success': False, 'error': '缺少下载目录或API地址'})
        save_netease_config(target_dir or NETEASE_DOWNLOAD_DIR, api_base or NETEASE_API_BASE)
        return jsonify({'success': True, 'download_dir': NETEASE_DOWNLOAD_DIR, 'api_base': NETEASE_API_BASE, 'max_concurrent': NETEASE_MAX_CONCURRENT})
    except Exception as e:
        logger.warning(f"更新网易云配置失败: {e}")
        return jsonify({'success': False, 'error': '保存失败'})

@app.route('/api/netease/debug')
def netease_debug():
    """调试用，查看 cookie 是否加载。"""
    info = {
        'cookie_loaded': bool(NETEASE_COOKIE),
        'cookie_len': len(NETEASE_COOKIE) if NETEASE_COOKIE else 0,
        'cookie_source': 'database',
        'download_dir': NETEASE_DOWNLOAD_DIR,
        'api_base': NETEASE_API_BASE,
        'max_concurrent': NETEASE_MAX_CONCURRENT,
    }
    if NETEASE_COOKIE:
        parsed = parse_cookie_string(NETEASE_COOKIE)
        info['cookie_keys'] = list(parsed.keys())
    return jsonify(info)

@app.route('/api/netease/resolve')
def netease_resolve():
    """通过分享链接或ID自动识别资源并返回歌曲列表。"""
    raw_input = request.args.get('input') or request.args.get('link') or request.args.get('id')
    parsed_input = _resolve_netease_input(raw_input)
    if not parsed_input:
        return jsonify({'success': False, 'error': '请粘贴网易云分享链接或输入ID'})
    try:
        if parsed_input['type'] == 'playlist':
            songs, name = _fetch_playlist_songs(parsed_input['id'])
            return jsonify({'success': True, 'type': 'playlist', 'id': parsed_input['id'], 'name': name, 'data': songs})
        songs = _fetch_song_detail(parsed_input['id'])
        return jsonify({'success': True, 'type': 'song', 'id': parsed_input['id'], 'data': songs})
    except Exception as e:
        logger.warning(f"解析网易云链接失败: {e}")
        return jsonify({'success': False, 'error': '解析失败，请确认歌曲或歌单链接有效'})

@app.route('/api/netease/playlist')
def netease_playlist_detail():
    """获取歌单详情及歌曲列表。"""
    raw_input = request.args.get('id') or request.args.get('link') or request.args.get('input')
    parsed_input = _resolve_netease_input(raw_input, prefer='playlist')
    if not parsed_input or parsed_input.get('type') != 'playlist':
        return jsonify({'success': False, 'error': '缺少歌单链接或无法识别'})
    try:
        songs, name = _fetch_playlist_songs(parsed_input['id'])
        return jsonify({'success': True, 'name': name, 'id': parsed_input['id'], 'data': songs})
    except Exception as e:
        logger.warning(f"歌单获取失败: {e}")
        return jsonify({'success': False, 'error': '获取歌单失败'})

@app.route('/api/netease/song')
def netease_song_detail():
    """根据单曲ID获取歌曲详情，用于解析而非直接下载。"""
    raw_input = request.args.get('id') or request.args.get('link') or request.args.get('input')
    parsed_input = _resolve_netease_input(raw_input, prefer='song')
    if not parsed_input:
        return jsonify({'success': False, 'error': '缺少歌曲链接或ID'})
    if parsed_input.get('type') == 'playlist':
        return jsonify({'success': False, 'error': '检测到歌单链接，请切换歌单解析'})
    try:
        parsed = _fetch_song_detail(parsed_input['id'])
        return jsonify({'success': True, 'id': parsed_input['id'], 'data': parsed})
    except Exception as e:
        logger.warning(f"获取单曲详情失败: {e}")
        return jsonify({'success': False, 'error': '获取歌曲信息失败'})

        # 索引文件
        index_single_file(target_path)
        
        DOWNLOAD_TASKS[task_id]['status'] = 'success'
        DOWNLOAD_TASKS[task_id]['progress'] = 100
        logger.info(f"网易云歌曲已下载: {filename} | {title} - {artist}")
        
    except Exception as e:
        logger.warning(f"网易云下载失败: {e}")
        DOWNLOAD_TASKS[task_id]['status'] = 'error'
        DOWNLOAD_TASKS[task_id]['message'] = str(e)
    finally:
        # 10分钟后清理任务状态
        def clean_task():
            time.sleep(600)
            DOWNLOAD_TASKS.pop(task_id, None)
        threading.Thread(target=clean_task, daemon=True).start()

@app.route('/api/netease/download', methods=['POST'])
def download_netease_music():
    """根据歌曲ID下载网易云音乐到本地库。(异步)"""
    payload = request.json or {}
    song_id = payload.get('id')
    if not song_id:
        return jsonify({'success': False, 'error': '缺少歌曲ID'})

    active = sum(1 for t in DOWNLOAD_TASKS.values() if t.get('status') in ('pending', 'preparing', 'downloading'))
    if active >= NETEASE_MAX_CONCURRENT:
        return jsonify({'success': False, 'error': f'并发下载已达上限 ({NETEASE_MAX_CONCURRENT})，请稍后再试'})
    
    task_id = f"task_{int(time.time()*1000)}_{os.urandom(4).hex()}"
    DOWNLOAD_TASKS[task_id] = {
        'status': 'pending', 
        'progress': 0, 
        'title': payload.get('title', '未知'),
        'artist': payload.get('artist', '未知')
    }
    
    threading.Thread(target=run_download_task, args=(task_id, payload), daemon=True).start()
    return jsonify({'success': True, 'task_id': task_id})

def _normalize_cover_url(url: str):
    if not url:
        return None
    u = url.replace('http://', 'https://')
    if '//' not in u:
        return None
    # NetEase 图片参数：确保有清晰尺寸
    if 'param=' not in u and '?param=' not in u:
        sep = '&' if '?' in u else '?'
        u = f"{u}{sep}param=1024y1024"
    return u

def run_download_task(task_id, payload):
    song_id = payload.get('id')
    title = (payload.get('title') or '').strip()
    artist = (payload.get('artist') or '').strip()
    album = (payload.get('album') or '').strip()
    level = payload.get('level') or 'exhigh'
    cover_url = _normalize_cover_url(payload.get('cover') or payload.get('album_art'))
    cover_bytes = fetch_cover_bytes(cover_url) if cover_url else None
    target_dir = payload.get('target_dir') or NETEASE_DOWNLOAD_DIR
    target_dir = os.path.abspath(target_dir)
    
    target_dir = os.path.abspath(target_dir)
    
    DOWNLOAD_TASKS[task_id]['status'] = 'preparing'

    try:
        os.makedirs(target_dir, exist_ok=True)
        need_detail_for_level = not payload.get('level')
        need_detail_for_cover = cover_bytes is None
        if not title or need_detail_for_level or need_detail_for_cover:
            # 拉取歌曲详情补充元信息、下载音质和封面
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
                base_filename = sanitize_filename(f"{artist or '未知艺术家'} - {title}")
        if not title:
            title = f"未命名 {song_id}"
        if not artist:
            artist = '未知艺术家'
        if 'base_filename' not in locals() or not base_filename:
            base_filename = sanitize_filename(payload.get('filename') or f"{artist} - {title}")
            
        # 更新任务信息
        DOWNLOAD_TASKS[task_id]['title'] = title
        DOWNLOAD_TASKS[task_id]['artist'] = artist

        api_resp = call_netease_api('/song/url/v1', {'id': song_id, 'level': level}, need_cookie=bool(NETEASE_COOKIE))
        data_list = api_resp.get('data') if isinstance(api_resp, dict) else None
        track_info = None
        if isinstance(data_list, list) and data_list:
            track_info = data_list[0]
        elif isinstance(data_list, dict):
            track_info = data_list

        if not track_info or (not track_info.get('url') and not track_info.get('proxyUrl')):
            # 回退到标准音质再试一次
            if level != 'standard':
                try:
                    api_resp_std = call_netease_api('/song/url/v1', {'id': song_id, 'level': 'standard'}, need_cookie=bool(NETEASE_COOKIE))
                    data_list = api_resp_std.get('data') if isinstance(api_resp_std, dict) else None
                    if isinstance(data_list, list) and data_list:
                        track_info = data_list[0]
                    elif isinstance(data_list, dict):
                        track_info = data_list
                except Exception:
                    track_info = track_info
            if not track_info or (not track_info.get('url') and not track_info.get('proxyUrl')):
                raise Exception('暂无可用下载地址，可能需要切换音质或登录')

        download_url = track_info.get('url') or track_info.get('proxyUrl')
        ext = (track_info.get('type') or track_info.get('encodeType') or 'mp3').lower()
        filename = base_filename if base_filename.lower().endswith(f".{ext}") else f"{base_filename}.{ext}"
        target_path = os.path.join(target_dir, filename)

        counter = 1
        while os.path.exists(target_path):
            filename = f"{base_filename} ({counter}).{ext}"
            target_path = os.path.join(target_dir, filename)
            counter += 1

        tmp_path = target_path + ".part"
        DOWNLOAD_TASKS[task_id]['status'] = 'downloading'
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
                                DOWNLOAD_TASKS[task_id]['progress'] = progress
                                
            shutil.move(tmp_path, target_path)
        finally:
            if os.path.exists(tmp_path):
                try: os.remove(tmp_path)
                except: pass
            
        # 索引文件
        base_name_for_cover = os.path.splitext(os.path.basename(target_path))[0]
        if cover_bytes:
            embed_cover_to_file(target_path, cover_bytes)
            save_cover_file(cover_bytes, base_name_for_cover)
        # 保存并内嵌歌词（无需登录）
        lrc_text, yrc_text = fetch_netease_lyrics(song_id)
        if lrc_text:
            try:
                os.makedirs(LYRICS_DIR, exist_ok=True)
                lrc_path = os.path.join(LYRICS_DIR, f"{base_name_for_cover}.lrc")
                with open(lrc_path, 'w', encoding='utf-8') as f:
                    f.write(lrc_text)
            except Exception as e:
                logger.warning(f"保存歌词失败: {e}")
            embed_lyrics_to_file(target_path, lrc_text)
        if yrc_text:
            try:
                with open(os.path.join(LYRICS_DIR, f"{base_name_for_cover}.yrc"), 'w', encoding='utf-8') as f:
                    f.write(yrc_text)
            except Exception as e:
                logger.warning(f"保存逐字歌词失败: {e}")
        index_single_file(target_path)
        
        DOWNLOAD_TASKS[task_id]['status'] = 'success'
        DOWNLOAD_TASKS[task_id]['progress'] = 100
        logger.info(f"网易云歌曲已下载: {filename} | {title} - {artist}")
        
    except Exception as e:
        logger.warning(f"网易云下载失败: {e}")
        DOWNLOAD_TASKS[task_id]['status'] = 'error'
        DOWNLOAD_TASKS[task_id]['message'] = str(e)
    finally:
        # 10分钟后清理任务状态
        def clean_task():
            time.sleep(600)
            DOWNLOAD_TASKS.pop(task_id, None)
        threading.Thread(target=clean_task, daemon=True).start()

@app.route('/api/netease/task/<task_id>')
def get_netease_task_status(task_id):
    task = DOWNLOAD_TASKS.get(task_id)
    if not task:
        return jsonify({'success': False, 'error': '任务不存在'})
    return jsonify({'success': True, 'data': task})

@app.route('/api/music/external/meta')
def get_external_meta():
    path = request.args.get('path')
    if not path or not os.path.exists(path): return jsonify({'success': False, 'error': '文件未找到'})
    try:
        meta = get_metadata(path)
        album_art = None
        base_name = os.path.splitext(os.path.basename(path))[0]
        cached_cover = os.path.join(MUSIC_LIBRARY_PATH, 'covers', f"{base_name}.jpg")
        if os.path.exists(cached_cover): album_art = f"/api/music/covers/{quote(base_name)}.jpg?filename={quote(base_name)}"
        return jsonify({'success': True, 'data': {'filename': path, 'title': meta['title'] or os.path.basename(path), 'artist': meta['artist'] or '未知艺术家', 'album': meta['album'] or '', 'album_art': album_art}})
    except Exception as e: return jsonify({'success': False, 'error': str(e)})

@app.route('/api/music/external/play')
def play_external_file():
    path = request.args.get('path')
    if path and os.path.exists(path): return send_file(path, conditional=True)
    return jsonify({'error': '文件未找到'}), 404

# --- 安装状态管理 ---
INSTALL_STATUS = {
    'status': 'idle', # idle, running, success, error
    'progress': 0,
    'step': '',
    'error': None
}

@app.route('/api/netease/install/status')
def get_install_status():
    return jsonify(INSTALL_STATUS)

@app.route('/api/netease/install_service', methods=['POST'])
def install_netease_service():
    """尝试自动拉取并运行网易云 API 容器"""
    import subprocess
    global INSTALL_STATUS
    
    if INSTALL_STATUS['status'] == 'running':
         return jsonify({'success': False, 'error': '安装任务正在进行中'})

    INSTALL_STATUS = {'status': 'running', 'progress': 0, 'step': '准备安装...', 'error': None}
    logger.info("API请求: 安装网易云服务")
    
    def run_install():
        global INSTALL_STATUS
        try:
            # 1. 检查 Docker 是否可用
            INSTALL_STATUS.update({'progress': 10, 'step': '检查 Docker 环境...'})
            subprocess.run(["docker", "--version"], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            
            # 2. 检查由我们创建的容器是否已存在
            container_name = "2fmusic-ncm-api"
            INSTALL_STATUS.update({'progress': 20, 'step': f'检查容器 {container_name}...'})
            
            check_proc = subprocess.run(
                ["docker", "ps", "-a", "--filter", f"name={container_name}", "--format", "{{.Names}}"],
                capture_output=True, text=True
            )
            
            if container_name in check_proc.stdout.strip():
                # 容器已存在，尝试启动
                INSTALL_STATUS.update({'progress': 60, 'step': '容器已存在，正在启动...'})
                logger.info("容器已存在，尝试启动...")
                subprocess.run(["docker", "start", container_name], check=True)
            else:
                # 容器不存在，拉取并运行
                INSTALL_STATUS.update({'progress': 30, 'step': '正在拉取镜像 (耗时较长)...'})
                logger.info("正在拉取镜像 moefurina/ncm-api...")
                subprocess.run(["docker", "pull", "moefurina/ncm-api:latest"], check=True)
                
                INSTALL_STATUS.update({'progress': 70, 'step': '镜像拉取完成，正在启动容器...'})
                logger.info("正在启动容器...")
                # 映射端口 23236:3000
                subprocess.run([
                    "docker", "run", "-d", 
                    "-p", "23236:3000", 
                    "--name", container_name, 
                    "--restart", "always",
                    "moefurina/ncm-api"
                ], check=True)
            
            INSTALL_STATUS.update({'status': 'success', 'progress': 100, 'step': '服务启动成功！'})
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

    # 异步执行，避免阻塞
    threading.Thread(target=run_install, daemon=True).start()
    
    return jsonify({'success': True, 'message': '安装任务已启动'})

if __name__ == '__main__':
    logger.info(f"服务启动，端口: {args.port} ...")
    try:
        init_db()
        app.run(host='0.0.0.0', port=args.port, threaded=True, use_reloader=False)
    except Exception as e:
        logger.exception(f"服务启动失败: {e}")
