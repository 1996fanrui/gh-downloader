package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/1996fanrui/gh-downloader/internal/analyzer"
	"github.com/1996fanrui/gh-downloader/internal/domain"
	gh "github.com/1996fanrui/gh-downloader/internal/github"
)

type Store interface {
	List(ctx context.Context) ([]domain.Repository, error)
	Get(ctx context.Context, owner string, repo string) (domain.Repository, bool, error)
	Save(ctx context.Context, repo domain.Repository) error
}

type GitHubClient interface {
	Repository(ctx context.Context, owner string, repo string) (gh.RepositoryInfo, error)
	StableReleases(ctx context.Context, owner string, repo string, limit int) ([]gh.Release, error)
	Readme(ctx context.Context, owner string, repo string) (string, error)
}

type Service struct {
	store    Store
	github   GitHubClient
	analyzer analyzer.Analyzer
	now      func() time.Time
}

func NewService(store Store, github GitHubClient, analyzer analyzer.Analyzer) *Service {
	return &Service{store: store, github: github, analyzer: analyzer, now: time.Now}
}

func (s *Service) ListRepos(ctx context.Context) ([]domain.RepoSummary, error) {
	repos, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]domain.RepoSummary, 0, len(repos))
	for _, repo := range repos {
		summary := domain.RepoSummary{
			Owner:        repo.Owner,
			Name:         repo.Name,
			FullName:     repo.FullName,
			Description:  repo.Description,
			LastSyncedAt: repo.LastSyncedAt,
		}
		if len(repo.Versions) > 0 {
			summary.LatestTag = repo.Versions[0].Tag
			summary.AssetTotal = repo.Versions[0].AssetTotal
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func (s *Service) GetRepo(ctx context.Context, owner string, repo string) (domain.Repository, bool, error) {
	return s.store.Get(ctx, owner, repo)
}

func (s *Service) AddRepo(ctx context.Context, spec string) (domain.Repository, error) {
	owner, repo, err := ParseRepoSpec(spec)
	if err != nil {
		return domain.Repository{}, err
	}
	return s.SyncRepo(ctx, owner, repo)
}

func (s *Service) SyncRepo(ctx context.Context, owner string, repoName string) (domain.Repository, error) {
	info, err := s.github.Repository(ctx, owner, repoName)
	if err != nil {
		return domain.Repository{}, err
	}
	readme, err := s.github.Readme(ctx, info.Owner, info.Name)
	if err != nil {
		return domain.Repository{}, err
	}
	releases, err := s.github.StableReleases(ctx, info.Owner, info.Name, domain.FetchVersions)
	if err != nil {
		return domain.Repository{}, err
	}
	if len(releases) == 0 {
		return domain.Repository{}, errors.New("repository has no stable releases")
	}
	description, err := s.analyzer.Describe(ctx, analyzer.DescriptionInput{
		RepoFullName:      info.FullName,
		GitHubDescription: info.Description,
		Readme:            readme,
	})
	if err != nil {
		return domain.Repository{}, fmt.Errorf("describe %s: %w", info.FullName, err)
	}
	installMethods, err := s.analyzer.InstallMethods(ctx, analyzer.InstallMethodsInput{
		RepoFullName: info.FullName,
		Readme:       readme,
	})
	if err != nil {
		return domain.Repository{}, fmt.Errorf("analyze install methods %s: %w", info.FullName, err)
	}

	current, _, err := s.store.Get(ctx, info.Owner, info.Name)
	if err != nil {
		return domain.Repository{}, err
	}
	byTag := map[string]domain.ReleaseVersion{}
	for _, version := range current.Versions {
		byTag[version.Tag] = version
	}

	for _, release := range releases {
		if _, exists := byTag[release.Tag]; exists {
			continue
		}
		platformMap, err := s.analyzer.Analyze(ctx, analyzer.Input{
			RepoFullName: info.FullName,
			Tag:          release.Tag,
			Assets:       release.Assets,
			Readme:       readme,
		})
		if err != nil {
			return domain.Repository{}, fmt.Errorf("analyze %s %s: %w", info.FullName, release.Tag, err)
		}
		byTag[release.Tag] = domain.ReleaseVersion{
			Tag:         release.Tag,
			Name:        release.Name,
			PublishedAt: release.PublishedAt,
			AssetTotal:  len(release.Assets),
			Assets:      release.Assets,
			PlatformMap: platformMap,
		}
	}

	versions := make([]domain.ReleaseVersion, 0, len(byTag))
	for _, version := range byTag {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i int, j int) bool {
		return versions[i].PublishedAt.After(versions[j].PublishedAt)
	})

	updated := domain.Repository{
		Owner:          info.Owner,
		Name:           info.Name,
		FullName:       info.FullName,
		Description:    description,
		HTMLURL:        info.HTMLURL,
		InstallMethods: installMethods,
		LastSyncedAt:   s.now().UTC(),
		Versions:       versions,
	}
	if err := s.store.Save(ctx, updated); err != nil {
		return domain.Repository{}, err
	}
	return updated, nil
}

func (s *Service) ResolveAsset(ctx context.Context, owner string, repoName string, tag string, assetName string) (domain.ReleaseAsset, bool, error) {
	repo, ok, err := s.store.Get(ctx, owner, repoName)
	if err != nil || !ok {
		return domain.ReleaseAsset{}, false, err
	}
	for _, version := range repo.Versions {
		if version.Tag != tag {
			continue
		}
		for _, asset := range version.Assets {
			if asset.Name == assetName {
				return asset, true, nil
			}
		}
	}
	return domain.ReleaseAsset{}, false, nil
}

func ParseRepoSpec(spec string) (string, string, error) {
	value := strings.TrimSpace(spec)
	value = strings.TrimSuffix(value, ".git")
	value = strings.TrimPrefix(value, "https://github.com/")
	value = strings.TrimPrefix(value, "http://github.com/")
	value = strings.TrimPrefix(value, "git@github.com:")
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repo must be owner/name or a github.com URL")
	}
	return parts[0], parts[1], nil
}
