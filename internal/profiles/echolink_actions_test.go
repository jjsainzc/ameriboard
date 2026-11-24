package profiles

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

func TestEchoLinkActionLoggingAndAuthorization(t *testing.T) {
	store := NewStore(t.TempDir())
	password, err := store.GenerateCredential("admin")
	if err != nil {
		t.Fatal(err)
	}
	token, err := store.GenerateCredential("site")
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(store)
	h.SetEchoLinkActionExecutor(func(_ context.Context, action EchoLinkAction) (string, error) {
		if action.Action == "mute" {
			return "", errors.New("exit status 7")
		}
		return "0", nil
	})
	e := echo.New()
	e.Logger.SetLevel(log.INFO)
	var logs bytes.Buffer
	e.Logger.SetOutput(&logs)
	h.Register(e.Group(""), "", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	request := func(body string, cookie *http.Cookie, authorization string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "https://local.example/api/admin/echolink/action", strings.NewReader(body))
		req.Header.Set("Origin", "https://local.example")
		req.Header.Set("X-Ameriboard-Request", "1")
		req.Header.Set("Authorization", authorization)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	if got := request(`{"action":"mute","callsign":"TEST1"}`, nil, "Bearer "+token); got.Code != http.StatusUnauthorized {
		t.Fatalf("read-only token allowed an action: %d", got.Code)
	}
	login := httptest.NewRequest(http.MethodPost, "https://local.example/api/admin/login", strings.NewReader(`{"password":"`+password+`"}`))
	login.Header.Set("Origin", "https://local.example")
	login.Header.Set("X-Ameriboard-Request", "1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, login)
	if rec.Code != http.StatusNoContent || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("login: %d", rec.Code)
	}
	cookie := rec.Result().Cookies()[0]
	if got := request(`{"action":"mute","callsign":"TEST1"}`, cookie, ""); got.Code != http.StatusBadRequest {
		t.Fatalf("failed action: %d", got.Code)
	}
	if got := request(`{"action":"message","message":"texto privado"}`, cookie, ""); got.Code != http.StatusOK {
		t.Fatalf("message action: %d", got.Code)
	}
	text := logs.String()
	for _, expected := range []string{"EchoLink local", "mute", "exit status 7", "message", "finalizado"} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in logs %s", expected, text)
		}
	}
	for _, secret := range []string{password, token, "texto privado"} {
		if strings.Contains(text, secret) {
			t.Fatal("secret found in logs")
		}
	}
}

func TestRemoteEchoLinkFailureIsLogged(t *testing.T) {
	store := NewStore(t.TempDir())
	password, err := store.GenerateCredential("admin")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := store.save(Profile{Name: "Remoto", URL: "https://remote.example", Password: "remote-password"}, "")
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(store)
	h.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	e := echo.New()
	e.Logger.SetLevel(log.INFO)
	var logs bytes.Buffer
	e.Logger.SetOutput(&logs)
	h.Register(e.Group(""), "", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	login := httptest.NewRequest(http.MethodPost, "https://local.example/api/admin/login", strings.NewReader(`{"password":"`+password+`"}`))
	login.Header.Set("Origin", "https://local.example")
	login.Header.Set("X-Ameriboard-Request", "1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, login)
	if rec.Code != http.StatusNoContent || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("login: %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "https://local.example/api/profiles/"+profile.ID+"/echolink/action", strings.NewReader(`{"action":"connect","callsign":"TEST1"}`))
	req.Header.Set("Origin", "https://local.example")
	req.Header.Set("X-Ameriboard-Request", "1")
	req.AddCookie(rec.Result().Cookies()[0])
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway || !strings.Contains(logs.String(), "HTTP=401") ||
		!strings.Contains(logs.String(), profile.ID) || strings.Contains(logs.String(), "remote-password") {
		t.Fatalf("status=%d logs=%q", rec.Code, logs.String())
	}
}
