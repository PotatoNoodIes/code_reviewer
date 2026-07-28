package review

import (
	"reflect"
	"testing"
)

func TestMatchesIgnorePattern(t *testing.T) {
	cases := []struct {
		path     string
		patterns []string
		want     bool
	}{
		{"vendor/foo/bar.go", []string{"**/vendor/**"}, true},
		{"pkg/vendor/foo/bar.go", []string{"**/vendor/**"}, true},
		{"pkg/foo/bar.go", []string{"**/vendor/**"}, false},
		{"go.sum", []string{"go.sum"}, true},
		{"sub/dir/go.sum", []string{"go.sum"}, true},
		{"main.go", []string{"go.sum"}, false},
		{"assets/app.min.js", []string{"**/*.min.js"}, true},
		{"assets/app.js", []string{"**/*.min.js"}, false},
		{"a/b/c.lock", []string{"**/*.lock"}, true},
	}
	for _, c := range cases {
		got := MatchesIgnorePattern(c.path, c.patterns)
		if got != c.want {
			t.Errorf("MatchesIgnorePattern(%q, %v) = %v, want %v", c.path, c.patterns, got, c.want)
		}
	}
}

func TestSelectFilesToReview_IgnoresAndCaps(t *testing.T) {
	files := []ChangedFile{
		{Path: "vendor/dep.go", Changes: 1000},
		{Path: "small.go", Changes: 5},
		{Path: "big.go", Changes: 500},
		{Path: "medium.go", Changes: 50},
	}

	sel := SelectFilesToReview(files, []string{"**/vendor/**"}, 2)

	if len(sel.SkippedIgnored) != 1 || sel.SkippedIgnored[0] != "vendor/dep.go" {
		t.Errorf("SkippedIgnored = %v", sel.SkippedIgnored)
	}

	wantKept := []string{"big.go", "medium.go"}
	var gotKept []string
	for _, f := range sel.Files {
		gotKept = append(gotKept, f.Path)
	}
	if !reflect.DeepEqual(gotKept, wantKept) {
		t.Errorf("Files = %v, want %v (largest diffs first)", gotKept, wantKept)
	}

	if !reflect.DeepEqual(sel.SkippedCapped, []string{"small.go"}) {
		t.Errorf("SkippedCapped = %v", sel.SkippedCapped)
	}
}

func TestSelectFilesToReview_NoCapWhenMaxFilesZeroOrNegative(t *testing.T) {
	files := []ChangedFile{{Path: "a.go", Changes: 1}, {Path: "b.go", Changes: 2}}
	sel := SelectFilesToReview(files, nil, 0)
	if len(sel.Files) != 2 {
		t.Errorf("expected no capping when maxFiles<=0, got %d files", len(sel.Files))
	}
}
