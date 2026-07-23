# 2FMusic 系统 API 与 WebSocket 通信契约规范 (docs/api.md)

本文档定义了 2FMusic 后端所有 RESTful API、WebSocket 帧交换协议、全量 Action 路由动作清单以及服务端主动广播事件总线。

---

## 1. 基础架构与统一鉴权规范

2FMusic 采用了 **“HTTP API（流媒体播放、封面图片、文件上传） + WebSocket（核心业务数据、状态控制与主动推送）”** 的通信架构。

### 1.1 URL 路径与子路径适配 (`BASE_URL`)
- **HTTP REST API 基础路径**：若配置了子路径前缀 `${BASE_URL}`（如 `/2fmusic`），所有 REST 接口前缀为 `http://<host>:<port>${BASE_URL}/api/...`。无配置时直接为 `/api/...`。
- **WebSocket 握手路径**：`ws://<host>:<port>${BASE_URL}/api/ws`。

### 1.2 密码鉴权机制
在配置了 `APP_AUTH_PASSWORD` 时，所有非白名单接口强制校验凭证。为保障传输安全，**网络中禁止传输明文密码，客户端必须统一传输密码的 SHA-256 哈希值**。
- **方式 A (HTTP Header)**：在 HTTP 请求头携带 `X-Password: <password_sha256>`。
- **方式 B (URL Query Param)**：在 URL 查询参数中追加 `?auth=<password_sha256>`（常用于 HTML5 `<audio>`/播放器及 WebSocket 握手）。

---

## 2. WebSocket 交互协议 (`/api/ws`)

### 2.1 消息数据帧规范

#### 1. 客户端请求帧 (Client Request Frame)
```json
{
  "seq": "android_1716912345678",  // 唯一请求序列号 (数字或字符串)
  "action": "music/get_list",      // 业务动作名
  "data": { ... }                  // 业务参数 (可为空对象 {})
}
```

#### 2. 服务端应答帧 (Server Response Frame)
依据 Python 真实源码 `server/core/routes/ws.py` 与 Go `backend/websocket/hub.go`：
```json
{
  "seq": "android_1716912345678",  // 对应请求的 seq
  "type": "response",             // 固定类型
  "action": "music/get_list",      // 对应的 Action 名称
  "success": true,                 // 业务处理成功为 true，失败为 false
  "data": { ... },                 // 成功时的业务载荷 (success 为 true)
  "error": "错误消息描述"           // 失败时的错误说明 (success 为 false)
}
```

#### 3. 心跳帧 (Heartbeat)
- **客户端发送**：`{"action": "ping"}`
- **服务端应答**：`{"type": "pong"}`
> **建议**：客户端每 20 ~ 30 秒发送一次心跳帧维系连接。

---

## 3. WebSocket 业务 Action 全量路由表

### 3.1 音频曲目动作
- **`music/get_list`**：获取全量去重歌曲列表。
  - **请求 `data`**：`{}`
  - **响应 `data`**：`List[SongDict]`
- **`music/delete`**：物理删除歌曲及附属文件。
  - **请求 `data`**：`{"song_id": "xxx"}`
  - **响应 `data`**：`{"success": true}`
- **`music/clear_metadata`**：清理封面/歌词缓存与标记。
  - **请求 `data`**：`{"song_id": "xxx"}` 或 `{"path": "xxx"}`
  - **响应 `data`**：`{"success": true}`
- **`music/lyrics`**：获取或刮削单曲歌词。
  - **请求 `data`**：`{"song_id": "xxx", "title": "...", "artist": "...", "filename": "...", "yrc": false}`
  - **响应 `data`**：`{"lyrics": "[00:00.00]..."}`
- **`music/album-art`**：获取或刮削单曲封面路径。
  - **请求 `data`**：`{"song_id": "xxx", "title": "...", "artist": "...", "album": "..."}`
  - **响应 `data`**：`{"album_art": "/api/music/covers/xxx.webp"}`
- **`music/scrape`**：手动检索最佳在线元数据。
  - **请求 `data`**：`{"title": "...", "artist": "...", "album": "..."}`
  - **响应 `data`**：`{"title": "...", "artist": "...", "album": "...", "cover": "...", "lyrics": "..."}`

### 3.2 收藏夹与歌单动作
> **说明**：通用协议路由强制使用单数前缀 `favorite/*`（Python 后端严格识别单数，Go 后端包含复数 `favorites/*` 兼容别名）。

- **`favorite/list_playlists`**（别名：`favorites/list`）：获取所有歌单及歌曲数量。
  - **响应 `data`**：`List[{"id": "default", "name": "默认收藏夹", "is_default": 1, "created_at": float, "song_count": int}]`
- **`favorite/playlist_songs`**（别名：`favorites/playlist_songs`）：获取指定歌单下的歌曲 ID 列表。
  - **请求 `data`**：`{"playlist_id": "xxx"}`
  - **响应 `data`**：`List[str]` (歌曲 ID 列表)
- **`favorite/add`**（别名：`favorites/add`）：添加歌曲至歌单。
  - **请求 `data`**：`{"song_ids": ["xxx"], "playlist_ids": ["default"], "songs": {...}}`
- **`favorite/delete`**（别名：`favorites/remove`）：从歌单移除歌曲。
  - **请求 `data`**：`{"song_ids": ["xxx"], "playlist_ids": ["default"]}`
- **`favorite/create_playlist`**（别名：`favorites/create_playlist`）：新建歌单。
  - **请求 `data`**：`{"name": "歌单名"}`
  - **响应 `data`**：`{"id": "xxx", "name": "歌单名"}`
- **`favorite/delete_playlist`**（别名：`favorites/delete_playlist`）：删除歌单 (默认歌单不可删)。
  - **请求 `data`**：`{"playlist_id": "xxx"}`
- **`favorite/batch_move`**（别名：`favorites/batch_move`）：批量移动歌曲歌单。
  - **请求 `data`**：`{"song_ids": ["xxx"], "from_playlist_id": "xxx", "to_playlist_id": "xxx"}`

### 3.3 播放历史动作
- **`history/get`**（别名：`history/list`）：获取历史播放记录。
  - **请求 `data`**：`{"limit": 100}`
  - **响应 `data`**：`List[{"time": 毫秒int, "song": SongDict}]`
- **`history/add`**：记录播放历史。
  - **请求 `data`**：`{"song_id": "xxx"}`
- **`history/remove`**：移除指定播放历史。
  - **请求 `data`**：`{"song_id": "xxx", "play_time": float}`
- **`history/clear`**：清空播放历史。

### 3.4 挂载点与磁盘扫描动作
> **说明**：通用协议路由强制使用单数前缀 `mount/*`（Python 后端严格识别单数，Go 后端包含复数 `mounts/*` 兼容别名）。

- **`mount/list`**（别名：`mounts/list`）：获取挂载点绝对路径列表。
  - **响应 `data`**：`List[str]`
- **`mount/add`**（别名：`mounts/add`）：添加新挂载点。
  - **请求 `data`**：`{"path": "D:\\Music"}`
- **`mount/delete`**（别名：`mounts/delete`）：删除挂载点并级联清理索引。
  - **请求 `data`**：`{"path": "D:\\Music"}`
- **`mount/scan`**：触发指定挂载点局部增量扫描。
  - **请求 `data`**：`{"path": "D:\\Music"}`
- **`mount/retry_scrape`**：触发指定挂载点重新在线刮削。
  - **请求 `data`**：`{"path": "D:\\Music"}`

### 3.5 系统偏好与状态动作
- **`system/get_status`**：获取库统计与扫描/刮削进度。
  - **响应 `data`**：`{"scanning": bool, "is_scraping": bool, "total": int, "processed": int, "failed": int, "current_file": str, "current_path": str, "music_count": int, "playlist_count": int, "library_version": float}`
- **`system/get_lyrics_preference`**：获取歌词来源偏好。
  - **响应 `data`**：`{"value": "embedded" | "network"}`
- **`system/save_lyrics_preference`**：保存歌词来源偏好。
  - **请求 `data`**：`{"value": "embedded" | "network"}`
- **`system/scan_library`**：触发全库增量扫描。

### 3.6 网易云音乐工具箱动作
- **`netease/search`**：网易云在线搜歌，返回经过 `FormatNeteaseSongs` 归一化的 `List[NeteaseSong]`。
  - **请求 `data`**：`{"keywords": "...", "limit": 30}`
- **`netease/resolve`**：解析网易云链接/歌单 ID/单曲 ID。返回 `{"type": "song"|"playlist", "id": "...", "name": "歌单名", "data": List[NeteaseSong]}`，曲目列表统一经 `FormatNeteaseSongs` 归一化。
  - **请求 `data`**：`{"input": "..."}`
- **`netease/recommend`**：获取网易云每日推荐歌曲，返回 `List[NeteaseSong]`。
  - **请求 `data`**：`{}`
- **`netease/download`**：创建网易云下载任务 (`payload: dict`)。
- **`netease/download_status`** / **`netease/task_status`**：获取下载任务列表。
- **`netease/login_qrcode`** / **`netease/qr_key`**：获取登录二维码 (`unikey` & `qrimg`)。
- **`netease/login_status`**：查询当前 Cookie 登录态与 VIP 信息。未配置 API 时返回 `{"logged_in": false, "error": "网易云 API 未配置"}`。
- **`netease/logout`**：注销网易云账号。
- **`netease/get_config`** / **`netease/save_config`**：读写网易云 API 根路径与下载目录配置。`api_base` 默认初始为空字符串 `""`；`save_config` 提交非空 `api_base` 时服务端将对 `${api_base}/login/status` 进行 3 秒超时连通性及 JSON 数据包特征（必须包含 `code` 或 `data` 键）校验，校验失败时拦截并返回具体错误说明；传入空串可清空配置。
- **`netease/clear_task`** / **`netease/clear_all_tasks`**：清理已完成下载任务。
- **`netease/check_container`** / **`netease/install_service`** / **`netease/install_status`**：Docker 容器检测与自动部署。


---

## 4. WebSocket 服务端主动广播事件总线 (Broadcast Events)

客户端建立 WS 连接后，服务端在特定事件触发时会主动推送不带 `seq` 属性的广播帧：
1. **`library_changed`**：
   - 载荷：`{"type": "broadcast", "action": "library_changed", "data": {"event_type": "update"|"insert"|"delete"|"reload_all", "song_ids": ["xxx"], "fields": ["cover"|"lyrics"|"metadata"], "timestamp": 1784689724530}}`
   - 触发时机：增量扫描/重新刮削/Watcher 捕获封面或歌词落盘、歌曲删除或挂载点变更。
   - 载荷规约：`event_type` 标识变更性质；`song_ids` 为变更曲目 ID 列表；`fields` 为具体变更数据类型。客户端收到后根据 `song_ids` 和 `fields` 精准擦除本地缓存并刷新当前播放试图。
2. **`download_status`**：
   - 载荷：`{"type": "download_status", "data": {"task_id": "xxx", "status": "downloading"|"success"|"error", "progress": 85, ...}}`
   - 触发时机：网易云异步下载任务进度更新。
3. **`netease_login_status`**：
   - 载荷：`{"type": "netease_login_status", "data": {"key": "xxx", "status": "waiting"|"scanned"|"authorized"|"expired", "message": "..."}}`
   - 触发时机：后台检测到网易云扫码状态发生改变。

---

## 5. HTTP REST 专属 API 清单

### 5.1 音频流式点播
- **`GET /api/music/play/:id`**：获取指定歌曲的音频二进制流。支持 `Byte-Range` 分片下载与拖动 (Seek)。
- **`GET /api/music/external/play?path=<escaped_path>`**：播放已授权挂载目录内的原始物理音频流。

### 5.2 封面图片托管
- **`GET /api/music/covers/:name`**：获取缓存目录 `covers/` 下的 WebP 封面。带有 30 天强缓存头 `Cache-Control: public, max-age=2592000`。

### 5.3 音频文件物理上传
- **`POST /api/music/upload`**：`multipart/form-data` 格式上传。
  - 参数：`file` (文件), `target_dir` (可选目标挂载目录)。自动执行 Watchdog 避让与后台异步索引。

### 5.4 网易云 API 代理与落地页
- **`GET /api/netease/download_page`**：重定向跳转网易云官方客户端下载页。
- **`ANY /api/netease/*path`**：代理请求本地/远程 NCM API，自动注入 Cookie，并剥离 `Set-Cookie` 头防止反代 502 报错。
