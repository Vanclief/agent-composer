package controller

import (
	"os"
	"path/filepath"

	"github.com/vanclief/ez"
)

// ConfigReport describes the effective configuration and where each
// load-bearing choice came from, so a user never has to guess why agc
// connected to one database or another.
type ConfigReport struct {
	Environment     string `json:"environment"`
	ConfigFile      string `json:"config_file"`
	ConfigFileFound bool   `json:"config_file_found"`
	Database        string `json:"database"`
	// Source explains why that database engine was selected.
	Source           string `json:"source"`
	SQLitePath       string `json:"sqlite_path,omitempty"`
	PostgresHost     string `json:"postgres_host,omitempty"`
	PostgresDatabase string `json:"postgres_database,omitempty"`
	HomeDir          string `json:"home_dir"`
	AppPort          string `json:"app_port"`
}

// ConfigReport works on a Load-ed controller — no database connection
// is needed or made.
func (controller *Controller) ConfigReport() (*ConfigReport, error) {
	report := &ConfigReport{
		Environment:     controller.Environment,
		ConfigFile:      controller.configFile,
		ConfigFileFound: controller.configFileFound,
		AppPort:         controller.Config.App.Port,
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, ez.Wrap(err)
	}
	report.HomeDir = filepath.Join(home, DEFAULT_DATA_DIR)

	if controller.usesPostgres() {
		report.Database = "postgres"
		report.Source = "postgres section in " + controller.configFile
		report.PostgresHost = controller.Config.Postgres.Host
		report.PostgresDatabase = controller.Config.Postgres.Database

		return report, nil
	}

	report.Database = "sqlite"

	sqlitePath := controller.Config.SQLite.Path
	if sqlitePath == "" {
		sqlitePath, err = defaultSQLitePath()
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}
	report.SQLitePath = sqlitePath

	if controller.configFileFound {
		report.Source = "no postgres section in " + controller.configFile + " — sqlite default"
	} else {
		report.Source = "no config file — sqlite default"
	}

	return report, nil
}
