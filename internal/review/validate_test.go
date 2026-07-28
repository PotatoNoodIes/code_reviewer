package review

import "testing"

func TestValidateFindings(t *testing.T) {
	validLines := map[int]bool{5: true, 6: true}

	findings := []Finding{
		{File: "wrong.go", Line: 5, Category: CategoryBug, Severity: SeverityHigh, Message: "ok, file gets forced"},
		{File: "main.go", Line: 99, Category: CategoryBug, Severity: SeverityHigh, Message: "line not in diff"},
		{File: "main.go", Line: 6, Category: Category("made-up"), Severity: SeverityHigh, Message: "bad category"},
		{File: "main.go", Line: 6, Category: CategorySecurity, Severity: Severity("critical"), Message: "bad severity"},
		{File: "main.go", Line: 6, Category: CategorySecurity, Severity: SeverityMedium, Message: "valid finding"},
	}

	got := ValidateFindings(findings, "main.go", validLines)

	if len(got) != 2 {
		t.Fatalf("expected 2 surviving findings, got %d: %+v", len(got), got)
	}
	if got[0].File != "main.go" || got[0].Line != 5 {
		t.Errorf("expected first finding's File forced to main.go, got %+v", got[0])
	}
	if got[1].Message != "valid finding" {
		t.Errorf("expected second surviving finding to be the fully valid one, got %+v", got[1])
	}
}
