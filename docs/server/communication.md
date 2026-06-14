# 网络与通信契约规范 (docs/server/communication.md)

## 1. 端口与 UNIX Domain Socket 启动规范
*   `server/app.py` 根据命令行及环境变量参数，自适应启动三种底层网络模式：
    1.  **纯端口模式**：监听 TCP 端口（如 0.0.0.0:port），支持多线程并发 (`threaded=True`)。
    2.  **纯 Socket 模式**：通过 `UnixWSGIServer` 绑定指定的物理 UNIX Domain Socket 文件。在绑定前自动删除历史残留 socket 并分配 `0o666` 物理读写权限，以便反向代理服务器（如 Caddy/Nginx）读取。
    3.  **端口+Socket 并发模式**：在单独的守护线程里运行 UNIX socket 监听，主线程阻塞运行 TCP 端口服务。

## 2. 基准子路径与鉴权拦截
*   **PrefixMiddleware**：若配置了 `BASE_URL`，在 Flask 顶层利用 WSGI 中间件统一剥离请求前缀（同时支持 `HTTP_X_FORWARDED_PREFIX` 标志识别），实现透明的子路径反向代理适配。
*   **密码鉴权**：在配置了 `APP_AUTH_PASSWORD` 时，自动拦截非法请求（返回 401 状态码）。
    - 鉴权验证值支持：**原始密码** 或 **密码的 SHA-256 哈希值**。
    - HTTP REST API：自动检测请求头 `X-Password`。
    - 流媒体及 WS 握手：自动在 URL params 中检索 `?auth=` 参数。
*   **安全机制：IP 登录暴力破解防范**：
    - 在后端内存中维护 IP 登录失败计数器。对于任何发起 `/login` 的 IP，记录其失败时间戳并清除一小时前的陈旧数据。
    - 若任一客户端 IP 在 **最近一小时 (3600 秒) 内登录失败次数达到或超过 3 次**，立即拦截该 IP 后续的一小时内所有登录尝试（返回 `'尝试次数过多，请一小时后再试'`），登录成功时重置并清除失败计数。这在外部暴露至外网时防范撞库至关重要。
*   **反向代理适配与 HTTP 安全缓存控制**：
    - **ProxyFix 中间件**：为了适配 Nginx / Caddy 反向代理，后端工厂函数中注入了 `ProxyFix(x_for=1, x_proto=1, x_host=1, x_prefix=1)`。这确保了后端获取客户端真实 IP（用于 IP 防爆破）以及识别外部反代协议的准确性。
    - **HTML 资源防缓存重绘**：为了保证重构部署后客户端浏览器能 100% 刷入最新静态资源包，对所有 HTML 页面响应统一强制写入 `Cache-Control: no-cache, no-store, must-revalidate` 头；同时通过对 HTML 二进制内容计算 MD5 哈希作为强 `ETag` 缓存检验指针写入 Header。
    - **全域 CORS 放行**：对所有 HTTP 响应默认注入跨域 CORS 允许响应头（`Access-Control-Allow-Origin: *` 等），以保障第三方 Android 客户端直接调用 REST 流媒体的灵活性。


## 3. WebSocket 数据帧契约 (`/api/ws`)

### 3.1 客户端请求帧
```json
{
  "seq": "android_1716912345678",  // 唯一请求序列号，支持数字或字符串
  "action": "music/get_list",      // 具体的业务行为动作名
  "data": { ... }                  // 业务参数（可以为空对象）
}
```

### 3.2 服务端应答帧
```json
{
  "seq": "android_1716912345678",  // 对应请求的 seq
  "type": "response",
  "action": "music/get_list",
  "success": true,                 // 业务成功为 true，失败为 false
  "data": { ... },                 // 成功时的具体结果
  "error": "错误信息"              // 失败时的错误说明 (success 为 false)
}
```

### 3.3 心跳帧格式
*   **客户端发送**：`{"action": "ping"}`
*   **服务端应答**：`{"type": "pong"}`
*   心跳超时判定：如果客户端 40000ms（40 秒）内未收到任何响应，判定为断线并启动断开重连逻辑。
