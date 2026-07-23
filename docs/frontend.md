# 2FMusic 前端 SPA 架构与通信规范 (docs/frontend.md)

本文档定义了 2FMusic 前端单页面应用 (Vue 3 + TypeScript + Pinia + Vite) 的技术架构、网络通信拦截、Pinia 状态管理、IndexedDB 本地缓存及子路径适配规范。

---

## 1. 登录门禁拦截 (Auth Gate) 与网络通信

### 1.1 登录门禁拦截规约 (Auth Gate)
- **登录先行**：应用挂载 (`App.vue`) 时，优先检查本地存储 `localStorage.getItem('2fmusic_password')`。若无凭证，直接触发未授权阻断遮罩 `LoginModal.vue`。
- **未登录安全红线**：未检测到有效凭据时，**绝对禁止拉起 WebSocket 连接**，**绝对禁止发起私有业务数据 REST 请求**。
- **登录密码 SHA-256 兼容机制**：前端密码通过 [crypto.ts](file:///d:/Users/yuyue/Documents/Code/2FMusic/frontend/src/utils/crypto.ts) 进行哈希处理。优先使用原生 Web Crypto API (`crypto.subtle.digest`)；在非安全上下文（如局域网/公网纯 HTTP 环境，`window.crypto.subtle` 为 `undefined`）时，自动降级切至纯 JavaScript (Pure JS) UTF-8 算法进行计算，确保 HTTP/HTTPS 场景下均可正常登录。
- **登录成功一键装载**：解锁成功后，集中初始化 WebSocket 连接，并按序装载 Pinia Store 业务数据。
- **会话注销与退出登录**：
  - 前端注销必须执行闭环擦除：
    1. 擦除本地持久化凭据 `localStorage.removeItem('2fmusic_password')`；
    2. 主动断开当前 WebSocket 信道连接（`wsClient.disconnect()`）；
    3. 重置并清空所有 Pinia 内存数据及播放器状态（`systemStore.clearUserData()`、`favoritesStore.clearUserData()`、`historyStore.clearUserData()`、`playerStore.stop()`）；
    4. 触发全局 `2fmusic-unauthorized` 事件，呼出 `LoginModal.vue` 解锁遮罩阻断交互。

### 1.2 REST 请求与 401 拦截
- **客户端文件**：`frontend/src/api/client.ts`。
- **请求头注入**：在 Axios 拦截器中读取本地密码并自动注入 Header `X-Password`。
- **401 重定向**：Response 拦截器捕获 HTTP 401 时，自动清除本地无效凭据并拉起登录遮罩。

### 1.3 WebSocket 客户端 (`wsClient`) 机制
- **客户端文件**：`frontend/src/api/ws.ts`。
- **Promise-seq 映射**：`sendRequest(action, data)` 生成自增 `seq`，转存入 `pendingRequests` Map 容器中，带 15000ms 超时控制。
- **离线排队队列 (`offlineQueue`)**：网络断开或建连中发起的请求推入排队队列；连接就绪后自动冲刷队列 (`flush`)；阻断或注销时静默清空队列。
- **广播路由唯一派发**：接收无 `seq` 消息时，优先取 `payload.action`（如 `scan_status`、`library_changed`），若无 `action` 则取 `payload.type`（如 `download_status`），执行唯一单次事件派发，无任何冗余分发。
- **广播订阅与解绑红线 (Anti-Leak)**：
  > [!WARNING]
  > 在组件中通过 `wsClient.subscribe(type, callback)` 注册广播监听时，**必须在 `onUnmounted` 生命钩子中执行返回的解绑函数**，严禁内存泄露。
  > 前端**严禁开启 Timer 轮询**进行下载/扫描状态轮询，必须且只能订阅 WS 主动广播。

---

## 2. Pinia 状态管理与组件通信 (`playerStore`)

### 2.1 播放器状态树恢复
- 自动恢复 `currentSong`、`playlist`（当前播放列表）、`queue`（待播插队队列）、`playMode`（顺序/单曲循环/随机）以及 `currentTime`。
- **物理删除与播放状态解封**：
  在发起物理删除 API（`systemStore.deleteSong(id)`）前，若要删除的歌曲包含当前正在播放的曲目（`playerStore.currentSong?.id`），必须优先切至下一首（`playerStore.next()`）或优雅停止（`playerStore.stop()`），断开 `<audio>` 对后端的 HTTP 音频流独占请求，确保后端 SafeRemoveFile 毫秒级物理秒删成功。与 `<audio>` 播放器引擎保持单向数据流映射：Store 控制 Source/Play/Pause，`<audio>` 原生事件 (`timeupdate`, `ended`, `error`) 闭环更新 Store 状态。
- **底栏播放器清空与重置规约 (`clearPlaylist` & `stop`)**：
  - 在当前播放列表（`playlist`）或待播队列（`queue`）执行清空时，调用 `clearPlaylist()` 与 `clearQueue()`。
  - 列表为空或手动重置时触发 `stop()`，自动执行 `<audio>` 暂停、`audio.src = ''` 清空、歌词 `currentLyric` 清空、进度与时长归零，并同步将 `currentSong` 置为 `null` 写入 `localStorage` (`saveState`)，实现底栏播放器全状态 100% 干净重置。
- **失效歌曲自动清洗与重置规约 (`cleanInvalidSongs`)**：
  - **优先级秩序（校验清理 $\rightarrow$ 网络请求 $\rightarrow$ 异常退避）**：在发起 `togglePlay` 或 `playSong` 时，系统优先校验曲目与清理失效状态，避免盲目向后端发送 404 网络音频请求。
  - 支持 `validSongs` 为空数组 `[]`（全库清空/全量删歌场景）时的深度清洗，确保有效Set提取与全局 `playlist` / `queue` 的彻底筛选。
  - 使用 `String(id)` 强制字符串化解决 `string` 与 `number` 类型差异导致的比对遗漏。
  - **`<audio>` 元素 `error` 事件全自动退避**：当音频资源 404/500 加载失败时，自动从列表中剔除坏歌并切至下一首 `next()`，列表为空时触发 `stop()` 重置底栏。

### 2.2 多媒体会话 (MediaSession API) 适配
适配 OS 级原生多媒体控制面板：在播放切换时自动更新 `navigator.mediaSession.metadata`（曲名、歌手、专辑、封面），并绑定原生 Play/Pause/Pre/Next 物理按键与蓝牙耳机交互。

---

## 3. 本地缓存与性能优化 (IndexedDB Cache)

- **精细化广播与缓存擦除规约**：监听到 `library_changed` 精细化载荷时按 `fields` 分类隔离处理：
  - 若 `fields` 包含 `'cover'`：调用 `musicDB.deleteCover(id)` 擦除 IndexedDB 中的封面 Blob，并调用 `coverCacheManager.delete(id)` 销毁 Blob URL；列表及组件使用带修改时间戳的 URL (`album_art + '?v=' + timestamp`) 彻底穿透 HTTP 300 天强缓存。
  - 若 `fields` 包含 `'lyrics'`：调用 `musicDB.deleteLyrics(id)` 擦除 IndexedDB 歌词缓存。
  - 若 `fields` 包含 `'favorite'`：仅触发 `favoritesStore.fetchPlaylists()` 刷新歌单与收藏夹列表。
  - 若 `fields` 包含 `'history'`：仅触发 `historyStore.fetchHistory()` 重新拉取最新云端播放历史。
  - 若 `fields` 包含 `'audio'` / `'metadata'` / `'cover'` / `'lyrics'`：触发 `fetchSongs()` 更新歌曲列表。

### 3.2 子路径与 Web 资源匹配 (`getApiUrl`)
- **部署适配**：应用支持子路径反向代理部署（如 `/app/yuexps-2fmusic/`）。
- **绑定红线**：绑定 `:src` 或静态资源路径时，必须调用 `getApiUrl(path)` 或使用 `v-cached-src` 指令处理，严禁硬编码根路径 `/api/`。

---

## 4. 视图路由与单页面架构 (Views & Routing)

- **主界面架构**：单一 Vue 页面主框架，配合浮层全屏播放器 overlay (`FullPlayerOverlay.vue`) 与设置弹窗 (`Settings.vue`)。在全屏播放器中，左侧元数据区域收敛为仅展示歌曲标题 (`title`) 与歌手名 (`artist`)，隐藏专辑名展示以保持视效精简与纯粹；同时歌名解耦固定宽度限制，支持长歌名最多 2 行自然折行 (`line-clamp-2`) 并降低一档字号以优化视觉层级。
- **路由防缓存 (ETag & Cache-Control)**：后端 HTML 页面强制写入 `Cache-Control: no-cache, no-store, must-revalidate`，确保前端改动构建后浏览器能 100% 刷入最新资源包。

---

## 5. UI 动效与微交互规范 (Animation & Micro-interactions)

- **页面路由切替动画**：主工作区 `<router-view>` 必须使用 Vue 3 原生 `<Transition name="page-fade" mode="out-in">` 包裹，确保切页流畅、不发僵。
- **列表/网格自动重排 (`v-auto-animate`)**：所有自定义渲染的 `v-for` 容器（如曲库卡片网格、收藏歌单列表、下载任务及挂载点列表）统一挂载 `@formkit/auto-animate` 指令，实现重排、增删时的平滑物理动画。
- **侧边栏折叠动画**：进度条等动态增隐元素必须使用 Naive UI `<n-collapse-transition>` 包裹，提供平滑的高度收展过渡。
