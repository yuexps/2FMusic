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
- **`backend/scanner/`**：曲库引擎（增量扫描 `scanner.go`、3 Watcher 闭环体系 `watcher.go`, `audio_watcher.go`, `db_watcher.go`, `lc_watcher.go`、Tag 提取及多格式 (FLAC/MP3/WAV/OGG/M4A/AAC) `audioduration` 开箱即用精准时长计算 `tag.go`, `tag_reader.go` [完美解析 VBR/Xing 帧头并提取 ID3v2/FLAC 标签与内嵌封面歌词]、在线刮削调度 `searcher.go`、打分与清洗引擎 `searcher_score.go` 及 Provider 接口 `searcher_provider.go`）；
- **`backend/downloader/`**：网易云下载与容器工具箱（`downloader.go`, `netease_api.go`, `netease_auth.go`, `netease_docker.go`, `parser.go`）。

### 2.4 在线刮削调度与高精度打分引擎 (`searcher.go` / `searcher_score.go`)
- **全平台轻量覆盖（与 Python 原版 100% 对齐）**：
  - **网易云 (`netease`)**：取 `al.picUrl` 高清封面，调用 `/api/song/lyric` 拿 `lrc` 与 `tlyric` 交错融合；
  - **酷狗音乐 (`kugou`)**：取搜歌响应 `Image` 并替换 `{size}` 为 `400` 获得高清封面，调用 `lyrics.kugou.com/download` 参数指定 `fmt=lrc` 直接 Base64 解码纯文本歌词（零 XOR 异或与 Flate 解压）。
- **集中化媒体处理引擎 (`EnsureSongMediaResolved`)**：
  - **单一高内聚入口**：将全量扫描 (`scanner.go`)、后台轮询 (`db_watcher.go`) 与文件监控 (`audio_watcher.go`) 的封面与歌词处理完全收敛至单一入口函数；
  - **严格主键后置契约**：必须且绝对在歌曲记录成功完成 `db.SaveSong` 主键入库并生成 `song.ID` 后调起，确保 Cache 文件名 (`cache/covers/<song.ID>.webp` / `cache/lyrics/<song.ID>.lrc`) 与 SQLite 主键 100% 绑定；
  - **双阶段降级保底**：阶段 1 集中提取物理内嵌/同级封面与内嵌歌词；阶段 2 仅在缺位时无缝下钻网络在线刮削补全，实现物理与网络资源的双向保底。
- **置信度防护**：设立 0.75 置信度门槛，防范错版 DJ/Live 误匹配；
- **并发安全、请求去重与 10s TTL 缓存 (SingleFlight + TTL Cache)**：内置 `singleflightGroup` 与 10 秒短生存期 `ttlCache`。当同一瞬间或短时间（10s）内针对相同歌曲（`title + artist + album + duration`）发起多次刮削请求时（如并发或依次请求歌词与封面），仅首个请求执行真实网络 HTTP 检索，后续请求复用相同检索结果，彻底消除重复的第三方 API 网络开销。
- **物理文件落盘前防重校验**：在 `GetOrScrapeLyrics` 与 `GetOrScrapeCover` 准备将检索结果写入磁盘前，先进行 `os.Stat` 文件存在性校验。若前序协程已成功落盘，后续并发协程直接返回，避免重复磁盘 IO 覆盖与重复日志输出。

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
    duration_ms INTEGER DEFAULT 0,-- 音频播放时长 (毫秒)
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
  "filename": "Track01.flac",                        // 文件名 (由 ExtractAudioMetadata 中 filepath.Base(path) 填充)
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
   - 职责：监听到 `covers/` 或 `lyrics/` 产生文件新增、修改或删除时（绝不漏报已有文件的覆盖更新），根据文件名（即 MD5 ID）更新数据库 `has_cover` 或 `has_lyrics` 状态，并调用 `NotifySongChangedDebounced` 将变更推入 200ms 防抖批处理队列，聚合下发精细化广播。
4. **WebSocket 广播防抖与聚合引擎 (`NotifySongChangedDebounced` / `NotifyLibraryChangedDetailed`)**：
   - 防抖机制：在 200ms 窗口内收集变动的 `song_ids` 与 `fields` 集中去重后一次性推送精细化 `library_changed`。若单批次变动数量 > 50，自动合并降级为 `event_type: "reload_all"` 全量刷新通知，防止通信满载抛帧。
   - 广播分类与触发规约：
     - 曲库/增量扫描/标签变动：调用 `NotifyLibraryChangedDetailed` 推送 `fields: ["audio"]` / `["cover"]` / `["lyrics"]` / `["metadata"]`。
     - 收藏夹动作 (`favorite/*`)：写入 SQLite 成功后调用 `NotifyLibraryChangedDetailed` 推送 `fields: ["favorite"]`。
     - 播放历史动作 (`history/*`)：写入 SQLite 成功后调用 `NotifyLibraryChangedDetailed` 推送 `fields: ["history"]`。

### 4.1.1 缓存隔离与 SafeRemoveFile
- **音频上传与在线刮削隔离**：音频上传、网易云下载及封面/歌词解出刮削均在 `.cache` 临时目录下完成。所有 Watcher 显式屏蔽忽略 `.cache` 目录。完全落盘后再使用同盘 `os.Rename` 移动至目标目录，确保 Watcher 捕获事件时文件内容完全闭合。
- **轻量安全文件删除引擎 (`SafeRemoveFile`)**：针对 Windows 环境下文件正被读写句柄短时占用导致 `os.Remove` 失败的问题，实施 5 次轻量重试与 `runtime.GC()` 句柄释放；若重试后依然发生强独占，直接显式返回可读错误提示，不隐瞒物理删除状态，代码极致直白纯粹。

### 4.2 曲库扫描与在线刮削
- **两阶段物理扫描与精准进度广播 (`ScanDirectoryInternal`)**：
  - **阶段一（曲目预收集）**：物理扫描在处理文件前，必须先遍历扫描目录 `scanDirs`，收集并去重待扫描音频文件列表 `audioFiles`，准确计算曲目总数 `totalFilesVal = len(audioFiles)` 并重置 `processedFilesVal = 0`。
  - **阶段二（顺序索引）**：按列表逐一调用 `IndexSingleFile` 建立索引，每完成一曲增加 `processedFilesVal`，确保进度的百分比和 `processed / total` 计数绝对精准。
- **状态转换强制广播 (`forceBroadcastStatus`)**：
  - 在扫描与在线刮削任务的启动、结束等关键状态转换节点，必须调用 `forceBroadcastStatus()` 绕过常规的 200ms 防抖间隔，确保 WebSocket 客户端能够及时接收到 `scanning` / `is_scraping` 的布尔值切换与隐藏/显示指令。
- **增量扫描**：全库遍历与路径比对，仅对未变更 `mtime`/`size` 的文件复用既有 ID。扫描完成后自动清理 `CleanStaleSongs` 失效记录并广播 `library_changed`。
- **双重刮削机制与合并请求**：两套算法均采用 `(title, artist, album string, durationMs int)` 4 个强类型必传参数，统一底层匹配打分引擎。
  - **批量/挂载扫描刮削 (`SearchSongFastSequential`)**：采取 `网易云` $\rightarrow$ `QQ` $\rightarrow$ `酷狗` 顺序单平台检索。先在单平台内全量打分选出冠军，若冠军高质（精确定位且带翻译，或综合评分 $\ge 0.85$ 且具备完整封面/歌词）直接短路返回，兼顾吞吐效率与精准防误配。
  - **单曲精细刮削 (`SearchSongBest`)**：并发向 3 大平台发起请求（6s超时防护），基于综合相似度打分与三级硬门槛（$\ge 0.75$ / $\ge 0.70$ / $\ge 0.65$）选出最优结果。当曲目同时缺失封面与歌词时，收敛为单次请求并发拉取。
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
- **网易云下载目录优先级与内嵌解析降级补偿**：网易云下载存储目录（支持从 `system_settings` 的 `key='netease_download_dir'` 动态装载）优先级高于全局 `LyricsPreference` 偏好设置。处于该目录下的音频文件，索引与刮削时**优先解出内嵌封面与内嵌歌词（`embeddedLyrics`）**。当内嵌数据缺失或为空时，**系统自动下钻降级至第三方在线网络刮削 (`SearchSongBest`)** 进行元数据补偿落盘，彻底解决因网易云下载文件缺少内嵌 Tag 导致的前端 404 与刮削永久失效问题。
