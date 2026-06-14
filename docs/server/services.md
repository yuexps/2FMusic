# 后端业务服务逻辑契约 (docs/server/services.md)

## 1. 物理库扫描与文件监听服务 (Watchdog & Scanner)

### 1.1 自动防抖防冲突监听机制 (Watchdog Worker)
*   **物理捕获**：使用 `watchdog` 库捕获挂载目录下的 `created`、`deleted`、`moved` 物理变动。
*   **延时防抖队列 (Debounce Worker)**：
    - 所有监听文件变动均写入线程安全队列 `watchdog_queue`。
    - **逻辑行为**：消费线程拉取到变动信号后，默认先向后延迟 **2.0 秒** 执行。
    - **自适应延迟**：若 2 秒延迟期满，比对发现文件大小正在变化（如大文件正在物理写入中），则将任务再次往后延迟 **1.0 秒**。只有当物理文件大小完全稳定不变时，才调起物理索引同步。这有效避免了在用户上传歌曲中触发部分写入导致提取报错的 Bug。
*   **监听路径计算**：由默认曲库路径 `MUSIC_LIBRARY_PATH` 加上 `mount_points` 表中挂载的所有路径组成，并自动排除嵌套的子目录路径。

### 1.2 并发控制与防重叠扫描锁机制 (Scan Locks)
为了避免用户手动触发扫描、文件夹挂载变动自动触发扫描以及 Watchdog 监听事件并发执行导致数据库竞争与 CPU 暴涨，必须实现以下双重锁机制：
*   **内存排他锁 (`scan_execution_lock`)**：使用全局线程互斥锁限制同时只能运行一个扫描任务。若尝试获取锁失败，应直接跳过本次全量增量扫描。
*   **物理文件锁 (`.scan_lock`)**：在默认音乐曲库路径下写入临时 `.scan_lock` 文件，其中记录当前扫描的启动时间戳。
    - **防死锁自愈机制**：如果检测到 `.scan_lock` 文件存在，且其最后修改时间已经超过 **300 秒 (5分钟)**，说明前一次扫描可能因进程意外崩溃或系统强杀而残留。此时应物理删除该过期锁文件，重置锁状态，并允许新扫描任务启动。

### 1.3 单文件物理入库逻辑 (index_single_file)
*   **防重哈希**：优先从库中提取 `path`, `size`, `mtime` 皆未发生改变的记录，直接继承原 `id`；为新文件时，采用文件内容 MD5 作为歌曲的全局唯一 ID。
*   **物理缓存位置**：
    - 封面：`app_config.COVERS_DIR/{song_id}.webp` (自动转换 WebP 格式)
    - 歌词：`app_config.LYRICS_DIR/{song_id}.lrc` (标准 LRC 文本)
*   **封面提取优先级**：外部物理同级同名 `.jpg`/`.jpeg`/`.png` $\rightarrow$ 缓存目录已有 $\rightarrow$ 原生音频内嵌 `extract_embedded_cover`。
*   **歌词提取优先级**：外部物理同级同名 `.lrc` $\rightarrow$ 缓存目录已有 $\rightarrow$ 原生音频内嵌 `extract_embedded_lyrics`。

### 1.4 后台并发刮削 (auto_scrape_missing_metadata)
*   **触发**：增量扫描完成后，若库中仍存在 `has_cover=0` 或 `has_lyrics=0` 的歌曲，后台线程池启动。
*   **线程配置**：使用 `ThreadPoolExecutor` 限制最多 5 个并发 workers。
*   **多源检索机制**：按 **QQ 音乐 -> 网易云 -> 酷狗** 接口顺序并发搜索标题、歌手、专辑信息。获取到有效封面/歌词后自动持久化下载存入缓存，并将数据库字段标记更新为 1。

---

## 2. 网易云异步下载管理器 (Downloader Service)

### 2.1 网易云本地容器接口桥接
*   **Cookie 注入机制**：调用本地 NCM API（`/song/url/v1` 等）时，自动拦截读取 `system_settings` 中的 `netease_cookie`，并作为 Header 和 params 透传，以保持 VIP 态。

### 2.2 异步下载任务状态机 (Download Task State Machine)
下载进程挂在单独后台线程中运行，各状态流转规范如下：
1.  **准备阶段 (`preparing`)**：
    - 初始创建任务，向本地 API 接口补充获取歌曲的专辑信息、大图 picUrl。
2.  **地址抓取与音质回退机制**：
    - 请求 `/song/url/v1`。
    - **回退逻辑**：若指定的音质等级（如 `lossless`/`exhigh`）获取到的 URL 为空或属于无版权无收费，自动**回退到 `standard`（标准音质）**重新请求一次，确保最大化下载成功率，绝不因音质限制闪退。
3.  **下载中阶段 (`downloading`)**：
    - 流式下载文件 (`stream=True`)，每次块大小 8192 字节，并临时在物理路径追加 `.part` 后缀。
    - 实时通过下载大小比对计算百分比进度：`progress = int((downloaded / total_size) * 100)`，并以 WS 广播通知前端。
4.  **保存入库阶段**：
    - 物理移除 `.part` 后缀。
    - 对文件进行 MD5 哈希计算作为最终入库的 `song_id`。
    - 异步保存专辑大图到 `covers/` 目录并调用 `embed_cover_to_file` 内嵌到音频。
    - 异步拉取 `/lyric` 接口，保存 `.lrc` 与 `.yrc` (逐字) 歌词，并将 `.lrc` 内嵌到音频。
5.  **结束阶段 (`success` / `error`)**：
    - 自动清理超过 10 分钟的历史残留任务状态。
    - 物理调用 `index_single_file` 入库，并触发 `notify_library_changed`。

### 2.3 启动级残留临时文件物理清理机制 (clean_temp_part_files)
*   **清理时机**：主服务每次在 `__main__` 启动初始化时（在未开始执行网络端口与套接字监听前）同步拉起。
*   **清理范围**：
    - 自动遍历默认音乐曲库路径 `app_config.MUSIC_LIBRARY_PATH`。
    - 若配置了 `app_config.NETEASE_DOWNLOAD_DIR`，也一并遍历。
    - 遍历挂载点表 `mount_points` 下登记的所有目录。
*   **清理逻辑**：检测并物理删除所有以 `.part` 结尾的文件（这些是在线流式下载中途中断所留下的历史残留碎片文件），从而保证磁盘存储空间的整洁。

