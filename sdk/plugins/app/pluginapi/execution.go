package pluginapi

import (
	"context"
	"encoding/json"
)

type Scope struct {
	DisabledTools []string `json:"disabled_tools,omitempty"`
	Accounts      []string `json:"accounts,omitempty"`
	Directories   []string `json:"directories,omitempty"`
}
type Request struct {
	TaskID    string         `json:"task_id"`
	PluginID  string         `json:"plugin_id"`
	Version   string         `json:"version"`
	ActionID  string         `json:"action_id"`
	Input     map[string]any `json:"input,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
	Scope     Scope          `json:"scope"`
	Confirmed bool           `json:"confirmed"`
	Event     *Event         `json:"event,omitempty"`
}
type Event struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	SchemaVersion int             `json:"schema_version"`
	Scope         Scope           `json:"scope"`
	OccurredAt    string          `json:"occurred_at"`
	Cause         string          `json:"cause,omitempty"`
	Data          json.RawMessage `json:"data"`
}
type Subscription struct {
	Type   string `json:"type"`
	Action string `json:"action"`
}
type Contribution struct {
	Point     string   `json:"point"`
	Label     string   `json:"label"`
	Action    string   `json:"action"`
	FileTypes []string `json:"file_types,omitempty"`
}

func ValidEventType(t string) bool {
	switch t {
	case "file.changed", "organize.completed", "audiobook.changed", "task.completed":
		return true
	}
	return false
}

type Progress struct {
	Done       int64           `json:"done"`
	Total      int64           `json:"total"`
	Percent    float64         `json:"percent"`
	Message    string          `json:"message,omitempty"`
	Checkpoint json.RawMessage `json:"checkpoint,omitempty"`
}
type requestKey struct{}
type progressKey struct{}

func WithRequest(ctx context.Context, r Request) context.Context {
	return context.WithValue(ctx, requestKey{}, r)
}
func CurrentRequest(ctx context.Context) (Request, bool) {
	r, ok := ctx.Value(requestKey{}).(Request)
	return r, ok
}
func WithProgress(ctx context.Context, f func(Progress) error) context.Context {
	return context.WithValue(ctx, progressKey{}, f)
}
func ReportProgress(ctx context.Context, p Progress) error {
	if f, ok := ctx.Value(progressKey{}).(func(Progress) error); ok {
		return f(p)
	}
	return nil
}
