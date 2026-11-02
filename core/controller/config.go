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
	// Notify says where a human's direct messages are forwarded (ADR 0011).
	Notify NotifySettings `mapstructure:"notify"`
}

// NotifySettings is the notify: block of the config file.
type NotifySettings struct {
	WhatsApp WhatsAppSettings `mapstructure:"whatsapp"`
}

// WhatsAppSettings is notify.whatsapp: the sending number's id at Meta and
// each human's own number, digits with the country code. The access token
// is not here: it is the secret WHATSAPP_TOKEN (ADR 0004).
type WhatsAppSettings struct {
	PhoneNumberID string `mapstructure:"phone_number_id"`
	// Version is the Graph API version, such as v21.0; empty uses the default.
	Version string `mapstructure:"version"`
	// Numbers maps a human's name, as in @<human>/<task>, to their number.
	Numbers map[string]string `mapstructure:"numbers"`
}
