package controller

import (
	"github.com/vanclief/compose/drivers/databases/relational/postgres"
	"github.com/vanclief/compose/drivers/databases/relational/sqlite"
	"github.com/vanclief/compose/integrations/promtail"
)

type AppSettings struct {
	Name string
	// Host is the address agc rest listens on; see ResolveAppHost. Nil
	// (no host in the config file) means BuiltinAppHost, an empty string
	// means every interface.
	Host            *string `mapstructure:"host"`
	Port            string
	Debug           bool
	Version         string
	RateLimit       int // In requests
	RateLimitWindow int // In seconds
}

// Config contains the configuration file settings. The database defaults to
// a local SQLite file, and a postgres section opts into PostgreSQL instead.
type Config struct {
	App      AppSettings               `mapstructure:"app"`
	Promtail promtail.Config           `mapstructure:"promtail"`
	Postgres postgres.ConnectionConfig `mapstructure:"postgres"`
	SQLite   sqlite.ConnectionConfig   `mapstructure:"sqlite"`
	// Defaults fill in a task's assignee, model and owner (ADR 0009).
	Defaults DefaultsSettings `mapstructure:"defaults"`
	// Triggers are outside events posted as messages (ADR 0014).
	Triggers TriggersSettings `mapstructure:"triggers"`
	// Sandboxes holds keep_done, the grace period before a done task's
	// stopped sandbox is collected (ADR 0008).
	Sandboxes SandboxesSettings `mapstructure:"sandboxes"`
	// Messaging holds archive_done, the grace period before a done task's
	// channel is archived (ADR 0005 "Archived channels").
	Messaging MessagingSettings `mapstructure:"messaging"`
}
