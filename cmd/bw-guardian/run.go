package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/polesnet/bw-guardian/internal/config"
	"github.com/polesnet/bw-guardian/internal/state"
	"github.com/polesnet/bw-guardian/internal/tc"
	"github.com/polesnet/bw-guardian/internal/virsh"
)

func cmdRun() {
	// Prevent concurrent runs (systemd OnUnitActiveSec does not guarantee exclusion)
	lockFile, err := os.OpenFile("/var/run/bw-guardian.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		writeLog(config.DefaultLogFile, "ERROR", "cannot open lock file: "+err.Error())
		return
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// Another instance is running
		return
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	cfg := config.Load()

	if err := os.MkdirAll(cfg.StateDir, 0755); err != nil {
		writeLog(cfg.LogFile, "ERROR", "cannot create state dir: "+err.Error())
		return
	}

	uuids, err := virsh.ListRunningUUIDs()
	if err != nil {
		writeLog(cfg.LogFile, "ERROR", "virsh list failed: "+err.Error())
		return
	}

	activeSet := make(map[string]bool, len(uuids))
	for _, uuid := range uuids {
		activeSet[uuid] = true
		if err := processVM(cfg, uuid); err != nil {
			writeLog(cfg.LogFile, "ERROR", fmt.Sprintf("processVM %s: %v", uuid, err))
		}
	}

	cleanupStale(cfg, activeSet)
}

func processVM(cfg *config.Config, uuid string) error {
	// Whitelist check
	if isWhitelisted(cfg.WhitelistFile, uuid) {
		return nil
	}

	// Get interfaces early — needed for both permanent re-throttle and normal logic.
	ifaces, err := virsh.GetInterfaces(uuid)
	if err != nil || len(ifaces) == 0 {
		return fmt.Errorf("no interfaces: %v", err)
	}

	// Permanent throttle: re-apply tc rules on every run so that a VM reboot
	// (which destroys the bridge interface and clears tc rules) doesn't let the
	// VM escape its permanent throttle.
	if state.Read(cfg.StateDir, uuid, "permanent") == "1" {
		for _, iface := range ifaces {
			tc.ApplyThrottle(iface, cfg.LimitMbit)
		}
		return nil
	}

	// Sum bytes across all interfaces
	totalBytes := sumInterfaceBytes(ifaces)
	now := time.Now().Unix()

	// Compute rate
	rateMbps := computeRate(cfg.StateDir, uuid, totalBytes, now)

	// Get package bandwidth (KB/s), cached in .pkg
	pkgKbps := getPkgKbps(cfg.StateDir, uuid)

	// Compute threshold (Mbps)
	threshold := computeThreshold(pkgKbps, cfg.OveruseRatio)

	overLimit := rateMbps > threshold
	isThrottled := state.Read(cfg.StateDir, uuid, "throttled") == "1"

	if overLimit {
		// Reset normal counter; only track over-limit count if not yet throttled
		state.Write(cfg.StateDir, uuid, "normal", "0")
		if isThrottled {
			// Already throttled, nothing to do
			return nil
		}
		count := readInt(cfg.StateDir, uuid, "count") + 1
		state.Write(cfg.StateDir, uuid, "count", strconv.Itoa(count))

		if count >= cfg.MaxCount {
			// Apply throttle to all interfaces; log any tc errors but still
			// mark as throttled to prevent repeated failed attempts.
			for _, iface := range ifaces {
				if err := tc.ApplyThrottle(iface, cfg.LimitMbit); err != nil {
					writeLog(cfg.LogFile, "ERROR", fmt.Sprintf("%s iface=%s throttle failed: %v", uuid, iface, err))
				}
			}

			times := readInt(cfg.StateDir, uuid, "times") + 1
			state.Write(cfg.StateDir, uuid, "throttled", "1")
			state.Write(cfg.StateDir, uuid, "times", strconv.Itoa(times))
			state.Write(cfg.StateDir, uuid, "count", "0")

			writeLog(cfg.LogFile, "THROTTLE", fmt.Sprintf("%s iface=%s rate=%.2f threshold=%.2f pkg=%d times=%d",
				uuid, strings.Join(ifaces, ","), rateMbps, threshold, pkgKbps, times))

			// Three-strikes permanent throttle
			if times >= cfg.MaxThrottleTimes {
				state.Write(cfg.StateDir, uuid, "permanent", "1")
				writeLog(cfg.LogFile, "PERMANENT", fmt.Sprintf("%s iface=%s times=%d", uuid, strings.Join(ifaces, ","), times))
			}
		}
	} else if isThrottled {
		normal := readInt(cfg.StateDir, uuid, "normal") + 1
		state.Write(cfg.StateDir, uuid, "normal", strconv.Itoa(normal))

		if normal >= cfg.NormalCountMax {
			// Restore package bandwidth via tc (not virsh domiftune — domiftune
			// does not reliably limit UDP traffic).
			for _, iface := range ifaces {
				if err := tc.ApplyOriginal(iface, pkgKbps); err != nil {
					writeLog(cfg.LogFile, "WARN", fmt.Sprintf("%s iface=%s restore failed: %v", uuid, iface, err))
				}
			}

			state.Write(cfg.StateDir, uuid, "throttled", "0")
			state.Write(cfg.StateDir, uuid, "normal", "0")
			state.Write(cfg.StateDir, uuid, "count", "0")

			writeLog(cfg.LogFile, "RECOVER", fmt.Sprintf("%s iface=%s rate=%.2f pkg=%d",
				uuid, strings.Join(ifaces, ","), rateMbps, pkgKbps))
		}
	} else {
		// Normal, not throttled: reset over-limit counter if nonzero
		if readInt(cfg.StateDir, uuid, "count") != 0 {
			state.Write(cfg.StateDir, uuid, "count", "0")
		}
	}

	return nil
}

// sumInterfaceBytes reads /sys/class/net/<iface>/statistics/{rx,tx}_bytes for all interfaces.
func sumInterfaceBytes(ifaces []string) uint64 {
	var total uint64
	for _, iface := range ifaces {
		total += readSysBytes(iface, "rx_bytes")
		total += readSysBytes(iface, "tx_bytes")
	}
	return total
}

func readSysBytes(iface, stat string) uint64 {
	path := fmt.Sprintf("/sys/class/net/%s/statistics/%s", iface, stat)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// computeRate calculates current Mbps, updates .state, and returns the rate.
// Returns 0 on first call (no previous state).
func computeRate(stateDir, uuid string, currentBytes uint64, nowEpoch int64) float64 {
	prevState := state.Read(stateDir, uuid, "state")
	state.Write(stateDir, uuid, "state", state.FormatState(currentBytes, nowEpoch))

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

	elapsed := nowEpoch - prevEpoch
	if elapsed <= 0 {
		elapsed = 60
	}

	var delta uint64
	if currentBytes >= prevBytes {
		delta = currentBytes - prevBytes
	} else {
		// 64-bit counter wraparound
		delta = math.MaxUint64 - prevBytes + currentBytes + 1
	}

	return float64(delta) * 8 / float64(elapsed) / 1_000_000
}

// getPkgKbps returns the package bandwidth in KB/s, using .pkg cache.
func getPkgKbps(stateDir, uuid string) int {
	if cached := state.Read(stateDir, uuid, "pkg"); cached != "" {
		v, err := strconv.Atoi(cached)
		if err == nil {
			return v // trust cache unconditionally; 0 means unlimited plan
		}
	}
	kbps, err := virsh.GetPkgKbps(uuid)
	if err != nil {
		return 0
	}
	// Cache even if 0 (unlimited plan), to avoid calling virsh every minute.
	state.Write(stateDir, uuid, "pkg", strconv.Itoa(kbps))
	return kbps
}

// computeThreshold returns the bandwidth threshold in Mbps.
// Minimum is 10 Mbps.
func computeThreshold(pkgKbps, overuseRatio int) float64 {
	if pkgKbps <= 0 {
		return 10.0
	}
	// pkgKbps * 8 / 1000 = Mbps; multiply by ratio/100
	t := float64(pkgKbps) * 8.0 / 1000.0 * float64(overuseRatio) / 100.0
	if t < 10.0 {
		return 10.0
	}
	return t
}

func readInt(stateDir, uuid, typ string) int {
	v, err := strconv.Atoi(state.Read(stateDir, uuid, typ))
	if err != nil {
		return 0
	}
	return v
}

func isWhitelisted(whitelistFile, uuid string) bool {
	f, err := os.Open(whitelistFile)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == uuid {
			return true
		}
	}
	return false
}

func cleanupStale(cfg *config.Config, activeSet map[string]bool) {
	uuids, err := state.ListUUIDs(cfg.StateDir)
	if err != nil {
		return
	}
	for _, uuid := range uuids {
		if activeSet[uuid] {
			continue
		}
		// Preserve state for permanently throttled VMs even when offline,
		// so the three-strikes record survives VM power-off/reboot cycles.
		if state.Read(cfg.StateDir, uuid, "permanent") == "1" {
			continue
		}
		state.DeleteAll(cfg.StateDir, uuid)
	}
}

func writeLog(logFile, eventType, msg string) {
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "%s %s %s\n", ts, eventType, msg)
}
