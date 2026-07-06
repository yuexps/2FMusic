import sqlite3
import os
import time
from core.config import app_config
from core.utils.logger import logger

def get_db():
    """获取 SQLite 数据库连接"""
    conn = sqlite3.connect(app_config.DB_PATH, timeout=30.0, check_same_thread=False)
    conn.row_factory = sqlite3.Row
    try:
        conn.execute("PRAGMA journal_mode=WAL;")
    except Exception as e:
        logger.warning(f"无法启用 WAL 模式: {e}")
    return conn

# 支持的音频文件后缀名
AUDIO_EXTS = ('.mp3', '.wav', '.ogg', '.flac', '.aac', '.m4a')

def init_db():
    """初始化数据库表结构与默认设置"""
    def _init_db_core():
        with get_db() as conn:
            conn.execute('''
                CREATE TABLE IF NOT EXISTS songs (
                    id TEXT PRIMARY KEY,
                    path TEXT UNIQUE,
                    filename TEXT,
                    title TEXT,
                    artist TEXT,
                    album TEXT,
                    album_artist TEXT,
                    mtime REAL,
                    size INTEGER,
                    has_cover INTEGER DEFAULT 0,
                    has_lyrics INTEGER DEFAULT 0,
                    scrape_retry_count INTEGER DEFAULT 0
                )
            ''')
            conn.execute('''
                CREATE TABLE IF NOT EXISTS favorite_playlists (
                    id TEXT PRIMARY KEY,
                    name TEXT NOT NULL,
                    is_default INTEGER DEFAULT 0,
                    created_at REAL
                )
            ''')
            conn.execute('''
                CREATE TABLE IF NOT EXISTS favorites (
                   song_id TEXT,
                   playlist_id TEXT,
                   title TEXT DEFAULT '',
                   artist TEXT DEFAULT '',
                   created_at REAL,
                   PRIMARY KEY (song_id, playlist_id)
                )
            ''')
            conn.execute('''
                CREATE TABLE IF NOT EXISTS play_history (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    song_id TEXT NOT NULL,
                    play_time REAL NOT NULL
                )
            ''')
            
            # 检查是否已有默认收藏夹，如果没有则创建
            default_count = conn.execute("SELECT COUNT(*) FROM favorite_playlists WHERE is_default = 1").fetchone()[0]
            if default_count == 0:
                conn.execute("INSERT INTO favorite_playlists (id, name, is_default, created_at) VALUES (?, ?, ?, ?)", 
                           ('default', '默认收藏夹', 1, time.time()))
                           
            conn.execute('''
                CREATE TABLE IF NOT EXISTS mount_points (
                    path TEXT PRIMARY KEY,
                    created_at REAL
                )
            ''')
            
            conn.execute('''
                CREATE TABLE IF NOT EXISTS system_settings (
                    key TEXT PRIMARY KEY,
                    value TEXT
                )
            ''')
            
            # 自动追加缺失的 album_artist 字段防止升级崩溃
            cursor = conn.cursor()
            try:
                cursor.execute("PRAGMA table_info(songs)")
                cols = [row['name'] for row in cursor.fetchall()]
                if 'album_artist' not in cols:
                    conn.execute("ALTER TABLE songs ADD COLUMN album_artist TEXT")
                    logger.info("数据库升级: 成功为 songs 表追加 album_artist 字段。")
            except Exception as migrate_err:
                logger.error(f"songs 表结构平滑追加 album_artist 失败: {migrate_err}")

            conn.commit()

    try:
        _init_db_core()
        logger.info("数据库初始化完成。")
        load_system_settings()
    except Exception as e:
        logger.error(f"数据库初始化失败: {e}，尝试重建数据库...")
        try:
            if os.path.exists(app_config.DB_PATH):
                os.remove(app_config.DB_PATH)
            _init_db_core()
            logger.info("数据库重建完成。")
            load_system_settings()
        except Exception as e2:
             logger.exception(f"数据库重建失败: {e2}")

def load_system_settings():
    """从数据库加载系统设置到内存配置中"""
    try:
        with get_db() as conn:
            cursor = conn.execute("SELECT name FROM sqlite_master WHERE type='table' AND name='system_settings'")
            if not cursor.fetchone():
                return
            cursor = conn.execute("SELECT key, value FROM system_settings")
            rows = cursor.fetchall()
            for row in rows:
                key, val = row['key'], row['value']
                if key == 'netease_cookie':
                    app_config.NETEASE_COOKIE = val
                elif key == 'netease_download_dir':
                    if val:
                        app_config.NETEASE_DOWNLOAD_DIR = val
                elif key == 'netease_api_base':
                    if val:
                        app_config.NETEASE_API_BASE = val
            logger.info("已成功从数据库载入系统设置。")
    except Exception as e:
        logger.warning(f"从数据库载入系统配置失败: {e}")
