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

// Index is a pre-built lookup structure for conntrack entries, keyed by IP.
// Build once with BuildIndex(), then query per-VM with StatsForIP().
type Index struct {
	// outbound stats keyed by source IP
	outbound map[string]*ipStats
	// inbound ESTABLISHED count keyed by destination IP
	inboundEstablished map[string]int
}

type ipStats struct {
	established int
	synSent     int
	timeWait    int
	uniqueDst   map[string]bool
}

// BuildIndex scans all entries once and builds an index keyed by IP.
// Total work: O(M) where M is len(entries).
func BuildIndex(entries []Entry) *Index {
	idx := &Index{
		outbound:           make(map[string]*ipStats),
		inboundEstablished: make(map[string]int),
	}
	for _, e := range entries {
		// Outbound stats by source IP
		switch e.State {
		case "ESTABLISHED":
			s := idx.getOrCreate(e.SrcIP)
			s.established++
			s.uniqueDst[e.DstIP] = true
			idx.inboundEstablished[e.DstIP]++
		case "SYN_SENT", "SYN_SENT2":
			idx.getOrCreate(e.SrcIP).synSent++
		case "TIME_WAIT":
			idx.getOrCreate(e.SrcIP).timeWait++
		}
	}
	return idx
}

func (idx *Index) getOrCreate(ip string) *ipStats {
	s, ok := idx.outbound[ip]
	if !ok {
		s = &ipStats{uniqueDst: make(map[string]bool)}
		idx.outbound[ip] = s
	}
	return s
}

// StatsForIP aggregates stats for a set of VM IPs. O(len(ips)).
func (idx *Index) StatsForIP(ips []string) Stats {
	var s Stats
	allDst := make(map[string]bool)
	for _, ip := range ips {
		if os, ok := idx.outbound[ip]; ok {
			s.OutboundEstablished += os.established
			s.SynSentCount += os.synSent
			s.TimeWaitCount += os.timeWait
			for dst := range os.uniqueDst {
				allDst[dst] = true
			}
		}
		s.InboundEstablished += idx.inboundEstablished[ip]
	}
	s.UniqueDestIPs = len(allDst)
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

	// Parse key=value pairs.
	// For TCP, fields[5] is the state string (no "="), so starting at fields[5]
	// works for both TCP and UDP — the state string is skipped by the len(kv)!=2 check.
	for _, f := range fields[5:] {
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
