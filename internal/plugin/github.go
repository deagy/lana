// Package plugin provides GitHub remote plugin discovery and installation.
package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GitHubPluginClient handles remote plugin operations with GitHub.
type GitHubPluginClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// GitHubPlugin represents a plugin available on GitHub.
type GitHubPlugin struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Repository  string    `json:"repository"`
	Version     string    `json:"version"`
	Homepage    string    `json:"homepage,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName string               `json:"tag_name"`
	Name    string               `json:"name"`
	Assets  []GitHubReleaseAsset `json:"assets"`
}

// GitHubReleaseAsset represents an asset in a GitHub release.
type GitHubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// NewGitHubPluginClient creates a new GitHub plugin client.
func NewGitHubPluginClient(token string) *GitHubPluginClient {
	return &GitHubPluginClient{
		baseURL: "https://api.github.com",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token: token,
	}
}

// SearchPlugins searches for Lana plugins on GitHub.
func (c *GitHubPluginClient) SearchPlugins(query string, limit int) ([]GitHubPlugin, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	url := fmt.Sprintf("%s/search/repositories?q=lana-plugin+%%22lana-plugin%%22&per_page=%d", c.baseURL, limit)
	if query != "" {
		url += "&q=" + query
	}

	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("search plugins: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Items []struct {
			Name        string    `json:"name"`
			Description string    `json:"description"`
			HTMLURL     string    `json:"html_url"`
			UpdatedAt   time.Time `json:"updated_at"`
			Topics      []string  `json:"topics"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search results: %w", err)
	}

	plugins := make([]GitHubPlugin, 0, len(result.Items))
	for _, item := range result.Items {
		hasTopic := false
		for _, topic := range item.Topics {
			if topic == "lana-plugin" {
				hasTopic = true
				break
			}
		}
		if !hasTopic {
			continue
		}

		plugins = append(plugins, GitHubPlugin{
			Name:        item.Name,
			Description: item.Description,
			Repository:  strings.TrimPrefix(item.HTMLURL, "https://github.com/"),
			UpdatedAt:   item.UpdatedAt,
		})
	}

	return plugins, nil
}

// GetPluginReleases retrieves releases for a GitHub repository.
func (c *GitHubPluginClient) GetPluginReleases(owner, repo string) ([]GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=10", c.baseURL, owner, repo)

	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("get releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get releases failed: %s - %s", resp.Status, string(body))
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	return releases, nil
}

// DownloadPluginArchive downloads a plugin archive from GitHub.
func (c *GitHubPluginClient) DownloadPluginArchive(owner, repo, version, destDir string) (string, error) {
	releases, err := c.GetPluginReleases(owner, repo)
	if err != nil {
		return "", err
	}

	var release *GitHubRelease
	for i := range releases {
		if releases[i].TagName == version {
			release = &releases[i]
			break
		}
	}

	if release == nil {
		return "", fmt.Errorf("version %s not found", version)
	}

	var assetURL string
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".tar.gz") || strings.HasSuffix(asset.Name, ".zip") {
			assetURL = asset.BrowserDownloadURL
			break
		}
	}

	if assetURL == "" {
		return "", fmt.Errorf("no archive found in release %s", version)
	}

	resp, err := c.doRequest("GET", assetURL, nil)
	if err != nil {
		return "", fmt.Errorf("download archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("download failed: %s - %s", resp.Status, string(body))
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create destination: %w", err)
	}

	archivePath := filepath.Join(destDir, filepath.Base(assetURL))
	f, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("create archive file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("write archive: %w", err)
	}

	return archivePath, nil
}

// doRequest performs an HTTP request with GitHub API authentication.
func (c *GitHubPluginClient) doRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "lana-plugin-client")

	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}

	return c.httpClient.Do(req)
}
