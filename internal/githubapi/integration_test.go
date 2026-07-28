//go:build integration

// Run with: go test -tags integration ./internal/githubapi/... -run TestIntegration -v
//
// These hit the real, unauthenticated GitHub API to catch go-github API
// drift (field renames, pagination changes) without needing a
// write-scoped token in normal CI. Kept out of the default test run
// because they depend on network access and GitHub's public rate limit.
package githubapi

import (
	"context"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestIntegration_ListChangedFilesAndGetFileContent(t *testing.T) {
	client := &Client{gh: github.NewClient(nil)}
	ctx := context.Background()

	// octocat/Hello-World#1 is the oldest pull request on GitHub's own
	// canonical demo repo — about as stable a fixture as exists.
	files, err := client.ListChangedFiles(ctx, "octocat", "Hello-World", 1)
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("expected at least one changed file")
	}

	f := files[0]
	t.Logf("file: %s status=%s changes=%d patch_len=%d", f.Path, f.Status, f.Changes, len(f.Patch))

	fd, err := ParsePatch(f.Path, f.Patch)
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	t.Logf("commentable lines: %v", fd.ValidRightLines())

	if f.BlobSHA != "" {
		content, err := client.GetFileContent(ctx, "octocat", "Hello-World", f.BlobSHA)
		if err != nil {
			t.Fatalf("GetFileContent: %v", err)
		}
		t.Logf("content length: %d bytes", len(content))
	}
}
