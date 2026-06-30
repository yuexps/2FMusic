@echo off
chcp 65001 >nul
cd /d "%~dp0"

if not exist ".venv\" (
    echo "[INFO] 未检测到本地虚拟环境 .venv,开始创建..."
    python -m venv .venv
    if errorlevel 1 (
        echo "[ERROR] 虚拟环境创建失败，请确保系统已安装 Python 且已加入环境变量。"
        pause
        exit /b 1
    )
)

echo "[INFO] 正在校验并同步安装依赖项"
call .venv\Scripts\pip install -r server\requirements.txt ^
    -i https://pypi.tuna.tsinghua.edu.cn/simple/ ^
    --extra-index-url https://mirrors.aliyun.com/pypi/simple/ ^
    --extra-index-url https://pypi.org/simple

if errorlevel 1 (
    echo "[ERROR] 依赖项安装失败,请检查网络连接。"
    pause
    exit /b 1
)

echo "[INFO] 正在启动 2FMusic 后端服务..."
call .venv\Scripts\python server/app.py --music-library-path ./test_tmp/Music --log-path ./test_tmp/app.log --port 23237 --password 123456

pause