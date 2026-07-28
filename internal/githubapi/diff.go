package githubapi

import (
	"fmt"
	"regexp"
	"strings"
)

// LineKind classifies one line of a unified diff hunk.
type LineKind int

const (
	LineContext LineKind = iota
	LineAdded
	LineRemoved
)

// DiffLine is a single line inside a Hunk.
type DiffLine struct {
	OldLine int
	NewLine int
	Kind    LineKind
	Content string
}

// Hunk is one "@@ -a,b +c,d @@" block of a unified diff.
type Hunk struct {
	OldStart, OldLines int
	NewStart, NewLines int
	Lines              []DiffLine
}

// FileDiff is the parsed patch for a single file.
type FileDiff struct {
	Path  string
	Hunks []Hunk
}

var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// ParsePatch parses a GitHub-style unified diff patch (as returned in
// CommitFile.Patch) for a single file. An empty patch (e.g. for renames
// with no content change, or files GitHub declines to diff) yields a
// FileDiff with no hunks, which callers should treat as "not commentable".
func ParsePatch(path, patch string) (*FileDiff, error) {
	fd := &FileDiff{Path: path}
	if strings.TrimSpace(patch) == "" {
		return fd, nil
	}

	var cur *Hunk
	oldLine, newLine := 0, 0

	// strings.Split leaves a trailing "" element when patch ends in "\n";
	// drop only that one. A "" line found anywhere else is a genuine blank
	// context/added line whose trailing space was trimmed by the tool that
	// produced the patch (a common real-world occurrence), not an artifact,
	// so it must still advance the line counters below.
	rawLines := strings.Split(patch, "\n")
	if n := len(rawLines); n > 0 && rawLines[n-1] == "" {
		rawLines = rawLines[:n-1]
	}

	for _, line := range rawLines {
		if strings.HasPrefix(line, "@@ ") {
			m := hunkHeaderRe.FindStringSubmatch(line)
			if m == nil {
				return nil, fmt.Errorf("%s: malformed hunk header: %q", path, line)
			}
			h := Hunk{
				OldStart: atoiDefault(m[1], 0),
				OldLines: atoiDefault(m[2], 1),
				NewStart: atoiDefault(m[3], 0),
				NewLines: atoiDefault(m[4], 1),
			}
			fd.Hunks = append(fd.Hunks, h)
			cur = &fd.Hunks[len(fd.Hunks)-1]
			oldLine = h.OldStart
			newLine = h.NewStart
			continue
		}
		if cur == nil {
			// Content before the first hunk header (e.g. "diff --git" style
			// preamble) shouldn't appear in GitHub's per-file Patch, but
			// skip defensively rather than erroring.
			continue
		}
		if line == `\ No newline at end of file` {
			continue
		}
		if line == "" {
			// Blank line with its prefix char trimmed by whatever produced
			// the patch; treat as an empty context line (see note above).
			cur.Lines = append(cur.Lines, DiffLine{OldLine: oldLine, NewLine: newLine, Kind: LineContext, Content: ""})
			oldLine++
			newLine++
			continue
		}

		switch line[0] {
		case '+':
			cur.Lines = append(cur.Lines, DiffLine{NewLine: newLine, Kind: LineAdded, Content: line[1:]})
			newLine++
		case '-':
			cur.Lines = append(cur.Lines, DiffLine{OldLine: oldLine, Kind: LineRemoved, Content: line[1:]})
			oldLine++
		case ' ':
			cur.Lines = append(cur.Lines, DiffLine{OldLine: oldLine, NewLine: newLine, Kind: LineContext, Content: line[1:]})
			oldLine++
			newLine++
		default:
			// Unexpected prefix; treat as context to stay conservative
			// about counter advancement rather than erroring the whole file.
			cur.Lines = append(cur.Lines, DiffLine{OldLine: oldLine, NewLine: newLine, Kind: LineContext, Content: line})
			oldLine++
			newLine++
		}
	}

	return fd, nil
}

// ValidRightLines returns the set of new-file (RIGHT-side) line numbers that
// a GitHub PR review comment may legally target: every added or context
// line that appears in the diff. Removed lines have no new-file line number
// and are excluded.
func (fd *FileDiff) ValidRightLines() map[int]bool {
	valid := make(map[int]bool)
	for _, h := range fd.Hunks {
		for _, l := range h.Lines {
			if l.Kind == LineAdded || l.Kind == LineContext {
				valid[l.NewLine] = true
			}
		}
	}
	return valid
}

// HasHunks reports whether the patch contained any parsable hunks. A false
// result means there is no valid-line set to check findings against, so the
// file should be skipped rather than reviewed.
func (fd *FileDiff) HasHunks() bool {
	return len(fd.Hunks) > 0
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	return n
}
