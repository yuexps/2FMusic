# 变更日志 (CHANGELOG.md)

所有针对 2FMusic 项目的技术升级、重构与规范改动均记录于此，以维持历史可追溯性。

---

## [2026-07-06]
### 新增
- 在 [LocalMusic.vue](frontend/src/views/LocalMusic.vue) 中实现了本地音乐的单曲、歌手、专辑、物理文件夹 4 个视图的聚合显示，支持 `localStorage` 视图记忆。
- 在“歌手”与“专辑”视图中接入了 A-Z 拼音姓名、修改时间（新曲入库置顶）以及歌曲数量排序的综合适配。
- 为“文件夹”视图重构了逐级下钻的物理面包屑文件浏览器，自适应在前端计算出了默认曲库的绝对根路径，支持多挂载盘符归并。

### 修复
- 修复了网易云下载时长时间卡在“排队中”（`pending`）状态，并在结束后瞬间拉满进度条的问题。调整了 [downloader.py](server/core/services/downloader.py) 中 [run_download_task](server/core/services/downloader.py#L356) 状态机流转，优先切为 `preparing`，并增加防御性 `try...except` 块。
- 修复了详情抽屉和文件夹点歌时，播放队列上下文（`playlist`）没有切换导致下一首播放断裂的体验问题。
- 修复了下载管理按钮在出现正在任务数角标时，内部 Flex 布局挤压导致下载图标偏斜的 Bug，改为使用官方标准的 [NBadge](frontend/src/views/NeteaseDownloader.vue) 组件包裹。

### 优化
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

