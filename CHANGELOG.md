# 变更日志 (CHANGELOG.md)

所有针对 2FMusic 项目的技术升级、重构与规范改动均记录于此，以维持历史可追溯性。

---

## [2026-07-08]
- **网易云 API 扫码与代理异常防崩溃优化**：
  - 更新 [docs/frontend/communication.md](./docs/frontend/communication.md) 和 [docs/server/communication.md](./docs/server/communication.md) 补全非 JSON 兼容解析及代理过滤规约。
  - 优化 [netease.ts](./folia-major/src/services/netease.ts) 中的 `fetchWithCreds` 接口解析逻辑，支持 `res.clone().json()` 尝试，失败时 fallback 至 `res.text()` 并兼容前端单元测试的 Mock 行为，避免 502 等 HTML 报错导致解析崩溃。
  - 修复 [netease.py](./server/core/routes/netease.py) 中的 `proxy_netease_api` 转发代理，在返回前将响应头中的 `Set-Cookie` 物理剔除，从源头根治扫码成功瞬间由超长 Header 导致的反代 502 Bad Gateway 报错。
- **全屏播放模式切换优化**：
  - 重构顶部导航为 Segmented Control 滑块菜单，支持“经典全屏”与“Folia”模式切换与本地偏好持久化。
  - 优化模式切换下的 Header 层级、磨砂背景渲染，以及切回经典播放器时的歌词自动重新对齐。
- **Folia 大模型主题配色与后端持久化联动**：
  - 支持在宿主设置页一站式配置 AI 配色，敏感密钥由后端 [folia.py](./server/core/routes/folia.py) 模块安全代发与出站代理，避免本地浏览器泄露。
  - 客户端通过 `get2FMusicBaseUrl()` 动态感知 BaseUrl 兼容子目录反代，修复 `foliaAiConfig` 漏返引发的 Vue 类型编译 Bug。
  - 更新 [.github/workflows/test-build.yml](./.github/workflows/test-build.yml) 自动化测试流以支持子模块拉取与自动集成打包。
- **Folia 界面与独立遥控优化**：
  - 移除 iframe 模式下的 `'account'` 选项卡，基于 Framer Motion 重建了 Tab 背景物理滑动动画。
  - 对 Iframe 从机模式的 localStorage 键自动加 `overlay_` 前缀隔离，屏蔽新手引导和自动更新弹窗，重构了统一的等待音频提示遮罩。
  - 优化 Iframe 从机初始化时序，支持从宿主根组件直接读取全局缓存以实现无延迟首帧首屏同步，彻底根治从机列表首帧卡空问题。
- **Folia 双向同步与控制链稳定性打通**：
  - 修复 `areTracksEqual` 比对遗漏 `liked` 字段导致收藏红心按钮无法点亮的 Bug，并在宿主初始化时补全默认收藏夹拉取。
  - 引入 Fisher-Yates 算法实现了播放列表随机打乱（Shuffle）的双向控制与状态同步。
  - 解决 iframe 握手竞态 Bug，放开从机端 `shouldPublishNowPlayingState` 门控以保证在初始化第一帧即完美同步宿主播放列表。
- **Folia Stage 模式多维联动升级**：
  - 修复并打通了从机播放列表（Queue）的播歌控制（`folia-play-song`）、数据结构映射规范化、以及收藏/喜欢（`liked`）状态及红心按钮的双向事件反向遥控。
  - 宿主与 Folia 之间实现了音量双向联动（`volume` / `folia-volume`），防死循环更新；在 Stage 模式下适配了当前歌曲的高亮与自动滚动定位。
  - 移除了过时的 `/folia` 路由及废弃的 [FoliaPlayer.vue](./frontend/src/views/FoliaPlayer.vue)；更新 [folia.md](./docs/folia.md) 中嵌入模式描述。

### 优化
- **Folia 入口体验优化**：
  - 将“辞曲新境 (Folia)”全功能入口从侧边栏移至“系统设置”页面的卡片选项中。


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

