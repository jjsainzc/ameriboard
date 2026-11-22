package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr      string `yaml:"listen_addr"`
	BasePath        string `yaml:"base_path"`
	Title           string `yaml:"title"`
	Subtitle        string `yaml:"subtitle"`
	Badge           string `yaml:"badge"`
	YSFReflector    string `yaml:"ysf_reflector"`
	TGIFTalkgroup   string `yaml:"tgif_talkgroup"`
	BridgeCall      string `yaml:"bridge_callsign"`
	YSFLogDir       string `yaml:"ysf_log_dir"`
	DMRLogDir       string `yaml:"dmr_log_dir"`
	EchoLinkCommand string `yaml:"echolink_command"`
}

func Defaults(getenv func(string) string) Config {
	get := func(key, fallback string) string {
		if value := getenv(key); value != "" {
			return value
		}
		return fallback
	}
	return Config{
		ListenAddr:      get("LISTEN_ADDR", "127.0.0.1:8087"),
		BasePath:        get("BASE_PATH", "/ameriboard"),
		Title:           get("DASHBOARD_TITLE", "CONFERENCIA AMERINET"),
		Subtitle:        get("DASHBOARD_SUBTITLE", "Reflector YSF {ysf_reflector} y TGIF DMR TG {tgif_tg}"),
		Badge:           get("DASHBOARD_BADGE", "Reflector + TGIF DMR"),
		YSFReflector:    get("YSF_REFLECTOR_ID", "74001"),
		TGIFTalkgroup:   get("TGIF_TG", "7401"),
		BridgeCall:      get("BRIDGE_CALLSIGN", "HC1LNM"),
		YSFLogDir:       get("YSF_LOG_DIR", "/var/log/YSFReflector"),
		DMRLogDir:       get("DMR_LOG_DIR", "/var/log/mmdvm"),
		EchoLinkCommand: get("ECHOLINK_COMMAND", "tlbcmd"),
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults(os.Getenv)
	if path != "" {
		file, err := os.Open(path)
		if err != nil {
			return cfg, fmt.Errorf("abrir configuración YAML: %w", err)
		}
		defer file.Close()
		decoder := yaml.NewDecoder(file)
		decoder.KnownFields(true)
		if err := decoder.Decode(&cfg); err != nil {
			return cfg, fmt.Errorf("leer configuración YAML: %w", err)
		}
	}
	if err := Validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func Validate(cfg Config) error {
	_, port, err := net.SplitHostPort(cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen_addr inválida: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("puerto inválido en listen_addr: %q", port)
	}
	return nil
}

// Initial creates a configuration without replacing an existing file.
func Initial(path, legacyEnv string) (Config, error) {
	values := map[string]string{}
	if legacyEnv != "" {
		file, err := os.Open(legacyEnv)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("abrir configuración anterior: %w", err)
		}
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				key, value, ok := strings.Cut(line, "=")
				if !ok {
					return Config{}, fmt.Errorf("línea inválida en %s", legacyEnv)
				}
				value = strings.TrimSpace(value)
				if strings.HasPrefix(value, `"`) {
					value, err = strconv.Unquote(value)
					if err != nil {
						return Config{}, fmt.Errorf("valor inválido de %s: %w", key, err)
					}
				} else if strings.HasPrefix(value, "'") {
					if len(value) < 2 || !strings.HasSuffix(value, "'") {
						return Config{}, fmt.Errorf("valor inválido de %s", key)
					}
					value = value[1 : len(value)-1]
				}
				values[strings.TrimSpace(key)] = value
			}
			if err := scanner.Err(); err != nil {
				return Config{}, fmt.Errorf("leer configuración anterior: %w", err)
			}
		}
	}
	cfg := Defaults(func(key string) string { return values[key] })
	if err := Validate(cfg); err != nil {
		return cfg, err
	}
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	fallback := false
	if errors.Is(err, syscall.EADDRINUSE) {
		host, _, _ := net.SplitHostPort(cfg.ListenAddr)
		listener, err = net.Listen("tcp", net.JoinHostPort(host, "0"))
		fallback = true
	}
	if err != nil {
		return cfg, fmt.Errorf("seleccionar puerto: %w", err)
	}
	if fallback {
		host, _, _ := net.SplitHostPort(cfg.ListenAddr)
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			listener.Close()
			return cfg, fmt.Errorf("determinar puerto disponible: %w", err)
		}
		cfg.ListenAddr = net.JoinHostPort(host, port)
	}
	if err := listener.Close(); err != nil {
		return cfg, err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return cfg, fmt.Errorf("generar configuración YAML: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".ameriboard-config-*")
	if err != nil {
		return cfg, fmt.Errorf("crear configuración YAML: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return cfg, fmt.Errorf("guardar configuración YAML: %w", err)
	}
	if err := file.Close(); err != nil {
		return cfg, fmt.Errorf("cerrar configuración YAML: %w", err)
	}
	if err := os.Link(file.Name(), path); err != nil {
		return cfg, fmt.Errorf("instalar configuración YAML: %w", err)
	}
	return cfg, nil
}
