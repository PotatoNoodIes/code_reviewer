package githubapi

import (
	"fmt"
	"os"

	"github.com/google/go-github/v66/github"
)

// EventContext identifies which pull request triggered this run.
type EventContext struct {
	Owner    string
	Repo     string
	PRNumber int
	HeadSHA  string
}

// ParseEventFile reads the GitHub Actions event payload (GITHUB_EVENT_PATH)
// and extracts the pull request it refers to. Only the pull_request event
// is supported; this action is meant to run on `pull_request: [opened,
// synchronize]`.
func ParseEventFile(eventPath, eventName string) (*EventContext, error) {
	if eventName != "pull_request" {
		return nil, fmt.Errorf("githubapi: unsupported event %q (want pull_request)", eventName)
	}

	data, err := os.ReadFile(eventPath)
	if err != nil {
		return nil, fmt.Errorf("githubapi: read event file %s: %w", eventPath, err)
	}

	raw, err := github.ParseWebHook(eventName, data)
	if err != nil {
		return nil, fmt.Errorf("githubapi: parse webhook payload: %w", err)
	}

	prEvent, ok := raw.(*github.PullRequestEvent)
	if !ok {
		return nil, fmt.Errorf("githubapi: expected *github.PullRequestEvent, got %T", raw)
	}

	repo := prEvent.GetRepo()
	pr := prEvent.GetPullRequest()

	ec := &EventContext{
		Owner:    repo.GetOwner().GetLogin(),
		Repo:     repo.GetName(),
		PRNumber: prEvent.GetNumber(),
		HeadSHA:  pr.GetHead().GetSHA(),
	}
	if ec.Owner == "" || ec.Repo == "" || ec.PRNumber == 0 || ec.HeadSHA == "" {
		return nil, fmt.Errorf("githubapi: incomplete pull_request event payload: %+v", ec)
	}
	return ec, nil
}
