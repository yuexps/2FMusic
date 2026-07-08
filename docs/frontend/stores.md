# Pinia 状态管理逻辑契约 (docs/frontend/stores.md)

## 1. 播放器核心 Store 逻辑 (`player.ts`)

### 1.1 初始化与本地状态恢复 (init)
*   **状态恢复**：从 `localStorage` 中的 `2fmusic_state` 提取配置并覆盖：
    - `volume` -> 设置原生 `audio.volume` 及 Store 响应值。
    - `playMode` -> 列表 `list`、单曲 `single`、随机 `random`。
    - `playlist` -> 恢复上次保存的当前播放列表。
    - `currentSong` -> 若存在当前歌曲，自动拼接并加载流地址：`/api/music/play/${song.id}`，并通过异步回调加载其本地缓存封面。
*   **状态保存**：在执行 `saveState` 时，将当前 `playlist` 同步写入 `localStorage`。存入前须过滤或重置歌曲对象中可能包含的临时 `blob:` 封面 URL 属性。
*   **原生 HTMLAudioElement 事件绑定**：
    - `play` / `pause` 事件：同步 `isPlaying` 变量并更新 `MediaSession.playbackState`。
    - `timeupdate` / `durationchange` 事件：实时更新 `currentTime` 及 `duration`，并同步系统的媒体会话位置。
    - `ended` 事件：自动触发切歌 `next()` 方法。

### 1.2 歌曲控制与待播插播队列逻辑
*   **待播优先插播队列 (Queue)**：
    - 在执行 `next()` 切下一首歌时，**优先判断 `queue` 队列中是否有歌曲**。
    - 若有，调用 `queue.shift()` 弹出并强制切入播放。
    - 仅当插播队列 `queue` 为空时，才轮询常规播放列表 `playlist`。
*   **切歌播放模式 (playMode)**：
    - `single`：再次 `seek(0)` 重置进度并 play。
    - `random`：通过 `Math.floor(Math.random() * playlist.length)` 检索切歌。
    - `list`：顺次查找 `playlist` 中的 `(index + 1) % length`。
*   **切歌动作逻辑 (playSong)**：
    - 加载流地址 `audio.src = /api/music/play/${song.id}`。
    - 自动上报播放历史：调用 `useHistoryStore().addHistory(song.id)` 实时将播放记录同步给后端。
    - **执行封面本地缓存校验与物理引用释放（参见 `cache.md`）**：
      - 在 `player.ts` 中维护有私有的 `lastRetainedSongId` 和 `lastRetainedSongUrl`，以跟踪当前正在持有的 Blob URL。
      - 调用 `loadSongCover` 时，若本地缓存 IndexedDB 中存在封面 Blob，通过 `coverCacheManager.getOrCreateUrl` 生成 Blob URL。
      - 比对发现新生成的 Blob URL 与之前持有的相同，则直接复用；若不同，则**必须且立即**调用 `coverCacheManager.release` 物理销毁上一首封面大图的引用，再调用 `coverCacheManager.retain` 登记并持有当前新封面 Blob URL 的引用。
      - 若 IndexedDB 无缓存，则开启异步提取或多源刮削；获取成功后若缓存开关开启，将 Blob 二进制写入 IndexedDB 并复用 `loadSongCover` 的引用控制闭环更新 `currentSong.album_art` 与系统 `MediaSession`。
    - 触发 `audio.play()` 并将本地状态持久化存储。

