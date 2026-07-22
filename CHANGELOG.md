# CHANGELOG - 2FMusic 变更日志

## [Go 重构 & 修复] - 2026-07-22

### 修复
- **双语翻译歌词解析与刮削打分加权 (`backend/scanner/searcher.go`, `docs/backend.md`)**：对齐 Python 版元数据刮削算法，在网易云等在线接口中同步提取 `tlyric` 翻译歌词，并实现按时间戳升序交错合并算法生成标准双语 LRC 歌词；在 `SearchSongBest` 与 `SearchSongFastSequential` 打分引擎中，对含有翻译的匹配项给予额外 `+0.02` 权重加分，并输出 `has_translation` 标记。
- **配置修改控制台日志增强 (`backend/db/sqlite.go`, `docs/backend.md`)**：在 `SaveSystemSetting` 中新增人性化的结构化控制台日志输出 (`core.Info`)，包含配置项中文名转换（如 `歌词来源偏好`、`网易云 API 地址` 等）与人性化取值格式，并对 `netease_cookie` 等敏感字段实施前缀脱敏打印，提升配置变动日志的可读性与追溯能力。
- **无封面/歌词纯音频文件增删广播遗漏修复 (`backend/scanner/db_watcher.go`, `docs/backend.md`)**：在 `DB_Watcher` 中补全兜底广播逻辑。当新增或删除无内嵌封面、无内嵌歌词且网络无匹配的纯音频文件时（因未产生或清除物理缓存文件导致 `LC_Watcher` 无法响应），在 `DB_Watcher` 处理尾声显式触发 `NotifyLibraryChanged()`，确保全场景 0 闪烁、0 遗漏地实时通知前端刷新视图。
- **网易云每日推荐与歌单曲目下载修复 (`backend/downloader/parser.go`, `ws_handler.go`, `downloader.go`, `utils/`)**：新增 `stringifyID` 避免 JSON 浮点数解析在 $ID \ge 10^7$ 时转换为科学计数法导致下载接口报错；规范下载文件命名格式为 `歌名 - 歌手` (`Title - Artist`)，并使用 `SanitizeFilename` 替换斜杠等非法字符（生成如 `I LOVE U - 阿良良木健, 洛天依Official.mp3`）；重构 Tag 写入策略为增量内嵌，保护音频文件原有的网易云官方文本标签；下载状态机增加专辑名抓取；实现 `SafeMoveFile` 支持跨磁盘驱动器移动文件。
- **文档与日志文本去浮夸化 (`docs/backend.md` & `backend/`)**：彻底清洗文档与日志中过度装腔作势的名词（如“3 Watcher 闭环响应式曲库架构”、“物理”前缀等），还原为“曲库后台监听架构”、“音频监听”等平实清晰的表述。
- **封面图片转换与渲染优化 (`backend/scanner/tag.go`, `docs/backend.md`, `frontend/src/`)**：后端采用长边最高 500px 零裁切等比例平滑放缩，并统一编码为高效率 WebP 格式（`Quality: 75`）落盘，发生异常时退避使用原图字节；前端同步采用 `object-contain` 配合衬底渲染，保障非 1:1 封面无损展示。
- **网易云下载界面已下载状态判定增强 (`frontend/src/views/NeteaseDownloader.vue`, `docs/frontend.md`)**：升级 `isSongDownloaded` 匹配机制，从原本的“歌名 + 歌手”组合比对扩展为“歌名 + 歌手 + 专辑名”三元组匹配，提高同名不同专辑歌曲已下载标记的精准度。
- **后端 DB 字段补全 (`backend/db/song.go`)**：在 `GetAllSongs`、`GetSongByID` 与 `GetSongByPath` 查询响应中补全 `s.AlbumArt` 的 `/api/music/covers/<id>.webp` 相对路径生成逻辑，解决手动上传歌曲或扫描新增时，音乐列表中歌曲 `album_art` 为空导致封面显示为占位图片的 Bug。

### 重构 Python 后端为 Go 服务端

