package rest

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"

	"github.com/vanclief/agent-composer/interfaces/rest/handler"
	"github.com/vanclief/agent-composer/interfaces/rest/server"
)

func Start(ctx context.Context, s *server.Server, log zerolog.Logger) error {
	e := echo.New()
	h := handler.NewHandler(s)

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

	serverErr := make(chan error, 1)

	// Start the server
	go func() {
		err := e.Start(":" + s.Ctrl.Config.App.Port)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}

		serverErr <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := e.Shutdown(shutdownCtx)
		if err != nil {
			isCanceled := errors.Is(err, context.Canceled)
			isServerClosed := errors.Is(err, http.ErrServerClosed)

			if !isCanceled && !isServerClosed {
				return err
			}
		}

		return <-serverErr
	case err := <-serverErr:
		return err
	}
}
