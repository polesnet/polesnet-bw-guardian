package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/polesnet/bw-guardian/internal/config"
	"github.com/polesnet/bw-guardian/internal/state"
	"github.com/polesnet/bw-guardian/internal/tc"
	"github.com/polesnet/bw-guardian/internal/virsh"
)

func cmdUnblock() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "用法: bw-guardian unblock <uuid>")
		os.Exit(1)
	}
	uuid := os.Args[2]
	cfg := config.Load()

	ifaces, err := virsh.GetInterfaces(uuid)
	if err != nil || len(ifaces) == 0 {
		fmt.Fprintf(os.Stderr, "警告: 无法获取 VM %s 的接口（VM 可能未运行）\n", uuid)
	}

	// Restore package bandwidth via tc, or just clear if pkg unknown
	pkgKbps, _ := strconv.Atoi(state.Read(cfg.StateDir, uuid, "pkg"))
	for _, iface := range ifaces {
		if err := tc.ApplyOriginal(iface, pkgKbps); err != nil {
			// pkgKbps was 0: rules cleared, interface is now unlimited
			fmt.Fprintf(os.Stderr, "警告: 无法还原套餐带宽 (%s): %v\n", iface, err)
		} else {
			fmt.Printf("已还原套餐带宽 (tc): %s → %d KB/s\n", iface, pkgKbps)
		}
	}

	// Remove all state files
	if err := state.DeleteAll(cfg.StateDir, uuid); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 清除状态文件失败: %v\n", err)
	} else {
		fmt.Printf("已清除所有状态文件: %s\n", uuid)
	}

	fmt.Printf("VM %s 已解除限速\n", uuid)
}
