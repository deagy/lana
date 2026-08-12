// Package gitlab provides a GitLab API client for MR operations.
package gitlab

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

// Client is a GitLab API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// NewClient creates a new GitLab API client.
func NewClient(token, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://gitlab.com/api/v4"
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{},
		token:      token,
	}
}

// FromEnv creates a client using GITLAB_TOKEN and GITLAB_URL environment variables.
func FromEnv() *Client {
	return NewClient(
		os.Getenv("GITLAB_TOKEN"),
		os.Getenv("GITLAB_URL"),
	)
}

// MROptions are options for creating a merge request.
type MROptions struct {
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	Draft        bool   `json:"draft,omitempty"`
}

// MROpenState represents the state of a merge request.
type MROpenState string

const (
	MRStateOpened MROpenState = "opened"
	MRStateClosed MROpenState = "closed"
	MRStateMerged MROpenState = "merged"
)

// MergeRequest represents a GitLab merge request.
type MergeRequest struct {
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	State        string `json:"state"`
	WebURL       string `json:"web_url"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	Draft        bool   `json:"draft"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// CreateMR creates a merge request.
func (c *Client) CreateMR(ctx context.Context, projectPath string, opts MROptions) (*MergeRequest, error) {
	if c.token == "" {
		return nil, fmt.Errorf("GitLab token not configured (set GITLAB_TOKEN)")
	}
	url := fmt.Sprintf("%s/projects/%s/merge_requests", c.baseURL, projectPath)
	body, err := json.Marshal(opts)
	if err != nil {
		return nil, fmt.Errorf("marshal MR options: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create MR request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create MR failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var mr MergeRequest
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return nil, fmt.Errorf("decode MR: %w", err)
	}
	return &mr, nil
}

// ListMRs lists merge requests for a project.
func (c *Client) ListMRs(ctx context.Context, projectPath string, state MROpenState) ([]MergeRequest, error) {
	if c.token == "" {
		return nil, fmt.Errorf("GitLab token not configured (set GITLAB_TOKEN)")
	}
	url := fmt.Sprintf("%s/projects/%s/merge_requests?state=%s", c.baseURL, projectPath, state)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list MRs request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list MRs failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var mrs []MergeRequest
	if err := json.NewDecoder(resp.Body).Decode(&mrs); err != nil {
		return nil, fmt.Errorf("decode MRs: %w", err)
	}
	return mrs, nil
}

// GetMRDiff gets the diff for a merge request.
func (c *Client) GetMRDiff(ctx context.Context, projectPath string, iid int) (string, error) {
	if c.token == "" {
		return "", fmt.Errorf("GitLab token not configured (set GITLAB_TOKEN)")
	}
	url := fmt.Sprintf("%s/projects/%s/merge_requests/%d/changes", c.baseURL, projectPath, iid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get MR diff request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get MR diff failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Changes []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			Diff        string `json:"diff"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
		} `json:"changes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode MR diff: %w", err)
	}

	var sb strings.Builder
	for _, change := range result.Changes {
		prefix := ""
		if change.NewFile {
			prefix = "(new file) "
		} else if change.DeletedFile {
			prefix = "(deleted) "
		} else if change.RenamedFile {
			prefix = fmt.Sprintf("(renamed: %s -> %s) ", change.OldPath, change.NewPath)
		}
		sb.WriteString(fmt.Sprintf("%s%s\n%s\n\n", prefix, change.NewPath, change.Diff))
	}
	return sb.String(), nil
}

// DetectProjectPath detects the project path from the git remote URL.
func DetectProjectPath() (string, error) {
	out, err := runGit("remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("get remote URL: %w", err)
	}
	remoteURL := strings.TrimSpace(string(out))
	// Handle SSH URLs: git@gitlab.com:owner/project.git
	if strings.Contains(remoteURL, ":") && !strings.HasPrefix(remoteURL, "http") {
		parts := strings.Split(remoteURL, ":")
		if len(parts) == 2 {
			projectPath := strings.TrimSuffix(parts[1], ".git")
			return projectPath, nil
		}
	}
	// Handle HTTPS URLs: https://gitlab.com/owner/project.git
	if strings.Contains(remoteURL, "gitlab.com") {
		parts := strings.Split(remoteURL, "gitlab.com/")
		if len(parts) == 2 {
			projectPath := strings.TrimSuffix(parts[1], ".git")
			return projectPath, nil
		}
	}
	return "", fmt.Errorf("unable to detect project path from remote URL: %s", remoteURL)
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
