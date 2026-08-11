package controller

import (
	"github.com/vanclief/compose/drivers/databases/relational/postgres"
	"github.com/vanclief/compose/drivers/databases/relational/sqlite"
	"github.com/vanclief/compose/integrations/promtail"
)

type AppSettings struct {
	Name            string
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
}
