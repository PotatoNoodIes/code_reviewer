package githubapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"

	"github.com/google/go-github/v66/github"
)

// FindingHash derives a stable identity for a finding so the same issue,
// re-reported after a follow-up push, is recognized as already-posted
// rather than duplicated.
func FindingHash(file string, line int, category, message string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s\x00%s", file, line, category, message)))
	return hex.EncodeToString(sum[:])[:16]
}

const markerPrefix = "<!-- ai-code-reviewer:hash="

var markerRe = regexp.MustCompile(`<!-- ai-code-reviewer:hash=([0-9a-f]+) -->`)

// WithHashMarker appends a hidden HTML-comment marker carrying hash to a
// comment body, so a later run can recognize this exact finding again.
func WithHashMarker(body, hash string) string {
	return body + "\n\n" + markerPrefix + hash + " -->"
}

// ExtractHashMarker returns the hash embedded by WithHashMarker, if any.
func ExtractHashMarker(body string) (string, bool) {
	m := markerRe.FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// ExistingFindingHashes returns the set of finding hashes already posted by
// login (the reviewer's own authenticated identity) on this pull request,
// across all of its review comments.
func (c *Client) ExistingFindingHashes(ctx context.Context, owner, repo string, prNumber int, login string) (map[string]bool, error) {
	hashes := make(map[string]bool)
	opts := &github.PullRequestListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}

	for {
		var page []*github.PullRequestComment
		var resp *github.Response

		err := c.withRetry(ctx, func() (*github.Response, error) {
			var e error
			page, resp, e = c.gh.PullRequests.ListComments(ctx, owner, repo, prNumber, opts)
			return resp, e
		})
		if err != nil {
			return nil, fmt.Errorf("githubapi: list PR comments: %w", err)
		}

		for _, comment := range page {
			if comment.GetUser().GetLogin() != login {
				continue
			}
			if hash, ok := ExtractHashMarker(comment.GetBody()); ok {
				hashes[hash] = true
			}
		}

		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return hashes, nil
}
