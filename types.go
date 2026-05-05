package tracker

// Level represents a severity level for CaptureMessage.
type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Config holds the options passed to Init.
type Config struct {
	APIKey      string // required
	Environment string // default: "production"
	Release     string // optional git SHA or version tag
	Debug       bool   // log to stdout when true
}

// UserContext identifies the user associated with an error.
type UserContext struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// Breadcrumb is a single entry in the breadcrumb trail.
type Breadcrumb struct {
	Message   string         `json:"message"`
	Category  string         `json:"category,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp string         `json:"timestamp"` // RFC3339
}

// StackFrame is one frame in a stack trace.
type StackFrame struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"` // always 0 — Go stack traces omit column numbers
	Fn   string `json:"fn"`
}

// errorDetail is the inner error object sent in the payload.
type errorDetail struct {
	Message    string       `json:"message"`
	Type       string       `json:"type"`
	StackTrace []StackFrame `json:"stackTrace"`
}

type osInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type nodeContext struct {
	OS     osInfo         `json:"os"`
	Custom map[string]any `json:"custom"`
}

// ErrorPayload is the JSON body posted to the ingest endpoint.
// Must match the schema expected by tracker-api.
type ErrorPayload struct {
	ID          string       `json:"id"`
	APIKey      string       `json:"apiKey"`
	Timestamp   string       `json:"timestamp"`
	Environment string       `json:"environment"`
	Release     string       `json:"release,omitempty"`
	Source      string       `json:"source"` // always "backend"
	Error       errorDetail  `json:"error"`
	Context     nodeContext  `json:"context"`
	User        *UserContext `json:"user"`
	Breadcrumbs []Breadcrumb `json:"breadcrumbs"`
	SessionID   string       `json:"sessionId"`
}
