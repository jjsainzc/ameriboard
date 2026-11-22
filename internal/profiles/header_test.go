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

func TestProfileHeaderOverrides(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	profile, err := store.save(Profile{
		Name: "Sitio", URL: "https://example.org/ameriboard", Token: strings.Repeat("x", 64),
		Visible: true, Header: &ProfileHeader{Title: "Título propio", Badge: "Local"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := NewStore(dir).list()
	if err != nil || len(stored) != 1 || stored[0].Header == nil || stored[0].Header.Title != "Título propio" {
		t.Fatalf("header not persisted: %+v, %v", stored, err)
	}
	h := NewHandler(store)
	h.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(
			`{"header":{"title":"Original","subtitle":"Remoto","badge":"Antigua","localBridgeStatus":"En línea","lastUpdate":"12:34"},"stats":{"ysfTx24h":42}}`))}, nil
	})
	e := echo.New()
	h.Register(e.Group(""), "", func(c echo.Context) error { return c.NoContent(200) })
	fetch := func() map[string]json.RawMessage {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest("GET", "https://local.example/api/profiles/"+profile.ID+"/status", nil))
		if rec.Code != 200 {
			t.Fatalf("profile status: %d %s", rec.Code, rec.Body)
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	result := fetch()
	var header map[string]string
	if err := json.Unmarshal(result["header"], &header); err != nil {
		t.Fatal(err)
	}
	if header["title"] != "Título propio" || header["subtitle"] != "Remoto" || header["badge"] != "Local" ||
		header["localBridgeStatus"] != "En línea" || header["lastUpdate"] != "12:34" || string(result["stats"]) != `{"ysfTx24h":42}` {
		t.Fatalf("unexpected overridden response: %s", result)
	}
	updated, err := store.save(Profile{Name: "Sitio", URL: profile.URL, Visible: true}, profile.ID)
	if err != nil || updated.Header == nil || updated.Header.Title != "Título propio" {
		t.Fatalf("omitted header should be preserved: %+v, %v", updated.Header, err)
	}
	_, err = store.save(Profile{Name: "Sitio", URL: profile.URL, Visible: true, Header: &ProfileHeader{}}, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fetch()["header"], &header); err != nil || header["title"] != "Original" || header["badge"] != "Antigua" {
		t.Fatalf("empty overrides should use remote values: %+v, %v", header, err)
	}
	if _, err := applyHeaderOverrides([]byte(`{"header":null}`), ProfileHeader{Title: "Test"}); err == nil {
		t.Fatal("accepted invalid remote header")
	}
	custom, err := applyHeaderOverrides(
		[]byte(`{"header":{"subtitle":"Original","lastUpdate":"12:34","localBridgeStatus":"En línea"}}`),
		ProfileHeader{Subtitle: "Mi subtítulo"},
	)
	if err != nil || !strings.Contains(string(custom), `"subtitle":"Mi subtítulo · última lectura 12:34"`) ||
		!strings.Contains(string(custom), `"localBridgeStatus":"En línea"`) {
		t.Fatalf("subtitle must retain the latest reading and bridge status: %s, %v", custom, err)
	}
}

func TestProfileHeaderLength(t *testing.T) {
	p := Profile{Name: "Sitio", URL: "https://example.org", Token: strings.Repeat("x", 64),
		Header: &ProfileHeader{Title: strings.Repeat("á", 200)}}
	if err := validProfile(p); err != nil {
		t.Fatalf("200-character title rejected: %v", err)
	}
	p.Header.Title += "á"
	if err := validProfile(p); err == nil {
		t.Fatal("title longer than 200 characters accepted")
	}
}
