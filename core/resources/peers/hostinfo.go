package peers

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// HostReport is a server's own report of its host: what it has and how
// much memory is free now, read live on every request (ADR 0018).
type HostReport struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	CPUs      int    `json:"cpus"`
	MemoryMiB uint64 `json:"memory_mib"`
	// FreeMemoryMiB is the memory the host could hand out now: Linux's
	// MemAvailable, macOS's free, inactive and speculative pages. Zero
	// with FreeMemoryError set when it cannot be read.
	FreeMemoryMiB   uint64 `json:"free_memory_mib"`
	FreeMemoryError string `json:"free_memory_error,omitempty"`
}

// readHost reports this host.
func readHost(ctx context.Context) HostReport {
	report := HostReport{OS: runtime.GOOS, Arch: runtime.GOARCH, CPUs: runtime.NumCPU()}

	total, free, err := readMemory(ctx)
	if err != nil {
		report.FreeMemoryError = err.Error()
		return report
	}

	report.MemoryMiB = total
	report.FreeMemoryMiB = free

	return report
}

// readMemory returns the total and free memory in MiB.
func readMemory(ctx context.Context) (uint64, uint64, error) {
	if runtime.GOOS == "darwin" {
		return readDarwinMemory(ctx)
	}

	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}

	return parseMeminfo(data)
}

// parseMeminfo reads MemTotal and MemAvailable (kB) from /proc/meminfo.
func parseMeminfo(data []byte) (uint64, uint64, error) {
	values := map[string]uint64{}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		n, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}

		values[strings.TrimSuffix(fields[0], ":")] = n
	}

	total, ok := values["MemTotal"]
	if !ok {
		return 0, 0, errNoMemory("MemTotal is not in /proc/meminfo")
	}

	available, ok := values["MemAvailable"]
	if !ok {
		return 0, 0, errNoMemory("MemAvailable is not in /proc/meminfo")
	}

	return total / 1024, available / 1024, nil
}

// readDarwinMemory asks sysctl for the total and vm_stat for the free
// pages.
func readDarwinMemory(ctx context.Context) (uint64, uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	memsize, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, 0, errNoMemory("sysctl -n hw.memsize failed: " + err.Error())
	}

	totalBytes, err := strconv.ParseUint(strings.TrimSpace(string(memsize)), 10, 64)
	if err != nil {
		return 0, 0, errNoMemory("sysctl -n hw.memsize printed " + strings.TrimSpace(string(memsize)))
	}

	vmstat, err := exec.CommandContext(ctx, "vm_stat").Output()
	if err != nil {
		return 0, 0, errNoMemory("vm_stat failed: " + err.Error())
	}

	freeBytes, err := parseVMStat(vmstat)
	if err != nil {
		return 0, 0, err
	}

	return totalBytes / (1 << 20), freeBytes / (1 << 20), nil
}

var pageSizeRe = regexp.MustCompile(`page size of ([0-9]+) bytes`)

// parseVMStat sums macOS's free, inactive and speculative pages, in bytes.
func parseVMStat(data []byte) (uint64, error) {
	match := pageSizeRe.FindSubmatch(data)
	if match == nil {
		return 0, errNoMemory("vm_stat printed no page size")
	}

	pageSize, err := strconv.ParseUint(string(match[1]), 10, 64)
	if err != nil {
		return 0, errNoMemory("vm_stat printed a bad page size")
	}

	wanted := map[string]bool{"Pages free": true, "Pages inactive": true, "Pages speculative": true}
	pages := uint64(0)
	found := 0

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		name, value, ok := strings.Cut(scanner.Text(), ":")
		if !ok || !wanted[strings.TrimSpace(name)] {
			continue
		}

		n, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(value), "."), 10, 64)
		if err != nil {
			continue
		}

		pages += n
		found++
	}

	if found == 0 {
		return 0, errNoMemory("vm_stat printed no free pages")
	}

	return pages * pageSize, nil
}

type errNoMemory string

func (e errNoMemory) Error() string {
	return string(e)
}
