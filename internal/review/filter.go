package review

import (
	"path/filepath"
	"sort"
	"strings"
)

// ChangedFile is the subset of a GitHub PR file entry the selection logic
// needs; internal/githubapi maps its API type into this one so this
// package has no dependency on go-github.
type ChangedFile struct {
	Path    string
	Changes int // additions + deletions, used to prioritize when capping
}

// Selection is the result of applying ignore patterns and the max-files cap
// to a PR's changed files.
type Selection struct {
	Files          []ChangedFile
	SkippedIgnored []string
	SkippedCapped  []string
}

// SelectFilesToReview filters out files matching ignorePatterns, then caps
// the remainder to maxFiles (largest diffs first), reporting what was
// dropped and why rather than silently truncating.
func SelectFilesToReview(files []ChangedFile, ignorePatterns []string, maxFiles int) Selection {
	var sel Selection

	var kept []ChangedFile
	for _, f := range files {
		if MatchesIgnorePattern(f.Path, ignorePatterns) {
			sel.SkippedIgnored = append(sel.SkippedIgnored, f.Path)
			continue
		}
		kept = append(kept, f)
	}

	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Changes > kept[j].Changes })

	if maxFiles > 0 && len(kept) > maxFiles {
		for _, f := range kept[maxFiles:] {
			sel.SkippedCapped = append(sel.SkippedCapped, f.Path)
		}
		kept = kept[:maxFiles]
	}

	sel.Files = kept
	return sel
}

// MatchesIgnorePattern reports whether path matches any of the given
// patterns. Patterns containing "/" support "**" (match zero or more path
// segments) in addition to filepath.Match's single-segment "*"/"?"/"[...]".
// Patterns with no "/" match against the path's basename at any depth,
// mirroring .gitignore convention (e.g. "go.sum" matches "sub/dir/go.sum").
func MatchesIgnorePattern(path string, patterns []string) bool {
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if !strings.Contains(p, "/") {
			if ok, _ := filepath.Match(p, filepath.Base(path)); ok {
				return true
			}
			continue
		}
		if matchGlob(p, path) {
			return true
		}
	}
	return false
}

func matchGlob(pattern, path string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(path, "/"))
}

func matchSegments(pat, name []string) bool {
	if len(pat) == 0 {
		return len(name) == 0
	}
	if pat[0] == "**" {
		if matchSegments(pat[1:], name) {
			return true
		}
		if len(name) == 0 {
			return false
		}
		return matchSegments(pat, name[1:])
	}
	if len(name) == 0 {
		return false
	}
	if ok, err := filepath.Match(pat[0], name[0]); err != nil || !ok {
		return false
	}
	return matchSegments(pat[1:], name[1:])
}
