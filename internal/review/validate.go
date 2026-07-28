package review

// ValidateFindings drops any finding that targets a line outside
// validRightLines or uses a category/severity outside the allowed enums —
// the model's own claim about a finding's validity is never trusted,
// everything is re-checked against ground truth here. Since the model is
// only ever asked about one file at a time, File is not used to filter
// (a smaller model may not echo the exact path back); instead every
// surviving finding's File is forced to the known ground-truth path.
func ValidateFindings(findings []Finding, file string, validRightLines map[int]bool) []Finding {
	var out []Finding
	for _, f := range findings {
		if !validRightLines[f.Line] {
			continue
		}
		if !ValidCategories[f.Category] {
			continue
		}
		if !ValidSeverities[f.Severity] {
			continue
		}
		f.File = file
		out = append(out, f)
	}
	return out
}
