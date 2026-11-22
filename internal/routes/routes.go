package routes

import (
	"net/http"

	"ameriboard/internal/controllers"
	"ameriboard/internal/profiles"

	"github.com/labstack/echo/v4"
)

func Register(e *echo.Echo, basePath string, frontend http.Handler, reflector *controllers.ReflectorService, sites *profiles.Handler) {
	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	group := e.Group(basePath)
	group.GET("/api/reflector-status", reflector.GetStatus)
	sites.Register(group, basePath, reflector.GetStatus)
	group.GET("/api/*", func(c echo.Context) error {
		return echo.ErrNotFound
	})

	if basePath != "" {
		group.GET("", func(c echo.Context) error {
			return c.Redirect(http.StatusMovedPermanently, basePath+"/")
		})
	}
	handler := echo.WrapHandler(http.StripPrefix(basePath, frontend))
	group.GET("/", handler)
	group.GET("/*", handler)
}
