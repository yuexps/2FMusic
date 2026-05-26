import os
import time
import threading
import requests
import concurrent.futures
import queue
import random
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

from core.config import app_config
from core.models.db import get_db, AUDIO_EXTS
from core.models.song import insert_or_replace_song, delete_song_by_path
from core.services.metadata import (
    get_metadata,
    extract_embedded_cover,
    extract_embedded_lyrics
)
from core.utils.logger import logger
from core.utils.common import COMMON_HEADERS
from core.utils.hasher import generate_song_id

# 挂载在 routes/ws 上的广播辅助函数
_ws_broadcast_callback = None

def register_ws_broadcast_callback(cb):
    """注册 WebSocket 广播回调函数"""
    global _ws_broadcast_callback
    _ws_broadcast_callback = cb

# 全局状态锁与变量
scan_status_lock = threading.Lock()
scan_execution_lock = threading.Lock()
watchdog_queue = queue.Queue()
SCAN_STATUS = {
    'scanning': False,
    'is_scraping': False,
    'total': 0,
    'processed': 0,
    'failed': 0,
    'current_file': '',
    'current_path': ''
}

LIBRARY_VERSION = time.time()

def update_scan_status(**kwargs):
    """线程安全地更新扫描状态，并通过 WebSocket 广播"""
    global SCAN_STATUS
    with scan_status_lock:
        for k, v in kwargs.items():
            if k in SCAN_STATUS:
                SCAN_STATUS[k] = v
        status_copy = dict(SCAN_STATUS)
        status_copy['library_version'] = LIBRARY_VERSION
        
    if _ws_broadcast_callback:
        try:
            _ws_broadcast_callback('scan_status', status_copy)
        except Exception as e:
            logger.debug(f"广播扫描状态失败: {e}")
            
    return status_copy

def notify_library_changed():
    """库发生变更时，更新版本戳并通过 WebSocket 广播通知前端"""
    global LIBRARY_VERSION
    LIBRARY_VERSION = time.time()
    
    if _ws_broadcast_callback:
        try:
            _ws_broadcast_callback('library_changed', {
                'library_version': LIBRARY_VERSION
            })
        except Exception as e:
            logger.debug(f"广播库变更通知失败: {e}")

# --- 文件监听器 ---
def _execute_watchdog_event(path, action):
    filename = os.path.basename(path)
    ext = os.path.splitext(filename)[1].lower()
    is_audio = ext in AUDIO_EXTS
    is_misc = ext in ('.lrc', '.yrc', '.jpg', '.jpeg', '.png', '.webp')
    
    logger.info(f"文件变动事件 [{action}]: {filename}")
    
    try:
        if action == 'created':
            if is_audio:
                index_single_file(path)
            elif is_misc:
                base = os.path.splitext(path)[0]
                for aud in AUDIO_EXTS:
                    aud_path = base + aud
                    if os.path.exists(aud_path):
                        index_single_file(aud_path)
                        
        elif action == 'deleted':
            if is_audio:
                delete_song_by_path(path)
            elif is_misc:
                base = os.path.splitext(path)[0]
                for aud in AUDIO_EXTS:
                    aud_path = base + aud
                    if os.path.exists(aud_path):
                        index_single_file(aud_path)
        
        notify_library_changed()
        
    except Exception as e:
        logger.error(f"处理物理文件同步索引失败: {e}")

def watchdog_worker():
    """Watchdog 异步防抖消费线程"""
    pending_events = {}  # path -> {'action': str, 'target_time': float, 'last_size': int}
    while True:
        try:
            try:
                path, action = watchdog_queue.get(timeout=0.5)
                now = time.time()
                size = 0
                try:
                    if os.path.exists(path):
                        size = os.path.getsize(path)
                except Exception:
                    pass
                pending_events[path] = {
                    'action': action,
                    'target_time': now + 2.0,  # 延迟 2 秒处理
                    'last_size': size
                }
                watchdog_queue.task_done()
            except queue.Empty:
                pass
            
            now = time.time()
            to_process = []
            for path, info in list(pending_events.items()):
                if now >= info['target_time']:
                    current_size = -1
                    try:
                        if os.path.exists(path):
                            current_size = os.path.getsize(path)
                    except Exception:
                        pass
                    
                    if info['action'] == 'deleted':
                        to_process.append((path, 'deleted'))
                        pending_events.pop(path, None)
                    elif current_size == info['last_size']:
                        to_process.append((path, info['action']))
                        pending_events.pop(path, None)
                    else:
                        info['last_size'] = current_size
                        info['target_time'] = now + 1.0
            
            for path, action in to_process:
                try:
                    _execute_watchdog_event(path, action)
                except Exception as ex:
                    logger.error(f"异步执行 Watchdog 事件失败: {ex}")
                    
        except Exception as e:
            logger.error(f"Watchdog 防抖消费线程异常: {e}")
            time.sleep(1)

class MusicFileEventHandler(FileSystemEventHandler):
    """监听音乐库物理文件变动"""
    def on_created(self, event):
        if event.is_directory: 
            return
        self._process(event.src_path, 'created')

    def on_deleted(self, event):
        if event.is_directory: 
            return
        self._process(event.src_path, 'deleted')

    def on_moved(self, event):
        if event.is_directory: 
            return
        self._process(event.src_path, 'deleted')
        self._process(event.dest_path, 'created')

    def _process(self, path, action):
        filename = os.path.basename(path)
        ext = os.path.splitext(filename)[1].lower()
        
        is_audio = ext in AUDIO_EXTS
        is_misc = ext in ('.lrc', '.yrc', '.jpg', '.jpeg', '.png' ,'.webp')
        
        if not is_audio and not is_misc:
            return

        logger.debug(f"监听到文件变动信号 [{action}]: {filename}，已加入异步防抖队列。")
        watchdog_queue.put((path, action))

global_observer = None

def init_watchdog():
    """初始化 Watchdog 监听服务"""
    global global_observer
    if not Observer: 
        return
    
    if global_observer:
        try:
            global_observer.stop()
            global_observer.join()
        except Exception:
            pass
            
    threading.Thread(target=watchdog_worker, daemon=True).start()
        
    global_observer = Observer()
    refresh_watchdog_paths()
    global_observer.start()
    logger.info("文件监听服务已启动")
    try:
        while True:
            time.sleep(1)
    except Exception:
        try:
            global_observer.stop()
        except Exception:
            pass
    global_observer.join()

def refresh_watchdog_paths():
    """根据最新数据库记录刷新监听文件夹目录"""
    global global_observer
    if not global_observer: 
        return
    
    global_observer.unschedule_all()
    
    try:
        raw_paths = {os.path.abspath(app_config.MUSIC_LIBRARY_PATH)}
        with get_db() as conn:
            rows = conn.execute("SELECT path FROM mount_points").fetchall()
            for r in rows: 
                if r['path']:
                    raw_paths.add(os.path.abspath(r['path']))
    except Exception: 
        raw_paths = {os.path.abspath(app_config.MUSIC_LIBRARY_PATH)}

    sorted_paths = sorted(list(raw_paths), key=len)
    final_targets = []
    for p in sorted_paths:
        if not any(p.startswith(parent + os.sep) or p == parent for parent in final_targets):
            final_targets.append(p)
    
    event_handler = MusicFileEventHandler()
    for path in final_targets:
        if os.path.exists(path):
            try:
                global_observer.schedule(event_handler, path, recursive=True)
                logger.info(f"监听目录: {path}")
            except Exception as e:
                logger.warning(f"无法监听目录 {path}: {e}")

# --- 单个文件索引 ---
def index_single_file(file_path: str):
    """对单首音频文件提取元数据并录入数据库"""
    try:
        if not os.path.exists(file_path): 
            return
        ext = os.path.splitext(file_path)[1].lower()
        if ext not in AUDIO_EXTS: 
            return
        
        stat = os.stat(file_path)
        
        # 1. 优先在数据库中找已有的 path 且大小、修改时间都未变的记录，以复用 ID，避免重新计算 MD5
        sid = None
        with get_db() as conn:
            row = conn.execute("SELECT id FROM songs WHERE path=? AND size=? AND mtime=?", (file_path, stat.st_size, stat.st_mtime)).fetchone()
            if row:
                sid = row['id']
        
        if not sid:
            sid = generate_song_id(file_path)
            
        meta = get_metadata(file_path)
        base_path = os.path.splitext(file_path)[0]
        cover_cache_path = os.path.join(app_config.COVERS_DIR, f"{sid}.webp")
        lrc_cache_path = os.path.join(app_config.LYRICS_DIR, f"{sid}.lrc")
        
        # 2. 封面匹配与缓存
        has_cover = 0
        cover_file_path = None
        for img_ext in ('.jpg', '.jpeg', '.png'):
            img_path = base_path + img_ext
            if os.path.exists(img_path):
                cover_file_path = img_path
                break
                
        if cover_file_path:
            has_cover = 1
            if not os.path.exists(cover_cache_path):
                try:
                    from core.services.metadata import save_cover_file
                    with open(cover_file_path, 'rb') as rf:
                        save_cover_file(rf.read(), sid)
                except Exception as e:
                    logger.warning(f"保存外部封面失败: {e}")
        elif os.path.exists(cover_cache_path):
            has_cover = 1
        else:
            if extract_embedded_cover(file_path, sid):
                has_cover = 1
                
        # 3. 歌词匹配与缓存
        has_lyrics = 0
        ext_lrc = base_path + ".lrc"
        if os.path.exists(ext_lrc):
            has_lyrics = 1
            if not os.path.exists(lrc_cache_path):
                import shutil
                try:
                    shutil.copy(ext_lrc, lrc_cache_path)
                except Exception:
                    pass
        elif os.path.exists(lrc_cache_path):
            has_lyrics = 1
        else:
            embedded_lrc = extract_embedded_lyrics(file_path)
            if embedded_lrc:
                try:
                    with open(lrc_cache_path, 'w', encoding='utf-8') as f:
                        f.write(embedded_lrc)
                    has_lyrics = 1
                except Exception:
                    pass
                    
        insert_or_replace_song(
            song_id=sid,
            path=file_path,
            filename=os.path.basename(file_path),
            title=meta['title'],
            artist=meta['artist'],
            album=meta['album'],
            mtime=stat.st_mtime,
            size=stat.st_size,
            has_cover=has_cover,
            has_lyrics=has_lyrics
        )
        logger.info(f"单文件索引完成: {file_path}")
    except Exception as e:
        logger.error(f"单文件索引失败: {e}")

# --- 增量文件扫描 ---
def clean_temp_part_files():
    """系统启动时物理清理残留的 .part 临时文件"""
    logger.info("系统启动：开始检索并清理历史残留的 .part 临时文件...")
    roots = [app_config.MUSIC_LIBRARY_PATH]
    if app_config.NETEASE_DOWNLOAD_DIR:
        roots.append(app_config.NETEASE_DOWNLOAD_DIR)
    try:
        with get_db() as conn:
            rows = conn.execute("SELECT path FROM mount_points").fetchall()
            roots.extend([r['path'] for r in rows])
    except Exception:
        pass
    
    cleaned_count = 0
    for r in set(roots):
        if not os.path.exists(r):
            continue
        try:
            for root, dirs, files in os.walk(r):
                for f in files:
                    if f.endswith('.part'):
                        path = os.path.join(root, f)
                        try:
                            os.remove(path)
                            cleaned_count += 1
                        except Exception:
                            pass
        except Exception as e:
            logger.warning(f"遍历目录 {r} 清理 part 文件失败: {e}")
            
    if cleaned_count > 0:
        logger.info(f"系统启动：成功清理了 {cleaned_count} 个残留的 .part 临时文件。")
    else:
        logger.info("系统启动：未发现残留的 .part 临时文件。")

def scan_library_incremental():
    """主程序：全量增量扫描音乐目录"""
    if not scan_execution_lock.acquire(blocking=False):
        logger.info("已有扫描任务在运行中，跳过本次全量增量扫描。")
        return

    lock_file = os.path.join(app_config.MUSIC_LIBRARY_PATH, '.scan_lock')
    if os.path.exists(lock_file):
        if time.time() - os.path.getmtime(lock_file) > 300:
            try:
                os.remove(lock_file)
                logger.info("过期扫描锁文件已移除。")
            except Exception as e:
                logger.warning(f"移除扫描锁文件失败: {e}")
                scan_execution_lock.release()
                return
        else:
            scan_execution_lock.release()
            return 

    try:
        update_scan_status(scanning=True, total=0, processed=0, current_file='正在遍历文件...', current_path='')
        with open(lock_file, 'w') as f: 
            f.write(str(time.time()))
        logger.info("开始增量扫描...")
        
        scan_roots = [app_config.MUSIC_LIBRARY_PATH]
        try:
            with get_db() as conn:
                rows = conn.execute("SELECT path FROM mount_points").fetchall()
                scan_roots.extend([r['path'] for r in rows])
        except Exception: 
            pass
        
        disk_files = {}
        for root_dir in scan_roots:
            if not os.path.exists(root_dir): 
                continue
            for root, dirs, files in os.walk(root_dir):
                dirs[:] = [d for d in dirs if d not in ('lyrics', 'covers')]
                for f in files:
                    if f.lower().endswith(AUDIO_EXTS):
                        path = os.path.join(root, f)
                        try:
                            stat = os.stat(path)
                            disk_files[path] = {
                                'mtime': stat.st_mtime,
                                'size': stat.st_size,
                                'path': path,
                                'filename': f
                            }
                        except Exception: 
                            pass

        with get_db() as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT id, path, mtime, size, has_cover, has_lyrics FROM songs")
            db_rows = {row['path']: row for row in cursor.fetchall()}
            
            to_delete_paths = set(db_rows.keys()) - set(disk_files.keys())
            to_delete_ids = {db_rows[p]['id']: p for p in to_delete_paths}
            
            files_to_process_list = []
            for path, info in disk_files.items():
                db_rec = db_rows.get(path)
                if not db_rec or db_rec['mtime'] != info['mtime'] or db_rec['size'] != info['size']:
                    files_to_process_list.append(info)

            total_files = len(files_to_process_list)
            update_scan_status(total=total_files, processed=0)
            
            to_update_db = []
            
            if total_files > 0:
                logger.info(f"使用线程池处理 {total_files} 个文件...")
                
                def process_file_metadata(info):
                    update_scan_status(current_path=info['path'])
                    sid = generate_song_id(info['path'])
                    meta = get_metadata(info['path'])
                    
                    base_path = os.path.splitext(info['path'])[0]
                    cover_cache_path = os.path.join(app_config.COVERS_DIR, f"{sid}.webp")
                    lrc_cache_path = os.path.join(app_config.LYRICS_DIR, f"{sid}.lrc")
                    
                    # 检查封面
                    has_cover = 0
                    cover_file_path = None
                    for img_ext in ('.jpg', '.jpeg', '.png'):
                        img_path = base_path + img_ext
                        if os.path.exists(img_path):
                            cover_file_path = img_path
                            break
                            
                    if cover_file_path:
                        has_cover = 1
                        if not os.path.exists(cover_cache_path):
                            try:
                                from core.services.metadata import save_cover_file
                                with open(cover_file_path, 'rb') as rf:
                                    save_cover_file(rf.read(), sid)
                            except Exception as e:
                                logger.warning(f"保存外部封面失败: {e}")
                    elif os.path.exists(cover_cache_path):
                        has_cover = 1
                    else:
                        if extract_embedded_cover(info['path'], sid):
                            has_cover = 1
                            
                    # 检查歌词
                    has_lyrics = 0
                    ext_lrc = base_path + ".lrc"
                    if os.path.exists(ext_lrc):
                        has_lyrics = 1
                        if not os.path.exists(lrc_cache_path):
                            import shutil
                            try:
                                shutil.copy(ext_lrc, lrc_cache_path)
                            except Exception:
                                pass
                    elif os.path.exists(lrc_cache_path):
                        has_lyrics = 1
                    else:
                        embedded_lrc = extract_embedded_lyrics(info['path'])
                        if embedded_lrc:
                            try:
                                with open(lrc_cache_path, 'w', encoding='utf-8') as f:
                                    f.write(embedded_lrc)
                                has_lyrics = 1
                            except Exception:
                                pass
                                
                    title = str(meta['title']) if meta['title'] is not None else ''
                    artist = str(meta['artist']) if meta['artist'] is not None else ''
                    album = str(meta['album']) if meta['album'] is not None else ''
                    
                    return (sid, info['path'], info['filename'], title, artist, album, info['mtime'], info['size'], has_cover, has_lyrics)

                with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
                    futures = {executor.submit(process_file_metadata, item): item for item in files_to_process_list}
                    for future in concurrent.futures.as_completed(futures):
                        try:
                            res = future.result()
                            to_update_db.append(res)
                        except Exception as pe:
                            logger.error(f"处理文件属性失败: {pe}")
                        
                        with scan_status_lock:
                            SCAN_STATUS['processed'] += 1
                            processed = SCAN_STATUS['processed']
                        
                        if processed % 10 == 0 or processed >= total_files:
                            update_scan_status(current_file=f"处理中... {int((processed/total_files)*100)}%")

                final_update_db = []
                seen_in_batch = set()

                for item in to_update_db:
                    c_id, c_path, c_fname = item[0], item[1], item[2]
                    
                    if c_id in seen_in_batch:
                        logger.info(f"扫描: 跳过批次内重复文件 (MD5一致) {c_path}")
                        continue
                    seen_in_batch.add(c_id)
                    
                    if c_id in to_delete_ids:
                        old_path = to_delete_ids[c_id]
                        logger.info(f"检测到文件移动/重命名: {old_path} -> {c_path}，ID={c_id}，继承原纪录属性。")
                        to_delete_paths.discard(old_path)
                        
                    final_update_db.append(item)

                if final_update_db:
                    cursor.executemany('''
                        INSERT OR REPLACE INTO songs (id, path, filename, title, artist, album, mtime, size, has_cover, has_lyrics)
                        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                    ''', final_update_db)
                    conn.commit()

            if to_delete_paths:
                logger.info(f"清理已失效文件记录共 {len(to_delete_paths)} 条...")
                cursor.executemany("DELETE FROM songs WHERE path=?", [(p,) for p in to_delete_paths])
                conn.commit()

        logger.info("库扫描结束。")
        notify_library_changed()
        
        # 4. 后台启动元数据刮削
        update_scan_status(is_scraping=True, current_file="正在准备自动刮削...")
        threading.Thread(target=auto_scrape_missing_metadata, daemon=True).start()
        
    except Exception as e:
        logger.error(f"扫描执行失败: {e}")
    finally:
        update_scan_status(scanning=False, current_file='')
        if os.path.exists(lock_file): 
            try: 
                os.remove(lock_file)
            except Exception: 
                pass
        scan_execution_lock.release()

# --- 后台自动刮削逻辑 ---
def auto_scrape_missing_metadata(target_dir=None):
    """自动刮削缺失封面与歌词"""
    logger.info(f"开始自动刮削缺失元数据... {f'(目录: {target_dir})' if target_dir else ''}")
    update_scan_status(current_file="正在准备自动刮削...", is_scraping=True, processed=0, total=0, failed=0)
    
    try:
        
        songs_to_scrape = []
        with get_db() as conn:
            sql = "SELECT id, path, title, artist, album, filename, has_cover, has_lyrics FROM songs WHERE has_cover = 0 OR has_lyrics = 0"
            params = ()
            if target_dir:
                sql = "SELECT id, path, title, artist, album, filename, has_cover, has_lyrics FROM songs WHERE (has_cover = 0 OR has_lyrics = 0) AND path LIKE ? || '%'"
                params = (target_dir,)
            cursor = conn.execute(sql, params)
            all_songs = cursor.fetchall()

        for song in all_songs:
            need_cover = (song['has_cover'] == 0)
            need_lyrics = (song['has_lyrics'] == 0)
            
            if need_cover or need_lyrics:
                songs_to_scrape.append({
                    'song': song,
                    'need_cover': need_cover,
                    'need_lyrics': need_lyrics
                })

        total = len(songs_to_scrape)
        if total == 0:
            logger.info("没有需要刮削的歌曲。")
            update_scan_status(current_file="刮削完成", is_scraping=False)
            time.sleep(1.5)
            update_scan_status(current_file='')
            return

        logger.info(f"发现 {total} 首歌曲需要刮削元数据")
        update_scan_status(total=total, processed=0, failed=0)

        max_workers = 5
        with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as executor:
            futures = [executor.submit(scrape_single_song, item, idx, total) for idx, item in enumerate(songs_to_scrape)]
            for future in concurrent.futures.as_completed(futures):
                try:
                    future.result()
                except Exception as e:
                    logger.error(f"刮削单任务异常: {e}")

    except Exception as e:
        logger.error(f"自动刮削总任务异常: {e}")
    finally:
        logger.info("自动刮削任务结束")
        with scan_status_lock:
            failed_count = SCAN_STATUS.get('failed', 0)
        
        if failed_count > 0:
            update_scan_status(current_file=f"刮削完成 ({failed_count}首失败)", is_scraping=False)
        else:
            update_scan_status(current_file="刮削完成", is_scraping=False)
            
        time.sleep(1.5)
        
        with scan_status_lock:
            is_scanning = SCAN_STATUS.get('scanning', False)
        if not is_scanning:
            update_scan_status(current_file='')
            
        notify_library_changed()

def scrape_single_song(item: dict, idx: int, total: int):
    """刮削单首歌曲"""
    song = item['song']
    update_scan_status(current_path=song['path'])
    
    import mod.searchx.qq
    import mod.searchx.netease
    import mod.searchx.kugou
    
    try:
        # 0. 优先尝试提取音频内嵌封面
        if item['need_cover']:
             if extract_embedded_cover(song['path'], song['id']):
                with get_db() as conn:
                    conn.execute("UPDATE songs SET has_cover=1 WHERE id=?", (song['id'],))
                    conn.commit()
                logger.info(f"刮削时发现内嵌封面，已提取: {song['title']}")
                item['need_cover'] = False

        if not item['need_cover'] and not item['need_lyrics']:
            with scan_status_lock:
                SCAN_STATUS['processed'] += 1
                current_processed = SCAN_STATUS['processed']
            if current_processed % 5 == 0 or current_processed >= total:
                update_scan_status(current_file="刮削中...")
            return

        def is_satisfied(res_list):
            ok_cover = not item['need_cover'] or any(r.get('cover') for r in res_list if r.get('cover'))
            ok_lyrics = not item['need_lyrics'] or any(r.get('lyrics') for r in res_list if r.get('lyrics'))
            return ok_cover and ok_lyrics

        results = []
        providers = [mod.searchx.qq, mod.searchx.netease, mod.searchx.kugou]
        
        for attempt in range(3):
            results = []
            for prov in providers:
                try:
                    time.sleep(random.uniform(0.1, 0.5))
                    p_res = prov.search(title=song['title'], artist=song['artist'], album=song['album'])
                    if p_res:
                        results.extend(p_res)
                    if is_satisfied(results):
                        break
                    
                    if item['need_cover'] and not any(r.get('cover') for r in results) and song['album']:
                         l_res = prov.search(title=song['title'], artist=song['artist'], album='')
                         if l_res:
                             results.extend(l_res)
                         if is_satisfied(results):
                             break
                except Exception as e:
                    logger.warning(f"提供商 {prov.__name__} 检索失败: {e}")
            if results:
                break
            if attempt < 2:
                time.sleep(1)

        if not results:
            with scan_status_lock:
                 SCAN_STATUS['failed'] += 1
                 SCAN_STATUS['processed'] += 1
            return
        
        is_partial_fail = False

        # 歌词下载
        if item['need_lyrics']:
            found_lyrics = next((r.get('lyrics') for r in results if r.get('lyrics')), None)
            if found_lyrics:
                save_lrc_path = os.path.join(app_config.LYRICS_DIR, f"{song['id']}.lrc")
                try:
                    with open(save_lrc_path, 'w', encoding='utf-8') as f:
                        f.write(found_lyrics)
                    with get_db() as conn:
                        conn.execute("UPDATE songs SET has_lyrics=1 WHERE id=?", (song['id'],))
                        conn.commit()
                    logger.info(f"自动保存歌词成功: {save_lrc_path}")
                except Exception as e:
                    logger.warning(f"保存歌词失败: {e}")
                    is_partial_fail = True
            else:
                 is_partial_fail = True

        # 封面下载与写入
        if item['need_cover']:
            found_cover = next((r.get('cover') for r in results if r.get('cover')), None)
            if found_cover:
                try:
                    resp = requests.get(found_cover, timeout=10, headers=COMMON_HEADERS)
                    if resp.status_code == 200:
                        from core.services.metadata import save_cover_file
                        saved_path = save_cover_file(resp.content, song['id'])
                        if saved_path:
                            with get_db() as conn:
                                conn.execute("UPDATE songs SET has_cover=1 WHERE id=?", (song['id'],))
                                conn.commit()
                            logger.info(f"自动保存封面成功: {saved_path}")
                        else:
                            logger.warning(f"自动保存及压缩转换封面失败: {song['id']}")
                            is_partial_fail = True
                    else:
                        logger.warning(f"下载封面失败: {resp.status_code} - {found_cover}")
                        is_partial_fail = True
                except Exception as e:
                    logger.warning(f"下载封面异常: {e}")
                    is_partial_fail = True
            else:
                is_partial_fail = True
        
        with scan_status_lock:
             if is_partial_fail:
                 SCAN_STATUS['failed'] += 1
             SCAN_STATUS['processed'] += 1
             current_processed = SCAN_STATUS['processed']
             
        if current_processed % 5 == 0 or current_processed >= total:
            update_scan_status(current_file="刮削中...")
            
    except Exception as e:
        logger.warning(f"刮削单曲失败 {song['title']}: {e}")
        with scan_status_lock:
             SCAN_STATUS['failed'] += 1
             SCAN_STATUS['processed'] += 1

def scan_directory_single(target_dir: str):
    """单独扫描特定文件夹目录并入库"""
    if not os.path.exists(target_dir):
        logger.error(f"目录不存在: {target_dir}")
        return

    if not scan_execution_lock.acquire(blocking=False):
        logger.info("已有扫描任务在运行中，跳过本次单独目录扫描。")
        return

    try:
        update_scan_status(scanning=True, total=0, processed=0, current_file='正在扫描目录...', current_path='')
        logger.info(f"开始单独扫描目录: {target_dir}")
        
        disk_files = {}
        for root, dirs, files in os.walk(target_dir):
             dirs[:] = [d for d in dirs if d not in ('lyrics', 'covers')]
             for f in files:
                 if f.lower().endswith(AUDIO_EXTS):
                     path = os.path.join(root, f)
                     try:
                         stat = os.stat(path)
                         disk_files[path] = {
                             'mtime': stat.st_mtime,
                             'size': stat.st_size,
                             'path': path,
                             'filename': f
                         }
                     except Exception:
                         pass

        with get_db() as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT id, path, mtime, size, has_cover, has_lyrics FROM songs WHERE path LIKE ? || '%'", (target_dir,))
            db_rows = {row['path']: row for row in cursor.fetchall()}
            
            to_delete_paths = set(db_rows.keys()) - set(disk_files.keys())
            to_delete_ids = {db_rows[p]['id']: p for p in to_delete_paths}
            
            files_to_process_list = []
            for path, info in disk_files.items():
                db_rec = db_rows.get(path)
                if not db_rec or db_rec['mtime'] != info['mtime'] or db_rec['size'] != info['size']:
                    files_to_process_list.append(info)

            total_files = len(files_to_process_list)
            update_scan_status(total=total_files, processed=0)
            
            to_update_db = []
            
            if total_files > 0:
                def process_file_metadata(info):
                    update_scan_status(current_path=info['path'])
                    sid = generate_song_id(info['path'])
                    meta = get_metadata(info['path'])
                    
                    base_path = os.path.splitext(info['path'])[0]
                    cover_cache_path = os.path.join(app_config.COVERS_DIR, f"{sid}.webp")
                    lrc_cache_path = os.path.join(app_config.LYRICS_DIR, f"{sid}.lrc")
                    
                    # 检查封面
                    has_cover = 0
                    cover_file_path = None
                    for img_ext in ('.jpg', '.jpeg', '.png'):
                        img_path = base_path + img_ext
                        if os.path.exists(img_path):
                            cover_file_path = img_path
                            break
                            
                    if cover_file_path:
                        has_cover = 1
                        if not os.path.exists(cover_cache_path):
                            try:
                                from core.services.metadata import save_cover_file
                                with open(cover_file_path, 'rb') as rf:
                                    save_cover_file(rf.read(), sid)
                            except Exception as e:
                                logger.warning(f"保存外部封面失败: {e}")
                    elif os.path.exists(cover_cache_path):
                        has_cover = 1
                    else:
                        if extract_embedded_cover(info['path'], sid):
                            has_cover = 1
                            
                    # 检查歌词
                    has_lyrics = 0
                    ext_lrc = base_path + ".lrc"
                    if os.path.exists(ext_lrc):
                        has_lyrics = 1
                        if not os.path.exists(lrc_cache_path):
                            import shutil
                            try:
                                shutil.copy(ext_lrc, lrc_cache_path)
                            except Exception:
                                pass
                    elif os.path.exists(lrc_cache_path):
                        has_lyrics = 1
                    else:
                        embedded_lrc = extract_embedded_lyrics(info['path'])
                        if embedded_lrc:
                            try:
                                with open(lrc_cache_path, 'w', encoding='utf-8') as f:
                                    f.write(embedded_lrc)
                                has_lyrics = 1
                            except Exception:
                                pass
                                
                    title = str(meta['title']) if meta['title'] is not None else ''
                    artist = str(meta['artist']) if meta['artist'] is not None else ''
                    album = str(meta['album']) if meta['album'] is not None else ''
                    
                    return (sid, info['path'], info['filename'], title, artist, album, info['mtime'], info['size'], has_cover, has_lyrics)

                with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
                     futures = {executor.submit(process_file_metadata, item): item for item in files_to_process_list}
                     for future in concurrent.futures.as_completed(futures):
                         try:
                             res = future.result()
                             to_update_db.append(res)
                         except Exception as pe:
                             logger.error(f"扫描单目录线程任务失败: {pe}")
                             
                         with scan_status_lock:
                             SCAN_STATUS['processed'] += 1
                             processed = SCAN_STATUS['processed']
                         if processed % 5 == 0 or processed >= total_files:
                             update_scan_status(current_file=f"处理中... {int((processed/total_files)*100)}%")

            final_update_db = []
            seen_in_batch = set()
            for item in to_update_db:
                c_id, c_path = item[0], item[1]
                if c_id in seen_in_batch:
                    logger.info(f"单独扫描: 跳过批次内重复文件 (MD5一致) {c_path}")
                    continue
                seen_in_batch.add(c_id)
                
                if c_id in to_delete_ids:
                    old_path = to_delete_ids[c_id]
                    logger.info(f"检测到文件移动/重命名(单目录): {old_path} -> {c_path}，ID={c_id}。")
                    to_delete_paths.discard(old_path)
                final_update_db.append(item)

            if final_update_db:
                conn.executemany('''
                    INSERT OR REPLACE INTO songs (id, path, filename, title, artist, album, mtime, size, has_cover, has_lyrics)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                ''', final_update_db)
                conn.commit()
                
            if to_delete_paths:
                logger.info(f"清理已失效文件记录共 {len(to_delete_paths)} 条...")
                cursor.executemany("DELETE FROM songs WHERE path=?", [(p,) for p in to_delete_paths])
                conn.commit()
        
        logger.info(f"单独扫描目录完成: {target_dir}")
        notify_library_changed()
        
        auto_scrape_missing_metadata(target_dir)

    except Exception as e:
        logger.exception(f"目录单独扫描异常失败: {e}")
    finally:
        update_scan_status(scanning=False, current_file='')
        scan_execution_lock.release()
