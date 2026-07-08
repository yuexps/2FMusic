# 前端通信与事件总线契约 (docs/frontend/communication.md)

## 1. REST 请求与拦截鉴权
*   **客户端文件**：[frontend/src/api/client.ts](../../frontend/src/api/client.ts)。
*   **前缀处理**：自动载入 `getBaseUrl()` 作为 Axios 的 `baseURL`。
*   **401 拦截处理**：
    - 在 Response 拦截器中，捕获 `status === 401`。
    - 动态生成当前页面的完整回退路径：`window.location.pathname + window.location.search + window.location.hash`。
    - 对其进行 `encodeURIComponent`，重定向跳转至：
      `${getBaseUrl()}/login?next=${nextPath}`

## 2. WebSocket 异步帧交换 (`wsClient`)
*   **客户端文件**：[frontend/src/api/ws.ts](../../frontend/src/api/ws.ts)。
*   **异步 Promise-seq 发送机制**：
    - 调用 `wsClient.sendRequest(action, data)` 返回 Promise。
    - 内部维护自增序列号 `seq`，拼装帧发送。
    - 在本地 `pendingRequests`（Map 容器）中注册，将该 `seq` 作为 Key，其 Promise 的 `resolve`, `reject` 以及 15000ms（15 秒）超时定时器包装存入。
    - 当 `onmessage` 收到响应帧匹配到对应的 `seq` 时，清除超时定时器，比对 `success` 决定 resolve 数据或 reject 报错，并在 Map 中物理移除该记录。
*   **离线请求排队队列 (`offlineQueue`)**：
    - **原理机制**：若发起 WS 请求时发现连接尚未建立（`this.ws` 为空或处于非 `OPEN` 状态），系统**严禁直接丢弃或报错**。
    - **暂存行为**：自动将该请求包装并推入 `offlineQueue` 数组，并同时开启 15 秒超时器以保证不会发生无休止的排队悬挂。若排队时发现尚未启动连接，应自动拉起 `connect()` 重新尝试连接握手。
    - **队列冲刷 (`flush`)**：当连接建立并在 `onopen` 阶段就绪时，提取 `offlineQueue` 中的所有未超时请求，批量执行 WS 发送并转存入 `pendingRequests`。这对于车载、手机端在网络不稳定重连后的顺畅通信恢复极其关键。

## 3. WebSocket 广播事件总线 (Event Bus)

### 3.1 广播事件列表
当 WebSocket 收到无 `seq` 属性且带 `type` 广播标识的帧时，自动在本地分发广播。
1.  **`library_changed`**：
    - 负载：`{"library_version": float}`
    - 行为：比对本地缓存的版本，如果不一致，则在合适时机（切页或静默）调用 `music/get_list` 刷新前端缓存。
2.  **`scan_status`**：文件增量扫描进度。
3.  **`download_status`**：网易云异步下载任务进度更新。
4.  **`netease_login_status`**：网易云二维码扫码状态。

### 3.2 订阅与解绑规范
*   **订阅方法**：使用 `const unsubscribe = wsClient.subscribe(type, callback)` 注册广播监听，函数会返回一个解绑函数。
*   **解绑红线 (Anti-Leak)**：
    > [!WARNING]
    > 为防止多次进出组件导致回调被反复重复注册产生内存泄露，**必须在组件生命周期注销钩子中执行解绑**：
    ```typescript
    onUnmounted(() => {
      unsubscribe()
    })
    ```
*   **禁止主动轮询**：前端对于下载进度、物理扫描、扫码状态等，**严禁开启任何定时器进行 REST 请求轮询**。必须且只能订阅 WS 主动广播。

## 4. 子路径部署与 API 路径适配 (Base URL 规约)
*   **适配背景**：当前端应用被部署在特定的子路径下（例如 `/app/yuexps-2fmusic/`）时，所有非绝对地址的后端 API 和静态资源（如封面、播放音频、背景图等以 `/api/` 或相对路径开头的资源）的请求路径也必须带有该子路径前缀。
*   **代码规范**：
    - 在模板中绑定可能包含 `/api/` 相对路径的资源到 DOM 属性（如 `<img>` 的 `:src`）时，必须使用 `getApiUrl()` 函数包裹，或者使用已封装该处理的自定义指令（如 `v-cached-src`）。
    - 严禁直接使用 `:src="item.cover"` 或 `:src="item.album_art"` 来绑定以 `/api/` 开头的相对路径，必须转换为 `:src="getApiUrl(item.cover)"`。
    - 外部的绝对 URL（包含 `http://` 或 `https://`）以及 Blob URL、Data URL 传入 `getApiUrl()` 时会自动原样返回，无需做额外条件判定。

## 5. 接口异常与非 JSON 响应容错规范
*   **设计原则**：对于所有由前端直接发起或通过后端代理转发的 REST API 请求（如网易云 API），前端网络库及数据接口方法**必须具备非 JSON 响应（如 502 Bad Gateway/504 Gateway Timeout 等返回的 HTML 页面）和网络错误的捕获及容错解析能力**。
*   **健壮性要求**：
    1.  解析响应时，需先判断响应头中的 `Content-Type` 是否包含 `application/json`；或在进行 `res.json()` 解析时使用 `try-catch` 包裹。
    2.  若响应类型非 JSON，或解析过程中抛出异常，必须平滑捕获并将返回格式统一转换为结构化的 `{ code, message }` 错误对象（例如网关 502 时构造为 `{ code: 502, message: 'Non-JSON response received' }`），避免直接在业务主线程抛出未捕获的 SyntaxError 异常引起崩溃。

