# 变更日志 (CHANGELOG.md)

所有针对 2FMusic 项目的技术升级、重构与规范改动均记录于此，以维持历史可追溯性。

---

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

