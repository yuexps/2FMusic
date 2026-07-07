# folia.md — 2FMusic ↔ Folia-major 集成协议参考

## 嵌入模式

| 模式 | 入口 | URL 参数 | `isIframeMode` | `activePlaybackContext` 初值 | 自动跳转播放器页 |
|---|---|---|---|---|---|
| 侧边栏完整体 | `#/folia` → `FoliaPlayer.vue` | 无（可选 `netease_api`） | `false` | `'main'` | 否 |
| 全屏 Stage 从机 | `FullPlayerOverlay.vue` `foliaMode=true` | `mode=iframe&from=FullPlayerOverlay` | `true` | `'stage'` | 是 |

---

## 关键判定逻辑

```typescript
// useStagePlaybackController.ts & IntegrationSettingsSubview.tsx & SettingsModal.tsx（三处独立判定，逻辑相同）
const isIframeMode =
    new URLSearchParams(window.location.search).get('mode') === 'iframe' &&
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
| `'2fmusic-track'` | `{id, title, author, album, cover, duration}` | `currentSong` 变化 |
| `'2fmusic-lyric'` | `{lrc: string, hasLyric: boolean}` | 歌词加载完成 |
| `'2fmusic-state'` | `{isPaused, progressMs, loopMode}` | `isPlaying`/`playMode` 变化 |
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
| `'folia-exit'` | — | **`FullPlayerOverlay.vue`** | `foliaMode.value = false`（卸载 iframe） |

---

## netease_api 透传

```typescript
// FoliaPlayer.vue — 侧边栏
const base = './folia/index.html'
return api ? `${base}?netease_api=${encodeURIComponent(api)}` : base

// FullPlayerOverlay.vue — 全屏 Stage
const base = './folia/index.html?mode=iframe&from=FullPlayerOverlay'
return api ? `${base}&netease_api=${encodeURIComponent(api)}` : base
```

---

## nowPlayingContentLoadKeyRef 缓存机制

- 文件：`useStagePlaybackController.ts` L191，L1314-1335
- 用途：去重 key，防止同一内容重复载入
- 重置时机：`activePlaybackContext` 离开 `'stage'` 时置 `null`（L1331-1335），保证下次进入舞台强制重载

---

## isIframeMode === true 额外解锁

- `isNowPlayingControlDisabled` 强制 `false`（解除 Stage 模式 UI 置灰）
- 屏蔽 `usePlaybackInteractionBridge` 键盘热键（`Space`/`ArrowLeft`/`ArrowRight`）
- 歌词匹配 UI 完全开放（在线匹配 / 清除匹配）

---

## Stage 歌曲 ID 与 IndexedDB 缓存（isIframeMode === true 专用）

- 歌曲 ID：基于 `(title, artist)` 哈希的**稳定负整数**，非随机临时值
- 载入时检索对应 ID 的 `onlineLyricsState` 缓存；命中则覆盖推送的默认歌词
