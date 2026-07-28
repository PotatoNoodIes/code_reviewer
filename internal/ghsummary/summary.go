// Package ghsummary writes a human-readable run report to GitHub Actions'
// step summary (GITHUB_STEP_SUMMARY) and emits ::warning:: workflow
// annotations for files that were skipped, so a run that posts nothing is
// never silently indistinguishable from "no issues found".
package ghsummary

import (
	"fmt"
	"io"
	"os"
)

// Report collects everything worth surfacing about one run.
type Report struct {
	FilesReviewed            int
	FindingsPosted           int
	FindingsSkippedDuplicate int
	SkippedIgnored           []string
	SkippedCapped            []string
	SkippedNoDiff            []string         // patch was empty/unparsable (rename, oversized diff, ...)
	SkippedMalformed         []string         // model output never parsed as valid JSON after retries
	SkippedErrors            map[string]error // per-file request-level failures
}

// Write renders the report to GITHUB_STEP_SUMMARY (if set) and prints
// ::warning:: annotations for every skipped file, so failures surface in
// the Actions UI even if nobody reads the step summary.
func (r *Report) Write() {
	if path := os.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err == nil {
			r.render(f)
			f.Close()
		}
	}
	r.emitWarnings(os.Stdout)
}

func (r *Report) render(w io.Writer) {
	fmt.Fprintf(w, "## AI Code Reviewer\n\n")
	fmt.Fprintf(w, "- Files reviewed: %d\n", r.FilesReviewed)
	fmt.Fprintf(w, "- Findings posted: %d\n", r.FindingsPosted)
	if r.FindingsSkippedDuplicate > 0 {
		fmt.Fprintf(w, "- Findings already posted on a prior push (skipped): %d\n", r.FindingsSkippedDuplicate)
	}
	writeList(w, "Ignored by pattern", r.SkippedIgnored)
	writeList(w, "Skipped (over max-files cap)", r.SkippedCapped)
	writeList(w, "Skipped (no reviewable diff)", r.SkippedNoDiff)
	writeList(w, "Skipped (model never returned valid JSON)", r.SkippedMalformed)
	if len(r.SkippedErrors) > 0 {
		fmt.Fprintf(w, "\n### Errors\n\n")
		for path, err := range r.SkippedErrors {
			fmt.Fprintf(w, "- `%s`: %s\n", path, err)
		}
	}
}

func writeList(w io.Writer, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "\n### %s (%d)\n\n", title, len(items))
	for _, item := range items {
		fmt.Fprintf(w, "- `%s`\n", item)
	}
}

func (r *Report) emitWarnings(w io.Writer) {
	for _, path := range r.SkippedMalformed {
		fmt.Fprintf(w, "::warning file=%s::AI Code Reviewer: model never returned valid JSON for this file after retries; skipped.\n", path)
	}
	for path, err := range r.SkippedErrors {
		fmt.Fprintf(w, "::warning file=%s::AI Code Reviewer: review failed: %s\n", path, err)
	}
}
