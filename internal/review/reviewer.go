package review

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jhenriquez/ai-code-reviewer/internal/ollama"
)

// ReviewInput is everything needed to review one changed file.
type ReviewInput struct {
	Path            string
	Content         string // full file content at head SHA; empty for removed files
	Patch           string // unified diff patch text; empty skips the LLM call entirely
	ValidRightLines map[int]bool
}

// Reviewer drives one file's review through Ollama: prompt -> chat ->
// extract -> validate.
type Reviewer struct {
	Client     *ollama.Client
	Schema     string // JSON schema passed to Ollama when using FormatSchema
	MaxRetries int    // retries on malformed model output, beyond the first attempt
}

// NewReviewer builds a Reviewer with the default findings schema and retry
// budget.
func NewReviewer(client *ollama.Client) *Reviewer {
	return &Reviewer{Client: client, Schema: FindingsJSONSchema, MaxRetries: 2}
}

// ReviewFile reviews a single file and returns validated findings.
//
// err is non-nil only for a request-level failure (network, timeout,
// non-2xx from Ollama) — the caller should treat that as this file's
// review having failed to run at all.
//
// malformedOutput is true when every attempt (1 + MaxRetries) produced
// output that couldn't be parsed as valid JSON; the caller should report
// this file as skipped rather than treat it as "no issues found".
func (r *Reviewer) ReviewFile(ctx context.Context, in ReviewInput) (findings []Finding, malformedOutput bool, err error) {
	if len(in.ValidRightLines) == 0 {
		// No commentable lines (empty/unparsed patch) means there's
		// nothing a finding could legally attach to; skip the LLM call.
		return nil, false, nil
	}

	lines := sortedKeys(in.ValidRightLines)
	basePrompt := BuildUserPrompt(in.Path, NumberLines(in.Content), lines, in.Patch)

	var lastParseErr error

	for attempt := 0; attempt <= r.MaxRetries; attempt++ {
		prompt := basePrompt
		if attempt > 0 {
			prompt += "\n\nYour previous response was not valid JSON matching the schema. Respond with ONLY the JSON object — no prose, no markdown code fences."
		}

		raw, chatErr := r.Client.Chat(ctx, SystemPrompt(), prompt, r.Schema)
		if chatErr != nil {
			return nil, false, fmt.Errorf("review %s: ollama request failed: %w", in.Path, chatErr)
		}

		obj, extractErr := ollama.ExtractJSONObject(raw)
		if extractErr != nil {
			lastParseErr = extractErr
			continue
		}

		var fr FindingsResponse
		if unmarshalErr := json.Unmarshal([]byte(obj), &fr); unmarshalErr != nil {
			lastParseErr = unmarshalErr
			continue
		}

		return ValidateFindings(fr.Findings, in.Path, in.ValidRightLines), false, nil
	}

	_ = lastParseErr // exhausted retries; caller is told via malformedOutput, not a hard error
	return nil, true, nil
}

func sortedKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
