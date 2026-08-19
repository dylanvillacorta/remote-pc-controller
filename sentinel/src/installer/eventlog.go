//go:build windows

package installer

import (
	"io"
	"strings"

	"golang.org/x/sys/windows/svc/eventlog"
)

// EventLogWriter wraps the Windows Event Log as an io.Writer so it can be used
// as the output destination for log.Logger when Sentinel runs as a service.
// Each Write call is logged as an informational event.
type EventLogWriter struct {
	elog *eventlog.Log
}

// OpenEventLog opens the Windows Event Log for the Sentinel service.
// The returned writer must be closed when no longer needed.
func OpenEventLog() (*EventLogWriter, error) {
	elog, err := eventlog.Open(ServiceName)
	if err != nil {
		return nil, err
	}
	return &EventLogWriter{elog: elog}, nil
}

func (w *EventLogWriter) Write(p []byte) (int, error) {
	// eventlog.Info expects a message ID (we use 1 as a generic info event)
	// and the string message. We strip any trailing newline that log.Logger adds.
	msg := strings.TrimRight(string(p), "\r\n")
	if msg == "" {
		return len(p), nil
	}
	return len(p), w.elog.Info(1, msg)
}

func (w *EventLogWriter) Close() error {
	if w.elog != nil {
		return w.elog.Close()
	}
	return nil
}

// Ensure EventLogWriter implements io.WriteCloser.
var _ io.WriteCloser = (*EventLogWriter)(nil)
