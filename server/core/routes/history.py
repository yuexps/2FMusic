from core.models.history import (
    add_play_history,
    get_play_history,
    clear_play_history,
    remove_from_history
)
from core.utils.logger import logger

def handle_add_play_history(song_id: str) -> tuple:
    """包装添加历史记录逻辑，返回 (success, data, error) 格式元组"""
    if not song_id:
        return False, None, "歌曲 ID 不能为空"
    try:
        add_play_history(song_id)
        return True, None, None
    except Exception as e:
        logger.exception(f"处理添加播放历史请求失败: {e}")
        return False, None, str(e)

def handle_get_play_history() -> tuple:
    """包装获取播放历史列表逻辑"""
    try:
        history = get_play_history()
        return True, history, None
    except Exception as e:
        logger.exception(f"处理读取播放历史请求失败: {e}")
        return False, None, str(e)

def handle_clear_play_history() -> tuple:
    """包装清空播放历史逻辑"""
    try:
        clear_play_history()
        return True, None, None
    except Exception as e:
        logger.exception(f"处理清空播放历史请求失败: {e}")
        return False, None, str(e)

def handle_remove_play_history(song_id: str, play_time: float) -> tuple:
    """包装移除单条历史记录逻辑"""
    if not song_id or play_time is None:
        return False, None, "缺少歌曲 ID 或时间戳"
    try:
        remove_from_history(song_id, play_time)
        return True, None, None
    except Exception as e:
        logger.exception(f"处理单条删除历史记录请求失败: {e}")
        return False, None, str(e)
