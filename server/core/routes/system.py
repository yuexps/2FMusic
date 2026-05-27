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


def handle_get_lyrics_preference() -> tuple:
    """获取歌词刮削来源偏好"""
    from core.models.preferences import get_preference
    try:
        value = get_preference('lyrics_source_preference', 'embedded')
        return True, {'value': value}, None
    except Exception as e:
        logger.exception(f"获取歌词偏好失败: {e}")
        return False, None, str(e)


def handle_save_lyrics_preference(value: str) -> tuple:
    """保存歌词刮削来源偏好 (embedded | network)"""
    from core.models.preferences import set_preference
    if value not in ('embedded', 'network'):
        return False, None, "无效的歌词偏好值，请传入 'embedded' 或 'network'"
    try:
        set_preference('lyrics_source_preference', value)
        logger.info(f"歌词刮削偏好已更新为: {value}")
        return True, None, None
    except Exception as e:
        logger.exception(f"保存歌词偏好失败: {e}")
        return False, None, str(e)
