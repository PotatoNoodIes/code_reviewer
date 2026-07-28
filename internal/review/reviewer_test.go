package review

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jhenriquez/ai-code-reviewer/internal/ollama"
)

// fakeOllamaServer serves a canned sequence of /api/chat responses, one per
// call, so retry behavior can be exercised deterministically.
func fakeOllamaServer(t *testing.T, responses []string) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		i := atomic.AddInt32(&calls, 1) - 1
		if int(i) >= len(responses) {
			t.Fatalf("unexpected extra call %d (have %d canned responses)", i, len(responses))
		}
		body, _ := json.Marshal(map[string]any{
			"message": map[string]string{"role": "assistant", "content": responses[i]},
			"done":    true,
		})
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func testReviewer(srv *httptest.Server) *Reviewer {
	client := ollama.NewClient(srv.URL, "test-model", ollama.FormatSchema, 5*time.Second)
	return NewReviewer(client)
}

func TestReviewFile_SucceedsFirstTry(t *testing.T) {
	srv, calls := fakeOllamaServer(t, []string{
		`{"findings": [{"file": "main.go", "line": 5, "category": "bug", "severity": "high", "message": "off by one"}]}`,
	})

	r := testReviewer(srv)
	findings, malformed, err := r.ReviewFile(context.Background(), ReviewInput{
		Path:            "main.go",
		Content:         "line1\nline2\nline3\nline4\nline5\n",
		Patch:           "@@ -1,5 +1,5 @@\n line1\n line2\n line3\n line4\n-old5\n+line5",
		ValidRightLines: map[int]bool{5: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if malformed {
		t.Fatalf("expected malformed=false")
	}
	if len(findings) != 1 || findings[0].Line != 5 {
		t.Fatalf("unexpected findings: %+v", findings)
	}
	if *calls != 1 {
		t.Errorf("expected exactly 1 call, got %d", *calls)
	}
}

func TestReviewFile_RetriesOnMalformedThenSucceeds(t *testing.T) {
	srv, calls := fakeOllamaServer(t, []string{
		"sorry, I cannot produce JSON right now",
		`{"findings": []}`,
	})

	r := testReviewer(srv)
	findings, malformed, err := r.ReviewFile(context.Background(), ReviewInput{
		Path:            "main.go",
		Content:         "a\nb\n",
		Patch:           "@@ -1,2 +1,2 @@\n a\n b",
		ValidRightLines: map[int]bool{1: true, 2: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if malformed {
		t.Fatalf("expected malformed=false after a successful retry")
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
	if *calls != 2 {
		t.Errorf("expected exactly 2 calls (1 retry), got %d", *calls)
	}
}

func TestReviewFile_ExhaustsRetriesOnPersistentMalformedOutput(t *testing.T) {
	srv, calls := fakeOllamaServer(t, []string{
		"nope", "still nope", "nope again",
	})

	r := testReviewer(srv)
	r.MaxRetries = 2 // 1 initial + 2 retries = 3 total attempts, matching the 3 canned responses

	findings, malformed, err := r.ReviewFile(context.Background(), ReviewInput{
		Path:            "main.go",
		Content:         "a\n",
		Patch:           "@@ -1,1 +1,1 @@\n a",
		ValidRightLines: map[int]bool{1: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !malformed {
		t.Fatalf("expected malformed=true after exhausting retries")
	}
	if findings != nil {
		t.Fatalf("expected nil findings, got %+v", findings)
	}
	if *calls != 3 {
		t.Errorf("expected exactly 3 calls, got %d", *calls)
	}
}

func TestReviewFile_DropsFindingsOnInvalidLines(t *testing.T) {
	srv, _ := fakeOllamaServer(t, []string{
		`{"findings": [
			{"file": "main.go", "line": 1, "category": "bug", "severity": "high", "message": "valid"},
			{"file": "main.go", "line": 999, "category": "bug", "severity": "high", "message": "invalid line"}
		]}`,
	})

	r := testReviewer(srv)
	findings, _, err := r.ReviewFile(context.Background(), ReviewInput{
		Path:            "main.go",
		Content:         "a\n",
		Patch:           "@@ -1,1 +1,1 @@\n a",
		ValidRightLines: map[int]bool{1: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Line != 1 {
		t.Fatalf("expected only the line-1 finding to survive, got %+v", findings)
	}
}

func TestReviewFile_SkipsCallWhenNoCommentableLines(t *testing.T) {
	srv, calls := fakeOllamaServer(t, nil)

	r := testReviewer(srv)
	findings, malformed, err := r.ReviewFile(context.Background(), ReviewInput{
		Path:            "renamed.go",
		Content:         "a\n",
		Patch:           "",
		ValidRightLines: map[int]bool{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if malformed || findings != nil {
		t.Fatalf("expected no-op result, got findings=%+v malformed=%v", findings, malformed)
	}
	if *calls != 0 {
		t.Errorf("expected no HTTP call when there are no commentable lines, got %d", *calls)
	}
}

func TestReviewFile_RequestLevelErrorIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer srv.Close()

	r := testReviewer(srv)
	_, _, err := r.ReviewFile(context.Background(), ReviewInput{
		Path:            "main.go",
		Content:         "a\n",
		Patch:           "@@ -1,1 +1,1 @@\n a",
		ValidRightLines: map[int]bool{1: true},
	})
	if err == nil {
		t.Fatalf("expected a request-level error")
	}
}
