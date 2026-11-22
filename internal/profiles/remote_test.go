package profiles

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

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
