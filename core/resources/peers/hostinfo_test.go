package peers

import (
	"testing"
)

func TestParseMeminfo(t *testing.T) {
	data := []byte("MemTotal:       16384000 kB\nMemFree:         1000000 kB\nMemAvailable:    8192000 kB\n")

	total, free, err := parseMeminfo(data)
	if err != nil {
		t.Fatal(err)
	}
	if total != 16000 || free != 8000 {
		t.Fatalf("total %d free %d, want 16000 8000", total, free)
	}

	_, _, err = parseMeminfo([]byte("MemTotal: 1 kB\n"))
	if err == nil {
		t.Fatal("no MemAvailable: want an error")
	}
}

func TestParseVMStat(t *testing.T) {
	// The shape macOS prints on Apple Silicon.
	data := []byte(`Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               10000.
Pages active:                            200000.
Pages inactive:                            5000.
Pages speculative:                         1000.
Pages throttled:                              0.
`)

	free, err := parseVMStat(data)
	if err != nil {
		t.Fatal(err)
	}
	if free != 16000*16384 {
		t.Fatalf("free = %d, want %d", free, 16000*16384)
	}

	_, err = parseVMStat([]byte("Pages free: 1.\n"))
	if err == nil {
		t.Fatal("no page size: want an error")
	}
}
