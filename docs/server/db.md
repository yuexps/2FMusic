# 数据库结构与配置规范 (docs/server/db.md)

## 1. 运行配置
*   **引擎**：SQLite 3。
*   **WAL 模式**：初始化时强制执行 `PRAGMA journal_mode=WAL;`，提高多线程高并发读写性能。

## 2. 数据表 DDL
```sql
-- 1. 歌曲元数据与物理缓存状态表
CREATE TABLE songs (
    id TEXT PRIMARY KEY,          -- 物理文件内容 MD5 哈希
    path TEXT UNIQUE,             -- 音频物理绝对路径
    filename TEXT,                -- 文件名
    title TEXT,                   -- 歌曲名
    artist TEXT,                  -- 歌手
    album TEXT,                   -- 专辑
    album_artist TEXT,            -- 专辑艺术家/唱片艺术家
    mtime REAL,                   -- 文件物理修改时间戳
    size INTEGER,                 -- 文件物理大小 (字节)
    has_cover INTEGER DEFAULT 0,  -- 封面缓存状态 (0: 无, 1: 有)
    has_lyrics INTEGER DEFAULT 0, -- 歌词缓存状态 (0: 无, 1: 有)
    scrape_retry_count INTEGER DEFAULT 0 -- 在线刮削尝试失败次数
);

-- 2. 自定义歌单（收藏夹）表
CREATE TABLE favorite_playlists (
    id TEXT PRIMARY KEY,          -- 歌单 ID (默认有且仅有一条记录 id='default', name='默认收藏夹', is_default=1)
    name TEXT NOT NULL,           -- 歌单名称
    is_default INTEGER DEFAULT 0, -- 是否是默认红心歌单 (0: 否, 1: 是)
    created_at REAL               -- 创建时间戳
);

-- 3. 歌单与歌曲关联表（多对多关系）
CREATE TABLE favorites (
    song_id TEXT,
    playlist_id TEXT,
    title TEXT DEFAULT '',
    artist TEXT DEFAULT '',
    created_at REAL,
    PRIMARY KEY (song_id, playlist_id)
);

-- 4. 播放历史纪录表
CREATE TABLE play_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    song_id TEXT NOT NULL,
    play_time REAL NOT NULL       -- 播放完成的时间戳
);

-- 5. 额外本地目录挂载点表
CREATE TABLE mount_points (
    path TEXT PRIMARY KEY,        -- 挂载的本地磁盘绝对路径
    created_at REAL
);

-- 6. 系统底层偏好设置表
CREATE TABLE system_settings (
    key TEXT PRIMARY KEY,         -- 配置 Key
    value TEXT                    -- 配置 Value
);
```

## 3. 系统配置加载机制
*   每次系统启动，会自动调用 `load_system_settings`。从 `system_settings` 表中提取并加载至内存：
    - `netease_cookie` -> `app_config.NETEASE_COOKIE` (保存已登录的网易云 Cookie，保持会员态)
    - `netease_download_dir` -> `app_config.NETEASE_DOWNLOAD_DIR` (网易云音乐物理下载保存的目标绝对目录)
    - `netease_api_base` -> `app_config.NETEASE_API_BASE` (本地 NCM API 服务的基础根路径)
*   **运行时偏好配置**：
    - 偏好键为 `lyrics_source_preference`，代表歌词获取优先级。默认值为 `embedded`（音频内嵌优先），若为 `network` 则代表在线多源刮削优先。调用路由处理器 `handle_get_lyrics` 时自动读取该偏好做分发决策。

## 4. SQLite 3 多线程并发规范
由于系统使用多线程架构（Watchdog 监听线程、后台刮削线程、网易云异步下载线程与 Flask HTTP 线程并发读写数据库），为确保 SQLite 3 数据一致性且避免数据库锁定，在代码构建数据库连接时必须严格遵循以下参数设计：
*   **多线程访问**：强制设定 `check_same_thread=False`，允许在不同的线程中复用同一个连接对象进行读写。
*   **锁定超时**：强制设定 `timeout=30.0`。在高并发读写（如全量扫描入库时）若数据库被独占锁锁定，其他线程的连接会自动等待最多 30 秒，极大降低了 `database is locked` 报错的发生几率。
*   **连接重试与事务**：必须结合 Python `with sqlite3.connect(...)` 的上下文管理器，执行 DDL 后必须手动或自动调用 `commit()` 确保事务原子落盘。

