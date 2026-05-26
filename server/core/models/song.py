from core.models.db import get_db
from core.utils.logger import logger

def get_all_songs_deduplicated():
    """获取所有去重后的音乐曲目列表 (按内容MD5去重)"""
    try:
        with get_db() as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM songs ORDER BY title")
            songs = []
            seen = set()
            
            for row in cursor.fetchall():
                # 统一通过内容 MD5 ID 进行去重
                unique_key = row['id']
                if unique_key in seen:
                    continue
                seen.add(unique_key)
                
                album_art = None
                if row['has_cover']:
                    # 封面图链接
                    album_art = f"/api/music/covers/{row['id']}.webp"
                songs.append({
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
                })
        return songs
    except Exception as e:
        logger.exception(f"获取音乐列表失败: {e}")
        raise e

def get_song_by_id(song_id: str):
    """根据 ID 获取歌曲记录"""
    with get_db() as conn:
        row = conn.execute("SELECT * FROM songs WHERE id=?", (song_id,)).fetchone()
        return dict(row) if row else None

def get_song_by_path(path: str):
    """根据路径获取歌曲记录"""
    with get_db() as conn:
        row = conn.execute("SELECT * FROM songs WHERE path=?", (path,)).fetchone()
        return dict(row) if row else None

def insert_or_replace_song(song_id: str, path: str, filename: str, title: str, artist: str, album: str, mtime: float, size: int, has_cover: int, has_lyrics: int = 0):
    """插入或更新一条歌曲记录"""
    with get_db() as conn:
        conn.execute(
            """
            INSERT OR REPLACE INTO songs (id, path, filename, title, artist, album, mtime, size, has_cover, has_lyrics)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (song_id, path, filename, title, artist, album, mtime, size, has_cover, has_lyrics)
        )
        conn.commit()

def delete_song_by_path(path: str):
    """从数据库中删除指定路径的歌曲记录"""
    with get_db() as conn:
        conn.execute("DELETE FROM songs WHERE path=?", (path,))
        conn.commit()
