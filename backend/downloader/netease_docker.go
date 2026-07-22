package downloader

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"2fmusic/backend/core"
	"2fmusic/backend/db"
)

var (
	dockerInstallStatusMu sync.Mutex
	dockerInstallStatus   = map[string]interface{}{
		"status":   "idle",
		"progress": 0,
		"step":     "",
		"error":    "",
	}
)

// CheckDockerContainer 检查宿主机 Docker 环境与目标容器状态
func CheckDockerContainer() map[string]interface{} {
	res := map[string]interface{}{
		"docker_installed":  false,
		"container_exists":   false,
		"container_running":  false,
	}

	containerName := "2fmusic-ncm-api"
	if err := exec.Command("docker", "--version").Run(); err != nil {
		return res
	}
	res["docker_installed"] = true

	out, err := exec.Command("docker", "ps", "-a", "--filter", "name="+containerName, "--format", "{{.Names}}").Output()
	if err == nil && strings.Contains(string(out), containerName) {
		res["container_exists"] = true

		runningOut, rErr := exec.Command("docker", "ps", "--filter", "name="+containerName, "--filter", "status=running", "--format", "{{.Names}}").Output()
		if rErr == nil && strings.Contains(string(runningOut), containerName) {
			res["container_running"] = true
		}
	}

	return res
}

// GetDockerInstallStatus 获取 Docker 安装部署任务进度
func GetDockerInstallStatus() map[string]interface{} {
	dockerInstallStatusMu.Lock()
	defer dockerInstallStatusMu.Unlock()

	cp := make(map[string]interface{})
	for k, v := range dockerInstallStatus {
		cp[k] = v
	}
	return cp
}

// updateInstallStatus 更新内部进度状态
func updateInstallStatus(status string, progress int, step string, errMsg string) {
	dockerInstallStatusMu.Lock()
	dockerInstallStatus["status"] = status
	dockerInstallStatus["progress"] = progress
	dockerInstallStatus["step"] = step
	dockerInstallStatus["error"] = errMsg
	dockerInstallStatusMu.Unlock()
}

// InstallNeteaseDockerService 自动部署网易云 NCM API Docker 容器
func InstallNeteaseDockerService() (bool, string) {
	dockerInstallStatusMu.Lock()
	if st, ok := dockerInstallStatus["status"].(string); ok && st == "running" {
		dockerInstallStatusMu.Unlock()
		return false, "安装部署任务正在执行中"
	}
	dockerInstallStatusMu.Unlock()

	updateInstallStatus("running", 10, "检查宿主机 Docker 环境...", "")
	core.Info("开始执行部署网易云 API Docker 容器")

	go func() {
		if err := exec.Command("docker", "--version").Run(); err != nil {
			updateInstallStatus("error", 0, "Docker 未安装或未在 PATH 中找到", err.Error())
			core.Error("部署 Docker 容器失败: 宿主机未安装 Docker")
			return
		}

		containerName := "2fmusic-ncm-api"
		updateInstallStatus("running", 20, fmt.Sprintf("检查容器 %s...", containerName), "")

		out, err := exec.Command("docker", "ps", "-a", "--filter", "name="+containerName, "--format", "{{.Names}}").Output()
		if err == nil && strings.Contains(string(out), containerName) {
			updateInstallStatus("running", 60, "容器已存在，正在启动...", "")
			core.Info("网易云 API 容器已存在，直接启动: %s", containerName)
			if err := exec.Command("docker", "start", containerName).Run(); err != nil {
				updateInstallStatus("error", 60, "启动现有容器失败", err.Error())
				return
			}
		} else {
			updateInstallStatus("running", 30, "正在拉取镜像 moefurina/ncm-api:latest (耗时较长)...", "")
			core.Info("正在拉取 Docker 镜像: moefurina/ncm-api:latest")
			if err := exec.Command("docker", "pull", "moefurina/ncm-api:latest").Run(); err != nil {
				updateInstallStatus("error", 30, "拉取 Docker 镜像失败", err.Error())
				return
			}

			updateInstallStatus("running", 70, "镜像拉取完成，正在启动 Docker 容器...", "")
			core.Info("正在启动 Docker 容器: %s", containerName)
			runCmd := exec.Command("docker", "run", "-d", "-p", "23236:3000", "--name", containerName, "--restart", "always", "moefurina/ncm-api")
			if err := runCmd.Run(); err != nil {
				updateInstallStatus("error", 70, "启动 Docker 容器失败", err.Error())
				return
			}
		}

		updateInstallStatus("success", 100, "服务启动成功！", "")
		defaultAPIBase := "http://127.0.0.1:23236"
		_ = db.SaveSystemSetting("netease_api_base", defaultAPIBase)
		core.GlobalConfig.NeteaseAPIBase = defaultAPIBase
		core.Info("网易云 API Docker 容器部署成功，已自动关联 %s", defaultAPIBase)
	}()

	return true, ""
}
