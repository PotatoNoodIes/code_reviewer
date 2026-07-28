package githubapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestFindingHash_StableAndDistinguishing(t *testing.T) {
	h1 := FindingHash("main.go", 5, "bug", "off by one")
	h2 := FindingHash("main.go", 5, "bug", "off by one")
	if h1 != h2 {
		t.Errorf("expected identical inputs to hash identically: %q vs %q", h1, h2)
	}

	variants := []string{
		FindingHash("other.go", 5, "bug", "off by one"),
		FindingHash("main.go", 6, "bug", "off by one"),
		FindingHash("main.go", 5, "security", "off by one"),
		FindingHash("main.go", 5, "bug", "different message"),
	}
	for _, v := range variants {
		if v == h1 {
			t.Errorf("expected a differing field to change the hash, got same hash %q", v)
		}
	}
}

func TestHashMarkerRoundTrip(t *testing.T) {
	hash := FindingHash("main.go", 5, "bug", "off by one")
	body := WithHashMarker("Off by one error here.", hash)

	got, ok := ExtractHashMarker(body)
	if !ok {
		t.Fatalf("expected marker to be found")
	}
	if got != hash {
		t.Errorf("got %q, want %q", got, hash)
	}
}

func TestExtractHashMarker_Absent(t *testing.T) {
	if _, ok := ExtractHashMarker("just a plain comment"); ok {
		t.Errorf("expected no marker to be found")
	}
}

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	gh := github.NewClient(nil)
	baseURL, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	gh.BaseURL = baseURL
	return &Client{gh: gh}
}

func TestExistingFindingHashes_FiltersByLoginAndExtractsHash(t *testing.T) {
	hashA := FindingHash("main.go", 5, "bug", "issue A")

	comments := []map[string]any{
		{"id": 1, "body": WithHashMarker("issue A", hashA), "user": map[string]any{"login": "ai-code-reviewer[bot]"}},
		{"id": 2, "body": "unrelated human comment", "user": map[string]any{"login": "someone-else"}},
		{"id": 3, "body": "a bot comment with no marker", "user": map[string]any{"login": "ai-code-reviewer[bot]"}},
	}

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(comments)
	})

	hashes, err := client.ExistingFindingHashes(context.Background(), "o", "r", 1, "ai-code-reviewer[bot]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hashes) != 1 || !hashes[hashA] {
		t.Fatalf("expected exactly {%q: true}, got %v", hashA, hashes)
	}
}

func TestPostReview_NoOpWhenNoComments(t *testing.T) {
	called := false
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})

	if err := client.PostReview(context.Background(), "o", "r", 1, "sha", "COMMENT", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Errorf("expected no HTTP call when comments is empty")
	}
}

func TestPostReview_SendsCommitAndEvent(t *testing.T) {
	var gotBody github.PullRequestReviewRequest
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})

	comments := []*github.DraftReviewComment{NewDraftComment("main.go", 5, "hi")}
	if err := client.PostReview(context.Background(), "o", "r", 1, "deadbeef", "COMMENT", comments); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody.GetCommitID() != "deadbeef" {
		t.Errorf("CommitID = %q", gotBody.GetCommitID())
	}
	if gotBody.GetEvent() != "COMMENT" {
		t.Errorf("Event = %q", gotBody.GetEvent())
	}
	if len(gotBody.Comments) != 1 || gotBody.Comments[0].GetLine() != 5 {
		t.Errorf("Comments = %+v", gotBody.Comments)
	}
}
