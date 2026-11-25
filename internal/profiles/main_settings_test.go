package profiles

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ameriboard/internal/monitoring"

	"github.com/labstack/echo/v4"
)

func TestMainMonitoringSettingsPersist(t *testing.T) {
	store := NewStore(t.TempDir())
	selection := monitoring.Options{YSF: true, NXDN: false, EchoLink: false, DVSwitch: true}
	saved, err := store.SaveMainSettings(MainSettings{Title: "Local", Monitoring: &selection})
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewStore(store.dir).LoadMainSettings(MainSettings{})
	if err != nil || reloaded.Monitoring == nil || *reloaded.Monitoring != selection || *saved.Monitoring != selection {
		t.Fatalf("monitoring settings not preserved: %+v, %v", reloaded, err)
	}
}

func TestMainSettingsRequireAdminAndPersistChanges(t *testing.T) {
	store := NewStore(t.TempDir())
	password, err := store.GenerateCredential("admin")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store)
	initial := MainSettings{
		Title: "Título inicial", Subtitle: "Subtítulo inicial", DVSwitchTitle: DefaultDVSwitchTitle,
	}
	var applied MainSettings
	handler.SetMainSettings(initial, func(settings MainSettings) { applied = settings })
	e := echo.New()
	handler.Register(e.Group(""), "", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	request := func(method, path, body string, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "https://local.example"+path, strings.NewReader(body))
		req.Header.Set("Origin", origin)
		req.Header.Set("X-Ameriboard-Request", "1")
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	if got := request("GET", "/api/admin/settings", "", nil, "https://local.example"); got.Code != http.StatusUnauthorized {
		t.Fatalf("settings without session: %d", got.Code)
	}
	login := request("POST", "/api/admin/login", `{"password":"`+password+`"}`, nil, "https://local.example")
	if login.Code != http.StatusNoContent || len(login.Result().Cookies()) != 1 {
		t.Fatalf("login: %d %s", login.Code, login.Body)
	}
	cookie := login.Result().Cookies()[0]
	if got := request("GET", "/api/admin/settings", "", cookie, "https://local.example"); got.Code != http.StatusOK ||
		!strings.Contains(got.Body.String(), `"title":"Título inicial"`) {
		t.Fatalf("get settings: %d %s", got.Code, got.Body)
	}
	if got := request("PUT", "/api/admin/settings", `{"title":" ","subtitle":"x"}`, cookie, "https://local.example"); got.Code != http.StatusBadRequest {
		t.Fatalf("blank title: %d %s", got.Code, got.Body)
	}
	if got := request("PUT", "/api/admin/settings", `{"title":" Título nuevo ","subtitle":" Subtítulo nuevo ","dvSwitchTitle":" Actividad nueva ","nxdnTitle":" NXDN nuevo ","echolinkTitle":" EchoLink nuevo "}`, cookie, "https://local.example"); got.Code != http.StatusOK ||
		!strings.Contains(got.Body.String(), `"title":"Título nuevo"`) ||
		!strings.Contains(got.Body.String(), `"dvSwitchTitle":"Actividad nueva"`) ||
		!strings.Contains(got.Body.String(), `"nxdnTitle":"NXDN nuevo"`) ||
		!strings.Contains(got.Body.String(), `"echolinkTitle":"EchoLink nuevo"`) {
		t.Fatalf("update settings: %d %s", got.Code, got.Body)
	}
	want := MainSettings{Title: "Título nuevo", Subtitle: "Subtítulo nuevo", DVSwitchTitle: "Actividad nueva", NXDNTitle: "NXDN nuevo", EchoLinkTitle: "EchoLink nuevo"}
	if applied != want {
		t.Fatalf("applied settings = %+v, want %+v", applied, want)
	}
	reloaded, err := NewStore(store.dir).LoadMainSettings(initial)
	if err != nil || reloaded != want {
		t.Fatalf("persisted settings = %+v, %v", reloaded, err)
	}
	if got := request("PUT", "/api/admin/settings", `{"title":"Local","monitoring":{"ysf":true,"nxdn":false,"echolink":false,"dvSwitch":true}}`, cookie, "https://local.example"); got.Code != http.StatusOK {
		t.Fatalf("update monitoring: %d %s", got.Code, got.Body)
	}
	selection := monitoring.Options{YSF: true, DVSwitch: true}
	if applied.Monitoring == nil || *applied.Monitoring != selection {
		t.Fatalf("monitoring not applied: %+v", applied)
	}
	if got := request("PUT", "/api/admin/settings", `{"title":"Local"}`, cookie, "https://local.example"); got.Code != http.StatusOK {
		t.Fatalf("legacy settings update: %d %s", got.Code, got.Body)
	}
	reloaded, err = NewStore(store.dir).LoadMainSettings(initial)
	if err != nil || reloaded.Monitoring == nil || *reloaded.Monitoring != selection {
		t.Fatalf("monitoring not preserved by legacy update: %+v, %v", reloaded, err)
	}
	if got := request("PUT", "/api/admin/settings", `{"title":"No autorizado","subtitle":""}`, cookie, "https://attacker.example"); got.Code != http.StatusUnauthorized {
		t.Fatalf("cross-origin update: %d", got.Code)
	}
}

func TestMainSettingsLimitsLengths(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.SaveMainSettings(MainSettings{Title: strings.Repeat("a", 201)}); err == nil {
		t.Fatal("accepted oversized title")
	}

	if _, err := store.SaveMainSettings(MainSettings{Title: "Título", Subtitle: strings.Repeat("a", 501)}); err == nil {
		t.Fatal("accepted oversized subtitle")
	}
	if _, err := store.SaveMainSettings(MainSettings{Title: "Título", DVSwitchTitle: strings.Repeat("a", 201)}); err == nil {
		t.Fatal("accepted oversized DVSwitch title")
	}
	if _, err := store.SaveMainSettings(MainSettings{Title: "Título", NXDNTitle: strings.Repeat("a", 201)}); err == nil {
		t.Fatal("accepted oversized NXDN title")
	}
	if _, err := store.SaveMainSettings(MainSettings{Title: "Título", EchoLinkTitle: strings.Repeat("a", 201)}); err == nil {
		t.Fatal("accepted oversized EchoLink title")
	}
	settings, err := store.SaveMainSettings(MainSettings{Title: "Título"})
	if err != nil || settings.NXDNTitle != DefaultNXDNTitle {
		t.Fatalf("missing NXDN title = %q, %v", settings.NXDNTitle, err)
	}
}

func TestLoadMainSettingsReplacesPreviousNXDNDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dashboard-settings.json")
	if err := os.WriteFile(path, []byte(`{"title":"Local","nxdnTitle":"Últimos escuchados NXDN"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := NewStore(dir).LoadMainSettings(MainSettings{})
	if err != nil || settings.NXDNTitle != DefaultNXDNTitle {
		t.Fatalf("previous NXDN default: %+v, %v", settings, err)
	}
}
