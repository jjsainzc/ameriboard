package profiles

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

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
	if got := request("PUT", "/api/admin/settings", `{"title":" Título nuevo ","subtitle":" Subtítulo nuevo ","dvSwitchTitle":" Actividad nueva ","nxdnTitle":" NXDN nuevo "}`, cookie, "https://local.example"); got.Code != http.StatusOK ||
		!strings.Contains(got.Body.String(), `"title":"Título nuevo"`) ||
		!strings.Contains(got.Body.String(), `"dvSwitchTitle":"Actividad nueva"`) ||
		!strings.Contains(got.Body.String(), `"nxdnTitle":"NXDN nuevo"`) {
		t.Fatalf("update settings: %d %s", got.Code, got.Body)
	}
	want := MainSettings{Title: "Título nuevo", Subtitle: "Subtítulo nuevo", DVSwitchTitle: "Actividad nueva", NXDNTitle: "NXDN nuevo"}
	if applied != want {
		t.Fatalf("applied settings = %+v, want %+v", applied, want)
	}
	reloaded, err := NewStore(store.dir).LoadMainSettings(initial)
	if err != nil || reloaded != want {
		t.Fatalf("persisted settings = %+v, %v", reloaded, err)
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
	settings, err := store.SaveMainSettings(MainSettings{Title: "Título"})
	if err != nil || settings.NXDNTitle != DefaultNXDNTitle {
		t.Fatalf("missing NXDN title = %q, %v", settings.NXDNTitle, err)
	}
}
