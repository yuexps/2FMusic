import time
from core.models.db import get_db
from core.utils.logger import logger

def add_play_history(song_id: str):
    """记录一次播放历史。添加新记录，若总数超出 100 条，则自动清理较旧的记录"""
    with get_db() as conn:
        try:
            # 插入新纪录
            conn.execute(
                "INSERT INTO play_history (song_id, play_time) VALUES (?, ?)",
                (song_id, time.time())
            )
            # 控制记录条数在 100 条以内
            count_row = conn.execute("SELECT COUNT(*) as count FROM play_history").fetchone()
            if count_row and count_row['count'] > 100:
                # 找到最新第 100 条的时间戳，以此为界清理更老的记录
                threshold_row = conn.execute(
                    "SELECT play_time FROM play_history ORDER BY play_time DESC LIMIT 1 OFFSET 99"
                ).fetchone()
                if threshold_row:
                    conn.execute("DELETE FROM play_history WHERE play_time < ?", (threshold_row['play_time'],))
            conn.commit()
        except Exception as e:
            conn.rollback()
            logger.exception(f"添加播放历史记录失败: {e}")
            raise e

def get_play_history(limit: int = 100) -> list:
    """获取播放历史，包含歌曲元数据，按播放时间由新到旧排序"""
    with get_db() as conn:
        try:
            # 关联 songs 表，过滤掉已经被物理删除的曲目 (INNER JOIN)
            rows = conn.execute("""
                SELECT ph.play_time, s.*
                FROM play_history ph
                JOIN songs s ON ph.song_id = s.id
                ORDER BY ph.play_time DESC
                LIMIT ?
            """, (limit,)).fetchall()
            
            history = []
            for row in rows:
                album_art = None
                if row['has_cover']:
                    album_art = f"/api/music/covers/{row['id']}.webp"
                
                history.append({
                    'time': int(row['play_time'] * 1000),  # 转成毫秒级 Unix 时间戳适配前端
                    'song': {
                        'id': row['id'],
                        'filename': row['filename'],
                        'title': row['title'],
                        'artist': row['artist'],
                        'album': row['album'],
                        'album_art': album_art,
                        'mtime': row['mtime'],
                        'size': row['size'],
                        'has_cover': bool(row['has_cover']),
                        'has_lyrics': bool(row['has_lyrics'])
                    }
                })
            return history
        except Exception as e:
            logger.exception(f"读取播放历史失败: {e}")
            raise e

def clear_play_history():
    """清空全部播放历史记录"""
    with get_db() as conn:
        try:
            conn.execute("DELETE FROM play_history")
            conn.commit()
        except Exception as e:
            conn.rollback()
            logger.exception(f"清空所有播放历史失败: {e}")
            raise e

def remove_from_history(song_id: str, play_time_ms: float):
    """从播放历史中移除特定时间点的一条记录"""
    play_time_sec = play_time_ms / 1000.0
    with get_db() as conn:
        try:
            # REAL 类型浮点数在 SQLite 中的精度匹配（保留 0.1s 偏差误差内）
            conn.execute(
                "DELETE FROM play_history WHERE song_id = ? AND ABS(play_time - ?) < 0.1",
                (song_id, play_time_sec)
            )
            conn.commit()
        except Exception as e:
            conn.rollback()
            logger.exception(f"删除单条播放历史失败 (song_id: {song_id}, play_time: {play_time_sec}): {e}")
            raise e
