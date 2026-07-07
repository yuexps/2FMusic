# 视图控制与多媒体映射契约 (docs/frontend/views.md)

## 1. 多 HTML 页面入口机制
*   **登录入口**：[login.html](../../frontend/login.html)，挂载 `LoginApp.vue`。负责密码的输入、SHA-256 计算鉴权及提交，登录成功后路由跳转并附加 `?next=` 回退地址。
*   **主应用入口**：[index.html](../../frontend/index.html)，挂载 `App.vue`（或通过 `main.ts` 分发）。如果检测到无鉴权 Cookie，则自动重定向至 `login.html`。

## 2. 车载蓝牙与锁屏滚动歌词映射 (MediaSession API)

### 2.1 锁屏滚动歌词原理
*   **物理映射机制**：智能手机锁屏或蓝牙车载大屏中，多媒体插件的 `artist`（歌手）和 `title`（歌曲名）是仅有的可渲染文本槽。
*   **业务逻辑实现**：
    - 前端播放器捕获到歌词更新时，调用 `updateLyric(lyricText)` 改变 `currentLyric` 值。
    - **系统同步**：更新 `navigator.mediaSession.metadata`：
      - `navigator.mediaSession.metadata.title` 映射为 `${song.title} - ${song.artist}` (将歌手拼入标题)。
      - `navigator.mediaSession.metadata.artist` **物理映射为当前的这行歌词文本 `currentLyric`**。
    - 这能在没有原生 LRC 显示支持的车载屏幕上，让蓝牙屏幕随着切歌或时间轴自动动态滚动显示正在唱的那一行歌词！

### 2.2 物理多媒体按键拦截
*   播放器加载时，必须向 `navigator.mediaSession` 登记以下多媒体物理按键行为处理器：
    - `setActionHandler('play', togglePlay)`：系统点击播放按钮。
    - `setActionHandler('pause', togglePlay)`：系统点击暂停。
    - `setActionHandler('previoustrack', prev)`：物理键盘切上一首。
    - `setActionHandler('nexttrack', next)`：物理键盘切下一首。
    - `setActionHandler('seekto', (details) => seek(details.seekTime))`：车载大屏拖动时间轴 Seek。
*   **位置状态同步**：在 `timeupdate` 和 `durationchange` 事件中，必须同步调用 `mediaSession.setPositionState`，更新播放器速度率、总时长和当前播放刻度，保证车载屏幕上的进度条与前端 100% 同步。

## 3. 视图容器布局规约
*   **双轨滚动架构**：
    - **虚拟滚动页面**（如本地音乐、播放记录、网易下载）：因含有海量项的虚拟列表，其根容器必须设置为 `flex flex-col flex-1 min-h-0` 结合 `overflow-hidden`，将滚动完全交由内部虚拟滚动组件接管。
    - **原生滚动页面**（如系统设置、我的收藏、目录管理）：因内容数量少且静态/动态面板高度不固定，其根容器直接使用常规流式布局（不带 `flex-1 min-h-0` 限制），将滚动条自然交由外层的主滚动区 `.main-scroll` 接管，以彻底杜绝虚拟化计算不准带来的底部截断 bug。
*   **宽度限制禁令**：严禁在任何视图的根容器上硬编码 `max-w-[1040px]` 等最大宽度，所有视图在水平维度一律全宽展示，保持整站设计语言一致。

## 4. 专辑与歌手视图聚合规约
*   **专辑视图聚合 (去碎与防碰撞)**：前端使用 `专辑名称 + 物理父目录` (同一张专辑本地文件通常在同一物理文件夹下) 作为聚合 Key。这可防止不同歌手的同名专辑碰撞合并，同时能保证整张合辑被收纳在同一个专辑入口中。
*   **专辑歌手呈现**：聚合后，优先使用后端解析出的标准的 `album_artist`（专辑艺术家）字段。若该字段缺失或未扫描（为 `null`），则无缝回退到基于单曲歌手的动态推导逻辑：若专辑内所有曲目歌手一致，则专辑歌手显示为该歌手；若包含多位歌手，则自动显示为 `"群星"`。
*   **歌手视图聚合**：歌手数据基于前端对歌曲 `artist` 字段按照分割符（如 `/`、`,`、`，`、`、`）拆分后聚合产生，首张有封面的歌曲封面作为该歌手的临时头像。


## 5. Folia 全屏播放器内嵌模式 (Folia 模式) 契约

*   **iframe 容器挂载**：在 `FullPlayerOverlay.vue` 中支持一键进入 Folia 模式，以 `absolute inset-0 z-10 w-full h-full` 的全屏 iframe 容器加载 `./folia/index.html?mode=iframe`。
*   **通信总线规范**：2FMusic 宿主和 Folia iframe 之间采用同源 `postMessage` 进行双向控制与数据同步，杜绝中介代理或网络转发：
    - **正向数据推送 (2FMusic -> Folia)**：
        - `2fmusic-track`：推送正在播放的歌曲元数据（ID、歌名、歌手、专辑、封面 URL、总时长）。
        - `2fmusic-lyric`：推送当前拉取并解析成功的歌词纯文本。
        - `2fmusic-state`：推送包含播放状态 `isPaused` 和精准毫秒级播放时间 `progressMs` 的复合包，由 Folia 进行插值时钟校准。
    - **反向遥控控制 (Folia -> 2FMusic)**：
        - `folia-exit`：用户在 Folia 右上角点击“经典模式”返回，2FMusic 切回常规播放器分栏界面。
        - `folia-toggle-play`：Folia 劫持其自身的播放/暂停动作并发送，2FMusic 执行 `playerStore.togglePlay()`。
        - `folia-next` / `folia-prev`：Folia 劫持切歌并发送，2FMusic 执行 `playerStore.next()` / `playerStore.prev()`。
        - `folia-seek`：Folia 劫持跳转并发送目标毫秒值 `positionMs`，2FMusic 执行 `playerStore.seek(positionMs / 1000)`。
