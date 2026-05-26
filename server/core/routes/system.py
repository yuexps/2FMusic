from core.models.db import get_db
from core.services.scanner import SCAN_STATUS, LIBRARY_VERSION
from core.utils.logger import logger


def handle_get_system_status() -> tuple:
    """获取当前的音乐扫描、刮削与库数据统计状态"""
    status = dict(SCAN_STATUS)
    status['library_version'] = LIBRARY_VERSION

    try:
        with get_db() as conn:
            music_cnt = conn.execute("SELECT COUNT(*) FROM songs").fetchone()[0]
            pl_cnt = conn.execute("SELECT COUNT(*) FROM favorite_playlists").fetchone()[0]
            status['music_count'] = music_cnt
            status['playlist_count'] = pl_cnt
        return True, status, None
    except Exception as e:
        logger.exception(f"查询库数据统计发生错误: {e}")
        return False, None, str(e)

def handle_get_preferences() -> tuple:
    """获取自定义背景偏好等用户系统参数配置"""
    from core.models.preferences import get_preference
    try:
        prefs = {
            'custom_bg_enabled': get_preference('custom_bg_enabled', '0') == '1',
            'custom_bg_sync': get_preference('custom_bg_sync', '0') == '1',
            'custom_bg_timestamp': get_preference('custom_bg_timestamp', '0')
        }
        return True, prefs, None
    except Exception as e:
        logger.exception(f"获取偏好设置失败: {e}")
        return False, None, str(e)

def handle_save_preferences(prefs: dict) -> tuple:
    """保存自定义背景偏好等用户系统参数配置"""
    from core.models.preferences import set_preference
    if not prefs:
        return False, None, "参数配置对象不能为空"
    try:
        if 'custom_bg_enabled' in prefs:
            set_preference('custom_bg_enabled', '1' if prefs['custom_bg_enabled'] else '0')
        if 'custom_bg_sync' in prefs:
            set_preference('custom_bg_sync', '1' if prefs['custom_bg_sync'] else '0')
        if 'custom_bg_timestamp' in prefs:
            set_preference('custom_bg_timestamp', prefs['custom_bg_timestamp'])
        return True, None, None
    except Exception as e:
        logger.exception(f"保存偏好设置失败: {e}")
        return False, None, str(e)
