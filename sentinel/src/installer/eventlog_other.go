//go:build !windows

package installer

import "fmt"

// EventLogWriter is a no-op on non-Windows platforms.
type EventLogWriter struct{}

func OpenEventLog() (*EventLogWriter, error) {
	return nil, fmt.Errorf("event log is only supported on Windows")
}

func (w *EventLogWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *EventLogWriter) Close() error                { return nil }
