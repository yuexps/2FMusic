# folia.md — 2FMusic ↔ Folia-major 集成协议参考

## 嵌入模式

| 模式 | 入口 | URL 参数 | `isIframeMode` | `activePlaybackContext` 初值 | 自动跳转播放器页 |
|---|---|---|---|---|---|
| Folia新标签页 | “设置”页 → Folia Player | 无（可选 `netease_api`） | `false` | `'main'` | 否 |
| 全屏 Stage 从机 | `FullPlayerOverlay.vue` `foliaMode=true` | `from=FullPlayerOverlay` | `true` | `'stage'` | 是 |

---

## 关键判定逻辑

```typescript
// useStagePlaybackController.ts & IntegrationSettingsSubview.tsx & SettingsModal.tsx（三处独立判定，逻辑相同）
const isIframeMode =
    new URLSearchParams(window.location.search).get('from') === 'FullPlayerOverlay';

// useStagePlaybackController.ts — 网页端强制 now-playing
const stageSource: StageSource | null = isElectronWindow
    ? (stageStatus?.modeEnabled ? (stageStatus?.source ?? 'stage-api') : null)
    : 'now-playing';

// IntegrationSettingsSubview.tsx & SettingsModal.tsx — 非 Electron 强制 true，开关置灰 + isElectron && 门控点击
const enableNowPlayingStage = !isElectron || enableNowPlayingStageFromStore;

// App.tsx — 任意 iframe 嵌入场景默认 stage context
const isEmbedded = typeof window !== 'undefined' && window.self !== window.top;
const [activePlaybackContext, setActivePlaybackContext] =
    useState<PlaybackContext>(isEmbedded ? 'stage' : 'main');
```

---

## 宿主 → Folia：postMessage 载荷

广播入口：`sendToAllFoliaIframes(type, data)` — 遍历页面中 `src` 含 `folia/` 的所有 iframe。

| type | data 字段 | 触发时机 |
|---|---|---|
| `'2fmusic-track'` | `{id, title, author, album, cover, duration, liked?: boolean}` | `currentSong` 变化 / 收藏状态变化 |
| `'2fmusic-lyric'` | `{lrc: string, hasLyric: boolean}` | 歌词加载完成 |
| `'2fmusic-state'` | `{isPaused, progressMs, loopMode, volume?: number}` | `isPlaying`/`playMode`/`volume` 变化 |
| `'2fmusic-progress'` | `{progressMs: number}` | `currentTime` 每次变化（高频） |
| `'2fmusic-queue'` | `{queue: [{id,title,artist,album,cover,durationMs}]}` | `playlist` 变化 |

**cover 绝对化**（`App.vue` `getAbsoluteCoverUrl`）：
- `http(s)://` → 原样返回
- `/` 或 `api/` 开头 → `${window.location.origin}/${stripped}`
- 其余 → `getApiUrl(art)`

**loopMode 映射**（`sendCurrentStateToFolia` 内）：

| 2FMusic `playMode` | Folia `loopMode` |
|---|---|
| `'list'` | `'all'` |
| `'single'` | `'one'` |
| `'random'` | `'off'` |

---

## Folia → 宿主：反向控制事件

| type | 载荷 | 处理者 | 执行 |
|---|---|---|---|
| `'folia-ready'` | — | `App.vue` | `handleAllFoliaReady()` → 全量推送 track+lyric+state+queue |
| `'folia-toggle-play'` | — | `App.vue` | `playerStore.togglePlay()` |
| `'folia-next'` | — | `App.vue` | `playerStore.next()` |
| `'folia-prev'` | — | `App.vue` | `playerStore.prev()` |
| `'folia-seek'` | `{positionMs}` | `App.vue` | `playerStore.seek(positionMs / 1000)` |
| `'folia-toggle-loop'` | — | `App.vue` | 轮换 `playMode`（list→single→random），写 `localStorage['2fmusic_state'].playMode` |
| `'folia-play-song'` | `{id}` | `App.vue` | `playerStore.playSong(playlist.find(id))` |
| `'folia-toggle-like'` | `{id}` | `App.vue` | 触发收藏 / 取消收藏当前曲目 |
| `'folia-volume'` | `{volume}` | `App.vue` | 调节宿主当前播放器音量 |
| `'folia-shuffle-queue'` | — | `App.vue` | 调用 `playerStore.shufflePlaylist()` 打乱宿主播放列表 |
| `'folia-exit'` | — | **`FullPlayerOverlay.vue`** | `foliaMode.value = false`（卸载 iframe） |

---

## netease_api 透传

```typescript
// Settings.vue — Folia新标签页
const base = './folia/'
const url = api ? `${base}?netease_api=${encodeURIComponent(api)}` : base
window.open(url, '_blank')

// FullPlayerOverlay.vue — 全屏 Stage
const base = './folia/?from=FullPlayerOverlay'
return api ? `${base}&netease_api=${encodeURIComponent(api)}` : base
```

---

## nowPlayingContentLoadKeyRef 缓存机制

- 文件：`useStagePlaybackController.ts` L191，L1314-1335
- 用途：去重 key，防止同一内容重复载入
- 重置时机：`activePlaybackContext` 离开 `'stage'` 时置 `null`（L1331-1335），保证下次进入舞台强制重载

---

## from=FullPlayerOverlay（即 fromFullPlayerOverlay === true）自定义与额外解锁

- `isNowPlayingControlDisabled` 强制 `false`（解除 Stage 模式 UI 置灰）。
- 拦截全局播歌与收藏函数：播放单曲、点击收藏及点击随机播放（Shuffle）时均拦截原本地行为，改为发送 `folia-play-song`、`folia-toggle-like` 或 `folia-shuffle-queue` 消息给宿主。
- 移除右上角的“经典模式”悬浮按钮。
- 左上角鼠标悬浮的返回按钮，劫持其回调为向宿主发送 `folia-exit` 消息。
- 右下角卡片的“播放列表（Queue）” Tab 不被屏蔽，即使是在 `isStage === true` 下也照常显示播放队列。
- 右下角卡片的左上角设置按钮置灰（`disabled`）且不可点击，左下角返回主页按钮劫持为发送 `folia-exit` 消息且 title 显示“返回经典模式”。
- 屏蔽右下角菜单按钮的左划手势唤出功能。
- 屏蔽 `usePlaybackInteractionBridge` 中非播放控制的快捷键（只允许 `Space`/`ArrowLeft`/`ArrowRight`），并屏蔽所有其他组件的全局 `keydown` 快捷键（`KeyS`/`KeyH`/`KeyP`/字符输入等）。
- 歌词匹配 UI 完全开放（在线匹配 / 清除匹配）。

---

## Stage 歌曲 ID 与 IndexedDB 缓存（isIframeMode === true 专用）

- 歌曲 ID：基于 `(title, artist)` 哈希的**稳定负整数**，非随机临时值
- 载入时检索对应 ID 的 `onlineLyricsState` 缓存；命中则覆盖推送的默认歌词

---

## AI 主题配色与后端安全代理协议

在 2FMusic 宿主的“设置”页面中，统一一站式配置 Folia 大模型配色参数（包括开关、OpenAI 接口 URL、Model、API Key 和可选的自定义 HTTP 出站代理地址）。

### 1. 配置读取与持久化（WebSocket 契约）

宿主前端通过 WebSocket 与 Python 后端进行深层通信读写配置，将偏好漫游写入 SQLite 数据库的 `system_settings` 中。

| 消息 Action | 载荷参数 | 返回数据 | 触发场景 |
|---|---|---|---|
| `'system/get_folia_ai_config'` | 无 | `{folia_enable_ai: boolean, openai_url: string, openai_model: string, openai_key: string, openai_proxy: string}` | 前端初始化 / 设置页加载 |
| `'system/save_folia_ai_config'` | `{enable_ai, openai_url, openai_model, openai_key, openai_proxy}` | `{success: boolean, error?: string}` | 宿主设置项修改时自动触发 |

为了保障绝对的安全凭证隐私，**API Key、URL 等敏感字段严禁写入客户端浏览器本地 localStorage 缓存**（前端会自动做拦截和历史残留清除，仅暴露无害的 `folia_enable_ai` 开关）。

### 2. 后端大模型请求代发代理（Flask 契约）

Folia 网页版 / 从机版启动大模型配色生成时，统一向同源后台路径发起 Post 请求，由后端代理“即用即取”地查询 SQLite 中配置的大模型密钥进行服务器代发。不仅消除了前端 CORS 跨域限制，而且保障了密钥的安全。

* **路由模块**：[folia.py](file:///d:/Users/yuyue/Documents/Code/2FMusic/server/core/routes/folia.py) (整合了 Folia 的 HTTP 路由及 WS 处理器，从 `music.py` 和 `system.py` 中解耦)。
* **API 请求路径**（客户端 fetch）：`/api/generate-theme_openai` 或 `/api/generate-theme`。
* **代理与前缀拼接**：
  * **baseUrl 自动感知**：Folia 客户端在 fetch 前，会调用 `get2FMusicBaseUrl()`（[path.ts](file:///d:/Users/yuyue/Documents/Code/2FMusic/folia-major/src/utils/path.ts)）动态拼接宿主反代的前缀（如 `https://example.com/music`），避免子目录部署时请求打在域名绝对根路径导致 404 挂死。
  * **HTTP 出站代理**：支持配置局域网/科学上网代理，后端通过 requests 附带 `proxies` 参数进行代理调用，从而完美解决国内网络环境下无法直连 OpenAI/DeepSeek 官方大模型端点的问题。
