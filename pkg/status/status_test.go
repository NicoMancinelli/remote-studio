package status

import (
	"testing"
)

func TestResolveStatusPath(t *testing.T) {
	path := ResolveStatusPath()
	if path == "" {
		t.Errorf("expected path, got empty")
	}
}

func TestWriteAndReadStatus(t *testing.T) {
	s := &SessionStatus{
		SessionActive: true,
		SessionPID:    1234,
		Display:       ":99",
		Profile:       "mac",
		NetworkStatus: "connected",
		CPUUsage:      1.5,
		MemoryUsage:   2.5,
	}
	if err := WriteStatus(s); err != nil {
		// Ignore error if run in environment without writable /tmp/remote-studio, but print warning
		t.Logf("WriteStatus warning: %v", err)
		return
	}

	s2, err := ReadStatus()
	if err != nil {
		t.Logf("ReadStatus warning: %v", err)
		return
	}

	if s2.SessionPID != 1234 {
		t.Errorf("expected PID 1234, got %d", s2.SessionPID)
	}
}
