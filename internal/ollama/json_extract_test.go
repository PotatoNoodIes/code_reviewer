package ollama

import "testing"

func TestExtractJSONObject_Clean(t *testing.T) {
	in := `{"findings": []}`
	got, err := ExtractJSONObject(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestExtractJSONObject_CodeFenced(t *testing.T) {
	in := "```json\n{\"findings\": [{\"file\": \"a.go\", \"line\": 3}]}\n```"
	got, err := ExtractJSONObject(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"findings": [{"file": "a.go", "line": 3}]}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractJSONObject_CodeFencedNoLangTag(t *testing.T) {
	in := "```\n{\"findings\": []}\n```"
	got, err := ExtractJSONObject(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `{"findings": []}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractJSONObject_StrayProseAroundObject(t *testing.T) {
	in := "Sure, here is the review:\n{\"findings\": []}\nLet me know if you need anything else!"
	got, err := ExtractJSONObject(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `{"findings": []}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractJSONObject_NestedBraces(t *testing.T) {
	in := `{"findings": [{"file": "a.go", "message": "use {} instead of new(T)"}]}`
	got, err := ExtractJSONObject(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestExtractJSONObject_BraceInsideString(t *testing.T) {
	in := `{"findings": [{"message": "unbalanced } inside a string should not end parsing early"}]}`
	got, err := ExtractJSONObject(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestExtractJSONObject_EscapedQuoteInString(t *testing.T) {
	in := `{"findings": [{"message": "escaped \" quote then } inside a string"}]}`
	got, err := ExtractJSONObject(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestExtractJSONObject_NoObject(t *testing.T) {
	_, err := ExtractJSONObject("I found no issues in this file.")
	if err == nil {
		t.Fatalf("expected error when no JSON object is present")
	}
}

func TestExtractJSONObject_Unbalanced(t *testing.T) {
	_, err := ExtractJSONObject(`{"findings": [{"file": "a.go"}`)
	if err == nil {
		t.Fatalf("expected error for unbalanced object")
	}
}
