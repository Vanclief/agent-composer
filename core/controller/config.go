package controller

import (
	"github.com/vanclief/compose/drivers/databases/relational/postgres"
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

// ConfigSettings contains the config.yml settings
type Config struct {
	App      AppSettings               `mapstructure:"app"`
	Promtail promtail.Config           `mapstructure:"promtail"`
	Postgres postgres.ConnectionConfig `mapstructure:"postgres"`
}
