package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/1996fanrui/gh-downloader/internal/domain"
)

type Client struct {
	token  string
	client *http.Client
}

func NewClient(token string, client *http.Client) *Client {
	return &Client{token: strings.TrimSpace(token), client: client}
}

type RepositoryInfo struct {
	Owner       string
	Name        string
	FullName    string
	Description string
	HTMLURL     string
}

type Release struct {
	Tag         string
	Name        string
	PublishedAt time.Time
	Assets      []domain.ReleaseAsset
}

func (c *Client) Repository(ctx context.Context, owner string, repo string) (RepositoryInfo, error) {
	var out repoResponse
	if err := c.getJSON(ctx, fmt.Sprintf("https://api.github.com/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo)), &out); err != nil {
		return RepositoryInfo{}, err
	}
	if out.FullName == "" {
		return RepositoryInfo{}, errors.New("GitHub repository response missing full_name")
	}
	return RepositoryInfo{
		Owner:       out.Owner.Login,
		Name:        out.Name,
		FullName:    out.FullName,
		Description: out.Description,
		HTMLURL:     out.HTMLURL,
	}, nil
}

func (c *Client) StableReleases(ctx context.Context, owner string, repo string, limit int) ([]Release, error) {
	var releases []Release
	for page := 1; len(releases) < limit && page <= 5; page++ {
		var out []releaseResponse
		endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=20&page=%d", url.PathEscape(owner), url.PathEscape(repo), page)
		if err := c.getJSON(ctx, endpoint, &out); err != nil {
			return nil, err
		}
		if len(out) == 0 {
			break
		}
		for _, release := range out {
			if release.Draft || release.Prerelease {
				continue
			}
			releases = append(releases, release.toDomain())
			if len(releases) == limit {
				break
			}
		}
	}
	return releases, nil
}

func (c *Client) Readme(ctx context.Context, owner string, repo string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/%s/readme", url.PathEscape(owner), url.PathEscape(repo)), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.raw")
	c.authorize(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch README: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub README returned %s: %s", resp.Status, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read README: %w", err)
	}
	return string(body), nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	c.authorize(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("call GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub returned %s: %s", resp.Status, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}

func (c *Client) authorize(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

type repoResponse struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	Owner       struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type releaseResponse struct {
	TagName    string          `json:"tag_name"`
	Name       string          `json:"name"`
	Draft      bool            `json:"draft"`
	Prerelease bool            `json:"prerelease"`
	Published  time.Time       `json:"published_at"`
	Assets     []assetResponse `json:"assets"`
}

func (r releaseResponse) toDomain() Release {
	assets := make([]domain.ReleaseAsset, 0, len(r.Assets))
	for _, asset := range r.Assets {
		assets = append(assets, domain.ReleaseAsset{
			Name:        asset.Name,
			Size:        asset.Size,
			DownloadURL: asset.BrowserDownloadURL,
			ContentType: asset.ContentType,
		})
	}
	return Release{
		Tag:         r.TagName,
		Name:        r.Name,
		PublishedAt: r.Published,
		Assets:      assets,
	}
}

type assetResponse struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}
