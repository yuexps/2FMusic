# 2FMusic 系统核心文档规范索引 (docs/INDEX.md)

2FMusic 系统的文档体系已完成全面集约重构与模块解耦。所有文档均经过对 Python 物理源码 (`server/`)、Go 重构实现 (`backend/`) 及前端 SPA (`frontend/`) 的 100% 物理代码比对校对。

全库由以下 **4 大顶级核心规范文档** 组成：

---

## 1. 核心规范与架构索引

* **[AGENTS.md](../AGENTS.md)**
  - 全局 Agent 工作流、红线、Token 控制及约束先行规约。

* **[docs/api.md](./api.md)** —— **系统 API 与 WebSocket 通信契约规范**
  - HTTP REST 接口清单、WebSocket 消息帧规范 (`type: response`, `success: bool`)、全量 Action 路由动作表与服务端主动广播事件总线。

* **[docs/backend.md](./backend.md)** —— **后端架构、数据库与服务引擎规范**
  - 6 种标准音频后缀 (`AUDIO_EXTS`) 硬收敛、SQLite 3 DDL 结构 (`INTEGER`0/1 与 `bool` 转换)、Watchdog 2.0s 物理防抖监听与网易云下载状态机。

* **[docs/frontend.md](./frontend.md)** —— **前端 SPA 架构与通信规范**
  - 未登录门禁拦截 (`Auth Gate`)、Pinia `playerStore` 状态恢复、WebSocket 离线队列 (`offlineQueue`) 与订阅解绑红线、IndexedDB 封面缓存与 Blob URL 引用计数。

* **[docs/android.md](./android.md)** —— **Android 客户端接入与高精度歌词解析规范**
  - Media3/ExoPlayer 点播与 `LrcParser` 高精度歌词解析算法（垃圾 JSON 丢弃、YRC 绝对时间计算与 LRC 弹性补齐）。
