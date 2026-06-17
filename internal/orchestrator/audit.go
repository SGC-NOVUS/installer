package orchestrator

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

const defaultInstallAuditLogPath = "/etc/novus/installer-audit.log"

// InstallAuditEvent is a single structured record of the installer lifecycle.
// It is serialized as one JSON object per line so the trail can be ingested by
// log shippers and replayed during incident review.
type InstallAuditEvent struct {
	Time       string `json:"time"`
	Event      string `json:"event"`
	Step       string `json:"step,omitempty"`
	Index      int    `json:"index,omitempty"`
	Total      int    `json:"total,omitempty"`
	Outcome    string `json:"outcome,omitempty"`
	Reason     string `json:"reason,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

// InstallAuditLogger writes installer lifecycle events as JSON lines. A disabled
// logger is a no-op so callers never have to branch on configuration.
type InstallAuditLogger struct {
	mu      sync.Mutex
	w       io.Writer
	enabled bool
	now     func() time.Time
}

// NewInstallAuditLogger builds a logger writing to w. A nil writer falls back to
// os.Stderr so audit records are never silently dropped.
func NewInstallAuditLogger(w io.Writer, enabled bool) *InstallAuditLogger {
	if w == nil {
		w = os.Stderr
	}
	return &InstallAuditLogger{
		w:       w,
		enabled: enabled,
		now:     time.Now,
	}
}

// Log serializes ev as a JSON line. Missing timestamps are populated with the
// logger clock. Disabled loggers ignore the call.
func (l *InstallAuditLogger) Log(ev InstallAuditEvent) {
	if l == nil || !l.enabled {
		return
	}
	if ev.Time == "" {
		ev.Time = l.now().UTC().Format(time.RFC3339Nano)
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.w.Write(append(line, '\n'))
}

// resolveInstallAuditWriter opens the audit log file in append mode with locked
// down permissions. When the file cannot be opened (for example during tests or
// on a read-only path) it falls back to os.Stderr so installs are never blocked
// by audit-logging failures.
func resolveInstallAuditWriter(path string) io.Writer {
	if path == "" {
		return os.Stderr
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return os.Stderr
	}
	return file
}
