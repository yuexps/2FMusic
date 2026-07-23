# CHANGELOG - 2FMusic 变更日志

## [广播解耦隔离与文案优化] - 2026-07-23

### 挂载点文案精准化
- **缺漏刮削文案更名**：更新 `docs/api.md`；将 `MountManager.vue` 按钮更名为「补全刮削」，提示语同步更新为「补全缺失的封面与歌词」，准确匹配仅针对缺失元数据（`!has_cover || !has_lyrics`）的底层逻辑。

### 广播解耦与多端隔离
- **Android 列表底部留白**：扩充所有 `LazyColumn` 的 `contentPadding` 至 `160.dp`，解决卡片被底栏遮挡问题。
- **模块广播解耦架构**：划分 `audio/metadata`、`favorite` 与 `history` 独立频道，Go 后端按 `fields` 隔离派发，Web 与 Android 端分支订阅刷新。
- **播放历史去重与裁切**：`GetPlayHistory` 聚合最新时间并自动清理失效记录，追加 >100 条历史旧记录自动裁切逻辑。


## [前端退出登录适配] - 2026-07-23

### 前端鉴权与注销
- **修复退出登录路由**：移除 `Settings.vue` 中废弃的 `window.location.href = '/logout'` 重定向逻辑（后端已无该路由），改为调用 `systemStore.logout()`。

## [物理扫描、自适应刮削与推送广播全链路重构] - 2026-07-23

### 曲库扫描与在线刮削引擎
- **扫描架构与进度广播**：重构 `scanner.go` 为两阶段预遍历搜集并去重；引入 `forceBroadcastStatus` 强制广播，彻底修复扫描入库进度恒为 100% 及面板状态滞后问题。
- **并发刮削与连接池优化**：`httpClient` 启用 TCP Keep-Alive 与连接池复用；升级 `SearchSongFastSequential` 并发策略；依据 CPU 核心数动态开辟 `[4, 12]` Worker，刮削 Wall Time 缩短 **32.71%**（110ms/首）。
- **SingleFlight 并发去重与 10s TTL 缓存**：引入 `singleflightGroup` 协作 10s `ttlCache` 内存缓存，彻底消除同一时间段针对同一歌曲的高频重复全网 HTTP 检索；增加落盘前 `os.Stat` 互斥保护，消除重复磁盘 IO。

### 音频元数据与时长解析
- **精准时长解析**：重构 `tag_reader.go` 引入 `audioduration`（解析 <1ms），跳过 ID3v2 报头并结合 MPEG Bitrate 解析，修复高品质 MP3 (320kbps) 误算为 6分27秒 的 Bug；补全 `Filename` 元数据赋值。
- **媒体解析收敛**：封装 `EnsureSongMediaResolved` 统一封面歌词解析入口，保证 Cache 文件名与 SQLite `song.ID` 严格绑定并实现双向降级。

### 物理文件安全删除与流解封
- **文件安全删除**：前置切歌解除 HTML5 `<audio>` 独占网络锁；`SafeRemoveFile` 改为 5 次退避重试配合 `runtime.GC()` 强释句柄，确保物理秒删成功。

### 实时推送广播与多端协同
- **广播协议扩展**：`library_changed` 扩展 `event_type`、`song_ids` 与 `fields` 字段；`ws_hub.go` 增加 200ms 防抖队列；`lc_watcher.go` 支持文件覆盖检测。
- **多端缓存协同擦除**：`AlbumArt` 链接追加 `?v=<mtime>` 穿透 HTTP 强缓存；Web 端与 Android 端接收广播后即时清空物理与内存缓存。

### 跨端兼容与规范
- **密码哈希降级**：`crypto.ts` 在非 Secure Context HTTP 环境下自动回退至纯 JS UTF-8 SHA-256。
- **Android 网络与协议对齐**：引入 `probeProtocol` 动态探活算法彻底解决缺失协议导致的封面空白与播放报错；修复 WS 广播分发关键 Bug；在 `MusicApi` 与 `SqlMusicRepository` 中补齐 `album` 字段传递，实现 Android 端、Web 端与 Go 后端多源刮削引擎 100% 精确对齐。


## [Go 重构 & 修复] - 2026-07-22

### 重构 Python 后端为 Go 服务端

