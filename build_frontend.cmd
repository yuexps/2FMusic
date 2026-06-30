@echo off
pushd "%~dp0frontend"
chcp 65001 >nul

if not exist "node_modules\" (
    echo "[INFO] 未检测到 node_modules 依赖，开始下载安装"
    call npm ci
) else (
    echo "[INFO] 依赖文件夹 node_modules 已存在，跳过依赖安装"
)

echo "[INFO] 开始编译打包前端资源"
call npm run build

popd
echo "[INFO] 前端构建编译完成"
pause