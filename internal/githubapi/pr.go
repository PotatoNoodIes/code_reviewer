package githubapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/go-github/v66/github"
)

// ChangedFile is one file changed in a pull request, with everything
// needed to review it: the diff patch (for line-mapping) and the blob SHA
// to fetch full content at the head commit.
type ChangedFile struct {
	Path    string
	Status  string // added, removed, modified, renamed, ...
	Changes int    // additions + deletions
	Patch   string // unified diff hunk text; empty for renames-with-no-change or oversized diffs
	BlobSHA string // new-file blob SHA; empty for removed files
}

// ListChangedFiles returns every file changed in the pull request,
// paginating through GitHub's results.
func (c *Client) ListChangedFiles(ctx context.Context, owner, repo string, prNumber int) ([]ChangedFile, error) {
	var all []ChangedFile
	opts := &github.ListOptions{PerPage: 100}

	for {
		var page []*github.CommitFile
		var resp *github.Response

		err := c.withRetry(ctx, func() (*github.Response, error) {
			var e error
			page, resp, e = c.gh.PullRequests.ListFiles(ctx, owner, repo, prNumber, opts)
			return resp, e
		})
		if err != nil {
			return nil, fmt.Errorf("githubapi: list PR files: %w", err)
		}

		for _, f := range page {
			all = append(all, ChangedFile{
				Path:    f.GetFilename(),
				Status:  f.GetStatus(),
				Changes: f.GetChanges(),
				Patch:   f.GetPatch(),
				BlobSHA: f.GetSHA(),
			})
		}

		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return all, nil
}

// GetPRHeadSHA fetches a pull request's head commit SHA directly — used
// when the CLI is pointed at a PR manually (--owner/--repo/--pr) rather
// than driven by a GitHub Actions pull_request event payload.
func (c *Client) GetPRHeadSHA(ctx context.Context, owner, repo string, prNumber int) (string, error) {
	var pr *github.PullRequest
	err := c.withRetry(ctx, func() (*github.Response, error) {
		var e error
		var resp *github.Response
		pr, resp, e = c.gh.PullRequests.Get(ctx, owner, repo, prNumber)
		return resp, e
	})
	if err != nil {
		return "", fmt.Errorf("githubapi: get PR #%d: %w", prNumber, err)
	}
	sha := pr.GetHead().GetSHA()
	if sha == "" {
		return "", fmt.Errorf("githubapi: PR #%d has no head SHA", prNumber)
	}
	return sha, nil
}

// maxBlobBytes is a sanity cap well above any real source file; blobs
// larger than this are skipped rather than reviewed.
const maxBlobBytes = 5 * 1024 * 1024

// GetFileContent fetches the full content of a file at the given blob SHA
// via the Git Blob API (chosen over the Contents API to avoid its 1MB cap
// and an extra path+ref round trip — the blob SHA is already known from
// ListChangedFiles). Returns ("", nil) for an empty blobSHA, since removed
// files have no blob on the new side.
func (c *Client) GetFileContent(ctx context.Context, owner, repo, blobSHA string) (string, error) {
	if blobSHA == "" {
		return "", nil
	}

	var blob *github.Blob
	err := c.withRetry(ctx, func() (*github.Response, error) {
		var e error
		var resp *github.Response
		blob, resp, e = c.gh.Git.GetBlob(ctx, owner, repo, blobSHA)
		return resp, e
	})
	if err != nil {
		return "", fmt.Errorf("githubapi: get blob %s: %w", blobSHA, err)
	}

	if blob.GetSize() > maxBlobBytes {
		return "", fmt.Errorf("githubapi: blob %s is %d bytes, exceeds %d byte cap", blobSHA, blob.GetSize(), maxBlobBytes)
	}
	if enc := blob.GetEncoding(); enc != "base64" {
		return "", fmt.Errorf("githubapi: blob %s has unsupported encoding %q", blobSHA, enc)
	}

	// GitHub wraps the base64 content with embedded newlines, which the
	// standard decoder rejects; strip them before decoding.
	compact := strings.ReplaceAll(blob.GetContent(), "\n", "")
	decoded, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		return "", fmt.Errorf("githubapi: decode blob %s: %w", blobSHA, err)
	}

	return string(decoded), nil
}
