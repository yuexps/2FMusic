# Android 客户端接入与高精度歌词解析规范 (docs/android.md)

本文档定义了 2FMusic-Android 原生客户端的音频流点播接入与 `LrcParser` 高精度歌词解析规范。

---

## 1. 原生音频点播接入 (Jetpack Media3 / ExoPlayer)

### 1.1 音频流与 HTTP Range 适配
- 推荐使用 Android **Jetpack Media3 / ExoPlayer** 核心。
- 接口地址：`GET http://<server-host>:<port>/api/music/play/<song_id>`
- **密码鉴权**：由于原生播放器数据源握手不方便添加 Header，请在 URL 参数中追加 `?auth=<password_or_sha256>`，例如：
  `String playUrl = serverBase + "/api/music/play/" + songId + "?auth=" + URLEncoder.encode(sha256Password, "UTF-8");`
- **拖动 (Seek) 支持**：服务端天然支持 `Byte-Range` 分片，Media3 可直接实现秒级拖动点播。

---

## 2. 高精度歌词解析算法 (`LrcParser`)

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
