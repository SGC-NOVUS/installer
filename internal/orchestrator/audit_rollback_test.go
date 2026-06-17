package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

// recordingSink is a minimal OutputSink used to drive the Runner in tests.
type recordingSink struct {
	mu       sync.Mutex
	statuses []StatusMessage
	buf      bytes.Buffer
}

func (s *recordingSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *recordingSink) EmitStatus(m StatusMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses = append(s.statuses, m)
	return nil
}

func parseAuditLines(t *testing.T, raw []byte) []InstallAuditEvent {
	t.Helper()
	events := make([]InstallAuditEvent, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev InstallAuditEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("invalid audit json line %q: %v", line, err)
		}
		events = append(events, ev)
	}
	return events
}

func TestInstallAuditLoggerDisabledIsNoop(t *testing.T) {
	var buf bytes.Buffer
	logger := NewInstallAuditLogger(&buf, false)
	logger.Log(InstallAuditEvent{Event: "step_start", Step: "x"})
	if buf.Len() != 0 {
		t.Fatalf("expected no output from disabled logger, got %q", buf.String())
	}
}

func TestInstallAuditLoggerWritesJSONLine(t *testing.T) {
	var buf bytes.Buffer
	logger := NewInstallAuditLogger(&buf, true)
	logger.Log(InstallAuditEvent{Event: "step_success", Step: "deps", Index: 1, Total: 3, Outcome: "ok"})

	events := parseAuditLines(t, buf.Bytes())
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Event != "step_success" || ev.Step != "deps" || ev.Outcome != "ok" {
		t.Fatalf("unexpected event payload: %+v", ev)
	}
	if ev.Time == "" {
		t.Fatalf("expected auto-populated time, got empty")
	}
}

func TestInstallAuditLoggerNilLoggerIsSafe(t *testing.T) {
	var logger *InstallAuditLogger
	// Must not panic on a nil receiver.
	logger.Log(InstallAuditEvent{Event: "noop"})
}

func TestRollbackRunsCompletedStepsInReverseOrder(t *testing.T) {
	sink := &recordingSink{}
	var auditBuf bytes.Buffer
	runner := NewRunner(sink, RunnerOptions{DryRun: true, AuditWriter: &auditBuf})

	var order []string
	completed := []Step{
		{Name: "first", Rollback: func(context.Context, SetupRequest, *Runner) error {
			order = append(order, "first")
			return nil
		}},
		{Name: "no-rollback"},
		{Name: "second", Rollback: func(context.Context, SetupRequest, *Runner) error {
			order = append(order, "second")
			return nil
		}},
	}

	runner.rollback(context.Background(), SetupRequest{}, completed)

	if len(order) != 2 || order[0] != "second" || order[1] != "first" {
		t.Fatalf("expected reverse order [second first], got %v", order)
	}

	events := parseAuditLines(t, auditBuf.Bytes())
	var started, completedCount int
	for _, ev := range events {
		switch ev.Event {
		case "rollback_start":
			started++
		case "rollback_complete":
			completedCount++
		}
	}
	if started != 1 || completedCount != 1 {
		t.Fatalf("expected one rollback_start and one rollback_complete, got start=%d complete=%d", started, completedCount)
	}
}

func TestRollbackIsBestEffortAndContinuesAfterFailure(t *testing.T) {
	sink := &recordingSink{}
	var auditBuf bytes.Buffer
	runner := NewRunner(sink, RunnerOptions{DryRun: true, AuditWriter: &auditBuf})

	var ran []string
	completed := []Step{
		{Name: "alpha", Rollback: func(context.Context, SetupRequest, *Runner) error {
			ran = append(ran, "alpha")
			return nil
		}},
		{Name: "beta", Rollback: func(context.Context, SetupRequest, *Runner) error {
			ran = append(ran, "beta")
			return errors.New("boom")
		}},
	}

	runner.rollback(context.Background(), SetupRequest{}, completed)

	// beta fails first (reverse order) but alpha must still run.
	if len(ran) != 2 || ran[0] != "beta" || ran[1] != "alpha" {
		t.Fatalf("expected best-effort run [beta alpha], got %v", ran)
	}

	events := parseAuditLines(t, auditBuf.Bytes())
	var failed bool
	for _, ev := range events {
		if ev.Event == "rollback_step_failed" && ev.Step == "beta" {
			failed = true
		}
	}
	if !failed {
		t.Fatalf("expected rollback_step_failed audit for beta")
	}
}

func TestRollbackNoCompensatorsEmitsNothing(t *testing.T) {
	sink := &recordingSink{}
	var auditBuf bytes.Buffer
	runner := NewRunner(sink, RunnerOptions{DryRun: true, AuditWriter: &auditBuf})

	runner.rollback(context.Background(), SetupRequest{}, []Step{{Name: "plain"}})

	if auditBuf.Len() != 0 {
		t.Fatalf("expected no rollback audit when no compensators exist, got %q", auditBuf.String())
	}
}

func TestRemoveInstallerArtifactsDryRunNoop(t *testing.T) {
	sink := &recordingSink{}
	runner := NewRunner(sink, RunnerOptions{DryRun: true, AuditDisabled: true})
	// A non-existent path would normally be ignored; in dry-run we never touch
	// the filesystem at all, so this must succeed regardless.
	if err := runner.removeInstallerArtifacts("/nonexistent/path/should/not/matter"); err != nil {
		t.Fatalf("expected dry-run removal to be a no-op, got %v", err)
	}
}
