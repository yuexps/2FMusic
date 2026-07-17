# 文档索引 (docs/INDEX.md)

*   [AGENTS.md](../AGENTS.md) - 全局 Agent 开发习惯、红线及检索控制。
*   **后端子模块 (Back-end Submodules)**：
    - [db.md](./server/db.md) - SQLite 表结构 DDL、主外键及配置持久化。
    - [services.md](./server/services.md) - Watchdog 2秒防抖监听、元数据扫描、多源刮削及下载器回退状态机。
    - [communication.md](./server/communication.md) - 多路监听启动分发、鉴权拦截器及 WebSocket 请求应答/心跳帧数据格式。
*   **前端子模块 (Front-end Submodules)**：
    - [stores.md](./frontend/stores.md) - Pinia player.ts 本地恢复、Audio事件映射、播放模式及 Queue 插播队列。
    - [communication.md](./frontend/communication.md) - Axios 鉴权拦截、wsClient 异步 Promise-seq 校验及广播订阅解绑红线。
    - [cache.md](./frontend/cache.md) - IndexedDB 本地封面缓存与 Blob URL 物理引用计数计数生命周期管理。
    - [views.md](./frontend/views.md) - 多入口路由判定、MediaSession 原生多媒体系统滚动歌词及按键拦截。
*   **Android子模块 (Android Submodules)**：
    - [lyrics.md](./android/lyrics.md) - 高精度歌词解析判定、绝对/相对时间自适应与垃圾元数据丢弃。
*   [API_DOCUMENTATION.md](./API_DOCUMENTATION.md) - HTTP REST API 及 WebSocket 业务 Action 载荷参数清单。
*   [folia.md](./folia.md) - 2FMusic 与 Folia-major 深度集成协议、多源在线歌词匹配及键盘与 UI 穿透设定指南。
