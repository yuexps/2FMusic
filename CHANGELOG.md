# 变更日志 (CHANGELOG.md)

所有针对 2FMusic 项目的技术升级、重构与规范改动均记录于此，以维持历史可追溯性。

---
## [2026-06-14]
- **视图布局与滚动一致性重构与规约统一**：
  - **规范修订 (views.md)**：新增“视图容器布局规约”，建立**双轨滚动架构**。规范了虚拟滚动页面（如本地音乐）和原生滚动页面（如系统设置）的容器高度与滚动分配，严禁硬编码物理宽度。
  - **物理页面重构**：
    - **系统设置页 (Settings.vue)**：废除了在高度差异大且有异步状态的卡片下极易计算失准的 `n-virtual-list`（虚拟滚动列表），重构为普通的 `v-for` 面板渲染；去除了容器高度与截断限制，使滚动完全由外层 `.main-scroll` 接管，彻底根治无法滚动到底部的缺陷。
    - **目录管理页 (MountManager.vue)**：同步去除了 `max-w` 宽度限制与 `flex-1 min-h-0` 高度截断，使其符合原生滚动页面的规约。
    - **共享组件样式 (components.css)**：移除了 `.main-scroll:has(...)` 对 `.settings-view` 的 `overflow: hidden !important` 强行禁滚策略，使设置页的原生主滚动条恢复生效。

- **文档审计与规范完善**：
  - **数据库规范 (db.md)**：修正偏好设置，补齐多线程环境下 SQLite 的并发运行参数（`check_same_thread`, `timeout`）限制。
  - **业务服务 (services.md)**：新增扫描并发锁机制（内存锁与过期自愈物理锁）及启动级残留 `.part` 临时文件清理的成文规范。
  - **状态管理 (stores.md)**：细化了切歌封面大图在 Pinia 中的引用计数（retain / release）释放机制。
  - **网络通信 (communication.md)**：补充 401 回退重定向路径规范，新增 WS 客户端离线请求队列 (`offlineQueue`) 与自动重连冲刷机制；补齐后端 IP 暴力破解登录限流、反代 ProxyFix 适配及 HTML 动态 ETag 强缓存禁用规范。
  - **缓存规范 (cache.md)**：将 IndexedDB 误写库名纠正为 `2fmusic_local_cache`，并补齐了 `lyrics` 歌词与 `background` 自定义背景图的对象仓库定义；新增前端自定义指令 `v-cached-src` 对大图 Blob 的 retain/release 自动引用计数与生命周期管理机制。

- **前后端逻辑架构重构与优化**：
  - **启动初始化同步**：重构 `server/app.py` 的启动流程，将数据库 `init_db()` 及 `.part` 临时文件清理 `clean_temp_part_files()` 改为网络端口拉起前的同步阻塞调用，规避了由于异步执行带来的冷启动竞态条件。
  - **CORS 支持 X-Password**：在 `server/core/__init__.py` 的 CORS `Access-Control-Allow-Headers` 中补全 `X-Password` 支持，确保外部跨域客户端的预检 OPTIONS 请求能够通过并安全携带访问密码请求头。
  - **WebSocket 重复订阅防泄漏**：重构 `frontend/src/stores/system.ts` 中的 `initWebSocket`，引入 `isWsInitialized` 状态标志，保障 WebSocket 广播订阅仅初始化注册一次，彻底解决多次进出“网易下载”页 `onMounted` 重复订阅匿名箭头函数引起的内存泄漏和数据重复更新。
