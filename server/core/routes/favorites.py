from core.models.favorite import (
    get_playlists,
    check_playlist_exists_by_name,
    create_playlist,
    delete_playlist,
    get_playlist_song_ids,
    add_song_to_favorite,
    remove_song_from_favorite,
    batch_add_to_favorites,
    batch_remove_from_favorites,
    batch_move_favorites
)
from core.utils.logger import logger

def handle_get_favorite_playlists() -> tuple:
    """获取所有收藏夹列表（含各自歌曲数）"""
    try:
        playlists = get_playlists()
        return True, playlists, None
    except Exception as e:
        logger.exception(f"获取收藏夹失败: {e}")
        return False, None, str(e)

def handle_create_favorite_playlist(name: str) -> tuple:
    """新建自定义歌单收藏夹"""
    if not name:
        return False, None, "收藏夹名称不能为空"
        
    try:
        if check_playlist_exists_by_name(name):
            return False, None, f"已存在名为'{name}'的收藏夹"
            
        result = create_playlist(name)
        logger.info(f"创建收藏夹成功: {name} (ID: {result['id']})")
        return True, result, None
    except Exception as e:
        logger.exception(f"创建收藏夹失败: {e}")
        return False, None, "创建失败"

def handle_delete_favorite_playlist(playlist_id: str) -> tuple:
    """删除指定自定义收藏夹（默认收藏夹禁止删除）"""
    if not playlist_id:
        return False, None, "缺少收藏夹ID"
    try:
        success = delete_playlist(playlist_id)
        if not success:
            return False, None, "默认收藏夹不能删除"
            
        return True, None, None
    except Exception as e:
        logger.exception(f"删除收藏夹失败，ID: {playlist_id}, 错误: {e}")
        return False, None, "删除失败"

def handle_get_playlist_songs(playlist_id: str) -> tuple:
    """获取指定收藏夹包含的所有歌曲 ID 集合"""
    if not playlist_id:
        return False, None, "缺少收藏夹ID"
    try:
        song_ids = get_playlist_song_ids(playlist_id)
        return True, song_ids, None
    except Exception as e:
        logger.exception(f"获取收藏夹歌曲失败，ID: {playlist_id}, 错误: {e}")
        return False, None, str(e)

def handle_add_favorite(song_id: str, playlist_id: str = 'default', title: str = '', artist: str = '') -> tuple:
    """将特定曲目添加至选定收藏夹"""
    if not song_id:
        return False, None, "歌曲ID不能为空"
    playlist_id = playlist_id or 'default'
    try:
        add_song_to_favorite(song_id, playlist_id, title, artist)
        return True, None, None
    except Exception as e:
        logger.exception(f"添加收藏失败: {e}")
        return False, None, "添加失败"

def handle_remove_favorite(song_id: str, playlist_id: str = 'default') -> tuple:
    """从选定收藏夹中移除特定曲目"""
    if not song_id:
        return False, None, "歌曲ID不能为空"
    playlist_id = playlist_id or 'default'
    try:
        remove_song_from_favorite(song_id, playlist_id)
        return True, None, None
    except Exception as e:
        logger.exception(f"取消收藏失败: {e}")
        return False, None, "移除失败"

def handle_batch_add_favorites(song_ids: list, playlist_ids: list = None, songs: dict = None) -> tuple:
    """批量添加歌曲到多个收藏夹"""
    if not song_ids:
        return False, None, "歌曲ID列表不能为空"
    if not playlist_ids:
        playlist_ids = ['default']
    songs = songs or {}
    try:
        res = batch_add_to_favorites(song_ids, playlist_ids, songs)
        return True, res, None
    except Exception as e:
        logger.exception(f"批量添加收藏失败: {e}")
        return False, None, "批量添加失败"

def handle_batch_remove_favorites(song_ids: list, playlist_ids: list) -> tuple:
    """批量从多个收藏夹中移除多首歌曲"""
    if not song_ids:
        return False, None, "歌曲ID列表不能为空"
    if not playlist_ids:
        return False, None, "收藏夹ID列表不能为空"
    try:
        res = batch_remove_from_favorites(song_ids, playlist_ids)
        return True, res, None
    except Exception as e:
        logger.exception(f"批量移除收藏失败: {e}")
        return False, None, str(e)

def handle_batch_move_favorites(song_ids: list, from_playlist_id: str, to_playlist_id: str) -> tuple:
    """将多首歌曲从源收藏夹批量移动到目标收藏夹"""
    if not song_ids:
        return False, None, "歌曲ID列表不能为空"
    if not from_playlist_id:
        return False, None, "源收藏夹ID不能为空"
    if not to_playlist_id:
        return False, None, "目标收藏夹ID不能为空"
    if from_playlist_id == to_playlist_id:
        return False, None, "源收藏夹和目标收藏夹不能相同"
    try:
        res = batch_move_favorites(song_ids, from_playlist_id, to_playlist_id)
        return True, res, None
    except Exception as e:
        logger.exception(f"批量移动收藏失败: {e}")
        return False, None, str(e)
