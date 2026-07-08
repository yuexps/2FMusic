# 变更日志 (CHANGELOG.md)

所有针对 2FMusic 项目的技术升级、重构与规范改动均记录于此，以维持历史可追溯性。

---

## [2026-07-08]
- **Folia 大模型主题配色与后端持久化深层联动**：
  - 宿主一站式配置：宿主设置页新增大模型开关、URL、Key、Model 及 HTTP 代理输入，统一管理配置。
  - 安全代理请求：新建后端 [folia.py](file:///d:/Users/yuyue/Documents/Code/2FMusic/server/core/routes/folia.py) 模块安全代理请求，支持 HTTP 出站代理，规避 CORS 限制；敏感密钥禁止写入 `localStorage` 且自动清除历史残留，防浏览器明文泄露。
  - 子目录反代兼容：Folia 发包前拼接 `get2FMusicBaseUrl()` 感知 BaseUrl，解决反代子目录下请求根域名引发的 404 挂死。
  - 修复宿主 Pinia Store [system.ts](file:///d:/Users/yuyue/Documents/Code/2FMusic/frontend/src/stores/system.ts) 漏返 `foliaAiConfig` 状态导致的 Vue 类型编译 Bug。
  - 流水线集成：更新了 [.github/workflows/test-build.yml](file:///d:/Users/yuyue/Documents/Code/2FMusic/.github/workflows/test-build.yml) 自动化测试包构建流，加入对 Folia 子模块的拉取、npm 依赖缓存以及编译产物拷贝，打通完整前端集成自动打包。
- **Folia 右下角面板与界面定制优化**：
  - 隐藏账户入口：在从机 iframe 模式下剔除选项卡中的 `'account'` 账户入口。
  - 滑块动画重建：在选项卡切换处基于 Framer Motion 重建了灵动的物理阻尼平滑背景滑动效果。
- **Folia iframe 独立遥控定制化**：
  - 数据物理隔离：代理 `Storage.prototype` 读写，在从机模式下存取 Key 自动加 `overlay_` 前缀，隔离常规缓存。
  - 屏蔽引导弹窗：拦截从机模式下自动触发的版本更新及新手引导弹窗，精简 UI。
  - 兼容状态修正：修复 Electron 嵌入 iframe 下 `stageSource` 误判为 null 的问题，使其强制定向至 `now-playing`。
  - 遮罩等待体验：重构了从机模式下“等待 now-playing”的遮罩层提示，不显示服务报错信息，而是呈现“等待宿主播放歌曲，请在 2FMusic 播放器中选择歌曲并播放”的一体化引导。
  - 同源无闪现初始同步：宿主广播时同步将歌曲、歌词、播放列表和状态挂载在全局 `window` 上；Folia 启动第一帧直接通过同源特权读取 `window.parent` 上的缓存作为 React 初始状态。从时序上彻底消灭了 postMessage 握手时延导致的 0.x 秒画面闪现，且保证了从机播放列表（Queue）首帧即 100% 完整展现，绝不为空。
  - 拦截从机播放列表清空：修复了 `loadNowPlayingIntoPlayback` 机制在解析及刷新 Now Playing 轨道时无差别执行 `setPlayQueue([])` 的历史遗留 Bug。改为当处于 `fromFullPlayerOverlay` 状态时拦截清空操作，完美保留宿主推送和同源同步的播放列表，彻底根治了从机列表卡空的问题。
- **Folia 双向同步稳定性与控制链打通（播放队列随机/收藏按钮比对/默认列表就绪）**：
  - 修复了 Folia 端 `areTracksEqual` 比对函数未将 `liked` 字段纳入比对的缺陷。由于此过滤漏洞，当宿主更新并推送新的收藏状态时，Folia 错误地判定轨道“无变化”并直接 return 忽略，导致 Folia 端的收藏红心按钮无论点击多少次都无法点亮。
  - 修复了宿主 `App.vue` 初始化时未拉取默认收藏夹歌曲列表的问题。在 `onMounted` 钩子中补充了 `favoritesStore.fetchPlaylistSongs('default')` 自动拉取，使得首次判定有了正确的初始数据。
  - 实现了播放列表的“随机打乱（Shuffle）”控制双向同步。在宿主端 `player.ts` 引入 `shufflePlaylist` 动作（使用 Fisher-Yates 算法，保留当前正在播放曲目，打乱余下列表）；在 Folia 端 `App.tsx` 代理并劫持 Stage 模式下的随机播放按钮，将其拦截并发送 `folia-shuffle-queue` 消息给宿主。宿主响应后打乱列表并深度 watch 重新推送新列表至 Folia 刷新显示，形成闭环。
  - 在宿主 `App.vue` 消息处理器中增加了更详尽的 `console.log` 和 `warn` 日志，以便更直观地跟踪消息类型及列表中的歌曲匹配结构。
  - 解决了握手初期 iframe 中 shouldPublishNowPlayingState 为 false 导致接收宿主全量推送播放列表被丢弃的竞态 Bug。通过在 `useStagePlaybackController.ts` 中声明 `shouldPublishNowPlayingState` 时放开对 `fromFullPlayerOverlay` 为真（iframe 遥控端）的拦截门控，使其从生命周期第一帧起即可完美同步宿主全量播放列表，解决了“播放列表经常为空”的硬伤。
- **Folia Stage 模式双向同步升级（播放列表/收藏按钮/音量联动）**：
  - 解决了在 `fromFullPlayerOverlay` 场景下，Folia 的 Queue 播放列表点击歌曲时未能向父窗口发送 `folia-play-song` postMessage 导致同步失效的 Bug。将全局 `playSong` 拦截层提前至 hook 解构后原地包装，使所有侧边栏及主控 UI 调用的播歌逻辑均能被正确拦截。
  - 修复了 Folia 播放列表（Queue）点击失效和字段渲染空白问题。在 `useStagePlaybackController.ts` 的 `onQueue` 消息回调里增加 `normalizeQueueSong` 规范化映射函数，将宿主原始的 `{ title, artist }` 属性映射为标准的 `SongResult` 结构（`name` 与 `ar` 数组），彻底解决因字段缺失引发的渲染与交互故障。
  - 解决跨窗口传输 ID 类型（String / Number）不匹配导致的收藏及播放失效故障。在宿主端 `App.vue` 中对所有 `folia-play-song` 及 `folia-toggle-like` 匹配逻辑、以及 `sendCurrentTrackToFolia`   - 入口跳转形式改为直接在新标签页中打开 Folia Player，彻底规避了 iframe 在剪贴板权限、热键监听和屏幕尺寸等方面的限制，极大提升了交互的流畅度与视野。
  - 彻底删除了原本的 `/folia` 路由及废弃的 [FoliaPlayer.vue](frontend/src/views/FoliaPlayer.vue) 视图文件。
  - 更新了 [folia.md](docs/folia.md) 中的嵌入模式描述。进行强制 `String()` 转换后比对，彻底打通收藏状态与点击切歌；同时在 Folia 端 `QueueTab.tsx` 针对 Stage 模式适配了当前歌曲的高亮与自动滚动定位（基于歌名和歌手匹配）。
  - 新增并打通了“单曲收藏/喜欢”状态的同步。在 `2fmusic-track` 消息负载中拓展了 `liked` 字段以推送当前曲目的收藏状态；并引入了 `folia-toggle-like` 反向控制事件，当在 Folia 界面点击红心/收藏按钮时，会将收藏变更消息发送至宿主 2FMusic，由宿主 `favoritesStore` 修改收藏并重新广播状态。引入了轻量级 `liked` 的 `useEffect` 局部监听，实现完美且高效的收藏状态闭环。
  - 实现了音量的双向联动。宿主通过 `2fmusic-state` 携带并 watch 广播 `volume` 属性，Folia 收到后调用 `handleSetVolumeOriginal` 静默对齐音量；当 Folia 侧进行音量拖拽时，触发包装后的 `handleSetVolume` 拦截向宿主发送 `folia-volume` 消息，在实现同步调节的同时天然规避了更新环路。

### 优化
- **Folia 全功能入口体验优化**：
  - 将“辞曲新境 (Folia)”全功能入口从侧边栏移除，改到“系统设置”页面中作为一个独立的卡片选项呈现。


## [2026-07-07]
### 新增
- **侧边栏集成独立 Folia 视图**：侧边栏新增“辞曲新境”入口，复用 Folia 完整页面。在未开启“舞台”模式下默认为独立播放器逻辑，保留完整功能交互，不使用全屏播放页 iframe 自动跳转与劫持逻辑。
- **Stage 模式歌词在线匹配与缓存重载**：在全屏 iframe 模式下放行歌词匹配 UI。基于歌曲标题和歌手计算稳定的负数歌曲 ID 作为标识，歌曲加载时自动读取 IndexedDB 缓存中的在线匹配歌词。
- **全屏播放器 Folia 模式**：[FullPlayerOverlay.vue](frontend/src/components/FullPlayerOverlay.vue) 顶部 Header 引入 Folia 模式切换，通过独立全屏 iframe 嵌入 Folia 界面。
- **Folia 握手就绪协议**：引入 `folia-ready` 反向通信信号，在 Folia 完成 `start()` 挂载后通知宿主，规避加载初期的推数竞态。
- **播放状态与队列双向遥控**：主机同步播放状态、循环模式及播放队列至 Folia（`2fmusic-state` / `2fmusic-queue`）；Folia 侧边栏及队列切歌事件反向控制主机播放（`folia-play-song` / `folia-toggle-loop`）。
- **多源自动歌词检索**：当 Stage 推送无精确词轨时，后台静默通过网易云、QQ 音乐、酷狗、AMLLDB 接口检索并下载匹配高精度逐字歌词。
- **外部 Base URL 透传**：Folia 自动从 query 参数 `netease_api` 中动态抓取当前 2FMusic 后端的 API 基址，实现多渠道云端歌词搜索。
- **经典模式悬浮按钮**：在 `folia-major/src/App.tsx` 中注入“经典模式”返回按钮（磨砂玻璃悬浮风格），当点击时向父窗口发送 `folia-exit`，通知宿主退回到经典播放模式。

### 修复
- **接口 401 及代理 404 报错**：在 Python 后端实现 `/api/lyric-proxy` 白名单代理路由（支持 QQ、酷狗、AMLLDB），并忽略 `accept-encoding` 头以解压缩响应数据，避免乱码。在 Folia 端请求中移除 `credentials: 'omit'` 以在同源请求时携带 Session 凭证。
- **解除 iframe 下的控制及按键锁定**：解除 Stage 模式下的 `isNowPlayingControlDisabled` 限制，恢复空格及左右键控制；修复在无物理音频流时播放状态 Toggle 的点击禁用问题。
- **屏蔽本地 Progress 接口报错**：在 `queryNowPlayingPreciseProgress` 轮询前拦截 `isIframeMode`，阻止持续对本地 9863 端口发起网络请求引发的连接拒绝错误。

### 优化
- **isIframeMode 条件解耦**：移除对 `window.self !== window.top` 的依赖，解耦 iframe 状态判定，仅对具备特定 query 参数的 FullPlayerOverlay 场景生效，隔离侧边栏常规内嵌场景。
- **Now Playing 设置项锁定**：在非 Electron 网页端（包含侧边栏与独立网页版），将 `enableNowPlayingStage` 强制置为 `true`。置灰锁定“启用 Now Playing”设置开关并限制点击，简化设置界面。
- **主工作区填充与样式优化**：移除 [FullPlayerOverlay.vue](frontend/src/components/FullPlayerOverlay.vue) 中的脉冲绿点并加入分割线。调整 [App.vue](frontend/src/App.vue) 中 `<router-view>` 容器的布局，处于 `/folia` 路由时自适应填充 100% 窗口，移除 Padding 缩进。
- **2FMusic 全局持久化 Folia 广播**：重构了广播机制，将数据推送与反向遥控逻辑统一上移至宿主根组件 [App.vue](frontend/src/App.vue)。支持持续向页面上所有 Folia 相关 iframe 广播，同时在 Folia 端 `nowPlayingProvider.ts` 开启了对非全屏模式 iframe 内嵌场景下的 message 消息接收，实现“辞曲新境”等常规内嵌页面在切换到“舞台”模式时对宿主播放状态的静默接收与同步。
- **FullPlayerOverlay.vue 冗余变量清理**：删除了从未被读取且无用的 `foliaIframe` 变量声明与模板 `ref="foliaIframe"` 绑定属性，清空了相关的控制台警告提示。
- **侧边栏图标更新**：在 `SvgIcon.vue` 中导入并映射了 `DiamondOutline` 图标，并将侧边栏菜单“辞曲新境”的图标升级为更具精致切面与设计美感的 `diamond` 图标。
- 升级 `build_frontend.cmd` 构建流程，引入 `BUILD_BASE=./` 环境变量支持，实现自动打包编译 Folia-major 播放器并部署至静态资源子目录 `www/folia` 的闭环链路。

## [2026-07-06]
### 新增
- 在 [LocalMusic.vue](frontend/src/views/LocalMusic.vue) 中实现了本地音乐的单曲、歌手、专辑、物理文件夹 4 个视图的聚合显示，支持 `localStorage` 视图记忆。
- 在“歌手”与“专辑”视图中接入了 A-Z 拼音姓名、修改时间（新曲入库置顶）以及歌曲数量排序的综合适配。
- 为“文件夹”视图重构了逐级下钻的物理面包屑文件浏览器，自适应在前端计算出了默认曲库的绝对根路径，支持多挂载盘符归并。
- 在后端 SQLite 中拓展了 [songs](docs/server/db.md) 表结构，新增 `album_artist` 字段，并在 [db.py](server/core/models/db.py) 中实现数据库平滑升级逻辑。
- 升级元数据解析引擎 [metadata.py](server/core/services/metadata.py)，支持 EasyID3、FLAC/Vorbis、MP4 等主流标签中专辑艺术家（albumartist）信息的解析。
- 升级前端 [types.ts](frontend/src/types.ts) 的 `Song` 接口与 [LocalMusic.vue](frontend/src/views/LocalMusic.vue) 专辑聚合逻辑，优先使用 `album_artist` 展现并提供群星回退兼容。

### 修复
- 修复了后端 [song.py](server/core/models/song.py) 中的 `get_all_songs_deduplicated` 接口未返回 `path` 字段，导致前端 [LocalMusic.vue](frontend/src/views/LocalMusic.vue) 文件夹视图在展开具体目录时路径过滤逻辑失效、显示为空的 Bug。
- 修复了 [LocalMusic.vue](frontend/src/views/LocalMusic.vue) 在部署至子路径下时，其歌手头像、专辑封面及详情抽屉封面的 `:src` 未能适配 Base URL 导致加载 404 的 Bug。引入了 `getApiUrl()` 对这些图片地址进行了幂等转换。
- 修复了网易云下载时长时间卡在“排队中”（`pending`）状态，并在结束后瞬间拉满进度条的问题。调整了 [downloader.py](server/core/services/downloader.py) 中 [run_download_task](server/core/services/downloader.py#L356) 状态机流转，优先切为 `preparing`，并增加防御性 `try...except` 块。
- 修复了详情抽屉和文件夹点歌时，播放队列上下文（`playlist`）没有切换导致下一首播放断裂的体验问题。
- 修复了下载管理按钮在出现正在任务数角标时，内部 Flex 布局挤压导致下载图标偏斜的 Bug，改为使用官方标准的 [NBadge](frontend/src/views/NeteaseDownloader.vue) 组件包裹。

### 优化
- 优化了 [LocalMusic.vue](frontend/src/views/LocalMusic.vue) 文件夹下钻视图中的单曲展示排版，引入规范的 `song-grid-header` 与 `.song-row` 布局，补全专辑封面图、高亮播放图标，并对齐行双击播放、右键上下文菜单和批量多选操作，消除其排版与功能体验的割裂感。
- 重构了 [LocalMusic.vue](frontend/src/views/LocalMusic.vue) 的本地专辑聚合算法。改用“专辑名 + 物理父目录”作为聚合 Key，防止不同歌手同名专辑碰撞，同时根据曲目歌手情况动态识别并归并为“群星”合辑，彻底解决多歌手/合唱曲目导致同一张专辑在界面被切碎成多个独立同名专辑的体验硬伤。
- 优化了三大歌曲表格（本地音乐、播放记录、网易下载）在宽屏下的列宽排版，在大小/音质与操作菜单之间引入了 `.col-spacer`（最大限制为 120px）弹性空列占位符。在保证长歌名自适应宽幅显示的同时，防止操作按钮在宽屏下被无限抛出，视线聚焦更为紧实。
- 同步更新了 [services.md](docs/server/services.md) 状态流转规范。

## [2026-06-15]
### 变更
- 修改了 `fn_build/cmd/main` 中的启动参数，由硬编码的端口 `23237` 改为读取向导端口配置 `${wizard_port:-23237}`。
- 在 `fn_build/wizard/install` 和 `fn_build/wizard/config` 中引入了 `wizard_run_mode` 运行方式配置项（支持并发模式、纯端口模式及纯 Socket 模式）。
- 修改了 `fn_build/cmd/main`，根据 `wizard_run_mode` 环境变量动态拼接 `--port` 和 `--unix-socket` 启动参数。
- 同步更新了 `docs/server/communication.md` 以明确 FNAS 部署脚本的动态端口与自定义运行方式规范。
- 在 `fn_build/wizard/install` 和 `fn_build/wizard/config` 中引入了 `wizard_music_library_type` 配置项（可选共享目录 `share` 与数据目录 `data`）。
- 修改了 `fn_build/cmd/main`，根据 `wizard_music_library_type` 动态设置 `MUSIC_LIBRARY_PATH`，并在选用数据目录时自动执行创建。
- 同步更新了 `docs/server/communication.md` 明确了音乐库物理路径配置的适配规范。

### 修复
- 修复了设置页面中点击“选择图片”按钮时抛出 `TypeError: o.value?.click is not a function` 错误从而导致“无法选择背景图片”的问题。
- 将 `Settings.vue` 里的 `ref="fileInputRef"` 改用原生 DOM 属性 `id="bg-file-input"` 定位，并替换为原生 `document.getElementById('bg-file-input')?.click()` 触发调用，彻底解决了 Vite 生产环境混淆分包可能导致 ref 引用失效的 Bug。

