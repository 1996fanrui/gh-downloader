package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestNginxDownloadCacheLimit(t *testing.T) {
	body, err := os.ReadFile("../../deploy/nginx.conf")
	if err != nil {
		t.Fatalf("read nginx config: %v", err)
	}
	if !strings.Contains(string(body), "max_size=60g") {
		t.Fatal("nginx download cache max_size must be 60g")
	}
}
