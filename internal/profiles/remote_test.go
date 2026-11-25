package profiles

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ameriboard/internal/controllers"
	"ameriboard/internal/monitoring"

	"github.com/labstack/echo/v4"
)

func TestRemoteMonitoringSelectionReachesStatusCollector(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().Format("2006-01-02")
	for _, name := range []string{"YSFReflector-" + today + ".log", "MMDVM_Bridge-" + today + ".log"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(dir, "echo-called")
	command := filepath.Join(dir, "tlbcmd")
	if err := os.WriteFile(command, []byte("#!/bin/sh\ntouch \""+marker+"\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	remoteStore := NewStore(t.TempDir())
	if err := remoteStore.setupAdmin("12345678"); err != nil {
		t.Fatal(err)
	}
	collector := controllers.NewReflectorService(controllers.ReflectorConfig{
		YSFLogDir: dir, DMRLogDir: dir, EchoLinkCommand: command, BridgeConfigDir: dir,
	})
	remote := echo.New()
	NewHandler(remoteStore).Register(remote.Group("/ameriboard"), "/ameriboard", collector.GetStatus)
	localStore := NewStore(t.TempDir())
	profile, err := localStore.save(Profile{
		Name: "Sitio", URL: "https://remote.example", Password: "12345678", Visible: true,
		Monitoring: &monitoring.Options{DVSwitch: true},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(localStore)
	handler.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		remote.ServeHTTP(recorder, request)
		return recorder.Result(), nil
	})
	local := echo.New()
	handler.Register(local.Group(""), "", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	rec := httptest.NewRecorder()
	local.ServeHTTP(rec, httptest.NewRequest("GET", "https://local.example/api/profiles/"+profile.ID+"/status", nil))
	var status controllers.DashboardStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || status.YSFAvailable || status.YSFTrafficAvailable ||
		status.EchoLinkAvailable || !status.DMRAvailable {
		t.Fatalf("remote selection not enforced: %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("remote EchoLink was queried: %v", err)
	}
}

func TestRemoteMonitoringSelectionAndLegacyServer(t *testing.T) {
	store := NewStore(t.TempDir())
	selection := monitoring.Options{YSF: true, NXDN: false, EchoLink: false, DVSwitch: true}
	profile, err := store.save(Profile{Name: "Sitio", URL: "https://remote.example", Password: "12345678", Visible: true, Monitoring: &selection}, "")
	if err != nil {
		t.Fatal(err)
	}
	profile, err = NewStore(store.dir).save(Profile{Name: "Sitio", URL: profile.URL, Visible: true}, profile.ID)
	if err != nil || profile.Monitoring == nil || *profile.Monitoring != selection {
		t.Fatalf("selection was not persisted or preserved: %+v, %v", profile, err)
	}
	h := NewHandler(store)
	capable := true
	statusCalls := 0
	h.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/capabilities") {
			body := `{"selectiveMonitoring":true}`
			if !capable {
				body = `{}`
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		}
		statusCalls++
		var got monitoring.Options
		if err := json.Unmarshal([]byte(r.Header.Get("X-Ameriboard-Monitoring")), &got); err != nil || got != selection {
			t.Errorf("remote selection = %+v, %v", got, err)
		}
		headers := make(http.Header)
		headers.Set("X-Ameriboard-Selective-Monitoring", "1")
		return &http.Response{StatusCode: http.StatusOK, Header: headers, Body: io.NopCloser(strings.NewReader(`{"header":{},"stats":{}}`))}, nil
	})
	e := echo.New()
	h.Register(e.Group(""), "", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "https://local.example/api/profiles/"+profile.ID+"/status", nil))
	if rec.Code != http.StatusOK || statusCalls != 1 {
		t.Fatalf("selected remote status: %d %s, calls %d", rec.Code, rec.Body, statusCalls)
	}
	capable = false
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "https://local.example/api/profiles/"+profile.ID+"/status", nil))
	if rec.Code != http.StatusBadGateway || statusCalls != 1 || !strings.Contains(rec.Body.String(), "actualizarse") {
		t.Fatalf("legacy server should not be queried: %d %s, calls %d", rec.Code, rec.Body, statusCalls)
	}
}

func TestPublicIP(t *testing.T) {
	for address, want := range map[string]bool{
		"127.0.0.1": false, "10.0.0.1": false, "169.254.169.254": false,
		"100.64.1.1": false, "0.0.0.1": false, "::1": false,
		"::ffff:127.0.0.1": false, "2001:db8::1": false,
		"8.8.8.8": true, "2606:4700:4700::1111": true,
	} {
		if got := publicIP(netip.MustParseAddr(address)); got != want {
			t.Errorf("publicIP(%s) = %v, want %v", address, got, want)
		}
	}
}

func TestRemoteHTTPAndAdminSession(t *testing.T) {
	remoteStore := NewStore(t.TempDir())
	remote := echo.New()
	NewHandler(remoteStore).Register(remote.Group("/ameriboard"), "/ameriboard", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"title": "remote"})
	})
	setup := httptest.NewRequest(http.MethodPost, "http://remote.example/ameriboard/api/admin/setup", strings.NewReader(`{"password":"12345678"}`))
	setup.Header.Set("Origin", "http://remote.example")
	setup.Header.Set("X-Ameriboard-Request", "1")
	rec := httptest.NewRecorder()
	remote.ServeHTTP(rec, setup)
	if rec.Code != http.StatusNoContent || len(rec.Result().Cookies()) != 1 || rec.Result().Cookies()[0].Secure {
		t.Fatalf("HTTP setup: %d, cookies: %+v", rec.Code, rec.Result().Cookies())
	}

	localStore := NewStore(t.TempDir())
	profile, err := localStore.save(Profile{Name: "HTTP", URL: "http://remote.example", Password: "12345678", Visible: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	if profile.URL != "http://remote.example/ameriboard" {
		t.Fatalf("unexpected profile URL: %s", profile.URL)
	}
	handler := NewHandler(localStore)
	handler.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		remote.ServeHTTP(recorder, request)
		return recorder.Result(), nil
	})
	local := echo.New()
	handler.Register(local.Group(""), "", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	rec = httptest.NewRecorder()
	local.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://local.example/api/profiles/"+profile.ID+"/status", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "remote") {
		t.Fatalf("HTTP profile: %d %s", rec.Code, rec.Body.String())
	}
}

func TestProfileDefaultPath(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"http://example.org/", "http://example.org/ameriboard"},
		{"https://example.org:8080", "https://example.org:8080/ameriboard"},
		{"https://example.org/ameriboard/", "https://example.org/ameriboard"},
		{"https://example.org/otro", "https://example.org/otro"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			store := NewStore(t.TempDir())
			p, err := store.save(Profile{Name: "Sitio", URL: tc.input, Password: "12345678"}, "")
			if err != nil || p.URL != tc.want {
				t.Fatalf("URL = %q, error = %v; want %q", p.URL, err, tc.want)
			}
		})
	}
}

func TestRemoteHTTPSFailureDoesNotAffectOtherSites(t *testing.T) {
	remoteStore := NewStore(t.TempDir())
	password := "12345678"
	if err := remoteStore.setupAdmin(password); err != nil {
		t.Fatal(err)
	}
	remoteEcho := echo.New()
	NewHandler(remoteStore).Register(remoteEcho.Group("/ameriboard"), "/ameriboard", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"title": "remote"})
	})
	dir := t.TempDir()
	localStore := NewStore(dir)
	good, err := localStore.save(Profile{Name: "good", URL: "https://remote.example/ameriboard", Password: password, Visible: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := localStore.save(Profile{Name: "good", URL: good.URL, Visible: true}, good.ID); err != nil {
		t.Fatal(err)
	}
	localStore = NewStore(dir)
	bad, err := localStore.save(Profile{Name: "bad", URL: "https://remote.example/ameriboard", Password: "incorrect password", Visible: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(localStore)
	h.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		request.TLS = &tls.ConnectionState{}
		recorder := httptest.NewRecorder()
		remoteEcho.ServeHTTP(recorder, request)
		return recorder.Result(), nil
	})
	localEcho := echo.New()
	h.Register(localEcho.Group(""), "", func(c echo.Context) error { return c.JSON(200, map[string]string{"title": "local"}) })
	for i := 0; i < 6; i++ {
		rec := httptest.NewRecorder()
		localEcho.ServeHTTP(rec, httptest.NewRequest("GET", "https://local.example/api/profiles/"+bad.ID+"/status", nil))
		if rec.Code != 502 {
			t.Fatalf("bad password: %d %s", rec.Code, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	localEcho.ServeHTTP(rec, httptest.NewRequest("GET", "https://local.example/api/profiles/"+good.ID+"/status", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "remote") {
		t.Fatalf("good profile: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	localEcho.ServeHTTP(rec, httptest.NewRequest("GET", "https://local.example/api/profiles", nil))
	if strings.Contains(rec.Body.String(), password) || strings.Contains(rec.Body.String(), "incorrect password") {
		t.Fatal("public profile exposed a password")
	}
}
