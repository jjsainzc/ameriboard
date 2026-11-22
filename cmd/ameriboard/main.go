package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"ameriboard/internal/web"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var validBasePath = regexp.MustCompile(`^(/[A-Za-z0-9._~-]+)*$`)

func main() {
	listenAddr := getEnv("LISTEN_ADDR", "127.0.0.1:8082")
	basePath, err := normalizeBasePath(getEnv("BASE_PATH", "/dashboard"))
	if err != nil {
		log.Fatal(err)
	}
	frontend, err := web.Handler(basePath)
	if err != nil {
		log.Fatal(err)
	}

	e := newServer(basePath, frontend)
	log.Printf("Escuchando en %s bajo subruta %s", listenAddr, basePath)
	if err := e.Start(listenAddr); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func newServer(basePath string, frontend http.Handler) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	group := e.Group(basePath)
	group.GET("/api/reflector-status", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"status": "ok"})
	})
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
	return e
}

func normalizeBasePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" || p == "/" {
		return "", nil
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimRight(p, "/")
	if !validBasePath.MatchString(p) {
		return "", fmt.Errorf("BASE_PATH inválido: %q", p)
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == "." || segment == ".." {
			return "", fmt.Errorf("BASE_PATH inválido: %q", p)
		}
	}
	return p, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
