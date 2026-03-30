package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func path(dir, uuid, typ string) string {
	return filepath.Join(dir, uuid+"."+typ)
}

// Read returns the trimmed content of a state file, or "" if it doesn't exist.
func Read(dir, uuid, typ string) string {
	data, err := os.ReadFile(path(dir, uuid, typ))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Write writes value to the state file (creates or overwrites).
func Write(dir, uuid, typ, value string) error {
	return os.WriteFile(path(dir, uuid, typ), []byte(value+"\n"), 0644)
}

// Delete removes a single state file (ignores missing).
func Delete(dir, uuid, typ string) error {
	err := os.Remove(path(dir, uuid, typ))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// DeleteAll removes all state files for a given UUID (<uuid>.*).
func DeleteAll(dir, uuid string) error {
	pattern := filepath.Join(dir, uuid+".*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	var lastErr error
	for _, f := range matches {
		if e := os.Remove(f); e != nil && !os.IsNotExist(e) {
			lastErr = e
		}
	}
	return lastErr
}

// ListUUIDs returns UUIDs that have a .state file in dir.
func ListUUIDs(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.state"))
	if err != nil {
		return nil, err
	}
	uuids := make([]string, 0, len(matches))
	for _, m := range matches {
		base := filepath.Base(m)
		uuid := strings.TrimSuffix(base, ".state")
		uuids = append(uuids, uuid)
	}
	return uuids, nil
}

// ReadInt returns the integer value of a state file, or 0 if missing/invalid.
func ReadInt(dir, uuid, typ string) int {
	v, err := strconv.Atoi(Read(dir, uuid, typ))
	if err != nil {
		return 0
	}
	return v
}

// FormatState returns the string to write into a .state file.
func FormatState(totalBytes uint64, epochSecs int64) string {
	return fmt.Sprintf("%d %d", totalBytes, epochSecs)
}
