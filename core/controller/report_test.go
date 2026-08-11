package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigReportSQLiteDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ENVIRONMENT", "LOCAL")

	loaded, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}

	report, err := loaded.ConfigReport()
	if err != nil {
		t.Fatal(err)
	}

	if report.Database != "sqlite" {
		t.Fatalf("expected sqlite without a config file, got %q", report.Database)
	}
	if report.ConfigFileFound {
		t.Fatal("no config file exists, the report must say so")
	}
	if report.SQLitePath != filepath.Join(home, DEFAULT_DATA_DIR, "agc.db") {
		t.Fatalf("unexpected sqlite path: %q", report.SQLitePath)
	}
	if !strings.Contains(report.Source, "no config file") {
		t.Fatalf("the source must explain the default: %q", report.Source)
	}
}

func TestConfigReportPostgresOptIn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ENVIRONMENT", "LOCAL")

	configDir := filepath.Join(home, DEFAULT_CONFIG_DIR)
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(configDir, "local.config.json"),
		[]byte(`{"postgres": {"host": "localhost:5432", "database": "agc_test"}}`),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}

	report, err := loaded.ConfigReport()
	if err != nil {
		t.Fatal(err)
	}

	if report.Database != "postgres" {
		t.Fatalf("expected postgres with a postgres section, got %q", report.Database)
	}
	if !report.ConfigFileFound {
		t.Fatal("the config file exists, the report must say so")
	}
	if report.PostgresHost != "localhost:5432" || report.PostgresDatabase != "agc_test" {
		t.Fatalf("unexpected postgres details: %q %q", report.PostgresHost, report.PostgresDatabase)
	}
	if !strings.Contains(report.Source, "postgres section in") {
		t.Fatalf("the source must explain the opt-in: %q", report.Source)
	}
}
