# 使用官方Python运行时作为基础镜像
FROM python:3.12-slim

# 设置工作目录
WORKDIR /app

# 安装系统依赖
RUN apt-get update && apt-get install -y \
    gcc \
    && rm -rf /var/lib/apt/lists/*

# 复制整个应用目录
COPY app/ ./app/

# 暴露端口
EXPOSE 23237

# 设置默认命令
CMD ["python", "app/server/app.py", "--music-library-path", "./Music", "--log-path", "./app.log", "--port", "23237"]

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD python -c "import requests; requests.get('http://localhost:23237')" || exit 1