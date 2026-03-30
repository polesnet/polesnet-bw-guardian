package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultStateDir      = "/var/lib/bw-guardian"
	DefaultLogFile       = "/var/log/bw-guardian.log"
	DefaultWhitelistFile = "/etc/bw-guardian/whitelist"
	DefaultConfigFile    = "/etc/bw-guardian/config"
)

type Config struct {
	OveruseRatio     int
	LimitMbit        int
	MaxCount         int
	NormalCountMax   int
	MaxThrottleTimes int
	StateDir         string
	LogFile          string
	WhitelistFile    string

	// Risk control
	RiskEnabled          bool
	RiskMaxConns         int
	RiskMaxUniqueDsts    int
	RiskScanThreshold    int
	RiskInboundThreshold int
	WebhookURL           string
}

func Load() *Config {
	cfg := &Config{
		OveruseRatio:     80,
		LimitMbit:        10,
		MaxCount:         10,
		NormalCountMax:   30,
		MaxThrottleTimes: 3,
		StateDir:         DefaultStateDir,
		LogFile:          DefaultLogFile,
		WhitelistFile:    DefaultWhitelistFile,

		RiskEnabled:          false,
		RiskMaxConns:         300,
		RiskMaxUniqueDsts:    150,
		RiskScanThreshold:    50,
		RiskInboundThreshold: 50,
	}

	f, err := os.Open(DefaultConfigFile)
	if err != nil {
		return cfg
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "OVERUSE_RATIO":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.OveruseRatio = v
			}
		case "LIMIT_MBIT":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.LimitMbit = v
			}
		case "MAX_COUNT":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.MaxCount = v
			}
		case "NORMAL_COUNT_MAX":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.NormalCountMax = v
			}
		case "MAX_THROTTLE_TIMES":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.MaxThrottleTimes = v
			}
		case "STATE_DIR":
			cfg.StateDir = val
		case "LOG_FILE":
			cfg.LogFile = val
		case "WHITELIST_FILE":
			cfg.WhitelistFile = val
		case "RISK_ENABLED":
			cfg.RiskEnabled = val == "true" || val == "1"
		case "RISK_MAX_CONNS":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.RiskMaxConns = v
			}
		case "RISK_MAX_UNIQUE_DSTS":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.RiskMaxUniqueDsts = v
			}
		case "RISK_SCAN_THRESHOLD":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.RiskScanThreshold = v
			}
		case "RISK_INBOUND_THRESHOLD":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.RiskInboundThreshold = v
			}
		case "WEBHOOK_URL":
			cfg.WebhookURL = val
		}
	}

	return cfg
}
