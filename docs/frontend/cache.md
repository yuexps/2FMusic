# 封面与歌词缓存本地数据库规范 (docs/frontend/cache.md)

## 1. IndexedDB 物理缓存数据库 (`musicDB`)
*   **客户端工具**：[frontend/src/utils/indexedDB.ts](../../frontend/src/utils/indexedDB.ts)。
*   **数据库名**：`2fmusic_local_cache`（前端实例为统一导出的 `musicDB`，数据库版本为 `2`）。
*   **数据仓库设计 (Object Stores)**：
    - **`covers` 仓库**：以歌曲的 `song_id` (MD5 哈希值) 作为 Key，保存封面图片的物理 `Blob` 二进制块。
    - **`lyrics` 仓库**：以歌曲的 `song_id` 作为 Key，保存对应的歌词原文本（LRC / YRC 格式字符串）。
    - **`background` 仓库**：以唯一的 `'local_bg'` 作为 Key，保存一个包含 `{ blob: Blob, timestamp: string }` 的自定义背景图配置对象。
*   **缓存加载与运行机制**：
    - **封面缓存开关**：由本地存储中的 `2fmusic_cache_covers` (Boolean 值) 字段控制。若开启，每次切歌加载时，会优先进入 `covers` 仓库检索缓存的 `Blob`，加载成功后转化为 Object URL；若未命中则通过 WS 或 REST 接口按需拉取，并异步存入 `covers` 仓库。
    - **歌词与背景缓存**：按需在相应组件与视图中读写，系统清空缓存操作时通过事务统一物理清理 `covers`, `lyrics`, `background` 三个仓库。


## 2. 封面内存引用计数管理器 (`coverCacheManager`)
*   **客户端工具**：[frontend/src/utils/coverCache.ts](../../frontend/src/utils/coverCache.ts)。
*   **痛点背景**：将 IndexedDB 二进制大图读取后，常调用 `URL.createObjectURL(blob)` 转换为图片地址渲染。由于大切歌场景频繁，如果直接丢弃不管，浏览器会积累大量未释放的 DOM Blob 缓存，导致系统内存泄露、前端崩溃。
*   **引用计数生命周期控制**：
    - 使用 `coverCacheManager` 统一代理 Blob URL。
    - **加载封面**：加载新封面时，调用 `coverCacheManager.retain(song_id, blobUrl)`。登记该 song_id 拥有当前 Blob URL 的一个有效引用。
    - **切歌释放**：在下一首歌曲加载时，**必须且立即**调用 `coverCacheManager.release(prev_song_id, prev_blobUrl)` 物理释放它。
    - 当 Blob URL 的引用计数归零时，内部物理调用 `URL.revokeObjectURL(url)`，确保物理内存立刻被垃圾回收。

### 2.1 Vue 自定义指令级组件周期绑定 (`v-cached-src`)
对于歌曲列表、网易云检索结果列表等场景，存在大量包含封面图片的 `<img>` 标签被挂载和卸载。为实现全自动、无感知的引用计数释放，前端设计了自定义指令 `v-cached-src`：
*   **绑定指令**：在 `<img>` 标签中通过 `v-cached-src="{ id: song.id, src: song.album_art }"` 绑定。
*   **自动注册与加载**：
    - 在指令的 `mounted` 与 `updated` 生命周期钩子中：
      1.  读取当前绑定的 `id`。若为空或缓存开关关闭，降级使用原生 `getApiUrl(src)` 直链。
      2.  若存在缓存，调用 `coverCacheManager.getOrCreateUrl(id)` 异步获取或生成 Blob URL 挂到 `el.src`。
      3.  为该图片元素绑定 `_cachedId` 和 `_cachedObjectUrl` 自定义属性，同时立即物理调用 `coverCacheManager.retain(id, url)` 增加该图片的引用计数。
    - **防时序冲突**：在图片元素上维护 `_lastReqId` 计数器。若异步从 IndexedDB 读取期间指令已被重复触发或切歌更新，自动舍弃过期回调，保证渲染的永远是最终正确的歌曲图片。
*   **自动注销与解绑**：
    - 在指令的 `unmounted` 生命周期钩子（即图片元素被 Vue 框架卸载、销毁，或者组件切换切出时）中：
      - 读取图片元素上绑定的 `_cachedId` 与 `_cachedObjectUrl`。
      - 自动物理触发一次 `coverCacheManager.release(_cachedId, _cachedObjectUrl)` 减少引用。
    - 这完成了**图片元素生命周期 $\leftrightarrow$ 物理 Blob 内存释放**的闭环，是规避海量列表切歌导致浏览器崩溃的底层红线。

