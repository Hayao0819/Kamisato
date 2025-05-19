package raiou

import (
	"fmt"
	"strings"
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
