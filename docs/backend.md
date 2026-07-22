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
- **`backend/scanner/`**：曲库引擎（增量扫描 `scanner.go`、3 Watcher 闭环体系 `watcher.go`, `audio_watcher.go`, `db_watcher.go`, `lc_watcher.go`、Tag 提取 `tag.go` 与在线刮削 `searcher.go`）；
- **`backend/downloader/`**：网易云下载与容器工具箱（`downloader.go`, `netease_api.go`, `netease_auth.go`, `netease_docker.go`, `parser.go`）。

### 2.4 在线刮削与高精度打分引擎 (`backend/scanner/searcher.go`)
- **全平台轻量覆盖（与 Python 原版 100% 对齐）**：
  - **网易云 (`netease`)**：取 `al.picUrl` 高清封面，调用 `/api/song/lyric` 拿 `lrc` 与 `tlyric` 交错融合；
  - **QQ 音乐 (`qq`)**：取 `albummid` 拼接腾讯 CDN 封面，调用 `fcg_query_lyric_new.fcg` 直接解码 Base64 提取纯文本歌词与翻译（零 3DES 加密）；
  - **酷狗音乐 (`kugou`)**：取搜歌响应 `Image` 并替换 `{size}` 为 `400` 获得高清封面，调用 `lyrics.kugou.com/download` 参数指定 `fmt=lrc` 直接 Base64 解码纯文本歌词（零 XOR 异或与 Flate 解压）。
- **融合打分与竞态保护**：
  - **算法融汇**：结合 2FMusic 经典 `LongestCommonSubstring` (最长公共子串) + `CharDuplicateRate` (字符交并比) 与 `normalizeLyricMatchText` (音乐变体/后缀清洗)、`calculateArtistMatchSimilarity` (多歌手切割与主歌手保护) 及 `calculateDurationMultiplier` (1s/3s/5s/10s 阶梯式时长降权)；
  - **置信度防护**：设立 0.75 置信度门槛，防范错版 DJ/Live 误匹配；
  - **并发安全**：以 `mu.Lock()` + `copy(resultsCopy, results)` 浅拷贝锁防护，彻底根治 6 秒超时触发后的 Slice Data Race。

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
-- 规范：配置修改时触发 SaveSystemSetting 必须记录结构化控制台日志，Cookie 等敏感字段实施前缀脱敏打印。
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

### 4.1 曲库后台监听架构
系统包含三个后台监听器：
1. **音频监听器 (`Audio_Watcher`)**：
   - 监听路径：`audios/`（音频基础路径）、`NetEase/`（网易云下载目录）及动态挂载点。
   - 职责：处理音频文件的增删。监听到音频新增时提取基础 Tag（ID, Title, Artist, Album 等），去重加入数据库索引（默认 `has_cover=0, has_lyrics=0`），并发送主键 Insert 事件；监听到音频文件删除时删除数据库记录并发送主键 Delete 事件。
2. **数据库监听器 (`DB_Watcher`)**：
   - 监听触发：Go 进程内数据库钩子（监听 `songs` 表主键 `id` 的 Insert / Delete）。
   - 职责：收到 Insert 事件时，解出内嵌封面/歌词或发起在线刮削落地至 `.cache`，转码压缩 WebP 移入 `covers/<md5>.webp` 与 `lyrics/<md5>.lrc`；收到 Delete 事件时，删除对应的 `covers/<md5>.webp` 及 `lyrics/<md5>.lrc` 文件。在提取/刮削处理完成后，若检测到该曲目最终未产生任何封面与歌词物理文件落盘（或被删除曲目此前无任何缓存文件被移除），显式触发 `NotifyLibraryChanged()` 兜底通知前端广播，确保无封面与歌词的纯音频文件增删时前端视图实时同步。
3. **媒体缓存监听器 (`LC_Watcher`)**：
   - 监听路径：`covers/` 与 `lyrics/` 缓存目录。
   - 职责：监听到 `covers/` 或 `lyrics/` 产生文件新增时，根据文件名（即 MD5 ID）更新数据库 `UPDATE songs SET has_cover=1` 或 `has_lyrics=1`；文件删除时更新为 `0`。更新完成后广播 `library_changed` 通知前端刷新。

### 4.1.1 缓存隔离与原子落盘
- 音频上传、网易云下载及封面/歌词解出刮削均在 `.cache` 临时目录下完成。
- 所有 Watcher 显式屏蔽忽略 `.cache` 目录。
- 完全落盘后再使用同盘 `os.Rename` 移动至目标目录，确保 Watcher 捕获事件时文件内容完全闭合。

### 4.2 曲库扫描与在线刮削
- **增量扫描**：全库遍历与路径比对，仅对未变更 `mtime`/`size` 的文件复用既有 ID。扫描完成后自动清理 `CleanStaleSongs` 失效记录并广播 `library_changed`。
- **双重刮削机制与合并请求**：
  - **批量/挂载扫描刮削 (`SearchSongFastSequential`)**：采取 `网易云` $\rightarrow$ `QQ` $\rightarrow$ `酷狗` 顺序单平台检索，完全匹配时立即短路返回以提升扫描效率。
  - **单曲精细刮削 (`SearchSongBest`)**：并发向 3 大平台发起请求，基于综合相似度打分选出最优结果。当曲目同时缺失封面与歌词时，在线刮削收敛为单次 `SearchSongBest` 请求并发拉取，同时提取 `cover` 与 `lyrics`。
  - **重试防护机制 (`scrape_retry_count`)**：在线刮削无有效匹配时，自动递增 `scrape_retry_count`。在全库扫描与启动兜底扫描中，凡 `scrape_retry_count >= 3` 的曲目自动忽略在线刮削，避免无意义的频繁全网请求。
  - **歌词与双语翻译解析**：支持网易云等平台原生的标准歌词与双语翻译解析；统一使用交错时间轴拼接并写盘为 `lyrics/<id>.lrc`。对于包含翻译的匹配项给予额外打分加成。
- **封面图片保存与 WebP 转换 (`SaveCoverWebP`)**：当长边超过 500px 时按最长边等比例平滑缩小，编码为 WebP 格式（`Quality: 80`）落盘；若解码/编码发生异常则退避使用原图字节。
- **歌词刮削来源偏好 (`LyricsPreference`)**：从 `system_settings` (`key='lyrics_source_preference'`) 动态加载。日常播放优先读取本地缓存，仅在触发刮削/索引时依据 `embedded`（优先内嵌）或 `network`（优先网络）策略生效。

### 4.3 网易云音乐下载状态机与 API 配置规范
- **模块架构解耦**：`downloader` 包划分为 `api.go` (HTTP 通信)、`auth.go` (扫码与身份)、`parser.go` (数据归一化)、`cookie_utils.go` (Cookie 提纯) 与 `downloader.go` (下载状态机)。
- **API 配置与初始化**：`NeteaseAPIBase` 在未显式配置时默认为空字符串 `""`（非连接状态），前端据此展示配置引导界面。未配置 API 时调用网易云相关功能友好拦截并退避。
- **Cookie 规范化清洗**：所有网易云 Cookie 在保存前使用 `NormalizeCookieString` 过滤 `Path`, `Domain`, `Expires`, `SameSite` 等属性，保留标准的 `key=value`（确保 `MUSIC_U` 凭证生效）。
- **配置保存与连通性校验**：通过 `netease/save_config` 保存非空 `api_base` 时，后端使用 3 秒超时 GET 请求 `${api_base}/login/status` 进行连通性与 JSON 特征（包含 `code` 或 `data` 属性）校验，校验通过保存入 SQLite 及更新全局变量。
- **曲目数据归一化**：`resolve` / `search` / `recommend` 接口返回的曲目，统一调用 `FormatNeteaseSongs` 转译为 `{id, title, artist, album, cover, duration, is_vip, level, max_level}` 标准 `NeteaseSong` 结构。
- **曲目 ID 序列化与格式转换**：网易云 ID 解析与序列化使用 `stringifyID` 处理。避免 JSON 解包 `float64` 在数额 $\ge 10^7$ 时因 `fmt.Sprintf("%v")` 格式化为科学计数法（如 `2.8830337e+07`）导致网易云下载 API `/song/url/v1` 请求失败。`stringifyID` 及 `extractString` 均支持科学计数法字符串解析并还原为整数字符串。
- **状态流转与自动降级**：`pending` $\rightarrow$ `preparing` $\rightarrow$ `downloading` $\rightarrow$ `success` / `error`。若高音质无有效 URL，自动尝试退避至 `standard` 标准音质重试。
- **文件名清洗与 SafeMove 落盘**：生成下载目标文件名时使用 `SanitizeFilename` 替换 `/ \ : * ? " < > |` 等操作系统非法路径字符（避免多歌手斜杠破坏 Windows 路径），并在落盘前确保目标目录存在 (`os.MkdirAll`)。移动至存储目录使用 `SafeMoveFile`，当跨磁盘驱动器（如 C 盘至 D 盘）时自动退避为复制与删除。
- **文件落盘、元数据增量内嵌与缓存隔离**：所有下载写操作在 `.cache` 临时目录下完成。音频下载的同时拉取封面与歌词，歌词格式遵循 **`yrc.lyric` (逐字歌词) > `lrc.lyric` (标准歌词)** 的优先级逻辑。Tag 内嵌遵循增量原则：主要写入封面（MP3 APIC / FLAC Picture）与歌词（MP3 USLT / FLAC LYRICS），保留音频文件原有的网易云官方文本标签（Title, Artist, Album），仅在文件原有标签缺失时才做补全，禁止过滤擦除原有的 VorbisComment 或 ID3 文本帧。对于 MP3 遇偶发奇数 UTF-16 字节解码报错时，自动退避为 `id3v2.Open` 提取 `TIT2`, `TPE1`, `TALB`, `APIC`, `USLT` 帧。
- **文件移入与 library_changed 主动广播**：元数据内嵌完成后，将音频文件重命名/移动至目标目录。落盘成功后，`updateTaskStatus` 在广播 `download_status` 的同时调用 `NotifyLibraryChanged()` 通知前端刷新歌曲列表。
- **下载进度广播节流**：在下载流读取循环中，对 `download_status` 消息广播加入进度变动（`prog`）及至少 100ms 时间节流控制，避免高频广播导致连接拥塞。
- **广播通道非阻塞容错**：`BroadcastJSON` 采用 `select-default` 非阻塞模式，当 Hub 广播队列满载时舍弃中间帧。
- **网易云下载目录优先级与内嵌解析**：网易云下载存储目录（支持从 `system_settings` 的 `key='netease_download_dir'` 动态装载）优先级高于全局 `LyricsPreference` 偏好设置。处于该目录下的音频文件，索引与刮削时优先解出内嵌封面与内嵌歌词（`embeddedLyrics`），且跳过第三方在线网络刮削，保持元数据一致。当本地缓存缺失或触发重新刮削时，优先自动从音频物理文件 Tag 中重新解包落盘，保障元数据不丢失。
