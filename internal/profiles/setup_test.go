package profiles

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestFirstAdministratorSetup(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	h := NewHandler(store)
	e := echo.New()
	h.Register(e.Group(""), "", func(c echo.Context) error { return c.NoContent(200) })
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "https://local.example"+path, strings.NewReader(body))
		req.Header.Set("Origin", "https://local.example")
		req.Header.Set("X-Ameriboard-Request", "1")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	if got := request("GET", "/api/admin/setup", ""); !strings.Contains(got.Body.String(), `"required":true`) {
		t.Fatalf("setup status: %s", got.Body)
	}
	password := "12345678"
	body, _ := json.Marshal(map[string]string{"password": password})
	got := request("POST", "/api/admin/setup", string(body))
	if got.Code != 204 {
		t.Fatalf("setup: %d %s", got.Code, got.Body)
	}
	cookies := got.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].MaxAge != 0 {
		t.Fatalf("setup must create a secure session: %+v", cookies)
	}
	info, err := os.Stat(filepath.Join(dir, "admin.hash"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("administrator hash permissions: %v %v", info, err)
	}
	if got := request("POST", "/api/admin/setup", string(body)); got.Code != 409 {
		t.Fatalf("setup repeated: %d", got.Code)
	}
	if got := request("GET", "/api/admin/setup", ""); !strings.Contains(got.Body.String(), `"required":false`) {
		t.Fatalf("setup status after creation: %s", got.Body)
	}
	login := request("POST", "/api/admin/login", `{"password":"`+password+`"}`)
	cookies = login.Result().Cookies()
	if login.Code != 204 || len(cookies) != 1 || cookies[0].MaxAge != 0 {
		t.Fatalf("login must use a session cookie: %d %+v", login.Code, cookies)
	}
}

func TestSetupRequiresStrongPassword(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.setupAdmin("1234567"); err != errInvalidPassword {
		t.Fatalf("short password: %v", err)
	}
}
