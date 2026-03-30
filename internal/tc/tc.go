package tc

import (
	"fmt"
	"io"
	"os/exec"
)

func run(args ...string) error {
	cmd := exec.Command("tc", args...)
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// ClearRules removes all tc rules from an interface (ignores errors).
func ClearRules(iface string) {
	run("qdisc", "del", "dev", iface, "root")
	run("qdisc", "del", "dev", iface, "ingress")
}

// burstFor returns a safe TBF/police burst size for a given rate in kbit/s.
// TBF requires burst >= rate_bytes / kernel_HZ. Debian default HZ=250.
// We use rate_bytes/250 with a floor of 16KB.
func burstFor(rateKbits int) string {
	b := rateKbits * 1000 / 8 / 250 // bytes
	if b < 16384 {
		b = 16384
	}
	return fmt.Sprintf("%db", b)
}

// applyRate sets up tbf + ingress policing at the given rate (in kbit/s).
// Clears existing rules first to ensure idempotency.
func applyRate(iface string, rateKbits int) error {
	ClearRules(iface)

	rateStr := fmt.Sprintf("%dkbit", rateKbits)
	burst := burstFor(rateKbits)

	// Outbound (egress from host bridge = download direction for VM)
	if err := run("qdisc", "add", "dev", iface, "root", "tbf",
		"rate", rateStr, "burst", burst, "latency", "50ms"); err != nil {
		return fmt.Errorf("tc add root tbf on %s: %w", iface, err)
	}

	// Inbound (ingress = upload direction from VM)
	if err := run("qdisc", "add", "dev", iface, "ingress"); err != nil {
		return fmt.Errorf("tc add ingress on %s: %w", iface, err)
	}
	// IPv4
	if err := run("filter", "add", "dev", iface, "parent", "ffff:",
		"protocol", "ip", "u32", "match", "u32", "0", "0",
		"police", "rate", rateStr, "burst", burst, "drop", "flowid", ":1"); err != nil {
		return fmt.Errorf("tc filter add ipv4 on %s: %w", iface, err)
	}
	// IPv6 (best-effort, some kernels may not support it)
	run("filter", "add", "dev", iface, "parent", "ffff:",
		"protocol", "ipv6", "u32", "match", "u32", "0", "0",
		"police", "rate", rateStr, "burst", burst, "drop", "flowid", ":1")

	return nil
}

// ApplyThrottle throttles an interface to limitMbit Mbps.
func ApplyThrottle(iface string, limitMbit int) error {
	return applyRate(iface, limitMbit*1000)
}

// ApplyOriginal restores the package bandwidth on an interface using tc only
// (does not rely on virsh domiftune, which does not handle UDP reliably).
// pkgKbps is the original package bandwidth in KB/s (from .pkg cache).
func ApplyOriginal(iface string, pkgKbps int) error {
	if pkgKbps <= 0 {
		// Unknown package bandwidth: just clear our throttle rules.
		// VirtFusion domiftune rules were already wiped when we throttled,
		// so this leaves the interface unlimited — caller should log a warning.
		ClearRules(iface)
		return fmt.Errorf("pkgKbps is 0, cannot restore original rate on %s", iface)
	}
	// pkgKbps (KB/s) * 8 = kbit/s
	return applyRate(iface, pkgKbps*8)
}
