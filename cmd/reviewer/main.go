// Command reviewer is the AI Code Reviewer entrypoint: it runs as a
// GitHub Actions container action (driven by GITHUB_EVENT_PATH) or, with
// --owner/--repo/--pr, as a local CLI against any PR for testing.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/go-github/v66/github"
	"github.com/jhenriquez/ai-code-reviewer/internal/config"
	"github.com/jhenriquez/ai-code-reviewer/internal/ghsummary"
	"github.com/jhenriquez/ai-code-reviewer/internal/githubapi"
	"github.com/jhenriquez/ai-code-reviewer/internal/ollama"
	"github.com/jhenriquez/ai-code-reviewer/internal/review"
)

func main() {
	owner := flag.String("owner", "", "repo owner; overrides GITHUB_EVENT_PATH for local/manual runs")
	repo := flag.String("repo", "", "repo name; overrides GITHUB_EVENT_PATH for local/manual runs")
	prNumber := flag.Int("pr", 0, "pull request number; overrides GITHUB_EVENT_PATH for local/manual runs")
	flag.Parse()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	gh := githubapi.NewClient(ctx, cfg.GitHubToken)

	target, err := resolveTarget(ctx, gh, *owner, *repo, *prNumber)
	if err != nil {
		log.Fatalf("resolve target PR: %v", err)
	}

	if err := run(ctx, gh, cfg, target); err != nil {
		log.Fatalf("%v", err)
	}
}

func resolveTarget(ctx context.Context, gh *githubapi.Client, owner, repo string, prNumber int) (githubapi.EventContext, error) {
	if owner != "" && repo != "" && prNumber != 0 {
		sha, err := gh.GetPRHeadSHA(ctx, owner, repo, prNumber)
		if err != nil {
			return githubapi.EventContext{}, err
		}
		return githubapi.EventContext{Owner: owner, Repo: repo, PRNumber: prNumber, HeadSHA: sha}, nil
	}

	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return githubapi.EventContext{}, fmt.Errorf("no --owner/--repo/--pr given and GITHUB_EVENT_PATH is not set")
	}
	ec, err := githubapi.ParseEventFile(eventPath, os.Getenv("GITHUB_EVENT_NAME"))
	if err != nil {
		return githubapi.EventContext{}, err
	}
	return *ec, nil
}

func run(ctx context.Context, gh *githubapi.Client, cfg *config.Config, target githubapi.EventContext) error {
	files, err := gh.ListChangedFiles(ctx, target.Owner, target.Repo, target.PRNumber)
	if err != nil {
		return fmt.Errorf("list changed files: %w", err)
	}

	toSelect := make([]review.ChangedFile, len(files))
	byPath := make(map[string]githubapi.ChangedFile, len(files))
	for i, f := range files {
		toSelect[i] = review.ChangedFile{Path: f.Path, Changes: f.Changes}
		byPath[f.Path] = f
	}
	sel := review.SelectFilesToReview(toSelect, cfg.IgnorePatterns, cfg.MaxFiles)

	report := &ghsummary.Report{
		SkippedIgnored: sel.SkippedIgnored,
		SkippedCapped:  sel.SkippedCapped,
		SkippedErrors:  map[string]error{},
	}
	defer report.Write()

	var login string
	var existingHashes map[string]bool
	if !cfg.DryRun {
		login, err = gh.CurrentLogin(ctx)
		if err != nil {
			return fmt.Errorf("get authenticated login: %w", err)
		}
		existingHashes, err = gh.ExistingFindingHashes(ctx, target.Owner, target.Repo, target.PRNumber, login)
		if err != nil {
			return fmt.Errorf("list existing PR comments: %w", err)
		}
	}

	ollamaClient := ollama.NewClient(cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaFormatMode, cfg.OllamaTimeout)
	reviewer := review.NewReviewer(ollamaClient)

	for _, sf := range sel.Files {
		cf := byPath[sf.Path]

		fd, err := githubapi.ParsePatch(cf.Path, cf.Patch)
		if err != nil {
			report.SkippedErrors[cf.Path] = err
			continue
		}
		if !fd.HasHunks() {
			report.SkippedNoDiff = append(report.SkippedNoDiff, cf.Path)
			continue
		}

		content, err := gh.GetFileContent(ctx, target.Owner, target.Repo, cf.BlobSHA)
		if err != nil {
			report.SkippedErrors[cf.Path] = err
			continue
		}

		findings, malformed, err := reviewer.ReviewFile(ctx, review.ReviewInput{
			Path:            cf.Path,
			Content:         content,
			Patch:           cf.Patch,
			ValidRightLines: fd.ValidRightLines(),
		})
		if err != nil {
			report.SkippedErrors[cf.Path] = err
			continue
		}
		if malformed {
			report.SkippedMalformed = append(report.SkippedMalformed, cf.Path)
			continue
		}

		report.FilesReviewed++

		if cfg.DryRun {
			printFindings(cf.Path, findings)
			continue
		}

		posted := postFindings(ctx, gh, target, cfg, findings, existingHashes, report)
		report.FindingsPosted += posted
	}

	return nil
}

func postFindings(ctx context.Context, gh *githubapi.Client, target githubapi.EventContext, cfg *config.Config, findings []review.Finding, existingHashes map[string]bool, report *ghsummary.Report) int {
	var comments []*github.DraftReviewComment
	for _, f := range findings {
		hash := githubapi.FindingHash(f.File, f.Line, string(f.Category), f.Message)
		if existingHashes[hash] {
			report.FindingsSkippedDuplicate++
			continue
		}
		body := githubapi.WithHashMarker(formatCommentBody(f), hash)
		comments = append(comments, githubapi.NewDraftComment(f.File, f.Line, body))
	}

	if len(comments) == 0 {
		return 0
	}

	if err := gh.PostReview(ctx, target.Owner, target.Repo, target.PRNumber, target.HeadSHA, cfg.ReviewEvent, comments); err != nil {
		if len(comments) > 0 {
			report.SkippedErrors[comments[0].GetPath()] = fmt.Errorf("post review: %w", err)
		}
		return 0
	}
	return len(comments)
}

func formatCommentBody(f review.Finding) string {
	body := fmt.Sprintf("**[%s/%s]** %s", f.Category, f.Severity, f.Message)
	if f.Suggestion != "" {
		body += fmt.Sprintf("\n\n```suggestion\n%s\n```", f.Suggestion)
	}
	return body
}

func printFindings(path string, findings []review.Finding) {
	if len(findings) == 0 {
		fmt.Printf("%s: no issues found\n", path)
		return
	}
	fmt.Printf("%s:\n", path)
	for _, f := range findings {
		fmt.Printf("  line %d [%s/%s] %s\n", f.Line, f.Category, f.Severity, f.Message)
		if f.Suggestion != "" {
			fmt.Printf("    suggestion: %s\n", f.Suggestion)
		}
	}
}
