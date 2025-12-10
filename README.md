# 2FMusic

轻量级 FNOS 本地音乐播放器，基于 Flask + 原生 Web 技术构建。

## ✨ 核心功能

*   **本地音乐库**：自动扫描上传的音乐文件，支持 ID3 元数据。
*   **Web 播放器**：响应式 UI（PC/移动端），支持歌词、封面、深色模式。
*   **目录挂载**：支持将服务器任意文件夹挂载`(软链接)`到音乐库。
*   **网易云集成**：内置搜索、下载、链接解析及扫码登录。

## 🚀 快速开始

**1. 启动服务**
```bash
python app/server/app.py --music-library-path ./Music --log-path ./app.log --port 23237
```

**2. 访问**
浏览器打开 `http://localhost:23237`

## 🛠️ 开源致谢

*   **图标库**: [Font Awesome](https://fontawesome.com/)
*   **取色库**: [ColorThief](https://lokeshdhakar.com/projects/color-thief/)
*   **歌词/封面 API**: [LrcApi](https://github.com/HisAtri/LrcApi)
*   **网易云 API**: [Api Enhanced](https://github.com/NeteaseCloudMusicApiEnhanced/api-enhanced)
