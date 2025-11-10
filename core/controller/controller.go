package controller

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/vanclief/agent-composer/models"
	"github.com/vanclief/compose/components/configurator"
	"github.com/vanclief/compose/components/ctrl"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
)

const CONFIG_DIR = ".agent_composer/config"

func resolveConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, CONFIG_DIR) + string(os.PathSeparator), nil
}

type Controller struct {
	ctrl.BaseController
	Config  Config
	EnvVars EnvVars
	DB      *relational.DB
}

func New() (*Controller, error) {
	const op = "Controller.New"

	// Create a new instance
	controller := &Controller{}

	// Load the configuration
	e := EnvVars{}
	c := Config{}

	configDir, err := resolveConfigDir()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	opts := []configurator.Option{}
	opts = append(opts, configurator.WithRequiredEnv("ENVIRONMENT"))
	opts = append(opts, configurator.WithRequiredEnv("POSTGRES_PASSWORD"))
	opts = append(opts, configurator.WithConfigPath(configDir))

	cfg, err := configurator.New(opts...)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	controller.Environment = cfg.Environment

	err = cfg.LoadEnvVars(&e)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = cfg.LoadConfiguration(&c)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.App.Name = "Agent Composer"
			c.App.Port = "8080"
			c.App.RateLimit = 60
			c.App.RateLimitWindow = 10
			c.Postgres.Host = "localhost:5432"
			c.Postgres.Username = "agent_composer"
			c.Postgres.Database = "agent_composer"
		} else {
			return nil, ez.Wrap(op, err)
		}
	}

	controller.EnvVars = e
	controller.Config = c

	controller.WithZerolog()

	err = controller.Setup()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return controller, nil
}

func (controller *Controller) Setup() error {
	const op = "Controller.Setup"

	// Connect to the database
	psqlConfig := &controller.Config.Postgres
	psqlConfig.Password = controller.EnvVars.PostgresPassword

	opts := []relational.Option{
		relational.WithRegistrableModels(models.REGISTRABLE),
		relational.WithExtensions([]string{"uuid-ossp", "unaccent"}),
	}

	db, err := controller.WithPostgres(psqlConfig, models.ALL, opts...)
	if err != nil {
		return ez.Wrap(op, err)
	}

	// NOTE: Disabled this for clean installs
	// Apply pending migrations
	// if controller.Environment != "TEST" {
	// 	err = db.RunMigrations(migrations.Migrations)
	// 	if err != nil {
	// 		return ez.Wrap(op, err)
	// 	}
	// }

	controller.DB = db

	return nil
}
