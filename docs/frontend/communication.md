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
