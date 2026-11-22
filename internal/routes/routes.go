package routes

import (
	"net/http"

	"ameriboard/internal/controllers"

	"github.com/labstack/echo/v4"
)

func Register(e *echo.Echo, basePath string, frontend http.Handler, reflector *controllers.ReflectorService) {
	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	group := e.Group(basePath)
	group.GET("/api/reflector-status", reflector.GetStatus)
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
