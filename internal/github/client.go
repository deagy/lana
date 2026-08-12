// Package github provides a GitHub API client for PR operations.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// Client is a GitHub API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// NewClient creates a new GitHub API client.
func NewClient(token string) *Client {
	return &Client{
		baseURL:    "https://api.github.com",
		httpClient: &http.Client{},
		token:      token,
	}
}

// NewClientWithBaseURL creates a client with a custom base URL.
func NewClientWithBaseURL(token, baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
		token:      token,
	}
}

// FromEnv creates a client using the GITHUB_TOKEN environment variable.
func FromEnv() *Client {
	return NewClient(os.Getenv("GITHUB_TOKEN"))
}

// PR represents a GitHub pull request.
type PR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	HeadRef   string `json:"head"`
	BaseRef   string `json:"base"`
	Draft     bool   `json:"draft"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreatePROptions are options for creating a pull request.
type CreatePROptions struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Draft bool   `json:"draft,omitempty"`
}

// CreatePR creates a pull request.
func (c *Client) CreatePR(ctx context.Context, owner, repo string, opts CreatePROptions) (*PR, error) {
	if c.token == "" {
		return nil, fmt.Errorf("GitHub token not configured (set GITHUB_TOKEN)")
	}
	url := fmt.Sprintf("%s/repos/%s/%s/pulls", c.baseURL, owner, repo)
	body, err := json.Marshal(opts)
	if err != nil {
		return nil, fmt.Errorf("marshal PR options: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create PR request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create PR failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var pr PR
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decode PR: %w", err)
	}
	return &pr, nil
}

// ListPRs lists pull requests for a repository.
func (c *Client) ListPRs(ctx context.Context, owner, repo string, state, head, base string) ([]PR, error) {
	if c.token == "" {
		return nil, fmt.Errorf("GitHub token not configured (set GITHUB_TOKEN)")
	}
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=%s", c.baseURL, owner, repo, state)
	if head != "" {
		url += "&head=" + head
	}
	if base != "" {
		url += "&base=" + base
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list PRs request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list PRs failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var prs []PR
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, fmt.Errorf("decode PRs: %w", err)
	}
	return prs, nil
}

// GetPRDiff gets the diff for a pull request.
func (c *Client) GetPRDiff(ctx context.Context, owner, repo string, number int) (string, error) {
	if c.token == "" {
		return "", fmt.Errorf("GitHub token not configured (set GITHUB_TOKEN)")
	}
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", c.baseURL, owner, repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3.diff")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get PR diff request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get PR diff failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}
	return string(data), nil
}

// DetectOwnerRepo detects the owner and repo from the git remote URL.
func DetectOwnerRepo() (string, string, error) {
	out, err := runGit("remote", "get-url", "origin")
	if err != nil {
		return "", "", fmt.Errorf("get remote URL: %w", err)
	}
	remoteURL := strings.TrimSpace(string(out))
	// Handle SSH URLs: git@github.com:owner/repo.git
	if strings.Contains(remoteURL, ":") {
		parts := strings.Split(remoteURL, ":")
		if len(parts) == 2 {
			repoPath := strings.TrimSuffix(parts[1], ".git")
			ownerRepo := strings.Split(repoPath, "/")
			if len(ownerRepo) == 2 {
				return ownerRepo[0], ownerRepo[1], nil
			}
		}
	}
	// Handle HTTPS URLs: https://github.com/owner/repo.git
	if strings.Contains(remoteURL, "github.com") {
		parts := strings.Split(remoteURL, "github.com/")
		if len(parts) == 2 {
			repoPath := strings.TrimSuffix(parts[1], ".git")
			ownerRepo := strings.Split(repoPath, "/")
			if len(ownerRepo) == 2 {
				return ownerRepo[0], ownerRepo[1], nil
			}
		}
	}
	return "", "", fmt.Errorf("unable to detect owner/repo from remote URL: %s", remoteURL)
}

func runGit(args ...string) ([]byte, error) {
	cmd := "git"
	allArgs := make([]string, 0, 2+len(args))
	allArgs = append(allArgs, "-C", ".")
	allArgs = append(allArgs, args...)
	return runCommand(cmd, allArgs...)
}

func runCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = "."
	return cmd.Output()
}
