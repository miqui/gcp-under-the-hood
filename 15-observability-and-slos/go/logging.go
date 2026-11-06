package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// JSONLogger writes one JSON object per line to stdout. Cloud Run's logging
// agent parses these into LogEntry.jsonPayload and promotes the special
// "logging.googleapis.com/*" keys to first-class LogEntry fields.
type JSONLogger struct {
	mu        sync.Mutex
	w         io.Writer
	projectID string
	now       func() time.Time
}

func NewJSONLogger(w io.Writer, projectID string) *JSONLogger {
	return &JSONLogger{w: w, projectID: projectID, now: time.Now}
}

// Log emits a structured entry. If ctx carries a valid span, the trace
// correlation fields are added so Logs Explorer links to Cloud Trace.
func (l *JSONLogger) Log(ctx context.Context, severity, msg string, fields map[string]any) {
	entry := map[string]any{}
	for k, v := range fields {
		entry[k] = v
	}
	entry["severity"] = severity
	entry["message"] = msg
	entry["time"] = l.now().UTC().Format(time.RFC3339Nano)
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		if l.projectID != "" {
			entry["logging.googleapis.com/trace"] = fmt.Sprintf("projects/%s/traces/%s", l.projectID, sc.TraceID())
		}
		entry["logging.googleapis.com/spanId"] = sc.SpanID().String()
		entry["logging.googleapis.com/trace_sampled"] = sc.IsSampled()
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.w.Write(append(b, '\n'))
}
