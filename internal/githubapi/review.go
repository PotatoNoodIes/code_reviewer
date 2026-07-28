package githubapi

import (
	"context"

	"github.com/google/go-github/v66/github"
)

// PostReview posts one PR review carrying inline comments for a single
// file. It's called once per changed file (never once for the whole PR):
// GitHub's CreateReview call is all-or-nothing, so scoping it to one file
// means a bad line-mapping there can't take down every other file's
// comments.
func (c *Client) PostReview(ctx context.Context, owner, repo string, prNumber int, commitSHA, event string, comments []*github.DraftReviewComment) error {
	if len(comments) == 0 {
		return nil
	}

	req := &github.PullRequestReviewRequest{
		CommitID: github.String(commitSHA),
		Event:    github.String(event),
		Comments: comments,
	}

	return c.withRetry(ctx, func() (*github.Response, error) {
		_, resp, err := c.gh.PullRequests.CreateReview(ctx, owner, repo, prNumber, req)
		return resp, err
	})
}

// NewDraftComment builds one single-line inline review comment on the
// "new file" (RIGHT) side of the diff, per go-github's "comfort fade"
// review-comment fields.
func NewDraftComment(path string, line int, body string) *github.DraftReviewComment {
	return &github.DraftReviewComment{
		Path: github.String(path),
		Line: github.Int(line),
		Side: github.String("RIGHT"),
		Body: github.String(body),
	}
}
