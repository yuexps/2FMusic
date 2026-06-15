# 变更日志 (CHANGELOG.md)

所有针对 2FMusic 项目的技术升级、重构与规范改动均记录于此，以维持历史可追溯性。

---

## [2026-06-15]
### 回退
- 移除 `waitress` 服务器及其依赖，改回使用内置的多线程 `Werkzeug` Web 服务器。
- 解决 Waitress 不支持 WebSocket 协议导致 `/api/ws` 路由失效的问题。
- 更新 [docs/server/communication.md](file:///d:/Users/yuyue/Documents/Code/2FMusic/docs/server/communication.md) 启动模式规范。
