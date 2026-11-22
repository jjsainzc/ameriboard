package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeBasePath(t *testing.T) {
	for input, want := range map[string]string{
		"/": "", "dashboard/": "/dashboard", "/one/two/": "/one/two",
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

func TestServerRoutes(t *testing.T) {
	for _, base := range []string{"", "/dashboard", "/one/two"} {
		frontend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("frontend:" + r.URL.Path))
		})
		server := newServer(base, frontend)
		for _, tc := range []struct {
			path string
			code int
			body string
		}{
			{"/healthz", http.StatusOK, `"status":"ok"`},
			{base + "/api/reflector-status", http.StatusOK, `"status":"ok"`},
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
