# Android 客户端接入与高精度歌词解析规范 (docs/android.md)

本文档定义了 2FMusic-Android 原生客户端的音频流点播接入与 `LrcParser` 高精度歌词解析规范。

---

## 1. 原生音频点播接入 (Jetpack Media3 / ExoPlayer)

### 1.1 音频流与 HTTP Range 适配
- 推荐使用 Android **Jetpack Media3 / ExoPlayer** 核心。
- 接口地址：`GET http://<server-host>:<port>/api/music/play/<song_id>`
- **密码鉴权**：由于原生播放器数据源握手不方便添加 Header，请在 URL 参数中追加 `?auth=<password_sha256>`，例如：
  `String playUrl = serverBase + "/api/music/play/" + songId + "?auth=" + URLEncoder.encode(sha256Password, "UTF-8");`
- **拖动 (Seek) 支持**：服务端天然支持 `Byte-Range` 分片，Media3 可直接实现秒级拖动点播。

### 1.2 BaseUrl 动态网络探活与协议自适应规范
- **协议显式保留**：若用户输入的地址已包含 `http://` 或 `https://` 协议前缀，尊重用户设置直接保留。
- **网络探活探测算法**（当未提供协议头，如 `192.168.31.253:23237` 或 `music.example.com`）：
  1. **HTTPS 优先探活**：使用 1500ms 短超时尝试发起 HTTPS 探测握手；若响应成功或达成 TLS 握手，确定使用 `https://`。
  2. **HTTP 退避探活**：若 HTTPS 探测失败（连接拒绝/TLS 握手错误/超时），降级发起 HTTP 探活；若响应成功，确定使用 `http://`。
  3. **保底回退**：探活无法达成时，默认使用 `http://` 避免解析报错。
- **全局生效**：保存探测结果至 `Platform.config.getBaseUrl()`，供 ExoPlayer、Coil/Seiko ImageLoader 及 WebSocket 统一消费。

### 1.3 多源在线刮削与元数据清除规范
- **全网深度刮削契约**：在客户端触发重新刮削或本地缺失封面时，必须显式调用 `api.getAlbumArt(...)` 发送 `music/album-art` WS 动作。
- **一致性保证**：严禁受旧 `song.albumArt` 字段误导跳过在线刮削。必须触发服务端多源搜索引擎（QQ音乐、网易云等）重新深度检索落盘 WebP 封面，保持 Android 端与 Web 端在线刮削算法严格一致。

---

## 2. 精细化广播与本地缓存重载规范
- 收到 `library_changed` 精细化广播帧时：
  - 若 `fields` 包含 `'cover'`：擦除 `covers/cover_${songId}.webp` 本地磁盘文件，并通知 Coil/图片加载器使 Memory/Disk Cache 失效。
  - 若 `fields` 包含 `'lyrics'`：擦除 `lyrics/lyrics_${songId}` 本地文件。
  - 若正在播放该曲目：`AndroidPlayerController` 重新发起 WS `music/lyrics` 请求并推流刷新 Compose `PlayerScreen` 界面。

---

## 3. 高精度歌词解析算法 (`LrcParser`)

为了保证 2FMusic-Android 客户端在展示不同格式（LRC、YRC）及不同来源的歌词时具有最佳展示效果，`LrcParser` 必须严格遵循以下算法红线：

### 2.1 垃圾与非标准元数据行丢弃
- **判定规则**：对一行文本执行 `trim()` 后，若以 `{` 开头且以 `}` 结尾，判定为非标准 JSON 演职员元数据。
- **处理红线**：为此类行执行静默 `continue` 丢弃，防止垃圾字符串混入歌词面板。

### 2.2 YRC 逐字绝对时间精度自适应
- **背景**：不同平台导出的 YRC/KRC 文件中，括号内字的起止时间既有采用**绝对毫秒时间戳**的，也有采用**相对本行起止的偏移量**的。
- **计算公式**：
  $$\text{absoluteStart} = \begin{cases} 
    \text{time} + \text{wStart} & \text{若 } \text{wStart} < \text{time} \\ 
    \text{wStart} & \text{若 } \text{wStart} \ge \text{time} 
 \end{cases}$$
  - `time` 为本行的起始绝对毫秒时间戳。
  - `wStart` 为字标签括号提取出的第一个数字。
- **解析红线**：统一转换为**绝对毫秒时间戳**存入 `YrcWord` 中，为 UI 渲染提供精确的时间对照基准。

### 2.3 LRC 毫秒弹性长度补齐
- **背景**：普通 LRC 歌词中毫秒位长度不固定（如 `[00:01.8]` 为 1 位，`[00:01.17]` 为 2 位，`[00:01.170]` 为 3 位）。
- **对准算法**：
  ```kotlin
  val ms = msStr.padEnd(3, '0').substring(0, 3).toLongOrNull() ?: 0L
  ```
- **解析红线**：通过智能补齐与截断使毫秒值统一为 3 位并正确折算。严禁由于长度不合规而将毫秒值清零。
