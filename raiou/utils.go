package raiou

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

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
