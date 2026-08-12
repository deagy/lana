package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateMR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects/owner/project/merge_requests" && r.Method == "POST" {
			var opts MROptions
			if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if opts.Title == "" {
				http.Error(w, "title is required", http.StatusBadRequest)
				return
			}
			mr := MergeRequest{
				IID:          42,
				Title:        opts.Title,
				Description:  opts.Description,
				State:        "opened",
				WebURL:       fmt.Sprintf("https://gitlab.com/owner/project/-/merge_requests/42"),
				SourceBranch: opts.SourceBranch,
				TargetBranch: opts.TargetBranch,
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(mr)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)
	mr, err := client.CreateMR(context.Background(), "owner/project", MROptions{
		Title:        "Test MR",
		Description:  "Test description",
		SourceBranch: "feature-branch",
		TargetBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mr.IID != 42 {
		t.Fatalf("expected MR IID 42, got %d", mr.IID)
	}
	if mr.Title != "Test MR" {
		t.Fatalf("expected title 'Test MR', got %q", mr.Title)
	}
	if mr.WebURL != "https://gitlab.com/owner/project/-/merge_requests/42" {
		t.Fatalf("expected URL 'https://gitlab.com/owner/project/-/merge_requests/42', got %q", mr.WebURL)
	}
}

func TestCreateMRMissingToken(t *testing.T) {
	client := NewClient("", "")
	_, err := client.CreateMR(context.Background(), "owner/project", MROptions{
		Title: "Test MR",
	})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestListMRs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects/owner/project/merge_requests" && r.Method == "GET" {
			mrs := []MergeRequest{
				{
					IID:    42,
					Title:  "MR 1",
					State:  "opened",
					WebURL: "https://gitlab.com/owner/project/-/merge_requests/42",
				},
				{
					IID:    43,
					Title:  "MR 2",
					State:  "opened",
					WebURL: "https://gitlab.com/owner/project/-/merge_requests/43",
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(mrs)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)
	mrs, err := client.ListMRs(context.Background(), "owner/project", MRStateOpened)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mrs) != 2 {
		t.Fatalf("expected 2 MRs, got %d", len(mrs))
	}
	if mrs[0].IID != 42 {
		t.Fatalf("expected first MR IID 42, got %d", mrs[0].IID)
	}
}

func TestGetMRDiff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects/owner/project/merge_requests/42/changes" && r.Method == "GET" {
			result := struct {
				Changes []struct {
					OldPath     string `json:"old_path"`
					NewPath     string `json:"new_path"`
					Diff        string `json:"diff"`
					NewFile     bool   `json:"new_file"`
					RenamedFile bool   `json:"renamed_file"`
					DeletedFile bool   `json:"deleted_file"`
				} `json:"changes"`
			}{
				Changes: []struct {
					OldPath     string `json:"old_path"`
					NewPath     string `json:"new_path"`
					Diff        string `json:"diff"`
					NewFile     bool   `json:"new_file"`
					RenamedFile bool   `json:"renamed_file"`
					DeletedFile bool   `json:"deleted_file"`
				}{
					{
						OldPath: "file.go",
						NewPath: "file.go",
						Diff:    "@@ -1,5 +1,5 @@\n-old\n+new\n",
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(result)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)
	diff, err := client.GetMRDiff(context.Background(), "owner/project", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
}
