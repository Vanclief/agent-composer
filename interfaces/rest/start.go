package rest

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"

	"github.com/vanclief/agent-composer/interfaces/rest/handler"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
)

func Start(ctx context.Context, app *restserver.Server, log zerolog.Logger) {
	ctrl := app.GetController()
	e := echo.New()
	h := handler.NewHandler(app)

	// Custom Error Handler
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
		}

		statusText := http.StatusText(code)

		log.Error().
			Str("Method", c.Request().Method).
			Int("Status Code", code).
			Str("Message", statusText).
			Str("URL", c.Request().URL.String()).
			Str("IP", c.RealIP()).
			Msg("HTTP Request Error")

		// Send response
		if err := c.JSON(code, map[string]string{"message": statusText}); err != nil {
			c.Logger().Error(err)
		}
	}

	// Midleware
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Routes
	addAPIRoutes(e, h)

	// Config
	e.HideBanner = true

	// Start the server
	go func() {
		err := e.Start(":" + ctrl.Config.App.Port)
		if err != nil && err != http.ErrServerClosed {
			e.Logger.Error(err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server with a timeout of 10 seconds.
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := e.Shutdown(ctx)
	if err != nil {
		e.Logger.Fatal(err)
	}
}
