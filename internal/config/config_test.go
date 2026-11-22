package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestInitialChoosesFreePortAndMigratesSettings(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	dir := t.TempDir()
	env := filepath.Join(dir, "ameriboard.env")
	if err := os.WriteFile(env, []byte(fmt.Sprintf("LISTEN_ADDR=%s\nDASHBOARD_TITLE=\"Test title\"\nBASE_PATH=/panel\n", listener.Addr())), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ameriboard.yaml")
	cfg, err := Initial(path, env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr == listener.Addr().String() || cfg.Title != "Test title" || cfg.BasePath != "/panel" {
		t.Fatalf("unexpected migrated config: %+v", cfg)
	}
	loaded, err := Load(path)
	if err != nil || loaded != cfg {
		t.Fatalf("Load: %+v, %v", loaded, err)
	}
	if _, err := Initial(path, env); err == nil {
		t.Fatal("expected existing configuration to be preserved")
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	for _, contents := range []string{"listen_addr: \"127.0.0.1:0\"\n", "listen_adrr: \"127.0.0.1:8087\"\n"} {
		path := filepath.Join(t.TempDir(), "ameriboard.yaml")
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("expected error for %q", contents)
		}
	}
}
