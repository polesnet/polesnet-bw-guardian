package virsh

import (
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

func run(args ...string) (string, error) {
	cmd := exec.Command("virsh", args...)
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	return string(out), err
}

// ListRunningUUIDs returns UUIDs of all running VMs.
func ListRunningUUIDs() ([]string, error) {
	out, err := run("list", "--uuid")
	if err != nil {
		return nil, err
	}
	var uuids []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			uuids = append(uuids, line)
		}
	}
	return uuids, nil
}

// GetInterfaces returns the network interface names for a VM.
// Output of `virsh domiflist <uuid>`:
//
//	Interface  Type   Source  Model    MAC
//	----------------------------------------------
//	vnet0      bridge br0     virtio   52:54:00:...
func GetInterfaces(uuid string) ([]string, error) {
	out, err := run("domiflist", uuid)
	if err != nil {
		return nil, err
	}
	var ifaces []string
	lines := strings.Split(out, "\n")
	// Skip header lines (first 2)
	for i, line := range lines {
		if i < 2 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		ifaces = append(ifaces, fields[0])
	}
	return ifaces, nil
}

// GetIPAddresses returns the IPv4 addresses of a VM from virsh domifaddr.
// Output example:
//
//	Name       MAC address         Protocol  Address
//	vnet0      52:54:00:xx:xx:xx   ipv4      192.168.100.10/24
func GetIPAddresses(uuid string) ([]string, error) {
	out, err := run("domifaddr", uuid)
	if err != nil {
		return nil, err
	}
	var ips []string
	for i, line := range strings.Split(out, "\n") {
		if i < 2 {
			continue
		}
		fields := strings.Fields(line)
		// fields: [ifname, mac, protocol, address/prefix]
		if len(fields) < 4 || fields[2] != "ipv4" {
			continue
		}
		addr := fields[3]
		if idx := strings.Index(addr, "/"); idx >= 0 {
			addr = addr[:idx]
		}
		if addr != "" {
			ips = append(ips, addr)
		}
	}
	return ips, nil
}

// GetPkgKbps reads the inbound.average (KB/s) from domiftune for the first interface.
// Returns 0 if unavailable.
func GetPkgKbps(uuid string) (int, error) {
	ifaces, err := GetInterfaces(uuid)
	if err != nil || len(ifaces) == 0 {
		return 0, fmt.Errorf("no interfaces for %s", uuid)
	}
	out, err := run("domiftune", uuid, ifaces[0])
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(out, "\n") {
		// virsh output: "inbound.average  : 10240" (spaces around colon vary by version)
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "inbound.average" {
			v, err := strconv.Atoi(val)
			if err == nil {
				return v, nil
			}
		}
	}
	return 0, fmt.Errorf("inbound.average not found for %s", uuid)
}
