package domain

import "time"

const FetchVersions = 3

type Platform string

const (
	PlatformWinX64     Platform = "win/x64"
	PlatformWinARM64   Platform = "win/arm64"
	PlatformMacX64     Platform = "mac/x64"
	PlatformMacARM64   Platform = "mac/arm64"
	PlatformLinuxX64   Platform = "linux/x64"
	PlatformLinuxARM64 Platform = "linux/arm64"
)

var Platforms = []Platform{
	PlatformWinX64,
	PlatformWinARM64,
	PlatformMacX64,
	PlatformMacARM64,
	PlatformLinuxX64,
	PlatformLinuxARM64,
}

type AssetKind string

const (
	AssetKindInstaller AssetKind = "installer"
	AssetKindBinary    AssetKind = "binary"
	AssetKindAppImage  AssetKind = "appimage"
	AssetKindArchive   AssetKind = "archive"
	AssetKindMacApp    AssetKind = "macapp"
)

type VerifySource string

const (
	VerifySourceReadme VerifySource = "readme"
)

type InstallSource string

const (
	InstallSourceReadme InstallSource = "readme"
)

type Repository struct {
	Owner          string           `json:"owner"`
	Name           string           `json:"name"`
	FullName       string           `json:"full_name"`
	Description    string           `json:"description,omitempty"`
	HTMLURL        string           `json:"html_url,omitempty"`
	InstallMethods []InstallMethod  `json:"install_methods,omitempty"`
	LastSyncedAt   time.Time        `json:"last_synced_at,omitempty"`
	Versions       []ReleaseVersion `json:"versions"`
}

type ReleaseVersion struct {
	Tag         string                          `json:"tag"`
	Name        string                          `json:"name,omitempty"`
	PublishedAt time.Time                       `json:"published_at"`
	AssetTotal  int                             `json:"asset_total"`
	Assets      []ReleaseAsset                  `json:"assets"`
	PlatformMap map[Platform]*PlatformSelection `json:"platform_map"`
}

type ReleaseAsset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
	ContentType string `json:"content_type,omitempty"`
}

type PlatformSelection struct {
	Asset         string         `json:"asset"`
	Kind          *AssetKind     `json:"kind"`
	Install       *string        `json:"install,omitempty"`
	InstallSource *InstallSource `json:"install_source,omitempty"`
	Verify        *string        `json:"verify"`
	VerifySource  *VerifySource  `json:"verify_source"`
	Note          *string        `json:"note,omitempty"`
}

type InstallMethod struct {
	Title        string        `json:"title"`
	Manager      string        `json:"manager"`
	Command      string        `json:"command"`
	UsageCommand *string       `json:"usage_command,omitempty"`
	Platforms    []Platform    `json:"platforms,omitempty"`
	Source       InstallSource `json:"source"`
	Recommended  bool          `json:"recommended,omitempty"`
}

type RepoSummary struct {
	Owner        string    `json:"owner"`
	Name         string    `json:"name"`
	FullName     string    `json:"full_name"`
	Description  string    `json:"description,omitempty"`
	LatestTag    string    `json:"latest_tag,omitempty"`
	AssetTotal   int       `json:"asset_total,omitempty"`
	LastSyncedAt time.Time `json:"last_synced_at,omitempty"`
}
