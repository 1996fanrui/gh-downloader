package app

import (
	"context"
	"testing"
	"time"

	"github.com/1996fanrui/gh-downloader/internal/analyzer"
	"github.com/1996fanrui/gh-downloader/internal/domain"
	gh "github.com/1996fanrui/gh-downloader/internal/github"
)

func TestParseRepoSpec(t *testing.T) {
	tests := []struct {
		spec  string
		owner string
		repo  string
	}{
		{spec: "openai/codex", owner: "openai", repo: "codex"},
		{spec: "https://github.com/openai/codex", owner: "openai", repo: "codex"},
		{spec: "git@github.com:openai/codex.git", owner: "openai", repo: "codex"},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			owner, repo, err := ParseRepoSpec(tt.spec)
			if err != nil {
				t.Fatalf("ParseRepoSpec returned error: %v", err)
			}
			if owner != tt.owner || repo != tt.repo {
				t.Fatalf("ParseRepoSpec(%q) = %s/%s, want %s/%s", tt.spec, owner, repo, tt.owner, tt.repo)
			}
		})
	}
}

func TestParseRepoSpecRejectsInvalidInput(t *testing.T) {
	if _, _, err := ParseRepoSpec("openai"); err == nil {
		t.Fatal("ParseRepoSpec accepted an invalid repo spec")
	}
}

func TestSyncRepoAddsNewVersionsWithoutDeletingHistory(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	store.repo = domain.Repository{
		Owner:    "openai",
		Name:     "codex",
		FullName: "openai/codex",
		Versions: []domain.ReleaseVersion{
			{Tag: "old-v1", PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
	}

	github := &fakeGitHub{}
	service := NewService(store, github, fakeAnalyzer{})
	service.now = func() time.Time { return time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC) }

	repo, err := service.SyncRepo(ctx, "openai", "codex")
	if err != nil {
		t.Fatalf("SyncRepo returned error: %v", err)
	}
	if len(repo.Versions) != 3 {
		t.Fatalf("version count = %d, want 3", len(repo.Versions))
	}
	if repo.Versions[0].Tag != "new-v2" || repo.Versions[1].Tag != "new-v1" || repo.Versions[2].Tag != "old-v1" {
		t.Fatalf("unexpected version order: %#v", repo.Versions)
	}
	if repo.Description != "\u4e2d\u6587\u9879\u76ee\u7b80\u4ecb" {
		t.Fatalf("description = %q", repo.Description)
	}
	if github.releaseLimit != domain.FetchVersions {
		t.Fatalf("release limit = %d, want %d", github.releaseLimit, domain.FetchVersions)
	}
}

type memoryStore struct {
	repo domain.Repository
}

func newMemoryStore() *memoryStore {
	return &memoryStore{}
}

func (s *memoryStore) List(context.Context) ([]domain.Repository, error) {
	if s.repo.FullName == "" {
		return nil, nil
	}
	return []domain.Repository{s.repo}, nil
}

func (s *memoryStore) Get(_ context.Context, owner string, repo string) (domain.Repository, bool, error) {
	if s.repo.Owner == owner && s.repo.Name == repo {
		return s.repo, true, nil
	}
	return domain.Repository{}, false, nil
}

func (s *memoryStore) Save(_ context.Context, repo domain.Repository) error {
	s.repo = repo
	return nil
}

type fakeGitHub struct {
	releaseLimit int
}

func (*fakeGitHub) Repository(context.Context, string, string) (gh.RepositoryInfo, error) {
	return gh.RepositoryInfo{
		Owner:       "openai",
		Name:        "codex",
		FullName:    "openai/codex",
		Description: "Lightweight coding agent that runs in your terminal",
	}, nil
}

func (*fakeGitHub) Readme(context.Context, string, string) (string, error) {
	return "Run codex --version to verify installation.", nil
}

func (g *fakeGitHub) StableReleases(_ context.Context, _ string, _ string, limit int) ([]gh.Release, error) {
	g.releaseLimit = limit
	return []gh.Release{
		{
			Tag:         "new-v2",
			PublishedAt: time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC),
			Assets:      []domain.ReleaseAsset{{Name: "codex-x86_64-pc-windows-msvc.exe", Size: 1, DownloadURL: "https://example.test/codex.exe"}},
		},
		{
			Tag:         "new-v1",
			PublishedAt: time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC),
			Assets:      []domain.ReleaseAsset{{Name: "codex-x86_64-pc-windows-msvc.exe", Size: 1, DownloadURL: "https://example.test/codex.exe"}},
		},
	}, nil
}

type fakeAnalyzer struct{}

func (fakeAnalyzer) Describe(context.Context, analyzer.DescriptionInput) (string, error) {
	return "\u4e2d\u6587\u9879\u76ee\u7b80\u4ecb", nil
}

func (fakeAnalyzer) InstallMethods(context.Context, analyzer.InstallMethodsInput) ([]domain.InstallMethod, error) {
	usageCommand := "codex"
	return []domain.InstallMethod{{
		Title:        "npm install",
		Manager:      "npm",
		Command:      "npm install -g @openai/codex",
		UsageCommand: &usageCommand,
		Source:       domain.InstallSourceReadme,
		Recommended:  true,
	}}, nil
}

func (fakeAnalyzer) Analyze(context.Context, analyzer.Input) (map[domain.Platform]*domain.PlatformSelection, error) {
	result := map[domain.Platform]*domain.PlatformSelection{}
	for _, platform := range domain.Platforms {
		result[platform] = nil
	}
	result[domain.PlatformWinX64] = &domain.PlatformSelection{Asset: "codex-x86_64-pc-windows-msvc.exe"}
	return result, nil
}
