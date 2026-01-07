# 🎵 2FMusic

> 这是一个轻量的本地音乐播放器，基于 Flask + 原生 Web 技术构建。

[![pre-release](https://img.shields.io/github/v/release/yuexps/2FMusic?label=pre--release&include_prereleases)](https://github.com/yuexps/2FMusic/releases/latest) [![latest action](https://img.shields.io/github/actions/workflow/status/yuexps/2FMusic/test-build.yml?label=latest%20action)](https://github.com/yuexps/2FMusic/actions/workflows/test-build.yml)

**目前项目处于早期开发阶段，功能暂不稳定，欢迎提交 PR 完善项目 或 Issue 反馈 Bug！**

---

## 核心功能

*   **🎧 本地音乐库**
    *   自动扫描并管理上传的音乐文件。
    *   支持识别 ID3 内嵌元数据（封面、歌词）。
    *   支持通过 网络源 自动获取封面、歌词。
*   **📱 沉浸式播放器**
    *   **响应式 UI**：PC Web端 提供原生 App 般的流畅体验。
    *   **动态视觉**：支持歌词滚动显示、根据封面自动提取主题色。
*   **📂 灵活目录管理**
    *   支持添加服务器上的任意文件夹到音乐库，无需重复移动文件。
*   **☁️ 网易云音乐集成**
    *   集成搜索、高品质下载功能。
    *   支持链接解析及扫码登录，轻松同步歌单。
*   **🔊 音频预览**
    *   支持右键快速预览播放音频文件（需安装 2FMusic Preview 扩展）。

## 直接启动

```bash
python app/server/app.py --music-library-path ./Music --log-path ./app.log --port 23237
```

参数：
- `--music-library-path`: 音乐文件存储目录
- `--log-path`: 日志文件路径
- `--port`: 服务端口 (默认 23237)
- `--password`: 设置访问密码


## Docker Compose

```yaml
services:
  2fmusic:
    image: ghcr.io/yuexps/2fmusic:latest
    container_name: 2fmusic
    ports:
      - "23237:23237"
    volumes:
      - ./Music:/app/Music
      - ./app.log:/app/app.log
    environment:
      - FLASK_ENV=production
      - TZ=Asia/Shanghai
      - PORT=23237
      - APP_PASSWORD=
    restart: unless-stopped
```

## 🛠️ 开源致谢

本项目使用了以下优秀的开源项目：

*   **UI 图标**: [Font Awesome](https://fontawesome.com/) (CC BY 4.0 / OFL)
*   **色彩算法**: [ColorThief](https://lokeshdhakar.com/projects/color-thief/) (MIT)
*   **歌词/封面 API**: [LrcApi](https://github.com/HisAtri/LrcApi) (GPL-3.0)
*   **网易云 API**: [NeteaseCloudMusicApiEnhanced](https://github.com/NeteaseCloudMusicApiEnhanced/api-enhanced) (MIT)
