package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		showHelp()
		return
	}
	switch os.Args[1] {
	case "run":
		cmdRun()
	case "status":
		cmdStatus()
	case "unblock":
		cmdUnblock()
	case "version", "-v", "--version":
		fmt.Println("bw-guardian", version)
	default:
		showHelp()
	}
}

func showHelp() {
	fmt.Printf(`bw-guardian %s - VPS 带宽监控与自动降速工具

用法:
  bw-guardian run              运行一次监控检查（由 systemd timer 调用）
  bw-guardian status           查看所有 VM 的监控状态
  bw-guardian unblock <uuid>   手动解除某台 VM 的限速
  bw-guardian version          显示版本信息

配置文件: /etc/bw-guardian/config
状态目录: /var/lib/bw-guardian/
日志文件: /var/log/bw-guardian.log
白名单:   /etc/bw-guardian/whitelist
`, version)
}
