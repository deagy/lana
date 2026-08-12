package plugin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewGitHubPluginClient(t *testing.T) {
	client := NewGitHubPluginClient("test-token")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.token != "test-token" {
		t.Fatalf("expected token 'test-token', got '%s'", client.token)
	}
}

func TestSearchPluginsEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/repositories" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	}))
	defer server.Close()

	client := NewGitHubPluginClient("")
	client.baseURL = server.URL

	plugins, err := client.SearchPlugins("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestSearchPluginsWithResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"name":        "test-plugin",
					"description": "A test plugin",
					"html_url":    "https://github.com/owner/test-plugin",
					"updated_at":  "2023-01-01T00:00:00Z",
					"topics":      []string{"lana-plugin", "test"},
				},
			},
		})
	}))
	defer server.Close()

	client := NewGitHubPluginClient("")
	client.baseURL = server.URL

	plugins, err := client.SearchPlugins("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name != "test-plugin" {
		t.Fatalf("expected name 'test-plugin', got '%s'", plugins[0].Name)
	}
	if plugins[0].Repository != "owner/test-plugin" {
		t.Fatalf("expected repository 'owner/test-plugin', got '%s'", plugins[0].Repository)
	}
}

func TestSearchPluginsFiltersByTopic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"name":        "not-a-plugin",
					"description": "Not a Lana plugin",
					"html_url":    "https://github.com/owner/not-a-plugin",
					"updated_at":  "2023-01-01T00:00:00Z",
					"topics":      []string{"other"},
				},
				map[string]interface{}{
					"name":        "real-plugin",
					"description": "A real Lana plugin",
					"html_url":    "https://github.com/owner/real-plugin",
					"updated_at":  "2023-01-01T00:00:00Z",
					"topics":      []string{"lana-plugin"},
				},
			},
		})
	}))
	defer server.Close()

	client := NewGitHubPluginClient("")
	client.baseURL = server.URL

	plugins, err := client.SearchPlugins("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name != "real-plugin" {
		t.Fatalf("expected name 'real-plugin', got '%s'", plugins[0].Name)
	}
}

func TestGetPluginReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"tag_name": "v1.0.0",
				"name":     "Version 1.0.0",
				"assets": []map[string]interface{}{
					{
						"name":                 "plugin-v1.0.0.tar.gz",
						"browser_download_url": "https://github.com/owner/repo/releases/download/v1.0.0/plugin-v1.0.0.tar.gz",
						"size":                 1024,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewGitHubPluginClient("")
	client.baseURL = server.URL

	releases, err := client.GetPluginReleases("owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(releases))
	}
	if releases[0].TagName != "v1.0.0" {
		t.Fatalf("expected tag 'v1.0.0', got '%s'", releases[0].TagName)
	}
}

func TestDownloadPluginArchive(t *testing.T) {
	// Create a handler that returns the server URL in the download link
	var serverURL string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/releases" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"tag_name": "v1.0.0",
					"name":     "Version 1.0.0",
					"assets": []map[string]interface{}{
						{
							"name":                 "plugin-v1.0.0.tar.gz",
							"browser_download_url": serverURL + "/download.tar.gz",
							"size":                 1024,
						},
					},
				},
			})
			return
		}
		if r.URL.Path == "/download.tar.gz" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("fake archive content"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	server := httptest.NewServer(handler)
	serverURL = server.URL
	defer server.Close()

	client := NewGitHubPluginClient("")
	client.baseURL = server.URL

	destDir := t.TempDir()
	archivePath, err := client.DownloadPluginArchive("owner", "repo", "v1.0.0", destDir)
	if err != nil {
		t.Fatal(err)
	}
	if archivePath == "" {
		t.Fatal("expected non-empty archive path")
	}

	// Verify file was created
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatal("archive file was not created")
	}
}

func TestDownloadPluginArchiveVersionNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"tag_name": "v1.0.0",
				"name":     "Version 1.0.0",
				"assets":   []interface{}{},
			},
		})
	}))
	defer server.Close()

	client := NewGitHubPluginClient("")
	client.baseURL = server.URL

	destDir := t.TempDir()
	_, err := client.DownloadPluginArchive("owner", "repo", "v2.0.0", destDir)
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestDownloadPluginArchiveNoArchiveAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"tag_name": "v1.0.0",
				"name":     "Version 1.0.0",
				"assets": []map[string]interface{}{
					{
						"name":                 "changelog.md",
						"browser_download_url": "https://github.com/owner/repo/releases/download/v1.0.0/changelog.md",
						"size":                 100,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewGitHubPluginClient("")
	client.baseURL = server.URL

	destDir := t.TempDir()
	_, err := client.DownloadPluginArchive("owner", "repo", "v1.0.0", destDir)
	if err == nil {
		t.Fatal("expected error for missing archive asset")
	}
}

func TestDoRequestWithToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "token test-token" {
			t.Fatalf("expected Authorization 'token test-token', got '%s'", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewGitHubPluginClient("test-token")
	client.baseURL = server.URL

	resp, err := client.doRequest("GET", server.URL+"/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestDoRequestWithoutToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" {
			t.Fatalf("expected no Authorization header, got '%s'", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewGitHubPluginClient("")
	client.baseURL = server.URL

	resp, err := client.doRequest("GET", server.URL+"/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}
