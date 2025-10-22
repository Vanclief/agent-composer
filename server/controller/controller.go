package controller

import (
	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/models"
	"github.com/vanclief/compose/components/configurator"
	"github.com/vanclief/compose/components/ctrl"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
)

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

	opts := []configurator.Option{}
	opts = append(opts, configurator.WithRequiredEnv("ENVIRONMENT"))
	opts = append(opts, configurator.WithRequiredEnv("POSTGRES_PASSWORD"))
	opts = append(opts, configurator.WithConfigPath("server/config/"))

	err := controller.LoadEnvVarsAndConfig(&e, &c, opts...)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	controller.EnvVars = e
	controller.Config = c

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

	log.Info().
		Str("Env", controller.Environment).
		Bool("Debug", controller.Config.App.Debug).
		Msg("Instantiated controller")

	return nil
}
