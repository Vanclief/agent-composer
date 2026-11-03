package controller

import (
	"os"
	"path/filepath"
	"testing"
)

// loadWithConfig loads the controller under a fresh HOME, with config as
// local.config.json when it is not empty.
func loadWithConfig(t *testing.T, config string) *Controller {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ENVIRONMENT", "LOCAL")

	if config != "" {
		configDir := filepath.Join(home, DEFAULT_CONFIG_DIR)
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(filepath.Join(configDir, "local.config.json"), []byte(config), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	loaded, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}

	return loaded
}

func TestListenAddress(t *testing.T) {
	cases := []struct {
		name   string
		env    string
		config string
		want   string
	}{
		{"no config, no env", "", "", "127.0.0.1:1202"},
		{"env opens it up", "0.0.0.0", "", "0.0.0.0:1202"},
		{"config without a host", "", `{"app": {"port": "1300"}}`, "127.0.0.1:1300"},
		{"config host", "", `{"app": {"host": "0.0.0.0", "port": "1202"}}`, "0.0.0.0:1202"},
		{"config empty host is every interface", "", `{"app": {"host": "", "port": "1202"}}`, ":1202"},
		{"env over config", "10.0.0.5", `{"app": {"host": "0.0.0.0", "port": "1202"}}`, "10.0.0.5:1202"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("AGC_APP_HOST", c.env)

			loaded := loadWithConfig(t, c.config)

			got := loaded.ListenAddress()
			if got != c.want {
				t.Fatalf("listen address = %q, want %q", got, c.want)
			}
		})
	}
}

func TestConfigReportAppHost(t *testing.T) {
	t.Setenv("AGC_APP_HOST", "")

	loaded := loadWithConfig(t, "")

	report, err := loaded.ConfigReport()
	if err != nil {
		t.Fatal(err)
	}

	if report.AppHost != "127.0.0.1" || report.AppPort != "1202" {
		t.Fatalf("report app = %q:%q, want 127.0.0.1:1202", report.AppHost, report.AppPort)
	}
}
