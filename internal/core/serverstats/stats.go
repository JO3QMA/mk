// Package serverstats provides server machine statistics (CPU, memory, disk)
// for the /api/admin/server-info endpoint.
package serverstats

import (
	"os"
	"runtime"
	"syscall"
)

// Stats holds server machine information matching Misskey's server-info response.
type Stats struct {
	Machine MachineInfo `json:"machine"`
	CPU     CPUInfo     `json:"cpu"`
	Mem     MemInfo     `json:"mem"`
	FS      FSInfo      `json:"fs"`
}

// MachineInfo holds the hostname.
type MachineInfo struct {
	Name string `json:"name"`
}

// CPUInfo holds CPU model and core count.
type CPUInfo struct {
	Model string `json:"model"`
	Cores int    `json:"cores"`
}

// MemInfo holds total physical memory in bytes.
type MemInfo struct {
	Total uint64 `json:"total"`
}

// FSInfo holds total and used disk space in bytes for the root filesystem.
type FSInfo struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

// Collect gathers current server statistics.
func Collect() Stats {
	hostname, _ := os.Hostname()

	var fs FSInfo
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		fs.Total = stat.Blocks * uint64(stat.Bsize)
		fs.Used = (stat.Blocks - stat.Bfree) * uint64(stat.Bsize)
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return Stats{
		Machine: MachineInfo{Name: hostname},
		CPU: CPUInfo{
			Model: cpuModel(),
			Cores: runtime.NumCPU(),
		},
		Mem: MemInfo{Total: totalMemory()},
		FS:  fs,
	}
}

// Empty returns zeroed stats (used when enableServerMachineStats is false).
func Empty() Stats {
	return Stats{
		Machine: MachineInfo{Name: "?"},
		CPU:     CPUInfo{Model: "?", Cores: 0},
		Mem:     MemInfo{Total: 0},
		FS:      FSInfo{Total: 0, Used: 0},
	}
}
