package review

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const systemPromptText = `You are a senior software engineer performing a thorough code review of a single file in a pull request.

Review it across exactly these four categories:
- bug: logic/correctness errors — things that will misbehave, crash, or produce wrong results.
- security: vulnerabilities — injection, hardcoded secrets, unsafe deserialization, missing auth/validation, etc.
- simplification: readability, duplication, or reuse opportunities — code that works but could be clearer or shorter.
- style: naming, formatting, and idiom issues.

Severity is one of: low, medium, high.

Hard rules:
1. Only report a finding on a line number listed in "Commentable lines" below. Never invent a line number.
2. Every finding's "line" must be an integer from that exact list.
3. If the file has no real issues, return {"findings": []}. Do not invent minor nits just to produce output.
4. Respond with ONLY a JSON object matching the required schema — no prose, no markdown code fences.`

// SystemPrompt returns the fixed senior-dev-persona system prompt.
func SystemPrompt() string {
	return systemPromptText
}

// BuildUserPrompt combines the file's full (line-numbered) content, the
// set of lines a comment may legally target, and the raw diff hunk text
// into the per-file review request.
func BuildUserPrompt(path, numberedContent string, commentableLines []int, diffText string) string {
	var ranges strings.Builder
	writeLineRanges(&ranges, commentableLines)

	if strings.TrimSpace(diffText) == "" {
		diffText = "(no diff available for this file)"
	}

	return fmt.Sprintf(`File: %s

Commentable lines: %s

Full file (line-numbered):
%s

Diff (what changed in this file):
%s

Respond with ONLY JSON matching the schema.`, path, ranges.String(), numberedContent, diffText)
}

// NumberLines renders content with right-aligned "N: " line prefixes,
// 1-indexed, matching the new-file line numbers used everywhere else so the
// model's line references line up with real GitHub diff positions. A
// trailing newline in content is not counted as an extra blank line.
func NumberLines(content string) string {
	trimmed := strings.TrimSuffix(content, "\n")
	lines := strings.Split(trimmed, "\n")
	width := len(strconv.Itoa(len(lines)))

	var b strings.Builder
	for i, l := range lines {
		fmt.Fprintf(&b, "%*d: %s\n", width, i+1, l)
	}
	return b.String()
}

// writeLineRanges renders a slice of line numbers (assumed already unique,
// as when derived from a map's keys) as compact comma-separated ranges,
// e.g. "3, 5-8, 12".
func writeLineRanges(b *strings.Builder, lines []int) {
	if len(lines) == 0 {
		b.WriteString("(none)")
		return
	}

	sorted := append([]int(nil), lines...)
	sort.Ints(sorted)

	first := true
	flush := func(start, end int) {
		if !first {
			b.WriteString(", ")
		}
		first = false
		if start == end {
			fmt.Fprintf(b, "%d", start)
		} else {
			fmt.Fprintf(b, "%d-%d", start, end)
		}
	}

	start, prev := sorted[0], sorted[0]
	for _, n := range sorted[1:] {
		if n == prev+1 {
			prev = n
			continue
		}
		flush(start, prev)
		start, prev = n, n
	}
	flush(start, prev)
}
