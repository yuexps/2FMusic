from core.models.db import get_db
from core.utils.logger import logger

def get_preference(key: str, default: str = None) -> str:
    """从数据库 system_settings 表中读取系统设置/偏好键值"""
    try:
        with get_db() as conn:
            row = conn.execute("SELECT value FROM system_settings WHERE key = ?", (key,)).fetchone()
            if row:
                return row['value']
            return default
    except Exception as e:
        logger.error(f"读取系统设置偏好失败 (key: {key}): {e}")
        return default

def set_preference(key: str, value: str):
    """保存或更新偏好设置/系统设置到 system_settings 中"""
    try:
        with get_db() as conn:
            conn.execute(
                "INSERT OR REPLACE INTO system_settings (key, value) VALUES (?, ?)",
                (key, str(value))
            )
            conn.commit()
    except Exception as e:
        logger.exception(f"保存系统设置偏好失败 (key: {key}, value: {value}): {e}")
        raise e
