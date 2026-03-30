package conntrack

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

const procFile = "/proc/net/nf_conntrack"

// Entry represents a single conntrack entry.
type Entry struct {
	Proto   string // tcp / udp
	SrcIP   string
	DstIP   string
	DstPort int
	State   string // ESTABLISHED / SYN_SENT / TIME_WAIT / ...
}

// Stats holds per-VM connection statistics derived from conntrack.
type Stats struct {
	OutboundEstablished int // src=VM_IP ESTABLISHED tcp
	InboundEstablished  int // dst=VM_IP ESTABLISHED tcp
	UniqueDestIPs       int // unique dst IPs among outbound entries
	SynSentCount        int // src=VM_IP SYN_SENT
	TimeWaitCount       int // src=VM_IP TIME_WAIT
}

// ParseAll reads /proc/net/nf_conntrack once and returns all entries.
// Returns nil, nil if the file does not exist (kernel module not loaded).
func ParseAll() ([]Entry, error) {
	f, err := os.Open(procFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		e, ok := parseLine(scanner.Text())
		if ok {
			entries = append(entries, e)
		}
	}
	return entries, scanner.Err()
}

// StatsForIP filters entries by VM IP list and returns aggregated stats.
func StatsForIP(entries []Entry, ips []string) Stats {
	ipSet := make(map[string]bool, len(ips))
	for _, ip := range ips {
		ipSet[ip] = true
	}

	uniqueDst := make(map[string]bool)
	var s Stats

	for _, e := range entries {
		srcIsVM := ipSet[e.SrcIP]
		dstIsVM := ipSet[e.DstIP]

		if srcIsVM {
			switch e.State {
			case "ESTABLISHED":
				s.OutboundEstablished++
				uniqueDst[e.DstIP] = true
			case "SYN_SENT":
				s.SynSentCount++
			case "TIME_WAIT":
				s.TimeWaitCount++
			}
		}
		if dstIsVM && e.State == "ESTABLISHED" {
			s.InboundEstablished++
		}
	}
	s.UniqueDestIPs = len(uniqueDst)
	return s
}

// parseLine parses a single /proc/net/nf_conntrack line.
// Format (tcp example):
//
//	ipv4 2 tcp 6 86399 ESTABLISHED src=1.2.3.4 dst=5.6.7.8 sport=1234 dport=443 ...
func parseLine(line string) (Entry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return Entry{}, false
	}
	if fields[0] != "ipv4" {
		return Entry{}, false
	}
	proto := fields[2]
	if proto != "tcp" && proto != "udp" {
		return Entry{}, false
	}

	var e Entry
	e.Proto = proto

	// State is only present for tcp (field index 5 for tcp, absent for udp)
	if proto == "tcp" && len(fields) > 5 {
		e.State = fields[5]
	}

	// Parse key=value pairs
	for _, f := range fields[6:] {
		kv := strings.SplitN(f, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "src":
			if e.SrcIP == "" {
				e.SrcIP = kv[1]
			}
		case "dst":
			if e.DstIP == "" {
				e.DstIP = kv[1]
			}
		case "dport":
			if e.DstPort == 0 {
				e.DstPort, _ = strconv.Atoi(kv[1])
			}
		}
	}

	if e.SrcIP == "" || e.DstIP == "" {
		return Entry{}, false
	}
	return e, true
}
