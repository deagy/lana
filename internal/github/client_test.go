package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreatePR(t *testing.T) {
	// Mock GitHub API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/pulls" && r.Method == "POST" {
			var opts CreatePROptions
			if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if opts.Title == "" {
				http.Error(w, "title is required", http.StatusBadRequest)
				return
			}
			pr := PR{
				Number:  42,
				Title:   opts.Title,
				Body:    opts.Body,
				State:   "open",
				HTMLURL: fmt.Sprintf("https://github.com/owner/repo/pull/42"),
				HeadRef: opts.Head,
				BaseRef: opts.Base,
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(pr)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClientWithBaseURL("test-token", server.URL)
	pr, err := client.CreatePR(context.Background(), "owner", "repo", CreatePROptions{
		Title: "Test PR",
		Body:  "Test body",
		Head:  "feature-branch",
		Base:  "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Number != 42 {
		t.Fatalf("expected PR number 42, got %d", pr.Number)
	}
	if pr.Title != "Test PR" {
		t.Fatalf("expected title 'Test PR', got %q", pr.Title)
	}
	if pr.HTMLURL != "https://github.com/owner/repo/pull/42" {
		t.Fatalf("expected URL 'https://github.com/owner/repo/pull/42', got %q", pr.HTMLURL)
	}
}

func TestCreatePRMissingToken(t *testing.T) {
	client := NewClient("")
	_, err := client.CreatePR(context.Background(), "owner", "repo", CreatePROptions{
		Title: "Test PR",
	})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestListPRs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/pulls" && r.Method == "GET" {
			prs := []PR{
				{
					Number:  42,
					Title:   "PR 1",
					State:   "open",
					HTMLURL: "https://github.com/owner/repo/pull/42",
				},
				{
					Number:  43,
					Title:   "PR 2",
					State:   "open",
					HTMLURL: "https://github.com/owner/repo/pull/43",
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(prs)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClientWithBaseURL("test-token", server.URL)
	prs, err := client.ListPRs(context.Background(), "owner", "repo", "open", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(prs))
	}
	if prs[0].Number != 42 {
		t.Fatalf("expected first PR number 42, got %d", prs[0].Number)
	}
}

func TestGetPRDiff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/pulls/42" && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "diff content here")
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClientWithBaseURL("test-token", server.URL)
	diff, err := client.GetPRDiff(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != "diff content here" {
		t.Fatalf("expected diff 'diff content here', got %q", diff)
	}
}
