package analyzer

import (
	"context"

	"github.com/1996fanrui/gh-downloader/internal/domain"
)

type Input struct {
	RepoFullName string
	Tag          string
	Assets       []domain.ReleaseAsset
	Readme       string
}

type DescriptionInput struct {
	RepoFullName      string
	GitHubDescription string
	Readme            string
}

type InstallMethodsInput struct {
	RepoFullName string
	Readme       string
}

type Analyzer interface {
	Describe(ctx context.Context, input DescriptionInput) (string, error)
	InstallMethods(ctx context.Context, input InstallMethodsInput) ([]domain.InstallMethod, error)
	Analyze(ctx context.Context, input Input) (map[domain.Platform]*domain.PlatformSelection, error)
}
