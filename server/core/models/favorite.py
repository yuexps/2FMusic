import time
import uuid
from core.models.db import get_db
from core.utils.logger import logger

def get_playlists():
    """获取所有收藏夹列表，并统计各个收藏夹下的歌曲数量"""
    with get_db() as conn:
        rows = conn.execute("""
            SELECT fp.*, COUNT(f.song_id) as song_count
            FROM favorite_playlists fp
            LEFT JOIN favorites f ON fp.id = f.playlist_id
            GROUP BY fp.id
            ORDER BY fp.is_default DESC, fp.created_at ASC
        """).fetchall()
        return [dict(row) for row in rows]

def check_playlist_exists_by_name(name: str) -> bool:
    """检查是否存在同名收藏夹"""
    with get_db() as conn:
        row = conn.execute("SELECT id FROM favorite_playlists WHERE name = ?", (name,)).fetchone()
        return row is not None

def create_playlist(name: str) -> dict:
    """创建收藏夹"""
    playlist_id = f"{time.time()}_{uuid.uuid4().hex[:8]}"
    with get_db() as conn:
        conn.execute("INSERT INTO favorite_playlists (id, name, created_at) VALUES (?, ?, ?)", 
                    (playlist_id, name, time.time()))
        conn.commit()
    return {'id': playlist_id, 'name': name}

def delete_playlist(playlist_id: str) -> bool:
    """删除收藏夹及其收藏记录，如果为默认收藏夹则返回 False"""
    with get_db() as conn:
        is_default = conn.execute("SELECT is_default FROM favorite_playlists WHERE id=?", (playlist_id,)).fetchone()
        if is_default and is_default['is_default'] == 1:
            return False
            
        conn.execute("DELETE FROM favorites WHERE playlist_id=?", (playlist_id,))
        conn.execute("DELETE FROM favorite_playlists WHERE id=?", (playlist_id,))
        conn.commit()
    return True

def get_playlist_song_ids(playlist_id: str) -> list:
    """获取指定收藏夹下所有的歌曲 ID"""
    with get_db() as conn:
        rows = conn.execute("SELECT song_id FROM favorites WHERE playlist_id=?", (playlist_id,)).fetchall()
        return [r['song_id'] for r in rows]

def add_song_to_favorite(song_id: str, playlist_id: str = 'default', title: str = '', artist: str = ''):
    """将单曲添加至收藏夹"""
    with get_db() as conn:
        conn.execute("INSERT OR IGNORE INTO favorites (song_id, playlist_id, title, artist, created_at) VALUES (?, ?, ?, ?, ?)", 
                    (song_id, playlist_id, title, artist, time.time()))
        conn.commit()

def remove_song_from_favorite(song_id: str, playlist_id: str = 'default'):
    """从指定收藏夹移除单曲"""
    with get_db() as conn:
        conn.execute("DELETE FROM favorites WHERE song_id=? AND playlist_id=?", (song_id, playlist_id))
        conn.commit()

def batch_add_to_favorites(song_ids: list, playlist_ids: list, songs_meta: dict) -> dict:
    """批量添加歌曲到收藏夹"""
    successful_count = 0
    failed_count = 0
    with get_db() as conn:
        try:
            for song_id in song_ids:
                for playlist_id in playlist_ids:
                    try:
                        song_info = songs_meta.get(song_id, {})
                        title = song_info.get('title', '')
                        artist = song_info.get('artist', '')
                        conn.execute("INSERT OR IGNORE INTO favorites (song_id, playlist_id, title, artist, created_at) VALUES (?, ?, ?, ?, ?)", 
                                    (song_id, playlist_id, title, artist, time.time()))
                        successful_count += 1
                    except Exception as e:
                        logger.warning(f"批量添加收藏失败 (歌曲 {song_id} -> 收藏夹 {playlist_id}): {e}")
                        failed_count += 1
            conn.commit()
        except Exception as e:
            conn.rollback()
            logger.error(f"批量添加收藏事务失败: {e}")
            raise e
    return {'successful': successful_count, 'failed': failed_count}

def batch_remove_from_favorites(song_ids: list, playlist_ids: list) -> dict:
    """批量从收藏夹移除歌曲，并进行一致性确认"""
    successful_count = 0
    failed_count = 0
    with get_db() as conn:
        try:
            for song_id in song_ids:
                for playlist_id in playlist_ids:
                    try:
                        cursor = conn.execute("DELETE FROM favorites WHERE song_id=? AND playlist_id=?", (song_id, playlist_id))
                        if cursor.rowcount > 0:
                            successful_count += 1
                        else:
                            # 记录不存在，结果是一样的，故也算入成功
                            successful_count += 1
                    except Exception as e:
                        logger.warning(f"批量移除收藏失败 (歌曲 {song_id} <- 收藏夹 {playlist_id}): {e}")
                        failed_count += 1
            conn.commit()
            
            # 数据库清理结果强制校验
            verify_failed = 0
            for song_id in song_ids:
                for playlist_id in playlist_ids:
                    remaining = conn.execute(
                        "SELECT COUNT(*) as count FROM favorites WHERE song_id=? AND playlist_id=?", 
                        (song_id, playlist_id)
                    ).fetchone()
                    if remaining and remaining['count'] > 0:
                        verify_failed += 1
                        logger.error(f"验证失败: 歌曲 {song_id} 仍在收藏夹 {playlist_id} 中，DELETE操作未生效！")
            
            if verify_failed > 0:
                raise RuntimeError(f"批量删除校验失败，有 {verify_failed} 个记录删除未生效")
                
        except Exception as e:
            conn.rollback()
            logger.error(f"批量移除收藏事务失败: {e}")
            raise e
            
    return {'successful': successful_count, 'failed': failed_count}

def batch_move_favorites(song_ids: list, from_playlist_id: str, to_playlist_id: str) -> dict:
    """批量移动歌曲到新收藏夹，带数据校验"""
    successful_count = 0
    failed_count = 0
    with get_db() as conn:
        try:
            for song_id in song_ids:
                try:
                    # 从源收藏夹删除
                    delete_cursor = conn.execute("DELETE FROM favorites WHERE song_id=? AND playlist_id=?", 
                                (song_id, from_playlist_id))
                    # 写入目标收藏夹
                    conn.execute("INSERT OR IGNORE INTO favorites (song_id, playlist_id, created_at) VALUES (?, ?, ?)", 
                                (song_id, to_playlist_id, time.time()))
                    if delete_cursor.rowcount > 0:
                        successful_count += 1
                    else:
                        successful_count += 1
                except Exception as e:
                    logger.warning(f"批量移动收藏失败 (歌曲 {song_id}: {from_playlist_id} -> {to_playlist_id}): {e}")
                    failed_count += 1
            conn.commit()
            
            # 数据移动结果校验
            verify_failed = 0
            for song_id in song_ids:
                # 校验源已清空
                rem_src = conn.execute("SELECT COUNT(*) as count FROM favorites WHERE song_id=? AND playlist_id=?",
                                       (song_id, from_playlist_id)).fetchone()
                if rem_src and rem_src['count'] > 0:
                    verify_failed += 1
                    logger.error(f"移动验证失败: 歌曲 {song_id} 仍在原收藏夹 {from_playlist_id}")
                
                # 校验目标已写入
                in_target = conn.execute("SELECT COUNT(*) as count FROM favorites WHERE song_id=? AND playlist_id=?",
                                         (song_id, to_playlist_id)).fetchone()
                if not in_target or in_target['count'] == 0:
                    verify_failed += 1
                    logger.error(f"移动验证失败: 歌曲 {song_id} 未写入目标收藏夹 {to_playlist_id}")
            
            if verify_failed > 0:
                raise RuntimeError(f"批量移动一致性校验失败，有 {verify_failed} 个冲突点")
                
        except Exception as e:
            conn.rollback()
            logger.error(f"批量移动事务异常: {e}")
            raise e
            
    return {'successful': successful_count, 'failed': failed_count}
