# 2FMusic 后端架构、数据库与服务引擎规范 (docs/backend.md)

本文档归纳总结了 2FMusic 后端系统架构、Python 原版后端基准模型、Go 高性能重构后端实现、SQLite 3 数据库 DDL 及后台服务引擎。

---

## 1. 架构总览与底层运行规范

2FMusic 后端提供 Python 原版 (`server/`) 和 Go 高性能重构版 (`backend/`) 两种实现，两者在数据库结构、REST/WS 接口载荷及缓存文件存放路径上实现 **100% 协议无缝互替换**。

### 1.1 音频格式支持硬收敛 (AUDIO_EXTS)
系统仅允许物理扫描、索引、播放与删除以下 **6 种标准音频后缀**：
`AUDIO_EXTS = ('.mp3', '.wav', '.ogg', '.flac', '.aac', '.m4a')`

### 1.2 后端核心目录架构
系统收敛为以下核心功能目录：
- **`backend/api/`**：接口与控制层（HTTP REST 与 WebSocket `ws_hub.go`, `ws_handler.go`）；
- **`backend/core/`**：底层基础设施（运行配置 `config.go`、模型定义 `model.go` 与日志引擎 `logger.go`）；
- **`backend/db/`**：数据库持久化层（SQLite 3 驱动、表结构与持久化操作 `sqlite.go`, `song.go`, `playlist.go`）；
- **`backend/static/`**：静态托管与鉴权拦截层（前端编译产物嵌入 `static.go` 与 SHA-256 密码鉴权中间件 `auth.go`）；
- **`backend/scanner/`**：曲库引擎（增量扫描 `scanner.go`、Watchdog 防抖监听 `watcher.go`、Tag 提取 `tag.go` 与在线刮削 `searcher.go`）；
- **`backend/downloader/`**：网易云下载与容器工具箱（`downloader.go`, `netease_api.go`, `netease_auth.go`, `netease_docker.go`, `parser.go`）。

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

依据 Python 物理源码 `server/core/models/song.py` 与 Go `backend/core/model.go`：
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
- **双重刮削与算法移植机制**：
  - **批量/挂载扫描刮削 (`SearchSongFastSequential`)**：采取 `网易云` $\rightarrow$ `QQ` $\rightarrow$ `酷狗` 顺序单平台检索。遇到歌名与歌手规范化完全匹配（`isExactMatch` 且含封面或歌词）**立即短路返回**，极大节省网络请求开销、防范错配并大幅提升批量扫描速度。
  - **单曲重新刮削 (`SearchSongBest`)**：同时向 3 大平台发起并发 HTTP 请求，汇总结果并基于综合相似度计算打分选出全局最优解，保障单曲精细化刮削的高精准度。
  - **算法模型移植 (`textcompare.py` 原版对齐)**：全面移植 Python 原版的非开头括号修饰符正则剥离 (`cleanSearchText`: 自动清洗 `(Live)` / `（现场版）` 后缀)、最长公共子串模型 (`LongestCommonSubstring`) 与字符交并比模型 (`CharDuplicateRate`)，大幅提升相似度分离度与匹配精准度。
- **歌词刮削来源偏好规约 (`LyricsPreference`)**：从 `system_settings` (`key='lyrics_source_preference'`) 动态加载。日常播放与已有歌词提取时**优先直接读取本地缓存，不产生额外网络请求**。偏好设置仅在触发刮削/索引时生效：`embedded`（优先内嵌）优先解出文件自带内嵌歌词；`network`（优先网络）优先发起网络并发检索，超时或失败时退避为内嵌歌词。

### 4.3 网易云音乐下载状态机与 API 配置规范
- **模块架构物理解耦**：`downloader` 包划分为 `api.go` (HTTP 通信)、`auth.go` (扫码与身份)、`parser.go` (数据归一化)、`cookie_utils.go` (Cookie 提纯) 与 `downloader.go` (下载状态机)，高内聚低耦合。
- **API 配置与初始化**：`NeteaseAPIBase` 在未显式配置时默认为空字符串 `""`（非连接状态），前端据此展示配置引导界面。未配置 API 时调用网易云相关功能统一友好拦截并退避。
- **Cookie 规范化清洗**：所有网易云 Cookie 在保存前强制使用 `NormalizeCookieString` 过滤 `Path`, `Domain`, `Expires`, `SameSite` 等属性，仅保留标准的 `key=value`（确保 `MUSIC_U` 凭证生效）。
- **配置保存与连通性校验**：通过 `netease/save_config` 保存非空 `api_base` 时，后端使用 3 秒超时 GET 请求 `${api_base}/login/status` 进行连通性与 JSON 特征（必须包含 `code` 或 `data` 属性）校验，校验通过方可保存入 SQLite 及更新全局变量。
- **曲目数据归一化**：`resolve` / `search` / `recommend` 接口返回的曲目，统一调用 `FormatNeteaseSongs` 转译为 `{id, title, artist, album, cover, duration, is_vip, level, max_level}` 标准 `NeteaseSong` 结构。
- **状态流转与自动降级**：`pending` $\rightarrow$ `preparing` $\rightarrow$ `downloading` $\rightarrow$ `success` / `error`。若高音质无有效 URL，自动尝试退避至 `standard` 标准音质重试。
- **文件落盘、元数据内嵌与缓存隔离**：所有下载写操作在 `.cache` 临时目录下完成。音频下载的同时拉取封面与歌词，歌词格式遵循 **`yrc.lyric` (逐字歌词) > `lrc.lyric` (标准歌词)** 的优先级逻辑。将元数据（Title, Artist, Album, AlbumArtist, Cover, Lyrics）在 `.cache` 下直接内嵌写入音频文件（MP3 写 ID3v2，FLAC 写 Vorbis Comment/Picture Block），并保存备份至 `.cache/covers/` 与 `.cache/lyrics/`。
- **文件物理移入与 library_changed 主动广播**：元数据内嵌完成后，将音频文件重命名/移动（`moveFile`）至 `NetEase` 目标目录。文件落盘成功后，`updateTaskStatus` 在广播 `download_status` 的同时**主动调用 `NotifyLibraryChanged()`**，直接触发前端刷新歌曲列表（`isSongDownloaded` 即时正确标记），不依赖 Watchdog 被动感应（Watchdog 仅作为兜底）。
- **下载进度广播节流**：在下载流读取循环中，对 `download_status` 消息广播加入进度变动（`prog`）及至少 100ms 时间节流控制，避免高频广播塞满 WebSocket Send Channel（256 深度）导致连接异常中断与前端反复重连。
- **广播通道非阻塞容错**：`BroadcastJSON` 采用 `select-default` 非阻塞模式，当 Hub 广播队列满载时自动舍弃中间帧，彻底防止后台业务 Goroutine 挂起卡死。
- **网易云下载目录最高优先级与强制内嵌**：网易云下载存储目录（支持从 `system_settings` 的 `key='netease_download_dir'` 动态装载）**优先级高于全局 `LyricsPreference` 偏好设置**。凡处于该目录下的音频文件，索引与刮削时强制 100% 优先解出并激活内嵌封面与内嵌歌词（`embeddedLyrics`），且**完全阻断第三方在线网络刮削**，确保元数据绝对纯净与一致。
