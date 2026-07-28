// Package review contains the LLM-driven code review pipeline: prompt
// construction, Ollama orchestration, and finding validation/filtering.
package review

// Category classifies the kind of issue a Finding represents.
type Category string

const (
	CategoryBug            Category = "bug"
	CategorySecurity       Category = "security"
	CategorySimplification Category = "simplification"
	CategoryStyle          Category = "style"
)

// ValidCategories lists every Category the reviewer accepts; findings with
// any other value are dropped rather than trusted.
var ValidCategories = map[Category]bool{
	CategoryBug:            true,
	CategorySecurity:       true,
	CategorySimplification: true,
	CategoryStyle:          true,
}

// Severity ranks how serious a Finding is.
type Severity string

const (
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

// ValidSeverities lists every Severity the reviewer accepts.
var ValidSeverities = map[Severity]bool{
	SeverityLow:    true,
	SeverityMedium: true,
	SeverityHigh:   true,
}

// Finding is a single review comment the LLM produced for one file.
type Finding struct {
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Category   Category `json:"category"`
	Severity   Severity `json:"severity"`
	Message    string   `json:"message"`
	Suggestion string   `json:"suggestion,omitempty"`
}

// FindingsResponse is the top-level JSON object the LLM must return for a
// single file review.
type FindingsResponse struct {
	Findings []Finding `json:"findings"`
}

// FindingsJSONSchema is passed as the Ollama "format" field to constrain
// decoding to this exact shape.
const FindingsJSONSchema = `{
  "type": "object",
  "properties": {
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "file": {"type": "string"},
          "line": {"type": "integer"},
          "category": {"type": "string", "enum": ["bug", "security", "simplification", "style"]},
          "severity": {"type": "string", "enum": ["low", "medium", "high"]},
          "message": {"type": "string"},
          "suggestion": {"type": "string"}
        },
        "required": ["file", "line", "category", "severity", "message"],
        "additionalProperties": false
      }
    }
  },
  "required": ["findings"],
  "additionalProperties": false
}`
