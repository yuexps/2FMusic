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

echo "[INFO] 开始编译打包 Folia-major 播放器"
pushd "%~dp0folia-major"
if not exist "node_modules\" (
    echo "[INFO] 未检测到 Folia-major 依赖，开始下载安装"
    call npm ci
) else (
    echo "[INFO] Folia-major 依赖已存在，跳过安装"
)
set BUILD_BASE=./
call npm run build
popd

echo "[INFO] 拷贝 Folia-major 打包产物"
if exist "%~dp0www\folia" (
    rmdir /S /Q "%~dp0www\folia"
)
mkdir "%~dp0www\folia"
xcopy /E /I /Y "%~dp0folia-major\dist" "%~dp0www\folia"

echo "[INFO] 前端与 Folia-major 集成构建编译完成"
pause