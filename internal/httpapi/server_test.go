package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStaticResponsesDisableLongLivedCaching(t *testing.T) {
	handler := NewServer(nil, nil).Handler()
	for _, path := range []string{"/", "/repos/openai/codex", "/assets/app.js", "/favicon.ico"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s returned %d", path, rec.Code)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache, max-age=0" {
			t.Fatalf("%s Cache-Control = %q", path, got)
		}
	}
}

func TestRepoPageRouteServesIndex(t *testing.T) {
	handler := NewServer(nil, nil).Handler()
	req := httptest.NewRequest(http.MethodGet, "/repos/openai/codex", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("repo page returned %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("repo page Content-Type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "Github Downloader") {
		t.Fatal("repo page did not serve index")
	}
}

func TestFaviconServesIcon(t *testing.T) {
	handler := NewServer(nil, nil).Handler()
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("favicon returned %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("favicon Content-Type = %q", got)
	}
	if len(rec.Body.Bytes()) == 0 {
		t.Fatal("favicon body is empty")
	}
}
