package githubapi

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "diffs", name))
	if err != nil {
		t.Fatalf("loadFixture(%s): %v", name, err)
	}
	return string(data)
}

func TestParsePatch_MultiHunk(t *testing.T) {
	patch := loadFixture(t, "multi_hunk.patch")

	fd, err := ParsePatch("main.go", patch)
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	if !fd.HasHunks() {
		t.Fatalf("expected hunks, got none")
	}
	if len(fd.Hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(fd.Hunks))
	}

	h1, h2 := fd.Hunks[0], fd.Hunks[1]
	if h1.OldStart != 1 || h1.OldLines != 4 || h1.NewStart != 1 || h1.NewLines != 5 {
		t.Errorf("hunk1 header mismatch: %+v", h1)
	}
	if h2.OldStart != 5 || h2.OldLines != 5 || h2.NewStart != 6 || h2.NewLines != 6 {
		t.Errorf("hunk2 header mismatch: %+v", h2)
	}

	valid := fd.ValidRightLines()

	// Added line "import "os"" lands at new line 4.
	if !valid[4] {
		t.Errorf("expected line 4 (added import) to be commentable")
	}
	// Context blank line after it lands at new line 5.
	if !valid[5] {
		t.Errorf("expected line 5 (blank context) to be commentable")
	}
	// The two added lines in hunk 2 (z := x + y / fmt.Println(z)) are new lines 9 and 10.
	if !valid[9] || !valid[10] {
		t.Errorf("expected lines 9 and 10 (added) to be commentable, got %v", valid)
	}
	// Removed lines carry no new-file line number, so they never appear
	// as a key at all (only as a fact about the *old* side, which
	// ValidRightLines doesn't expose).
	for _, h := range fd.Hunks {
		for _, l := range h.Lines {
			if l.Kind == LineRemoved && l.NewLine != 0 {
				t.Errorf("removed line unexpectedly has a new-file line number: %+v", l)
			}
		}
	}
}

func TestParsePatch_AddOnly(t *testing.T) {
	patch := loadFixture(t, "add_only.patch")
	fd, err := ParsePatch("main.go", patch)
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	valid := fd.ValidRightLines()
	for _, line := range []int{1, 2, 3, 4} {
		if !valid[line] {
			t.Errorf("expected line %d to be commentable, got %v", line, valid)
		}
	}
}

func TestParsePatch_DeleteOnly(t *testing.T) {
	patch := loadFixture(t, "delete_only.patch")
	fd, err := ParsePatch("main.go", patch)
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	valid := fd.ValidRightLines()
	if len(valid) != 2 {
		t.Fatalf("expected only the 2 surviving context lines to be commentable, got %v", valid)
	}
	if !valid[1] || !valid[2] {
		t.Errorf("expected lines 1 and 2 to be commentable, got %v", valid)
	}
}

func TestParsePatch_Empty(t *testing.T) {
	// Renames with no content change, or diffs GitHub declines to compute,
	// come back with an empty Patch field.
	fd, err := ParsePatch("renamed.go", "")
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	if fd.HasHunks() {
		t.Errorf("expected no hunks for empty patch")
	}
	if len(fd.ValidRightLines()) != 0 {
		t.Errorf("expected no commentable lines for empty patch")
	}
}

func TestParsePatch_MalformedHunkHeader(t *testing.T) {
	_, err := ParsePatch("bad.go", "@@ not a real header @@\n foo\n")
	if err == nil {
		t.Fatalf("expected error for malformed hunk header")
	}
}

func TestParsePatch_BlankContextLineTrimmed(t *testing.T) {
	// Regression test: some tools trim trailing whitespace from blank
	// context lines, so they appear as a bare "" rather than " " mid-hunk.
	// That must still be treated as a context line advancing both counters,
	// not skipped as a Split artifact.
	patch := "@@ -1,3 +1,3 @@\n a\n\n b\n"
	fd, err := ParsePatch("f.go", patch)
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	valid := fd.ValidRightLines()
	if !valid[1] || !valid[2] || !valid[3] {
		t.Errorf("expected lines 1-3 all commentable, got %v", valid)
	}
}
