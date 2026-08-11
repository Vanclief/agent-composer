package controller

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/models"
	"github.com/vanclief/compose/components/configurator"
	"github.com/vanclief/compose/components/ctrl"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
)

const DEFAULT_CONFIG_DIR = ".agent_composer/config"

const DEFAULT_DATA_DIR = ".agent_composer"

const defaultEnvironment = "LOCAL"

type Controller struct {
	ctrl.BaseController
	Config  Config
	EnvVars EnvVars
	DB      *relational.DB
}

func New() (*Controller, error) {
	return NewWithLogWriter(nil)
}

func NewWithLogWriter(writer io.Writer) (*Controller, error) {
	// The configurator requires ENVIRONMENT, but a CLI tool should boot with
	// zero setup, so an unset environment defaults to LOCAL
	if os.Getenv("ENVIRONMENT") == "" {
		err := os.Setenv("ENVIRONMENT", defaultEnvironment)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	// Create a new instance
	controller := &Controller{}

	// Create the logger
	controller.WithZerolog()
	if writer != nil {
		output := zerolog.ConsoleWriter{Out: writer}
		output.FormatMessage = func(value interface{}) string {
			message, ok := value.(string)
			if ok {
				return fmt.Sprintf("%-50s", message)
			}
			return ""
		}

		log.Logger = log.Output(output)
	}

	// Load the configuration
	e := EnvVars{}
	c := Config{}

	configDir, err := resolveConfigDir()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	opts := []configurator.Option{}
	opts = append(opts, configurator.WithRequiredEnv("ENVIRONMENT"))
	opts = append(opts, configurator.WithOptionalEnv("POSTGRES_PASSWORD"))
	opts = append(opts, configurator.WithConfigPath(configDir))

	cfg, err := configurator.New(opts...)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = cfg.LoadEnvVars(&e)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	controller.Environment = cfg.Environment

	err = cfg.LoadConfiguration(&c)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || ez.ErrorCode(err) == ez.ENOTFOUND {
			log.Info().Msg("Configuration file not found, using default configuration")
			c.App.Name = "Agent Composer"
			c.App.Port = "8080"
			c.App.RateLimit = 60
			c.App.RateLimitWindow = 10
		} else {
			return nil, ez.Wrap(err)
		}
	}

	controller.EnvVars = e
	controller.Config = c

	log.Info().Str("Env", controller.Environment).Msg("Starting Agent Composer")

	err = controller.Setup()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return controller, nil
}

func (controller *Controller) Setup() error {
	opts := []relational.Option{
		relational.WithRegistrableModels(models.REGISTRABLE),
	}

	// A postgres section in the configuration opts into PostgreSQL. The
	// default is a local SQLite file so the CLI runs without any external
	// database dependency.
	if controller.usesPostgres() {
		psqlConfig := &controller.Config.Postgres
		if controller.EnvVars.PostgresPassword != "" {
			psqlConfig.Password = controller.EnvVars.PostgresPassword
		}

		opts = append(opts, relational.WithExtensions([]string{"uuid-ossp", "unaccent"}))

		db, err := controller.WithPostgres(psqlConfig, models.ALL, opts...)
		if err != nil {
			return ez.Wrap(err)
		}

		controller.DB = db

		return nil
	}

	sqliteConfig := &controller.Config.SQLite
	if sqliteConfig.Path == "" {
		path, err := defaultSQLitePath()
		if err != nil {
			return ez.Wrap(err)
		}

		sqliteConfig.Path = path
	}

	db, err := controller.WithSQLite(sqliteConfig, models.ALL, opts...)
	if err != nil {
		return ez.Wrap(err)
	}

	controller.DB = db

	return nil
}

func (controller *Controller) usesPostgres() bool {
	return controller.Config.Postgres.Host != "" || controller.Config.Postgres.Database != ""
}

func defaultSQLitePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, DEFAULT_DATA_DIR, "agc.db"), nil
}

func resolveConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, DEFAULT_CONFIG_DIR) + string(os.PathSeparator), nil
}
