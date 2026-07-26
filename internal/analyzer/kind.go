package analyzer

import (
	"strings"

	"github.com/1996fanrui/gh-downloader/internal/domain"
)

func InferKind(name string) *domain.AssetKind {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".appimage"):
		return kind(domain.AssetKindAppImage)
	case hasAnySuffix(lower, ".exe", ".msi", ".dmg", ".pkg", ".deb", ".rpm"):
		return kind(domain.AssetKindInstaller)
	case hasAnySuffix(lower, ".tar.gz", ".tgz", ".tar.xz", ".tar.zst", ".tar.bz2", ".zip"):
		return kind(domain.AssetKindArchive)
	}

	return nil
}

func IsValidKind(value domain.AssetKind) bool {
	switch value {
	case domain.AssetKindInstaller, domain.AssetKindBinary, domain.AssetKindAppImage, domain.AssetKindArchive, domain.AssetKindMacApp:
		return true
	default:
		return false
	}
}

func IsValidVerifySource(value domain.VerifySource) bool {
	switch value {
	case domain.VerifySourceReadme:
		return true
	default:
		return false
	}
}

func IsValidInstallSource(value domain.InstallSource) bool {
	switch value {
	case domain.InstallSourceReadme:
		return true
	default:
		return false
	}
}

func kind(value domain.AssetKind) *domain.AssetKind {
	return &value
}

func hasAnySuffix(value string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}
