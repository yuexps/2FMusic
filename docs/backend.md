# 2FMusic 后端架构、数据库与服务引擎规范 (docs/backend.md)

本文档归纳总结了 2FMusic 后端系统架构、Python 原版后端基准模型、Go 高性能重构后端实现、SQLite 3 数据库 DDL 及后台服务引擎。

---

## 1. 架构总览与底层运行规范

2FMusic 后端提供 Python 原版 (`server/`) 和 Go 高性能重构版 (`backend/`) 两种实现，两者在数据库结构、REST/WS 接口载荷及缓存文件存放路径上实现 **100% 协议无缝互替换**。

### 1.1 音频格式支持硬收敛 (AUDIO_EXTS)
系统仅允许物理扫描、索引、播放与删除以下 **6 种标准音频后缀**：
`AUDIO_EXTS = ('.mp3', '.wav', '.ogg', '.flac', '.aac', '.m4a')`

### 1.2 网络服务监听模式
- **纯 TCP 端口模式**：基于 Gin HTTP Server 监听指定端口 (如 `0.0.0.0:23237`)。
- **纯 UNIX Domain Socket 模式**：绑定物理套接字文件（权限 `0666`），供 Caddy/Nginx 反代使用。
- **并发模式**：协程中监听 Domain Socket，主线程运行 TCP 端口服务。

---

## 2. 数据库结构与持久化规范 (SQLite 3)

### 2.1 数据库引擎与 WAL 模式
- **引擎**：SQLite 3。初始化时强制执行 `PRAGMA journal_mode=WAL;` 提升高并发读写性能。
- **Go 驱动**：纯 Go 原生 CGO-free 驱动 `modernc.org/sqlite`。
- **并发与锁定**：连接池 `timeout=30.0`，`check_same_thread=False`。

### 2.2 数据库 DDL 规约
```sql
-- 1. 歌曲元数据与物理缓存状态表
CREATE TABLE IF NOT EXISTS songs (
    id TEXT PRIMARY KEY,          -- 物理文件 MD5 哈希
    path TEXT UNIQUE,             -- 音频物理绝对路径
    filename TEXT,                -- 文件名
    title TEXT,                   -- 歌曲名
    artist TEXT,                  -- 歌手
    album TEXT,                   -- 专辑
    album_artist TEXT,            -- 专辑艺术家
    mtime REAL,                   -- 物理修改时间戳
    size INTEGER,                 -- 文件字节数
    has_cover INTEGER DEFAULT 0,  -- 封面缓存状态 (物理存储: 0/1; JSON 导出: true/false)
    has_lyrics INTEGER DEFAULT 0, -- 歌词缓存状态 (物理存储: 0/1; JSON 导出: true/false)
    scrape_retry_count INTEGER DEFAULT 0 -- 在线刮削失败重试次数
);

-- 2. 自定义歌单（收藏夹）表
CREATE TABLE IF NOT EXISTS favorite_playlists (
    id TEXT PRIMARY KEY,          -- 默认包含条目 id='default', name='默认收藏夹', is_default=1
    name TEXT NOT NULL,           -- 歌单名称
    is_default INTEGER DEFAULT 0, -- 是否是默认收藏夹 (0: 否, 1: 是)
    created_at REAL               -- 创建时间戳
);

-- 3. 歌单与歌曲关联表 (多对多)
CREATE TABLE IF NOT EXISTS favorites (
    song_id TEXT,
    playlist_id TEXT,
    title TEXT DEFAULT '',
    artist TEXT DEFAULT '',
    created_at REAL,
    PRIMARY KEY (song_id, playlist_id)
);

-- 4. 播放历史纪录表
CREATE TABLE IF NOT EXISTS play_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    song_id TEXT NOT NULL,
    play_time REAL NOT NULL       -- 播放完成的时间戳
);

-- 5. 额外本地目录挂载点表
CREATE TABLE IF NOT EXISTS mount_points (
    path TEXT PRIMARY KEY,        -- 挂载的本地磁盘绝对路径
    created_at REAL
);

-- 6. 系统底层偏好设置表
CREATE TABLE IF NOT EXISTS system_settings (
    key TEXT PRIMARY KEY,         -- 配置 Key
    value TEXT                    -- 配置 Value
);
```

---

## 3. 标准歌曲字典对象 (`SongDict`)

依据 Python 物理源码 `server/core/models/song.py` 与 Go `backend/model/model.go`：
```json
{
  "id": "dced594e5a04668d31ff9abdde010ae0",           // 32 位小写 MD5 签名
  "path": "D:\\Users\\yuyue\\Music\\Track01.flac",     // 绝对路径
  "filename": "Track01.flac",                        // 文件名
  "title": "晴天",                                    // 歌名
  "artist": "周杰伦",                                 // 歌手
  "album": "叶惠美",                                  // 专辑
  "album_artist": "周杰伦",                           // 专辑艺术家
  "mtime": 1784689724.53,                             // 修改时间戳
  "size": 31457280,                                  // 文件字节数 (int)
  "has_cover": true,                                 // 封面存在标志 (bool: true/false)
  "has_lyrics": true,                                // 歌词存在标志 (bool: true/false)
  "album_art": "/api/music/covers/dced594e5a04668d31ff9abdde010ae0.webp" // 封面 Web 相对路径
}
```

---

## 4. 后台服务引擎规范 (Services Engine)

### 4.1 Watchdog 物理防抖监听
- 采用原生 `fsnotify` 物理监听音乐库目录及挂载点。
- 引入 **2.0 秒事件防抖** 与 **1.0 秒文件写入平稳性检测**。
- 对后台写操作（如网易云下载落盘、文件上传）引入 `AddWatchdogIgnorePath` 动态临时避让，防止重复被触发二次索引。

### 4.2 曲库扫描与在线刮削
- **增量扫描**：全库遍历与路径比对，仅对未变更 `mtime`/`size` 的文件复用既有 ID。扫描完成后自动物理清理 `CleanStaleSongs` 失效记录并广播 `library_changed`。
- **多源并发刮削**：并发检索在线网络源，提取文本并基于最长公共子串 (`longest_common_substring`)、字符重复率 (`str_duplicate_rate`) 与繁简转换 (`ttscn.py`) 计算组合权重相似度打分。刮削失败自动增加 `scrape_retry_count`（满 3 次退避避让）。

### 4.3 网易云音乐下载状态机
- **状态流转**：`pending` $\rightarrow$ `preparing` $\rightarrow$ `downloading` $\rightarrow$ `success` / `error`。
- **进度广播**：状态变动与进度实时推送到 WebSocket 广播通道 `download_status`。
- **文件落盘**：写入 `.part` 临时文件 $\rightarrow$ ID3/FLAC 元数据内嵌 $\rightarrow$ 计算 MD5 物理 ID $\rightarrow$ 重命名移动到挂载目录 $\rightarrow$ 自动触发 `IndexSingleFile` 入库。
