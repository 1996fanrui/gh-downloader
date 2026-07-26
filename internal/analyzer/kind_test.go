package analyzer

import (
	"testing"

	"github.com/1996fanrui/gh-downloader/internal/domain"
)

func TestInferKind(t *testing.T) {
	tests := []struct {
		name string
		want *domain.AssetKind
	}{
		{name: "tool.exe", want: ptr(domain.AssetKindInstaller)},
		{name: "tool.msi", want: ptr(domain.AssetKindInstaller)},
		{name: "tool.AppImage", want: ptr(domain.AssetKindAppImage)},
		{name: "tool-linux-x64.tar.gz", want: ptr(domain.AssetKindArchive)},
		{name: "tool.zip", want: ptr(domain.AssetKindArchive)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferKind(tt.name)
			if got == nil && tt.want == nil {
				return
			}
			if got == nil || tt.want == nil || *got != *tt.want {
				t.Fatalf("InferKind(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func ptr(value domain.AssetKind) *domain.AssetKind {
	return &value
}
