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
	// Defaults fill in a task's assignee, model and owner (ADR 0009).
	Defaults DefaultsSettings `mapstructure:"defaults"`
	// Servers are the peers this server allocates sandboxes on; the local
	// server is always "local" and needs no entry (ADR 0018).
	Servers []ServerSettings `mapstructure:"servers"`
	// Primary is set on a peer: the server its sandboxes' sockets forward
	// to (ADR 0018).
	Primary PrimarySettings `mapstructure:"primary"`
	// Listen is the address agc rest binds, such as 127.0.0.1:1202. Empty
	// keeps ":" + app.port.
	Listen string `mapstructure:"listen"`
}
