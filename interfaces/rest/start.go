package rest

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"

	"github.com/vanclief/agent-composer/interfaces/rest/handler"
	"github.com/vanclief/agent-composer/interfaces/rest/server"
	"github.com/vanclief/agent-composer/panel"
	"github.com/vanclief/agent-composer/web"
)

func Start(ctx context.Context, s *server.Server, log zerolog.Logger) error {
	e := echo.New()
	h := handler.NewHandler(s)

	// Custom Error Handler
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		he, ok := err.(*echo.HTTPError)
		if ok {
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
		err = c.JSON(code, map[string]string{"message": statusText})
		if err != nil {
			c.Logger().Error(err)
		}
	}

	// Midleware
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Routes
	addAPIRoutes(e, h)
	err := useSPA(e)
	if err != nil {
		return err
	}
	err = usePanel(e)
	if err != nil {
		return err
	}

	// Config
	e.HideBanner = true

	serverErr := make(chan error, 1)

	// Start the server
	go func() {
		err := e.Start(s.Ctrl.ListenAddress())
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

func useSPA(e *echo.Echo) error {
	dist, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return err
	}

	redirectToRoot := func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/")
	}
	e.GET("/index.html", func(c echo.Context) error {
		executionID := strings.TrimSpace(c.QueryParam("execution_id"))
		if executionID == "" {
			return redirectToRoot(c)
		}

		return c.Redirect(
			http.StatusMovedPermanently,
			"/runs/"+url.PathEscape(executionID),
		)
	})
	e.GET("/workflow.html", func(c echo.Context) error {
		workflowID := strings.TrimSpace(c.QueryParam("id"))
		if workflowID == "" {
			return redirectToRoot(c)
		}

		target := "/workflow/" + url.PathEscape(workflowID)
		runID := strings.TrimSpace(c.QueryParam("run"))
		if runID != "" {
			target += "?run=" + url.QueryEscape(runID)
		}

		return c.Redirect(http.StatusMovedPermanently, target)
	})

	e.Use(middleware.StaticWithConfig(middleware.StaticConfig{
		Skipper: func(c echo.Context) bool {
			path := c.Request().URL.Path
			isAPI := path == "/api" || strings.HasPrefix(path, "/api/")
			isLegacyRedirect := path == "/index.html" || path == "/workflow.html"
			isPanel := path == "/panel" || strings.HasPrefix(path, "/panel/")
			return isAPI || isLegacyRedirect || isPanel
		},
		HTML5:      true,
		Filesystem: http.FS(dist),
	}))

	return nil
}

// usePanel serves the control panel build embedded from panel/dist at
// /panel/. For development, set AGC_PANEL_DIR to a build directory (such
// as panel/dist) to serve it from disk, so a rebuild shows without
// rebuilding agc.
func usePanel(e *echo.Echo) error {
	dist, err := fs.Sub(panel.Dist, "dist")
	if err != nil {
		return err
	}

	dir := os.Getenv("AGC_PANEL_DIR")
	if dir != "" {
		dist = os.DirFS(dir)
	}

	servePanel(e, dist)

	return nil
}

// servePanel serves files from dist at /panel/. Unknown paths get
// index.html, so the panel's router handles deep links. Without
// index.html the panel was never built and every path answers 503.
func servePanel(e *echo.Echo, dist fs.FS) {
	e.GET("/panel", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/panel/")
	})
	e.GET("/panel/*", func(c echo.Context) error {
		_, err := fs.Stat(dist, "index.html")
		if err != nil {
			return c.String(http.StatusServiceUnavailable, "control panel not built: run make panel")
		}

		name := c.Param("*")
		info, err := fs.Stat(dist, name)
		if err != nil || info.IsDir() {
			name = "index.html"
		}

		return echo.StaticFileHandler(name, dist)(c)
	})
}
