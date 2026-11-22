package profiles

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProfileVisibilityAndRemoteAuthentication(t *testing.T) {
	store := NewStore(t.TempDir())
	password, err := store.GenerateCredential("admin")
	if err != nil {
		t.Fatal(err)
	}
	siteToken, err := store.GenerateCredential("site")
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(store)
	h.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://remote.example/ameriboard/api/remote/status" ||
			r.Header.Get("Authorization") != "Bearer "+siteToken {
			t.Errorf("unexpected remote request: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"header":{},"stats":{}}`)), Header: make(http.Header)}, nil
	})
	e := echo.New()
	h.Register(e.Group(""), "", func(c echo.Context) error { return c.JSON(200, map[string]string{"status": "ok"}) })

	request := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "https://local.example"+path, strings.NewReader(body))
		req.Header.Set("Origin", "https://local.example")
		req.Header.Set("X-Ameriboard-Request", "1")
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	if got := request("GET", "/api/admin/profiles", "", nil); got.Code != 401 {
		t.Fatalf("admin list without session: %d", got.Code)
	}
	login := request("POST", "/api/admin/login", `{"password":"`+password+`"}`, nil)
	if login.Code != 204 || len(login.Result().Cookies()) != 1 {
		t.Fatalf("login: %d %s", login.Code, login.Body)
	}
	cookie := login.Result().Cookies()[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("insecure session cookie")
	}
	body := `{"name":"Privado","url":"https://remote.example/ameriboard","token":"` + siteToken + `","visible":false}`
	created := request("POST", "/api/admin/profiles", body, cookie)
	if created.Code != 200 || strings.Contains(created.Body.String(), siteToken) {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	var item publicProfile
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	public := request("GET", "/api/profiles", "", nil)
	if strings.Contains(public.Body.String(), item.ID) || request("GET", "/api/profiles/"+item.ID+"/status", "", nil).Code != 404 {
		t.Fatal("hidden profile is accessible to visitors")
	}
	if got := request("GET", "/api/profiles/"+item.ID+"/status", "", cookie); got.Code != 200 {
		t.Fatalf("administrator cannot view hidden profile: %d", got.Code)
	}
	body = `{"name":"Público","url":"https://remote.example/ameriboard","token":"","visible":true}`
	if got := request("PUT", "/api/admin/profiles/"+item.ID, body, cookie); got.Code != 200 {
		t.Fatalf("update: %d %s", got.Code, got.Body)
	}
	if got := request("GET", "/api/profiles/"+item.ID+"/status", "", nil); got.Code != 200 {
		t.Fatalf("visitor cannot view public profile: %d", got.Code)
	}
	if got := request("GET", "/api/profiles", "", nil); strings.Contains(got.Body.String(), siteToken) || strings.Contains(got.Body.String(), "remote.example") {
		t.Fatalf("public profile exposes private configuration: %s", got.Body)
	}
	remotePassword := "remote administrator password"
	created = request("POST", "/api/admin/profiles", `{"name":"Nuevo","url":"https://remote.example/ameriboard","password":"`+remotePassword+`","visible":true}`, cookie)
	if created.Code != 200 || strings.Contains(created.Body.String(), remotePassword) {
		t.Fatalf("create password profile: %d %s", created.Code, created.Body)
	}
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	updated := request("PUT", "/api/admin/profiles/"+item.ID, `{"name":"Editado","url":"https://remote.example/ameriboard","password":"","visible":true}`, cookie)
	if updated.Code != 200 || strings.Contains(updated.Body.String(), remotePassword) {
		t.Fatalf("update password profile: %d %s", updated.Code, updated.Body)
	}
	profiles, err := NewStore(store.dir).list()
	if err != nil || len(profiles) != 2 || profiles[1].Password != remotePassword {
		t.Fatalf("password was not preserved: %v", err)
	}
	if got := request("GET", "/api/admin/profiles", "", cookie); strings.Contains(got.Body.String(), remotePassword) || strings.Contains(got.Body.String(), siteToken) {
		t.Fatalf("admin profile exposes credentials: %s", got.Body)
	}
	req := httptest.NewRequest("GET", "https://local.example/api/remote/status", nil)
	req.Header.Set("Authorization", "Bearer "+siteToken)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("valid site token: %d", rec.Code)
	}
	req = httptest.NewRequest("GET", "https://local.example/api/remote/status", nil)
	req.Header.Set("Authorization", "Bearer incorrect")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("invalid site token: %d", rec.Code)
	}
	if _, err := store.GenerateCredential("admin"); err != nil {
		t.Fatal(err)
	}
	if got := request("GET", "/api/admin/profiles", "", cookie); got.Code != 401 {
		t.Fatal("administrator session survived password rotation")
	}
}

func TestRejectInvalidProfileAndCrossOriginLogin(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.save(Profile{Name: "Bad", URL: "ftp://example.org", Token: strings.Repeat("x", 32)}, ""); err == nil {
		t.Fatal("accepted invalid profile URL")
	}
	h := NewHandler(store)
	e := echo.New()
	h.Register(e.Group(""), "", func(c echo.Context) error { return c.NoContent(200) })
	req := httptest.NewRequest("POST", "https://local.example/api/admin/login", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://attacker.example")
	req.Header.Set("X-Ameriboard-Request", "1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("cross-origin login: %d", rec.Code)
	}
}
