package serverstats

import (
	"os"
	"strings"
	"syscall"
)

// cpuModel reads CPU model name from /proc/cpuinfo (Linux only).
func cpuModel() string {
	data, _ := os.ReadFile("/proc/cpuinfo")
	result := "?"
	for _, line := range strings.Split(string(data), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "model name" {
			result = strings.TrimSpace(v)
			break
		}
	}
	return result
}

// totalMemory reads total physical memory via sysinfo(2) (Linux only).
func totalMemory() uint64 {
	var info syscall.Sysinfo_t
	if syscall.Sysinfo(&info) == nil {
		return info.Totalram * uint64(info.Unit)
	}
	return 0
}
