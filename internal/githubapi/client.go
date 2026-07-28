package githubapi

import (
	"context"
	"errors"
	"time"

	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

// Client wraps go-github with a retry decorator for rate limiting and
// transient server errors.
type Client struct {
	gh *github.Client
}

// NewClient builds a Client authenticated with a personal/installation
// GitHub token.
func NewClient(ctx context.Context, token string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return &Client{gh: github.NewClient(tc)}
}

// CurrentLogin returns the authenticated user/bot's login, used to
// recognize the reviewer's own past comments for dedup.
func (c *Client) CurrentLogin(ctx context.Context) (string, error) {
	var login string
	err := c.withRetry(ctx, func() (*github.Response, error) {
		user, resp, err := c.gh.Users.Get(ctx, "")
		if user != nil {
			login = user.GetLogin()
		}
		return resp, err
	})
	return login, err
}

const maxRetryAttempts = 4

// withRetry runs fn, retrying on rate-limit errors (sleeping until the
// limit resets) and transient 5xx responses (exponential backoff). Any
// other error is returned immediately.
func (c *Client) withRetry(ctx context.Context, fn func() (*github.Response, error)) error {
	var lastErr error

	for attempt := 0; attempt < maxRetryAttempts; attempt++ {
		resp, err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		var rateErr *github.RateLimitError
		var abuseErr *github.AbuseRateLimitError

		switch {
		case errors.As(err, &rateErr):
			if !sleepUntil(ctx, rateErr.Rate.Reset.Time) {
				return ctx.Err()
			}
			continue
		case errors.As(err, &abuseErr):
			wait := abuseErr.GetRetryAfter()
			if wait <= 0 {
				wait = 30 * time.Second
			}
			if !sleepFor(ctx, wait) {
				return ctx.Err()
			}
			continue
		case resp != nil && resp.StatusCode >= 500:
			backoff := time.Duration(1<<attempt) * time.Second
			if !sleepFor(ctx, backoff) {
				return ctx.Err()
			}
			continue
		default:
			return err
		}
	}

	return lastErr
}

func sleepUntil(ctx context.Context, t time.Time) bool {
	return sleepFor(ctx, time.Until(t))
}

func sleepFor(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
