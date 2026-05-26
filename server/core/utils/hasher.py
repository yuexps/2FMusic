import hashlib
import os
import time

# 缓存静态文件的MD5值，避免重复计算
static_file_md5_cache = {}

def generate_song_id(path: str) -> str:
    """由音乐文件内容生成唯一标识 ID (MD5)"""
    return get_file_md5(path)

def get_file_md5(file_path: str) -> str:
    """计算并缓存文件的 MD5 值 (主要用于前端静态资源的版本控制)"""
    try:
        mtime = os.path.getmtime(file_path)
    except Exception:
        mtime = int(time.time())
    
    # 检查缓存中是否有该文件的MD5值和修改时间
    if file_path in static_file_md5_cache:
        cached_md5, cached_mtime = static_file_md5_cache[file_path]
        # 如果文件没有修改，直接返回缓存的MD5值
        if mtime == cached_mtime:
            return cached_md5
    
    try:
        with open(file_path, 'rb') as f:
            md5_hash = hashlib.md5()
            for chunk in iter(lambda: f.read(4096), b''):
                md5_hash.update(chunk)
            md5_value = md5_hash.hexdigest()
            # 缓存MD5值和修改时间
            static_file_md5_cache[file_path] = (md5_value, mtime)
            return md5_value
    except Exception:
        # 如果计算失败，返回当前时间戳作为 fallback
        return str(int(time.time()))
