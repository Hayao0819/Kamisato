package raiou

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type keyValue [2]string

func (kv keyValue) Key() string {
	return kv[0]
}
func (kv keyValue) Value() string {
	return kv[1]
}

func (kv keyValue) String() string {
	return fmt.Sprintf("%s = %s", kv[0], kv[1])
}

func parseKeyValues(lines []string) ([]keyValue, error) {
	kv := make([]keyValue, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid line: %s", trimmed)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			return nil, fmt.Errorf("invalid line: %s", trimmed)
		}
		kv = append(kv, keyValue{key, value})
	}
	return kv, nil
}

func KvToMap(kvs []keyValue) map[string]string {
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		m[kv.Key()] = kv.Value()
	}
	return m
}

func readLines(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading lines: %w", err)
	}
	return lines, nil
}

func parseUnixTimestamp(lines []string) (time.Time, error) {
	if len(lines) == 0 {
		return time.Time{}, fmt.Errorf("missing timestamp")
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, 0), nil
}

func parseInt(lines []string) (int64, error) {
	if len(lines) == 0 {
		return 0, fmt.Errorf("missing integer")
	}
	return strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64)
}
