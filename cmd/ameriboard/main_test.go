package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ameriboard/internal/controllers"
	"ameriboard/internal/profiles"
)

func TestNormalizeBasePath(t *testing.T) {
	for input, want := range map[string]string{
		"/": "", "ameriboard/": "/ameriboard", "/one/two/": "/one/two",
	} {
		got, err := normalizeBasePath(input)
		if err != nil || got != want {
			t.Errorf("normalizeBasePath(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"/a//b", "/../admin", `/x"><script>`} {
		if _, err := normalizeBasePath(input); err == nil {
			t.Errorf("normalizeBasePath(%q) should fail", input)
		}
	}
}

func TestNotifyReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.sock")
	listener, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("NOTIFY_SOCKET", path)
	if err := notifyReady(); err != nil {
		t.Fatal(err)
	}
	if err := listener.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 32)
	n, _, err := listener.ReadFromUnix(buffer)
	if err != nil || string(buffer[:n]) != "READY=1" {
		t.Fatalf("notification %q, error %v", buffer[:n], err)
	}
}

func TestServerRoutes(t *testing.T) {
	dir := t.TempDir()
	for _, base := range []string{"", "/ameriboard", "/one/two"} {
		frontend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("frontend:" + r.URL.Path))
		})
		reflector := controllers.NewReflectorService(controllers.ReflectorConfig{Title: "Test", YSFLogDir: dir, DMRLogDir: dir})
		server := newServer(base, frontend, reflector, profiles.NewHandler(profiles.NewStore(t.TempDir())))
		for _, tc := range []struct {
			path string
			code int
			body string
		}{
			{"/healthz", http.StatusOK, `"status":"ok"`},
			{base + "/api/version", http.StatusOK, fmt.Sprintf(`"version":%q`, version)},
			{base + "/api/reflector-status", http.StatusOK, `"title":"Test"`},
			{base + "/api/missing", http.StatusNotFound, ""},
			{base + "/nested/route", http.StatusOK, "frontend:/nested/route"},
			{base + "/", http.StatusOK, "frontend:/"},
		} {
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if response.Code != tc.code || !strings.Contains(response.Body.String(), tc.body) {
				t.Errorf("base %q path %q: status %d, body %s", base, tc.path, response.Code, response.Body.String())
			}

		}
		if base != "" {
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, base, nil))
			if response.Code != http.StatusMovedPermanently || response.Header().Get("Location") != base+"/" {
				t.Errorf("base %q: redirect status %d to %q", base, response.Code, response.Header().Get("Location"))
			}
		}
	}
}

func TestReflectorRouteReportsMissingLogDirectory(t *testing.T) {
	dmrDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dmrDir, "MMDVM_Bridge-"+time.Now().Format("2006-01-02")+".log"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	service := controllers.NewReflectorService(controllers.ReflectorConfig{
		YSFLogDir: filepath.Join(t.TempDir(), "missing"), DMRLogDir: dmrDir, EchoLinkCommand: "/nonexistent/tlbcmd",
	})
	server := newServer("/ameriboard", http.NotFoundHandler(), service, profiles.NewHandler(profiles.NewStore(t.TempDir())))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ameriboard/api/reflector-status", nil))
	var data controllers.DashboardStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || data.YSFAvailable || !data.DMRAvailable || len(data.Warnings) != 0 {
		t.Fatalf("expected partial response, got %d: %s", response.Code, response.Body.String())
	}
	if data.EchoLinkAvailable {
		t.Fatal("EchoLink should be unavailable")
	}
}

func TestReflectorRouteMissingSourcesAreSilent(t *testing.T) {
	dir := t.TempDir()
	service := controllers.NewReflectorService(controllers.ReflectorConfig{
		YSFLogDir: dir, DMRLogDir: dir, EchoLinkCommand: "/nonexistent/tlbcmd",
	})
	server := newServer("/ameriboard", http.NotFoundHandler(), service, profiles.NewHandler(profiles.NewStore(t.TempDir())))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ameriboard/api/reflector-status", nil))
	var data controllers.DashboardStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || data.YSFAvailable || data.YSFTrafficAvailable ||
		data.DMRAvailable || data.EchoLinkAvailable || len(data.Warnings) != 0 {
		t.Fatalf("expected unavailable sources without warnings, got %d: %s", response.Code, response.Body.String())
	}
}

func TestReflectorRouteReadsLogs(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	name := filepath.Join(dir, "YSFReflector-"+now.Format("2006-01-02")+".log")
	stamp := now.Format("2006-01-02 15:04:05")
	log := fmt.Sprintf("M: %s Currently linked repeaters/gateways:\nM: %s TEST1 : 192.0.2.1:1234\nM: %s TEST1 : 192.0.2.1:1234\nM: %s TEST1 : 127.0.0.1:5678\n",
		stamp, stamp, stamp, stamp)
	if err := os.WriteFile(name, []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	service := controllers.NewReflectorService(controllers.ReflectorConfig{
		Title: "Live", Subtitle: "YSF {ysf_reflector}, TG {tgif_tg}", Badge: "Test",
		YSFReflector: "42", TGIFTalkgroup: "77", YSFLogDir: dir, DMRLogDir: dir,
	})
	server := newServer("/ameriboard", http.NotFoundHandler(), service, profiles.NewHandler(profiles.NewStore(t.TempDir())))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ameriboard/api/reflector-status", nil))

	var data controllers.DashboardStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || data.Header.Title != "Live" || data.Header.Badge != "Test" ||
		!strings.HasPrefix(data.Header.Subtitle, "YSF 42, TG 77") || data.Stats.YSFConnectedNow != 2 ||
		len(data.YSFConnectedList) != 2 || data.YSFConnectedList[0].Callsign != "TEST1" ||
		data.YSFConnectedList[0].IP != "192.0.2.1" || data.YSFConnectedList[1].IP != "127.0.0.1" ||
		data.YSFConnectedList[1].Type != "Puente" || data.Header.LocalBridgeStatus != "—" {
		t.Fatalf("unexpected status response: %d %s", response.Code, response.Body.String())
	}
}
