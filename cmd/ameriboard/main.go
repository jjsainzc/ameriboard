package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"ameriboard/internal/config"
	"ameriboard/internal/controllers"
	"ameriboard/internal/profiles"
	"ameriboard/internal/routes"
	"ameriboard/internal/web"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	gommonlog "github.com/labstack/gommon/log"
)

var validBasePath = regexp.MustCompile(`^(/[A-Za-z0-9._~-]+)*$`)

var version = "desarrollo"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	configPath := flag.String("config", "", "ruta de configuración YAML")
	initPath := flag.String("init-config", "", "crear configuración YAML inicial")
	legacyEnv := flag.String("legacy-env", "", "archivo de entorno anterior")
	proxySocket := flag.String("proxy-socket", "", "socket Unix para el proxy")
	checkProxy := flag.Bool("check-proxy", false, "validar la configuración para el proxy instalado")
	stateDir := flag.String("state-dir", "/var/lib/ameriboard", "directorio privado de perfiles y credenciales")
	generateCredential := flag.String("generate-credential", "", "generar credencial admin o site")
	flag.Parse()
	store := profiles.NewStore(*stateDir)
	if *generateCredential != "" {
		secret, err := store.GenerateCredential(*generateCredential)
		if err != nil {
			return fmt.Errorf("generar credencial: %w", err)
		}
		fmt.Println(secret)
		return nil
	}
	if *initPath != "" {
		cfg, err := config.Initial(*initPath, *legacyEnv)
		if err != nil {
			return err
		}
		log.Printf("Configuración creada con dirección %s", cfg.ListenAddr)
		return nil
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	basePath, err := normalizeBasePath(cfg.BasePath)
	if err != nil {
		return err
	}
	if *checkProxy {
		if basePath != "/ameriboard" {
			return fmt.Errorf("el proxy automático requiere base_path /ameriboard, no %q", basePath)
		}
		return nil
	}
	mainSettings, err := store.LoadMainSettings(profiles.MainSettings{
		Title: cfg.Title, Subtitle: cfg.Subtitle, DVSwitchTitle: profiles.DefaultDVSwitchTitle, NXDNTitle: profiles.DefaultNXDNTitle, EchoLinkTitle: profiles.DefaultEchoLinkTitle,
	})
	if err != nil {
		return fmt.Errorf("cargar configuración principal guardada: %w", err)
	}
	cfg.Title = mainSettings.Title
	cfg.Subtitle = mainSettings.Subtitle
	frontend, err := web.Handler(basePath)
	if err != nil {
		return err
	}

	reflector := controllers.NewReflectorService(controllers.ReflectorConfig{
		Title: cfg.Title, Subtitle: cfg.Subtitle, Badge: cfg.Badge, DVSwitchTitle: mainSettings.DVSwitchTitle, NXDNTitle: mainSettings.NXDNTitle, EchoLinkTitle: mainSettings.EchoLinkTitle,
		YSFReflector: cfg.YSFReflector, TGIFTalkgroup: cfg.TGIFTalkgroup,
		YSFReflectorConfig:   cfg.YSFReflectorConfig,
		PYSFReflectorConfig:  cfg.PYSFReflectorConfig,
		PYSFReflectorService: cfg.PYSFReflectorService,
		BridgeCall:           cfg.BridgeCall, YSFLogDir: cfg.YSFLogDir, DMRLogDir: cfg.DMRLogDir,
		EchoLinkCommand:     cfg.EchoLinkCommand,
		EchoLinkHistoryFile: filepath.Join(*stateDir, "echolink-history.json"),
		BridgeConfigDir:     "/opt",
		Monitoring:          mainSettings.Monitoring,
	})
	sites := profiles.NewHandler(store)
	sites.SetEchoLinkActionExecutor(func(ctx context.Context, action profiles.EchoLinkAction) (string, error) {
		return reflector.ExecuteEchoLinkAction(ctx, controllers.EchoLinkActionRequest{
			Action: action.Action, Callsign: action.Callsign, IP: action.IP, Message: action.Message,
			Permanent: action.Permanent,
		})
	})
	sites.SetMainSettings(mainSettings, func(settings profiles.MainSettings) {
		reflector.SetDashboardSettings(settings.Title, settings.Subtitle, settings.DVSwitchTitle, settings.NXDNTitle, settings.EchoLinkTitle, settings.Monitoring)
	})
	defer reflector.Close()
	e := newServer(basePath, frontend, reflector, sites)
	tcp, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("escuchar en %s: %w", cfg.ListenAddr, err)
	}
	defer tcp.Close()

	server := &http.Server{Handler: e}
	errors := make(chan error, 2)
	if *proxySocket != "" {
		socket, err := net.Listen("unix", *proxySocket)
		if err != nil {
			return fmt.Errorf("abrir socket del proxy: %w", err)
		}
		defer socket.Close()
		if err := os.Chmod(*proxySocket, 0666); err != nil {
			return fmt.Errorf("configurar socket del proxy: %w", err)
		}
		go func() { errors <- server.Serve(socket) }()
	}
	log.Printf("Escuchando en %s bajo subruta %s", cfg.ListenAddr, basePath)
	go func() { errors <- server.Serve(tcp) }()
	if err := notifyReady(); err != nil {
		server.Close()
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err = <-errors:
		server.Close()
		if err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			server.Close()
			return fmt.Errorf("detener servidor: %w", err)
		}
	}
	return nil
}

func notifyReady() error {
	address := os.Getenv("NOTIFY_SOCKET")
	if address == "" {
		return nil
	}
	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: address, Net: "unixgram"})
	if err != nil {
		return fmt.Errorf("conectar a systemd: %w", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("READY=1")); err != nil {
		return fmt.Errorf("notificar a systemd: %w", err)
	}
	return nil
}

func newServer(basePath string, frontend http.Handler, reflector *controllers.ReflectorService, sites *profiles.Handler) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Logger.SetLevel(gommonlog.INFO)
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	routes.Register(e, basePath, frontend, reflector, sites)
	e.GET(basePath+"/api/version", func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(http.StatusOK, map[string]string{"version": version})
	})
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
