package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/polesnet/bw-guardian/internal/config"
	"github.com/polesnet/bw-guardian/internal/state"
	"github.com/polesnet/bw-guardian/internal/virsh"
)

func cmdStatus(args []string) {
	onlyThrottled := false
	for _, a := range args {
		if a == "--throttled" || a == "-t" {
			onlyThrottled = true
		}
	}

	cfg := config.Load()

	uuids, err := virsh.ListRunningUUIDs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: virsh list 失败: %v\n", err)
		uuids = nil
	}

	// Also include UUIDs that have state files but may not be running (e.g., permanent)
	stateUUIDs, _ := state.ListUUIDs(cfg.StateDir)
	seen := make(map[string]bool)
	for _, u := range uuids {
		seen[u] = true
	}
	for _, u := range stateUUIDs {
		if !seen[u] {
			uuids = append(uuids, u)
			seen[u] = true
		}
	}

	if len(uuids) == 0 {
		fmt.Println("暂无 VM 数据")
		return
	}

	header := fmt.Sprintf("%-38s  %10s  %10s  %-9s  %-4s  %5s  %10s",
		"UUID", "RATE(Mbps)", "THRESH(Mbps)", "THROTTLED", "PERM", "TIMES", "PKG(Mbps)")
	sep := strings.Repeat("-", len(header))
	fmt.Println(header)
	fmt.Println(sep)

	for _, uuid := range uuids {
		rateMbps := currentRateMbps(cfg, uuid)
		pkgKbps := readIntState(cfg.StateDir, uuid, "pkg")
		pkgMbps := float64(pkgKbps) * 8 / 1000
		threshold := computeThreshold(pkgKbps, cfg.OveruseRatio)
		throttled := boolLabel(state.Read(cfg.StateDir, uuid, "throttled") == "1")
		permanent := boolLabel(state.Read(cfg.StateDir, uuid, "permanent") == "1")

		if onlyThrottled && throttled == "no" && permanent == "no" {
			continue
		}

		times := readIntState(cfg.StateDir, uuid, "times")

		fmt.Printf("%-38s  %10.2f  %10.2f  %-9s  %-4s  %5d  %10.2f\n",
			uuid, rateMbps, threshold, throttled, permanent, times, pkgMbps)
	}
}

// currentRateMbps computes the current rate from the .state file without writing.
func currentRateMbps(cfg *config.Config, uuid string) float64 {
	prevState := state.Read(cfg.StateDir, uuid, "state")
	if prevState == "" {
		return 0
	}
	parts := strings.Fields(prevState)
	if len(parts) != 2 {
		return 0
	}
	prevBytes, err1 := strconv.ParseUint(parts[0], 10, 64)
	prevEpoch, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0
	}

	// Get current bytes
	ifaces, err := virsh.GetInterfaces(uuid)
	if err != nil || len(ifaces) == 0 {
		return 0
	}
	var currentBytes uint64
	for _, iface := range ifaces {
		currentBytes += readSysBytes(iface, "rx_bytes")
		currentBytes += readSysBytes(iface, "tx_bytes")
	}

	elapsed := time.Now().Unix() - prevEpoch
	if elapsed <= 0 {
		elapsed = 60
	}

	var delta uint64
	if currentBytes >= prevBytes {
		delta = currentBytes - prevBytes
	} else {
		delta = ^uint64(0) - prevBytes + currentBytes + 1
	}

	return float64(delta) * 8 / float64(elapsed) / 1_000_000
}

func readIntState(stateDir, uuid, typ string) int {
	v, err := strconv.Atoi(state.Read(stateDir, uuid, typ))
	if err != nil {
		return 0
	}
	return v
}

func boolLabel(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
