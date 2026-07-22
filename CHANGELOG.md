# CHANGELOG - 2FMusic 变更日志

## [Go 重构] - 2026-07-22

### 重构 Python 后端为 Go 服务端
- **并发性能与架构升级**：采用 **Gin Web Framework** + **Gorilla WebSocket** 替换 Python Werkzeug/Flask 架构，消除 GIL 锁限制，提升高并发能力与降低内存消耗。
- **物理监听与防抖 (fsnotify Watcher)**：使用 Go 原生 `fsnotify` 库，结合 Channel 与 `time.Timer` 实现 **2.0s 物理写入防抖** 与 **1.0s 文件平稳性检测**。
- **音频 Tag 元数据与零 CGO 封面生成**：采用 `github.com/dhowden/tag` 解析 ID3v2/Vorbis/MP4 标签与嵌入封面，完美落盘。
- **多源并发刮削引擎**：Goroutine 并发调度 QQ/网易云/酷狗 API，结合确定的 Levenshtein 相似度打分算法与 0.55 匹配阈值。
- **网易云下载状态机**：完整移植 `preparing` -> `downloading` -> `success`/`error` 状态流转、音质自动降级回退、`.part` 碎片管理与 WebSocket 实时进度广播。
- **安全与防爆破拦截器**：内置 IP 登录失败计数器（1小时内满3次封禁1小时），支持 `X-Password` 与 `?auth=` 密码鉴权。
- **双路监听与平滑升级**：完美支持 `--port` (TCP Listen)、`--unix-socket` (Unix Domain Socket 0666) 双路并发绑定，与原有 SQLite 3 数据库 `2fmusic.db` 100% 兼容。
- **CI/CD 构建脚本重构**：修改 `.github/workflows/test-build.yml`，采用 `fn_build_python` (`platform=all`)、`fn_build_x86` (`platform=x86`, `${TRIM_APPDEST}/2fmusic`) 和 `fn_build_arm` (`platform=arm`, `${TRIM_APPDEST}/2fmusic`) 独立目录平行构建，分别输出对应的 3 个 FPK 安装包。
