# 2FMusic

> 轻量级本地音乐播放器 Web 应用，基于 Flask + Vue 3 + Vite + Pinia + Naive UI + Tailwind CSS 构建。

可部署在服务器或 NAS 上，通过浏览器提供在线音乐播放。

## 核心功能

- **本地音乐库**: 自动扫描目录、识别 ID3 元数据（封面、歌词），支持网络源自动补充
- **沉浸式播放器**: 响应式 UI、歌词滚动显示、封面主题色提取、快捷键控制
- **目录管理**: 添加服务器任意文件夹到音乐库，无需移动文件
- **网易云集成**: 搜索、高品质下载、链接解析、扫码登录同步歌单
- **音乐上传**: 通过浏览器上传音乐到服务器
- **精致界面**: 暗色/亮色模式、毛玻璃、自定义封面

## 直接启动

```bash
python server/app.py --music-library-path ./Music --port 23237
```

命令行参数:
- `--music-library-path`: 音乐文件存储目录
- `--log-path`: 日志文件路径
- `--port`: 服务端口（默认 23237）
- `--unix-socket`: Unix Domain Socket 路径
- `--base-url`: 自定义基准子路径（如 `/2fmusic`）
- `--password`: 设置访问密码

启动模式:
1. **端口模式**（仅传 `--port`）：仅监听 TCP 端口
2. **Socket 模式**（仅传 `--unix-socket`）：仅在 Unix Domain Socket 上运行
3. **混合模式**（同时传入二者）：多线程并发运行

## 预览图

<img width="2560" height="1440" alt="image" src="https://github.com/user-attachments/assets/1fea2a24-e433-4fba-9d80-6274ac944667" />
<img width="2560" height="1440" alt="image" src="https://github.com/user-attachments/assets/465f80cc-cbd2-480d-ae1f-f7dc62630c97" />
<img width="2560" height="1440" alt="image" src="https://github.com/user-attachments/assets/38586ddf-a421-4613-87ab-2b8b9743fc13" />




## 开源致谢

- **歌词/封面 API**: [LrcApi](https://github.com/HisAtri/LrcApi) (GPL-3.0)
- **网易云 API**: [NeteaseCloudMusicApiEnhanced](https://github.com/NeteaseCloudMusicApiEnhanced/api-enhanced) (MIT)
