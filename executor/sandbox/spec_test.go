package sandbox

import (
	"runtime"
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

func TestValidateDefaults(t *testing.T) {
	spec := Spec{Name: "task-1"}
	err := spec.Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if spec.Image != "agc-agent" {
		t.Errorf("Image = %q, want agc-agent", spec.Image)
	}
	if spec.CPUs != 2 {
		t.Errorf("CPUs = %d, want 2", spec.CPUs)
	}
	if spec.MemoryMiB != 4096 {
		t.Errorf("MemoryMiB = %d, want 4096", spec.MemoryMiB)
	}
	if spec.User != "agent" {
		t.Errorf("User = %q, want agent", spec.User)
	}
}

func TestValidateKeepsValues(t *testing.T) {
	spec := Spec{Name: "task-1", Image: "debian:bookworm", CPUs: 1, MemoryMiB: 1024, User: "root"}
	err := spec.Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if spec.Image != "debian:bookworm" || spec.CPUs != 1 || spec.MemoryMiB != 1024 || spec.User != "root" {
		t.Errorf("Validate changed set values: %+v", spec)
	}
}

func TestValidateErrors(t *testing.T) {
	if runtime.NumCPU() >= 255 {
		t.Skip("host has too many CPUs to exceed")
	}
	tests := []struct {
		name string
		spec Spec
		want string
	}{
		{"missing name", Spec{}, "name"},
		{"long name", Spec{Name: strings.Repeat("a", 129)}, "name"},
		{"too many cpus", Spec{Name: "t", CPUs: 255}, "cpus"},
		{"too little memory", Spec{Name: "t", MemoryMiB: 64}, "memory"},
		{"volume without source", Spec{Name: "t", Volumes: []Volume{{Dest: "/data"}}}, "volumes[0]: source"},
		{"volume without dest", Spec{Name: "t", Volumes: []Volume{{Source: "cache"}}}, "volumes[0] (cache): dest"},
		{"secret without name", Spec{Name: "t", Secrets: []Secret{{Hosts: []string{"github.com"}}}}, "secrets[0]: name"},
		{"secret without hosts", Spec{Name: "t", Secrets: []Secret{{Name: "GITHUB_TOKEN"}}}, "secrets[0] (GITHUB_TOKEN): hosts"},
		{"vsock without port", Spec{Name: "t", VsockPath: "/tmp/agc.sock"}, "vsock port"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if err == nil {
				t.Fatal("Validate returned nil")
			}
			if ez.ErrorCode(err) != ez.EINVALID {
				t.Errorf("code = %s, want %s", ez.ErrorCode(err), ez.EINVALID)
			}
			if !strings.Contains(ez.ErrorMessage(err), tt.want) {
				t.Errorf("message %q does not mention %q", ez.ErrorMessage(err), tt.want)
			}
		})
	}
}
