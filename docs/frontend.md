# 2FMusic 前端 SPA 架构与通信规范 (docs/frontend.md)

本文档定义了 2FMusic 前端单页面应用 (Vue 3 + TypeScript + Pinia + Vite) 的技术架构、网络通信拦截、Pinia 状态管理、IndexedDB 本地缓存及子路径适配规范。

---

## 1. 登录门禁拦截 (Auth Gate) 与网络通信

### 1.1 登录门禁拦截规约 (Auth Gate)
- **登录先行**：应用挂载 (`App.vue`) 时，优先检查本地存储 `localStorage.getItem('2fmusic_password')`。若无凭证，直接触发未授权阻断遮罩 `LoginModal.vue`。
- **未登录安全红线**：未检测到有效凭据时，**绝对禁止拉起 WebSocket 连接**，**绝对禁止发起私有业务数据 REST 请求**。
- **登录成功一键装载**：解锁成功后，集中初始化 WebSocket 连接，并按序装载 Pinia Store 业务数据。

### 1.2 REST 请求与 401 拦截
- **客户端文件**：`frontend/src/api/client.ts`。
- **请求头注入**：在 Axios 拦截器中读取本地密码并自动注入 Header `X-Password`。
- **401 重定向**：Response 拦截器捕获 HTTP 401 时，自动清除本地无效凭据并拉起登录遮罩。

### 1.3 WebSocket 客户端 (`wsClient`) 机制
- **客户端文件**：`frontend/src/api/ws.ts`。
- **Promise-seq 映射**：`sendRequest(action, data)` 生成自增 `seq`，转存入 `pendingRequests` Map 容器中，带 15000ms 超时控制。
- **离线排队队列 (`offlineQueue`)**：网络断开或建连中发起的请求推入排队队列；连接就绪后自动冲刷队列 (`flush`)；阻断或注销时静默清空队列。
- **广播订阅与解绑红线 (Anti-Leak)**：
  > [!WARNING]
  > 在组件中通过 `wsClient.subscribe(type, callback)` 注册广播监听时，**必须在 `onUnmounted` 生命钩子中执行返回的解绑函数**，严禁内存泄露。
  > 前端**严禁开启 Timer 轮询**进行下载/扫描状态轮询，必须且只能订阅 WS 主动广播。

---

## 2. Pinia 状态管理与组件通信 (`playerStore`)

### 2.1 播放器状态树恢复
- 恢复 `currentSong`、`playlist`、`playMode`（顺序/单曲循环/随机）以及 `currentTime`。
- 与 `<audio>` 播放器引擎保持单向数据流映射：Store 控制 Source/Play/Pause，`<audio>` 原生事件 (`timeupdate`, `ended`, `error`) 闭环更新 Store 状态。

### 2.2 多媒体会话 (MediaSession API) 适配
适配 OS 级原生多媒体控制面板：在播放切换时自动更新 `navigator.mediaSession.metadata`（曲名、歌手、专辑、封面），并绑定原生 Play/Pause/Pre/Next 物理按键与蓝牙耳机交互。

---

## 3. 本地缓存与性能优化 (IndexedDB Cache)

### 3.1 封面大图与 Blob URL 物理引用计数
- 采用 **IndexedDB** 在本地离线持久化存储音频 WebP 封面。
- **Blob URL 生命周期控制**：在创建 `URL.createObjectURL(blob)` 时记录引用计数；在 DOM 销毁或封面切替时，严格执行 `URL.revokeObjectURL(url)` 释放内存，防止内存泄露。

### 3.2 子路径与 Web 资源匹配 (`getApiUrl`)
- **部署适配**：应用支持子路径反向代理部署（如 `/app/yuexps-2fmusic/`）。
- **绑定红线**：绑定 `:src` 或静态资源路径时，必须调用 `getApiUrl(path)` 或使用 `v-cached-src` 指令处理，严禁硬编码根路径 `/api/`。

---

## 4. 视图路由与单页面架构 (Views & Routing)

- **主界面架构**：单一 Vue 页面主框架，配合浮层全屏播放器 overlay (`FullPlayerOverlay.vue`) 与设置弹窗 (`Settings.vue`)。
- **路由防缓存 (ETag & Cache-Control)**：后端 HTML 页面强制写入 `Cache-Control: no-cache, no-store, must-revalidate`，确保前端改动构建后浏览器能 100% 刷入最新资源包。
