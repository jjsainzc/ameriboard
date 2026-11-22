package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	files := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<html><base href="/"><script src="main.js"></script></html>`)},
		"main.js":    &fstest.MapFile{Data: []byte("javascript")},
	}
	for _, base := range []string{"", "/dashboard", "/some/panel"} {
		handler, err := newHandler(files, base)
		if err != nil {
			t.Fatal(err)
		}
		for _, url := range []string{"/", "/index.html", "/nested/route"} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, url, nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `<base href="`+base+`/">`) {
				t.Errorf("base %q path %q: status %d, body %s", base, url, response.Code, response.Body.String())
			}
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/main.js", nil))
		if response.Code != http.StatusOK || response.Body.String() != "javascript" {
			t.Errorf("asset: status %d, body %s", response.Code, response.Body.String())
		}
	}
}

func TestHandlerRequiresBuild(t *testing.T) {
	if _, err := newHandler(fstest.MapFS{}, ""); err == nil {
		t.Fatal("expected an error when the frontend has not been built")
	}
}
