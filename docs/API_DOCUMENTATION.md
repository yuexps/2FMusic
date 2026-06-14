# 2FMusic 后端 API 适配与调用指南

为了方便安卓客户端以及其他第三方应用进行原生 App 开发与适配，本文档整理了 2FMusic 后端所有最新的 API 调用方法、鉴权机制、通信协议以及实时推送事件的详细规范。

---

## 1. 基础架构与鉴权规范

2FMusic 采用了 **“HTTP API 为辅（流媒体播放、图片流、上传） + WebSocket 为主（核心业务、主动推送）”** 的轻量化通信架构。

### 1.1 基础通信路径说明
*   **HTTP API 路径**：如果服务端配置了子路径前缀 `BASE_URL`（例如 `/2fmusic`），则所有的 HTTP 接口请求也**需要**拼上该前缀。完整路径格式为：`http://<server-ip>:<port><base-url>/api/...`。如果未配置，则直接请求 `/api/...`（例：`http://<server-ip>:<port>/api/music/play/<song_id>`）。
*   **WebSocket 握手路径**：WebSocket 握手连接路径也需要适配 `BASE_URL` 前缀，其通信链路为：`ws://<server-ip>:<port><base-url>/api/ws`。如果未配置，则为 `ws://<server-ip>:<port>/api/ws`。

### 1.2 鉴权机制 (Auth)
若服务器配置文件中设置了访问密码 (`APP_AUTH_PASSWORD`)，则除了静态资源和特定豁免页外，其余所有接口（包括 HTTP 接口及 WebSocket 握手阶段）均需要通过凭证验证。

验证支持以下两种值（任选其一）：
1. **明文密码**：用户输入的原始密码字符串。
2. **密码的 SHA-256 哈希值**（大小写不限，推荐）。

#### 方式 A：在 HTTP 请求头部携带
适用于绝大多数标准的 HTTP API 请求：
```http
X-Password: <你的密码或SHA-256哈希值>
```

#### 方式 B：在 URL 查询参数中携带 `auth`
常用于不支持携带自定义头部的原生组件（如 Android 的 `MediaPlayer` / `ExoPlayer` / `Media3` 播放流媒体，或 WebSocket 握手链接）：
```
http://<server-ip>:<port>/api/music/play/<song_id>?auth=<你的密码或SHA-256哈希值>
ws://<server-ip>:<port>/api/ws?auth=<你的密码或SHA-256哈希值>
```

---

## 2. WebSocket 统一交互协议 (`/api/ws`)

所有的核心业务数据交互都基于单条 WebSocket 连接，其承载于 `/api/ws` 路径下。

### 2.1 帧交换规范

#### ① 客户端请求帧格式 (Client Request Frame)
客户端向服务器发起请求时，必须包含以下三个核心字段：
```json
{
  "seq": "android_1716912345678",  // 任意唯一的序列号，用于在响应时匹配上下文
  "action": "music/get_list",      // 具体的业务行为动作名
  "data": {                        // 具体的业务参数（可以为空对象）
    "song_id": "xxxx"
  }
}
```

#### ② 服务端响应帧格式 (Server Response Frame)
服务端在处理完对应的 action 后，会向该客户端回写应答帧：
```json
{
  "seq": "android_1716912345678",  // 对应请求的 seq
  "type": "response",
  "action": "music/get_list",
  "success": true,                 // 业务是否处理成功
  "data": { ... },                 // 返回的具体结果（若成功）
  "error": "错误说明信息"           // 错误说明（若失败，success 为 false）
}
```

#### ③ 心跳帧格式 (Heartbeat)
为了避免安卓后台连接被运营商或系统防火墙掐断，必须建立心跳包。
- **客户端发送**：
  ```json
  {
    "action": "ping"
  }
  ```
- **服务端响应**：
  ```json
  {
    "type": "pong"
  }
  ```
> [!TIP]
> 建议安卓及其他第三方应用端每隔 **20 ~ 30 秒** 发送一次心跳包。

---

## 3. HTTP 专属 API 接口清单

这部分接口只能通过 HTTP 协议请求，返回的内容可能是音频流、图片流或文件上传结果。

### 3.1 音频流式播放 (Play Music)
*   **请求路径**：`GET /api/music/play/<song_id>`
*   **鉴权要求**：需要鉴权。若使用系统播放器，请在 URL 参数中追加 `?auth=xxx`
*   **响应内容**：音频二进制流。
*   **关键特征**：服务端支持 `Byte-Range` 分片下载，允许播放器在播放过程中进行拖动（Seek）。

### 3.2 播放挂载目录外的其他本地音频 (Play External File)
用于播放非音乐库索引，但在挂载目录内的原始物理音频。
*   **请求路径**：`GET /api/music/external/play`
*   **请求参数**（Query Params）：
    *   `path`：物理文件在服务端的绝对路径（须经 `URLEncode`）
*   **鉴权要求**：需要鉴权。
*   **响应内容**：音频二进制流（支持分片）。

### 3.3 物理获取缓存封面图片 (Get Cover Image)
*   **请求路径**：`GET /api/music/covers/<cover_name>`
*   **路径变量**：`<cover_name>` 通常为 `<song_id>.webp`
*   **鉴权要求**：需要鉴权。在设置访问密码时，须在 URL 查询参数中追加 `?auth=xxx` 或携带 `X-Password` 头部。
*   **响应内容**：高压缩比的 WebP 格式图片，默认携带强缓存头（30天）。

### 3.4 上传本地歌曲文件 (Upload File)
将安卓手机上的本地音乐物理上传至服务端。
*   **请求路径**：`POST /api/music/upload`
*   **鉴权要求**：需要头部带上 `X-Password`
*   **请求格式**：`multipart/form-data`
*   **参数列表**：
    *   `file` (File)：音频文件
    *   `target_dir` (String，可选)：物理保存的目录。必须是挂载点子目录。留空则上传至默认音乐库路径。
*   **响应结果**：
    ```json
    {
      "success": true
    }
    ```
    *(注：重复的全路径物理文件或已被哈希查重的歌曲会返回 `"success": false` 与对应的提示。)*

---

## 4. WebSocket Action 业务列表

应用端与服务器握手成功后，通过发送对应的 `action` 请求来执行以下操作。

### 4.1 音乐库管理模块

#### ① 获取去重后的音乐曲目列表
*   **Action**：`music/get_list`
*   **参数**：无
*   **成功响应 `data`**：歌曲数组 `[Song]`，单条 `Song` 的结构定义如下：
    ```json
    {
      "id": "c1a2e3...",          // 物理文件内容MD5生成的唯一标识
      "filename": "歌手 - 歌曲名.mp3",
      "title": "歌曲名",
      "artist": "歌手",
      "album": "专辑名",
      "album_art": "/api/music/covers/c1a2e3....webp", // 封面静态获取路由，若无封面则为 null
      "mtime": 1716912345.67,     // 音频文件物理修改时间
      "size": 8432192,            // 音频物理文件大小（字节数）
      "has_cover": true,          // 是否已有封面
      "has_lyrics": true          // 是否已有歌词
    }
    ```

#### ② 物理删除歌曲
*   **Action**：`music/delete`
*   **参数**：
    ```json
    {
      "song_id": "c1a2e3..."
    }
    ```
*   **说明**：会彻底从服务端物理磁盘中删除对应的音频文件、同级的 `.lrc`/`.yrc`/`.jpg`/`.webp` 附属文件以及元数据缓存，并在数据库中移除记录。
#### ③ 清除指定歌曲的元数据缓存
*   **Action**：`music/clear_metadata`
*   **参数**：
    ```json
    {
      "song_id": "c1a2e3...",   // 可选，指定清空的歌曲ID
      "path": "/volume1/music/周杰伦/晴天.mp3" // 可选，指定清空的歌曲物理路径（与 song_id 二选一）
    }
    ```
*   **说明**：会物理删除该歌曲保存在系统集中化 covers 和 lyrics 目录中的缓存文件，并将数据库中对应记录的 `has_cover` 和 `has_lyrics` 状态重置为 0，以便触发重新刮削或重新提取。


#### ④ 获取单曲歌词
*   **Action**：`music/lyrics`
*   **参数**：
    ```json
    {
      "song_id": "c1a2e3...",   // 歌曲ID
      "title": "歌曲标题",      // 必须，用于当无缓存时自动在线刮削
      "artist": "歌手",         // 可空
      "filename": "歌手 - 歌曲名.mp3"  // 可空
    }
    ```
*   **成功响应 `data`**：
    ```json
    {
      "lyrics": "[00:10.00]歌词内容...\n[00:12.00]下一句..." // 可能是标准 LRC 或逐字 YRC 格式
    }
    ```
*   **获取逻辑**：优先从缓存提取 $\rightarrow$ 寻找同名物理外部 `.lrc` 复制并命中 $\rightarrow$ 音频内嵌提取 $\rightarrow$ 网络多源刮削。

#### ⑤ 获取/在线刮削单曲封面
*   **Action**：`music/album-art`
*   **参数**：
    ```json
    {
      "song_id": "c1a2e3...",
      "title": "歌曲标题",
      "artist": "歌手",
      "filename": "歌手 - 歌曲名.mp3"
    }
    ```
*   **成功响应 `data`**：
    ```json
    {
      "album_art": "/api/music/covers/c1a2e3....webp"
    }
    ```

---

### 4.2 网易云音乐助手模块

应用端可以用此模块调用云端搜索、扫码登录，以及远程后台下载无损音乐并自动录入系统。

#### ① 云端检索网易云歌曲
*   **Action**：`netease/search`
*   **参数**：
    ```json
    {
      "keywords": "周杰伦",
      "limit": 20
    }
    ```
*   **成功响应 `data`**：网易云歌曲列表，包含付费VIP标记、音质等级、文件预计大小等：
    ```json
    [
      {
        "id": 185672,
        "title": "晴天",
        "artist": "周杰伦",
        "album": "叶惠美",
        "cover": "https://p2.music.126.net/...",
        "duration": 269.0,
        "is_vip": false,
        "level": "lossless",     // 当前登录账号可用的音质级别
        "max_level": "lossless", // 该曲目包含的最高音质级别
        "size": 31920831         // 对应音质下的预计大小（字节）
      }
    ]
    ```

#### ② 获取扫码登录二维码
*   **Action**：`netease/login_qrcode`
*   **参数**：无
*   **成功响应 `data`**：
    ```json
    {
      "unikey": "a1b2c3d4-xxxx-xxxx",
      "qrimg": "data:image/png;base64,iVBORw0KGgoAAA..." // Base64 格式的二维码图片，直接在 App 中渲染
    }
    ```


#### ④ 获取当前网易云登录账户状态
*   **Action**：`netease/login_status`
*   **参数**：无
*   **成功响应 `data`**：
    ```json
    {
      "logged_in": true,
      "nickname": "网易云用户昵称",
      "user_id": 1234567,
      "avatar": "https://...",
      "is_vip": true, // 是否是黑胶VIP会员（决定能否下载VIP专享曲目）
      "vip_info": { ... }
    }
    ```

#### ⑤ 获取网易云每日推荐歌曲
*   **Action**：`netease/recommend`
*   **参数**：无
*   **成功响应 `data`**：网易云每日推荐的 30 首歌曲列表（格式与检索返回的歌曲数组一致，要求已登录）。

#### ⑥ 退出网易云登录
*   **Action**：`netease/logout`

#### ⑦ 异步发起下载任务
*   **Action**：`netease/download`
*   **参数**：
    ```json
    {
      "id": 185672,                // 网易云歌曲 ID
      "title": "晴天",              // 歌曲标题
      "artist": "周杰伦",           // 歌手
      "album": "叶惠美",            // 专辑名
      "cover": "https://...",      // 封面图片 URL (可选，会自动转换保存)
      "level": "lossless",         // 期望下载音质 (standard|higher|exhigh|lossless|hires，默认为 exhigh)
      "target_dir": "/absolute/..." // 下载保存的绝对路径 (可选，通常为空，留空下载至服务端配置的网易云下载目录)
    }
    ```
*   **成功响应 `data`**：
    ```json
    {
      "task_id": "task_1716912345678_abcdef" // 下载任务ID，用于后续查询或匹配广播
    }
    ```
> [!TIP]
> 这是一个完全的**异步任务**，接口会立即返回 `task_id`，具体的下载进度和成败状态，服务器会在后台通过 WebSocket 实时广播通知客户端。

#### ⑧ 查询指定下载任务的详情
*   **Action**：`netease/task_status`
*   **参数**：
    ```json
    {
      "task_id": "task_1716912345678_abcdef"
    }
    ```
*   **成功响应 `data`**：
    ```json
    {
      "task_id": "task_1716912345678_abcdef",
      "status": "pending" | "preparing" | "downloading" | "success" | "error",
      "progress": 72,             // 下载百分比进度 (0 - 100)
      "title": "晴天",
      "artist": "周杰伦",
      "completed_at": 1716912400, // 仅在状态为 success 或 error 时存在
      "message": "出错的原因说明"   // 仅在状态为 error 时存在
    }
    ```

#### ⑨ 获取与配置网易云下载路径参数
*   **Action**：`netease/get_config`
*   **参数**：无
*   **成功响应 `data`**：
    ```json
    {
      "download_dir": "/volume1/music/netease", // 服务端存放下载曲目的目录
      "api_base": "http://127.0.0.1:23236",      // 网易云 API 镜像服务地址
      "max_concurrent": 5,                       // 最大并发下载数
      "quality": "exhigh"                        // 默认品质
    }
    ```
*   **Action**：`netease/save_config`
*   **参数 `data`**：
    ```json
    {
      "download_dir": "/volume1/music/netease", // 可选，服务端存放下载曲目的绝对路径
      "api_base": "http://127.0.0.1:23236"      // 可选，网易云 API 镜像服务地址
    }
    ```
*   **成功响应 `data`**：返回修改后的网易云下载路径及 API 根路径设置参数（格式与 get_config 响应一致）。

#### ⑩ 网易云链接/ID 短链自动解析
*   **Action**：`netease/resolve`
*   **参数**：
    ```json
    {
      "input": "https://music.163.com/#/playlist?id=123456" // 支持网易云分享链接、分享文案（含短链）、或纯歌单/歌曲数字ID
    }
    ```
*   **成功响应 `data`**（如果是歌单类型）：
    ```json
    {
      "type": "playlist",
      "id": "123456",
      "name": "",
      "data": [
        {
          "id": 185672,
          "title": "晴天",
          "artist": "周杰伦",
          "album": "叶惠美",
          "cover": "https://p2.music.126.net/...",
          "duration": 269.0,
          "is_vip": false,
          "level": "lossless",
          "max_level": "lossless",
          "size": 31920831
        }
      ]
    }
    ```
*   **成功响应 `data`**（如果是单曲类型）：
    ```json
    {
      "type": "song",
      "id": "185672",
      "data": [
        {
          "id": 185672,
          "title": "晴天",
          "artist": "周杰伦",
          "album": "叶惠美",
          "cover": "https://p2.music.126.net/...",
          "duration": 269.0,
          "is_vip": false,
          "level": "lossless",
          "max_level": "lossless",
          "size": 31920831
        }
      ]
    }
    ```

#### ⑪ 挂载 Docker 内网服务的维护接口
*   **Action**：`netease/check_container`：检测服务端的 Docker 环境以及是否已部署 API 容器。
    *   **参数**：无
    *   **成功响应 `data`**：
        ```json
        {
          "docker_installed": true,    // 系统是否已安装 Docker
          "container_exists": true,    // 容器 2fmusic-ncm-api 是否存在
          "container_running": true    // 容器是否正处于运行中
        }
        ```
*   **Action**：`netease/install_service`：自动部署/启动网易云镜像服务容器。
    *   **参数**：无
    *   **成功响应 `data`**：返回字符串 `"安装部署任务已挂起后台启动"`（成功触发后台异步安装）。
*   **Action**：`netease/install_status`：查询部署进度。
    *   **参数**：无
    *   **成功响应 `data`**：
        ```json
        {
          "status": "idle" | "running" | "success" | "error",
          "progress": 70,              // 安装百分比进度 (0 - 100)
          "step": "镜像拉取完成，正在启动容器...",
          "error": null                // 失败时的错误原因
        }
        ```

---

### 4.3 收藏夹与自定义歌单模块

#### ① 获取所有自定义收藏夹列表
*   **Action**：`favorite/list_playlists`
*   **成功响应 `data`**：
    ```json
    [
      {
        "id": "default",       // 默认收藏夹（红心歌单）
        "name": "默认收藏夹",
        "created_at": 1716912345,
        "song_count": 48       // 该收藏夹包含的音乐总数
      },
      {
        "id": "1",
        "name": "开车专用嗨歌",
        "created_at": 1716912499,
        "song_count": 12
      }
    ]
    ```

#### ② 新建自定义收藏夹歌单
*   **Action**：`favorite/create_playlist`
*   **参数**：`{"name": "歌单名称"}`
*   **成功响应 `data`**：返回该歌单的完整对象（包含新生成的 `id`）。

#### ③ 删除自定义收藏夹歌单
*   **Action**：`favorite/delete_playlist`
*   **参数**：`{"playlist_id": "1"}` (默认 `'default'` 收藏夹禁止删除)

#### ④ 获取指定歌单收藏夹内的所有歌曲 ID 列表
*   **Action**：`favorite/playlist_songs`
*   **参数**：`{"playlist_id": "default"}`
*   **成功响应 `data`**：
    ```json
    [
      "c1a2e3...", // 歌曲唯一 MD5 ID 列表
      "d4e5f6..."
    ]
    ```
> [!TIP]
> 推荐的做法是：客户端拉取 `music/get_list` 并在本地建立缓存映射，再通过 `playlist_songs` 获取到 ID 列表，从而在本地拼装展示歌单。

#### ⑤ 批量添加歌曲到收藏夹
*   **Action**：`favorite/add`
*   **参数格式**：
    ```json
    {
      "song_ids": ["c1a2e3...", "d4e5f6..."],
      "playlist_ids": ["default"], // 可选，默认为 ["default"]
      "songs": {}                  // 可选，歌曲元数据映射 { song_id: { title, artist } }
    }
    ```
*   **成功响应 `data`**：添加成功的数据详情（成功时 `success: true`）

#### ⑥ 批量从收藏夹移除歌曲
*   **Action**：`favorite/delete`
*   **参数格式**：
    ```json
    {
      "song_ids": ["c1a2e3...", "d4e5f6..."],
      "playlist_ids": ["default"]
    }
    ```
*   **成功响应 `data`**：删除结果的数据详情（成功时 `success: true`）

#### ⑦ 批量移动歌曲至新收藏夹
*   **Action**：`favorite/batch_move`
*   **参数**：
    ```json
    {
      "song_ids": ["c1a2e3...", "d4e5f6..."],
      "from_playlist_id": "1",
      "to_playlist_id": "default"
    }
    ```
*   **说明**：将多首歌曲在事务中安全地从源收藏夹批量移动到目标收藏夹，并由服务端自动执行数据一致性强制校验。

---

### 4.4 播放历史模块

#### ① 获取播放历史记录列表
*   **Action**：`history/get`
*   **成功响应 `data`**：按时间倒序排列的历史记录：
    ```json
    [
      {
        "id": "c1a2e3...",
        "title": "晴天",
        "artist": "周杰伦",
        "album": "叶惠美",
        "album_art": "/api/music/covers/c1a2e3....webp",
        "play_count": 5,           // 累计播放次数
        "last_played": 1716912345  // 最后一次播放的时间戳（秒级）
      }
    ]
    ```

#### ② 添加播放历史
*   **Action**：`history/add`
*   **参数**：`{"song_id": "c1a2e3..."}`
*   **说明**：每次歌曲在客户端完整播放完（或播放超过一分钟）时应主动上报。

#### ③ 移除单条或清空历史
*   `history/remove`：参数 `{"song_id": "c1a2e3...", "play_time": 1716912345}`
*   `history/clear`：清空所有播放历史记录。

---

### 4.5 挂载路径模块 (`mount/*`)

#### ① 获取所有已挂载的物理目录路径
*   **Action**：`mount/list`
*   **参数**：无
*   **成功响应 `data`**：
    ```json
    [
      "/volume1/music/custom",
      "/volume1/music/周杰伦专辑"
    ]
    ```

#### ② 添加本地目录挂载点
*   **Action**：`mount/add`
*   **参数**：
    ```json
    {
      "path": "/volume1/music/周杰伦专辑" // 需要挂载的本地绝对目录路径，在服务端上必须真实存在
    }
    ```
*   **说明**：添加成功后会刷新 Watchdog 监控路径并后台异步自动发起全库增量扫描。

#### ③ 移除目录挂载
*   **Action**：`mount/delete`
*   **参数**：
    ```json
    {
      "path": "/volume1/music/周杰伦专辑"
    }
    ```
*   **说明**：仅从数据库中移除对该目录下所有音乐的索引关系并通知库变更，**绝对不会**物理删除磁盘中的任何音乐文件。

#### ④ 手动触发目录局部增量扫描
*   **Action**：`mount/scan`
*   **参数**：
    ```json
    {
      "path": "/volume1/music/周杰伦专辑"
    }
    ```
*   **成功响应 `data`**：返回字符串 `"已启动局部扫描目录任务"`（后台会异步执行该目录的局部扫描）。

#### ⑤ 手动触发目录下歌曲的元数据重新在线刮削
*   **Action**：`mount/retry_scrape`
*   **参数**：
    ```json
    {
      "path": "/volume1/music/周杰伦专辑"
    }
    ```
*   **成功响应 `data`**：返回字符串 `"已启动重新刮削元数据任务"`（后台会异步针对缺失元数据的音乐执行在线补刮）。

### 4.6 系统状态与偏好设置模块 (`system/*`)

#### ① 获取当前的音乐扫描、刮削与库数据统计状态
*   **Action**：`system/get_status`
*   **参数**：无
*   **成功响应 `data`**：
    ```json
    {
      "scanning": false,            // 服务端当前是否正在增量扫描物理文件
      "is_scraping": false,         // 服务端当前是否正在执行在线元数据刮削
      "total": 0,                   // 待刮削或待扫描的任务队列总数
      "processed": 0,               // 当前任务队列已处理完成数
      "failed": 0,                  // 当前任务队列处理失败数
      "current_file": "",           // 当前正在扫描/刮削的文件名
      "current_path": "",           // 当前正在扫描/刮削的文件绝对路径
      "library_version": 1716912345.67, // 音乐库最新的全局版本号（更新时间戳）
      "music_count": 148,           // 音乐库中去重后的歌曲记录总数
      "playlist_count": 3           // 服务端已创建的自定义歌单总数
    }
    ```

#### ② 获取歌词刮削来源的优先级偏好设置
*   **Action**：`system/get_lyrics_preference`
*   **参数**：无
*   **成功响应 `data`**：
    ```json
    {
      "value": "embedded"           // 歌词提取偏好：'embedded' (优先音频内嵌提取) 或 'network' (优先网络在线刮削)
    }
    ```

#### ③ 更新并保存歌词刮削的来源偏好设置
*   **Action**：`system/save_lyrics_preference`
*   **参数**：
    ```json
    {
      "value": "network"            // 可选值必须为 'embedded' 或 'network'
    }
    ```
*   **成功响应 `data`**：无（成功时返回 `success: true`，服务端会物理持久化该偏好设置）。

---

## 5. WebSocket 实时推送事件 (Server Broadcasts)

服务端会随时向所有的 WebSocket 客户端广播推送关键事件。应用端需要根据 `type` 字段进行捕获：

### 5.1 库发生变动 (`library_changed`)
每当有文件增加、删除、收藏夹变动或网易云成功下载歌曲入库时，服务端会发出广播。
```json
{
  "type": "library_changed",
  "data": {
    "library_version": 1716912345.67  // 最新库版本号（时间戳）
  }
}
```
> [!IMPORTANT]
> 捕获本消息后，如果该版本号与应用端本地记录的版本号不一致，说明音乐库被修改了。客户端应在合适的时机（例如回到列表页面）静默调用 `music/get_list` 刷新列表缓存。

### 5.2 音乐库后台扫描/元数据刮削进度 (`scan_status`)
用于在系统状态页展示进度条。
```json
{
  "type": "scan_status",
  "data": {
    "scanning": true,                // 是否处于文件扫描中
    "is_scraping": true,             // 是否处于网络刮削中
    "total": 45,                     // 待处理任务总数
    "processed": 12,                 // 已处理完成数
    "failed": 1,                     // 刮削失败数
    "current_file": "晴天.mp3",
    "current_path": "/volume1/music/周杰伦/晴天.mp3",
    "library_version": 1716912345.67
  }
}
```

### 5.3 异步下载网易云音乐进度 (`download_status`)
当有网易云下载后台任务状态改变或进度更新时，会发生广播。
```json
{
  "type": "download_status",
  "data": {
    "task_id": "task_1716912345678_abcdef",
    "status": "downloading",         // pending | preparing | downloading | success | error
    "progress": 45,                  // 下载百分比 (0 - 100)
    "title": "晴天",
    "artist": "周杰伦",
    "completed_at": null,            // 完成时间戳
    "message": ""                    // 报错信息
  }
}
```

### 5.4 网易云扫码登录状态变动 (`netease_login_status`)
在网易云扫码登录发起后，服务端会在扫码状态改变时，主动向所有的 WebSocket 客户端广播推送最新状态。
```json
{
  "type": "netease_login_status",
  "data": {
    "key": "a1b2c3d4-xxxx-xxxx",       // 对应的 unikey
    "status": "waiting" | "scanned" | "authorized" | "expired", // 当前的扫码状态
    "message": "等待扫码 / 已扫码 / 授权登录成功 / 二维码已失效"
  }
}
```
> [!IMPORTANT]
> 引入此主动推送后，应用端**不需要**再建立任何轮询计时器。只需在获取二维码后，订阅本推送，并匹配对应的 `key`；一旦监听到状态变为 `authorized`，说明授权成功，即可静默刷新界面用户态。

---

## 6. 应用端适配避坑与最佳实践

### 6.1 音频流的边下边播与 Seek 支持
在原生端开发音频播放器时，极力推荐使用成熟的支持流式解析的播放器核心。
- Android 端推荐使用 Google 的 **Jetpack Media3 / ExoPlayer**。其内置支持 `HTTP Range`，能够直接适配后端的流式音频接口 `/api/music/play/<id>`。
- **防止断网闪退**：建议配置播放器的连接超时（例如 8 秒）和自动重试策略。

### 6.2 密码鉴权的统一拦截
在客户端应用中封装 Axios 或网络库（如 OkHttp、Retrofit）：
*   **拦截器**：编写一个 `Interceptor`，在所有请求头部中自动填入 `X-Password`。
*   **流媒体播放器特例**：由于某些播放器的 Native 音频流数据源握手不方便追加 Header，应当使用 URL 拼接参数形式传递密码，例如：
    `String playUrl = serverBase + "/api/music/play/" + songId + "?auth=" + URLEncoder.encode(sha256Password, "UTF-8");`

### 6.3 二维码扫码登录的数据流
1. 调用 `netease/login_qrcode` 获取 `unikey` 和 `qrimg`。同时服务端会在后台自动启动监听线程。
2. 在 App 页面中渲染二维码，并监听 WebSocket 推送事件 `netease_login_status`。
3. 匹配对应的 `key`，收到 `scanned` 推送时，将界面置为“已在手机上扫码，等待确认”。
4. 收到 `authorized` 推送时，说明授权已完成，立即刷新界面并调用 `netease/login_status` 展示用户信息。
5. 收到 `expired` 推送时，提示“二维码已失效，请点击刷新”。

### 6.4 歌词和封面的懒加载
- `music/get_list` 响应返回的歌曲列表很大时，千万不要并发在初始化阶段去请求所有歌曲的歌词。
- 歌词只应在 **用户点击进入“播放详情页”** 时，才发起 WebSocket 动作 `music/lyrics` 进行按需获取并高亮解析。
