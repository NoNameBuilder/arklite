package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

func parseThreads(value string) (int, error) {
	v := strings.TrimSpace(strings.ToLower(value))
	if v == "" || v == "auto" {
		return defaultThreads(), nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid --threads value %q; use auto or a positive integer", value)
	}
	host := runtime.NumCPU()
	if n > host {
		return 0, fmt.Errorf("invalid --threads value %q; host limit is %d", value, host)
	}
	return n, nil
}
