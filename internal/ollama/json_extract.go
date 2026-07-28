package ollama

import (
	"errors"
	"strings"
)

// ExtractJSONObject pulls the first balanced {...} object out of s, after
// stripping any markdown code-fence wrapping. Local/quantized models
// occasionally wrap otherwise-valid JSON in ```json fences or add stray
// prose even under format constraints; this recovers the object without
// needing a full JSON parser pass.
func ExtractJSONObject(s string) (string, error) {
	s = stripCodeFences(s)

	start := strings.IndexByte(s, '{')
	if start == -1 {
		return "", errors.New("ollama: no JSON object found in response")
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		c := s[i]

		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}

		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], nil
			}
		}
	}

	return "", errors.New("ollama: unbalanced JSON object in response")
}

// stripCodeFences removes a leading/trailing ``` or ```json fence, if the
// whole trimmed string is wrapped in one.
func stripCodeFences(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return s
	}
	t = strings.TrimPrefix(t, "```")
	if nl := strings.IndexByte(t, '\n'); nl != -1 {
		// Drop an optional language tag on the fence's opening line
		// (e.g. "json") rather than assuming its exact contents.
		t = t[nl+1:]
	}
	t = strings.TrimSuffix(strings.TrimSpace(t), "```")
	return t
}
