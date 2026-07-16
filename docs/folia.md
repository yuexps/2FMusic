# 2FMusic ↔ Folia-major 集成协议参考 (docs/folia.md)

## 嵌入模式

*   **Folia 新标签页模式**
    *   入口：`../frontend/src/views/Settings.vue` 中的 Folia Player 选项
    *   URL 参数：无（可选附带 `netease_api`）
    *   `fromFullPlayerOverlay`：`false`
    *   `activePlaybackContext` 初始值：`'main'`
    *   自动跳转播放器页：否
*   **全屏 Stage 模式**
    *   入口：`../frontend/src/components/FullPlayerOverlay.vue`（当 `foliaMode === true` 时以 iframe 载入）
    *   URL 参数：`from=FullPlayerOverlay`
    *   `fromFullPlayerOverlay`：`true`
    *   `activePlaybackContext` 初始值：`'stage'`
    *   自动跳转播放器页：是

---

## 关键判定逻辑

```typescript
// ../folia-major/src/index.tsx & ../folia-major/src/App.tsx 等（统一判定）
const fromFullPlayerOverlay =
    new URLSearchParams(window.location.search).get('from') === 'FullPlayerOverlay';

// ../folia-major/src/hooks/useStagePlaybackController.ts — 网页端强制 now-playing
const stageSource: StageSource | null = isElectronWindow
    ? (stageStatus?.modeEnabled ? (stageStatus?.source ?? 'stage-api') : null)
    : 'now-playing';

// ../folia-major/src/components/modal/settings/IntegrationSettingsSubview.tsx & ../folia-major/src/components/modal/SettingsModal.tsx
const enableNowPlayingStage = !isElectron || enableNowPlayingStageFromStore;

// ../folia-major/src/App.tsx — 任意 iframe 嵌入场景默认使用 stage context
const isEmbedded = typeof window !== 'undefined' && window.self !== window.top;
const [activePlaybackContext, setActivePlaybackContext] =
    useState<PlaybackContext>(isEmbedded ? 'stage' : 'main');
```

---

## 宿主 → Folia：postMessage 载荷

广播入口：`sendToAllFoliaIframes(type, data)` — 遍历页面中 `src` 包含 `folia/` 的所有 iframe 元素。

*   **`'2fmusic-track'`**
    *   载荷数据：`{id, title, author, album, cover, duration, liked?: boolean}`
    *   触发时机：`currentSong` 变更或歌曲收藏状态变化时
*   **`'2fmusic-lyric'`**
    *   载荷数据：`{lrc: string, hasLyric: boolean}`
    *   触发时机：歌词文件加载完成时
*   **`'2fmusic-state'`**
    *   载荷数据：`{isPaused, progressMs, loopMode, volume?: number}`
    *   触发时机：`isPlaying`/`playMode`/`volume` 发生变化时
*   **`'2fmusic-progress'`**
    *   载荷数据：`{progressMs: number}`
    *   触发时机：`currentTime` 每次发生变化时（高频上报）
*   **`'2fmusic-queue'`**
    *   载荷数据：`{queue: [{id,title,artist,album,cover,durationMs}]}`
    *   触发时机：`playlist` 播放队列发生变化时

**时序与高精度对齐机制**：
*   **切歌立即同步**：在切换歌曲（`currentSong` 变更）时，在发送 `'2fmusic-track'` 的同时，必须立即补发一次 `'2fmusic-state'`，清零展示端的播放起点。
*   **异步装载同步**：由于歌词通过异步网络请求拉取，拉取完成并发送 `'2fmusic-lyric'` 后，必须立即补发一次 `'2fmusic-state'`，将网络缓冲期间宿主已前移的真实播放进度重新对齐给展示端。
*   **Seek 纠偏防抖**：在展示端发生 Seek 动作（发送 `'folia-seek'`）后的 1 秒（1000ms）时间窗口内，展示端必须主动忽略来自宿主的所有高频进度纠偏 `'2fmusic-progress'` 广播，以防止由于网络往返延迟导致的进度条及歌词拉扯回弹；宿主在执行 seek 完毕后，可补发一次 `'2fmusic-state'` 消息以让展示端立即建立新的时钟基准。

**cover 绝对化**（位于 `../frontend/src/App.vue` 中的 `getAbsoluteCoverUrl`）：
*   `http(s)://` -> 原样返回
*   `/` 或 `api/` 开头 -> `${window.location.origin}/${stripped}`
*   其余 -> `getApiUrl(art)`

**loopMode 映射**（在 `sendCurrentStateToFolia` 中定义）：
*   2FMusic `'list'` -> Folia `'all'`
*   2FMusic `'single'` -> Folia `'one'`
*   2FMusic `'random'` -> Folia `'off'`

**同源跨标签页通信 (BroadcastChannel)**：
为了支持 Folia 在独立新标签页中运行，引入了 BroadcastChannel 频道 `'2fmusic-folia-sync-channel'`。
* 宿主端不仅通过 `postMessage` 向 iframe 广播数据，也向该 Channel 投递相同格式的消息 `{type, data}`。
* 展示端（包括 iframe、popup 或独立标签页）的反向控制指令在向父窗口 `postMessage` 的同时，也一并向该 Channel 广播。宿主在接收到 Channel 指令时以相同逻辑处理，实现跨 Tab 级别的双向播放同步。

---

## Folia → 宿主：反向控制事件

*   **`'folia-ready'`**
    *   载荷数据：无
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：调用 `handleAllFoliaReady()`，向 iframe 全量推送 track + lyric + state + queue
*   **`'folia-toggle-play'`**
    *   载荷数据：无
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：调用 `playerStore.togglePlay()` 切换播放状态
*   **`'folia-next'`**
    *   载荷数据：无
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：调用 `playerStore.next()` 切换至下一首
*   **`'folia-prev'`**
    *   载荷数据：无
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：调用 `playerStore.prev()` 切换至上一首
*   **`'folia-seek'`**
    *   载荷数据：`{positionMs}`
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：调用 `playerStore.seek(positionMs / 1000)` 跳转音频播放进度
*   **`'folia-toggle-loop'`**
    *   载荷数据：无
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：轮换切换 `playMode`（list -> single -> random）并将本地配置状态持久化
*   **`'folia-play-song'`**
    *   载荷数据：`{id}`
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：从播放队列检索对应歌曲，执行 `playerStore.playSong(song)`
*   **`'folia-toggle-like'`**
    *   载荷数据：`{id}`
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：对当前曲目执行收藏或取消收藏操作
*   **`'folia-volume'`**
    *   载荷数据：`{volume}`
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：调节宿主播放器音量大小
*   **`'folia-shuffle-queue'`**
    *   载荷数据：无
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：调用 `playerStore.shufflePlaylist()` 对播放列表进行洗牌
*   **`'folia-remove-song'`**
    *   载荷数据：`{index}`
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：在播放列表 `playerStore.playlist` 中删除该索引处的歌曲，同步存入 `localStorage`。如果被删的是当前播放曲目，执行防悬空保护（自动切入下一首或列表为空时停止播放）。
*   **`'folia-move-song-to-end'`**
    *   载荷数据：`{index}`
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：将该索引处的歌曲从播放列表中提取，追加移动到列表最末尾，同步存入 `localStorage`。
*   **`'folia-move-song-to-next'`**
    *   载荷数据：`{index}`
    *   宿主处理者：`../frontend/src/App.vue`
    *   行为执行：将该索引处的歌曲移动到当前正在播放歌曲的下一首（即 `currentIndex + 1` 位置），同步存入 `localStorage`。如操作的是当前歌曲本身，则静默忽略。
*   **`'folia-exit'`**
    *   载荷数据：无
    *   宿主处理者：`../frontend/src/components/FullPlayerOverlay.vue`
    *   行为执行：设置 `foliaMode.value = false`（卸载 iframe 并退出全屏 Stage 模式）


---

## netease_api 透传

```typescript
// ../frontend/src/views/Settings.vue — Folia新标签页
const base = './folia/'
const url = api ? `${base}?netease_api=${encodeURIComponent(api)}` : base
window.open(url, '_blank')

// ../frontend/src/components/FullPlayerOverlay.vue — 全屏 Stage
const base = './folia/?from=FullPlayerOverlay'
return api ? `${base}&netease_api=${encodeURIComponent(api)}` : base
```

---

## nowPlayingContentLoadKeyRef 缓存机制

*   文件路径：`../folia-major/src/hooks/useStagePlaybackController.ts`（L191，L1314-1335）
*   功能用途：缓存去重 Key，防止相同内容重复加载
*   重置时机：在 `activePlaybackContext` 离开 `'stage'` 状态时置为 `null`，确保下次进入舞台时强制重载内容

---

## from=FullPlayerOverlay（即 fromFullPlayerOverlay === true）行为定制与解锁

*   强制将 `isNowPlayingControlDisabled` 设为 `false`（解除全屏 Stage 模式下 UI 的置灰及禁用状态）。
*   拦截全局播歌及收藏功能：播放单曲、触发收藏、随机洗牌播放时均拦截本地原生行为，改为向宿主发送相应的 `'folia-play-song'`、`'folia-toggle-like'` 或 `'folia-shuffle-queue'` 消息。
*   隐藏右上角的标准模式悬浮切换按钮。
*   劫持左上角返回按钮的点击回调为发送 `'folia-exit'` 消息给宿主。
*   右下角卡片的“播放列表 (Queue)”标签正常启用，在 `isStage === true` 下亦提供渲染支持。
*   置灰右下角卡片的设置按钮，且将其中的“返回主页”按钮劫持为向宿主发送 `'folia-exit'` 消息。
*   禁用右下角菜单按钮的滑动唤出侧边栏手势。
*   屏蔽 `usePlaybackInteractionBridge` 模块中除常规播放控制按键（`Space`/`ArrowLeft`/`ArrowRight`）外的全局键盘 `keydown` 拦截与全局快捷键逻辑。
*   完全放开在线歌词匹配交互 UI（包括在线匹配与清除匹配）。

---

## Stage 歌曲 ID 与 IndexedDB 缓存（fromFullPlayerOverlay === true 专用）

*   歌曲 ID 生成：基于 `(title, artist)` 计算出的哈希生成稳定负整数，以保证唯一且不易漂移。
*   加载逻辑：根据稳定 ID 在本地 IndexedDB 缓存中查询 `onlineLyricsState`。如存在，则用缓存的歌词覆盖推送的默认歌词。

---

## AI 主题配色与后端安全代理协议

在 2FMusic 宿主的配置页面中，支持一站式配制 Folia 大模型配色参数（含启用开关、API URL、Model、API Key 及可选的出站代理配置）。

### 1. 配置读取与持久化（WebSocket 接口协议）

宿主前端通过 WebSocket 连接与后端进行通信读写配置，并将其持久化在 SQLite 的 `system_settings` 表中。

*   **`'system/get_folia_ai_config'`**
    *   发送参数：无
    *   返回参数：`{folia_enable_ai: boolean, openai_url: string, openai_model: string, openai_key: string, openai_proxy: string}`
    *   触发场景：宿主前端初始化或配置页面加载时
*   **`'system/save_folia_ai_config'`**
    *   发送参数：`{enable_ai, openai_url, openai_model, openai_key, openai_proxy}`
    *   返回参数：`{success: boolean, error?: string}`
    *   触发场景：在宿主配置页面中修改 AI 相关配置时自动触发

为了保证安全凭证隐私，**API Key、URL 等敏感字段禁止在客户端 localStorage 中进行持久化缓存**。

### 2. 后端大模型请求代理（HTTP 请求接口协议）

Folia 生成 AI 主题配色时，向宿主同源后端接口发起 Post 请求，由后端代理读取 SQLite 凭证进行请求代发，以绕过前端跨域限制，并保护 Key 安全。

*   **路由模块**：`../server/core/routes/folia.py` (整合了 Folia 的 HTTP 路由及 WebSocket 处理器，解耦自 `music.py` 和 `system.py`)
*   **API 请求路径**：`/api/generate-theme_openai` 或 `/api/generate-theme`
*   **代理与前缀拼接**：
    *   **baseUrl 动态感知**：客户端调用 `../folia-major/src/utils/path.ts` 中的 `get2FMusicBaseUrl()` 动态拼装子路径前缀，避免在非根路径部署时出现 404 错误。
    *   **HTTP 出站代理**：后端支持通过 `proxies` 参数进行代理调用，以解决服务器环境无法连接官方大模型端点的问题。



---

## 上游同步合并与回归适配指南

当未来需要拉取合并上游 `folia-major` 的最新 Commit 时，建议遵循以下流程进行兼容性核对与适配，防止功能丢失：

### 1. 代码层面核对（宿主集成代理）
*   **确保 Hook 兼容**：上游合并后，首先核对 `App.tsx` 中的 `useFoliaHostBridge` 调用及 `recordLocalSeek` 的绑定是否被冲掉。
*   **依赖解耦**：所有从 `wrappedCallbacks` 中导出的方法（如 `removeQueueSong` 等），如上游组件（如 `UnifiedPanel`）有重构，只需将这几项包装后的方法对齐传入其新参数即可，无需碰其内部逻辑。

### 2. 回归验证优先级列表
*   **时钟平滑性与 Seek 防抖**：手动拖动进度条，确保 ignore 隔离（1s内不回弹）在最新的渲染机制中工作正常。
*   **同源 BroadcastChannel 跨 Tab 联调**：用新标签页独立打开 `./folia/` 并在宿主端切歌、点击播放，验证同源 BroadcastChannel 在无 DOM 关系下是否能稳定对准。
*   **播放队列操纵**：在列表里点击删除和置于下一首，验证宿主列表是否保持强同步更改。

