package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
)

const releaseBaseURL = "https://github.com/polesnet/polesnet-bw-guardian/releases/latest/download/bw-guardian-linux-"

func cmdUpgrade() {
	if os.Getuid() != 0 {
		fmt.Fprintln(os.Stderr, "请使用 root 权限运行: sudo bw-guardian upgrade")
		os.Exit(1)
	}

	arch := runtime.GOARCH
	switch arch {
	case "amd64", "arm64":
	default:
		fmt.Fprintf(os.Stderr, "不支持的架构: %s\n", arch)
		os.Exit(1)
	}

	fmt.Println("=== bw-guardian 升级 ===")
	fmt.Println()
	fmt.Printf("[INFO] 当前版本: %s\n", version)

	downloadURL := releaseBaseURL + arch
	fmt.Printf("[INFO] 正在下载最新版本 (%s)...\n", arch)

	resp, err := http.Get(downloadURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] 下载失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "[ERROR] 下载失败，服务器返回状态码: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] 无法获取当前程序路径: %v\n", err)
		os.Exit(1)
	}

	tmpFile := exePath + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] 无法创建临时文件: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	if _, err = io.Copy(out, resp.Body); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] 写入临时文件失败: %v\n", err)
		os.Exit(1)
	}
	out.Close()

	if err := os.Chmod(tmpFile, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] 无法设置权限: %v\n", err)
		os.Exit(1)
	}

	if err := os.Rename(tmpFile, exePath); err != nil {
		cmd := exec.Command("mv", "-f", tmpFile, exePath)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] 替换程序失败: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println()
	fmt.Println("[INFO] 升级完成！")
}
