import os
import time
import threading
from core.models.db import get_db
from core.services.scanner import (
    SCAN_STATUS,
    scan_library_incremental,
    scan_directory_single,
    auto_scrape_missing_metadata,
    refresh_watchdog_paths
)
from core.utils.logger import logger

def handle_list_mount_points() -> tuple:
    """获取所有已挂载的目录路径"""
    try:
        with get_db() as conn:
            rows = conn.execute("SELECT path, created_at FROM mount_points ORDER BY created_at ASC").fetchall()
            paths = [r['path'] for r in rows]
            return True, paths, None
    except Exception as e:
        logger.exception(f"获取挂载目录失败: {e}")
        return False, None, str(e)

def handle_add_mount_point(path: str) -> tuple:
    """添加新的目录挂载点并启动增量扫描监控"""
    if not path:
        return False, None, "目录路径不能为空"
    try:
        path = os.path.abspath(path)
        if not os.path.exists(path):
            return False, None, "指定的本地路径在服务器上不存在"
            
        with get_db() as conn:
            # 查重
            exists = conn.execute("SELECT 1 FROM mount_points WHERE path=?", (path,)).fetchone()
            if exists:
                return False, None, "该目录路径已在挂载列表中"
                
            conn.execute("INSERT INTO mount_points (path, created_at) VALUES (?, ?)", (path, time.time()))
            conn.commit()
            
        logger.info(f"成功添加挂载目录: {path}")
        
        # 刷新 Watchdog 文件变动监听器
        refresh_watchdog_paths()
        
        # 异步启动全库增量扫描
        threading.Thread(target=scan_library_incremental, daemon=True).start()
        return True, None, None
    except Exception as e:
        logger.exception(f"添加挂载目录失败: {e}")
        return False, None, str(e)

def handle_delete_mount_point(path: str) -> tuple:
    """删除指定的挂载目录 (仅移除索引，不会物理删除音乐文件)"""
    if not path:
        return False, None, "目录路径不能为空"
    try:
        path = os.path.abspath(path)
        with get_db() as conn:
            # 清理数据库中该目录下的所有歌曲记录
            conn.execute("DELETE FROM songs WHERE path LIKE ? || '%'", (path,))
            conn.execute("DELETE FROM mount_points WHERE path=?", (path,))
            conn.commit()
            
        logger.info(f"成功移除挂载目录并清理索引: {path}")
        
        # 重新刷新 Watchdog 监听器
        refresh_watchdog_paths()
        
        from core.services.scanner import notify_library_changed
        notify_library_changed()
        
        return True, None, None
    except Exception as e:
        logger.exception(f"移除挂载目录失败: {e}")
        return False, None, str(e)

def handle_update_mount_point(path: str) -> tuple:
    """手动触发指定挂载目录的局部增量扫描"""
    if not path:
         return False, None, "未指定路径"
    try:
        path = os.path.abspath(path)
        if SCAN_STATUS.get('is_scraping') or SCAN_STATUS.get('scanning'):
             return False, None, "后台扫描或刮削正在运行中，请稍后再试"
             
        threading.Thread(target=scan_directory_single, args=(path,), daemon=True).start()
        return True, "已启动局部扫描目录任务", None
    except Exception as e:
        logger.exception(f"手动局部更新目录失败: {e}")
        return False, None, str(e)

def handle_retry_scrape_mount(path: str) -> tuple:
    """手动触发指定挂载目录下的元数据重新在线刮削"""
    if not path:
         return False, None, "未指定路径"
    try:
        path = os.path.abspath(path)
        if SCAN_STATUS.get('is_scraping') or SCAN_STATUS.get('scanning'):
              return False, None, "后台扫描或刮削正在运行中，请稍后再试"
             
        threading.Thread(target=auto_scrape_missing_metadata, args=(path,), daemon=True).start()
        return True, "已启动重新刮削元数据任务", None
    except Exception as e:
        logger.exception(f"手动触发刮削失败: {e}")
        return False, None, str(e)
