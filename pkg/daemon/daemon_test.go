package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestGetListenFDs(t *testing.T) {
	pid := strconv.Itoa(os.Getpid())

	cases := []struct {
		name      string
		listenPID string
		listenFDS string
		wantCount int
		wantOK    bool
	}{
		{
			name:      "both unset",
			listenPID: "",
			listenFDS: "",
			wantCount: 0,
			wantOK:    false,
		},
		{
			name:      "only LISTEN_PID set",
			listenPID: pid,
			listenFDS: "",
			wantCount: 0,
			wantOK:    false,
		},
		{
			name:      "only LISTEN_FDS set",
			listenPID: "",
			listenFDS: "2",
			wantCount: 0,
			wantOK:    false,
		},
		{
			name:      "PID mismatch (different process)",
			listenPID: "999999", // some other PID
			listenFDS: "2",
			wantCount: 0,
			wantOK:    false,
		},
		{
			name:      "non-numeric LISTEN_PID",
			listenPID: "not-a-number",
			listenFDS: "2",
			wantCount: 0,
			wantOK:    false,
		},
		{
			name:      "non-numeric LISTEN_FDS",
			listenPID: pid,
			listenFDS: "abc",
			wantCount: 0,
			wantOK:    false,
		},
		{
			name:      "LISTEN_FDS=0 is invalid",
			listenPID: pid,
			listenFDS: "0",
			wantCount: 0,
			wantOK:    false,
		},
		{
			name:      "LISTEN_FDS=-1 is invalid",
			listenPID: pid,
			listenFDS: "-1",
			wantCount: 0,
			wantOK:    false,
		},
		{
			name:      "happy path: 1 FD",
			listenPID: pid,
			listenFDS: "1",
			wantCount: 1,
			wantOK:    true,
		},
		{
			name:      "happy path: 2 FDs (WS + HTTP)",
			listenPID: pid,
			listenFDS: "2",
			wantCount: 2,
			wantOK:    true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Clean slate: clear both vars before each subtest
			t.Setenv("LISTEN_PID", "")
			t.Setenv("LISTEN_FDS", "")
			if c.listenPID != "" {
				t.Setenv("LISTEN_PID", c.listenPID)
			}
			if c.listenFDS != "" {
				t.Setenv("LISTEN_FDS", c.listenFDS)
			}

			count, ok := getListenFDs()
			if ok != c.wantOK {
				t.Errorf("ok = %v, want %v", ok, c.wantOK)
			}
			if count != c.wantCount {
				t.Errorf("count = %d, want %d", count, c.wantCount)
			}
		})
	}
}

func TestFindConfigDir(t *testing.T) {
	// Build a temporary tree:
	//
	//	<tmp>/
	//	├── config/             ← found here
	//	└── deep/
	//	    └── nested/
	//
	// When the working directory is <tmp>/deep/nested, FindConfigDir
	// should walk up and return <tmp>/config.
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	nested := filepath.Join(tmp, "deep", "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	// Save the real cwd and chdir into the test tree.
	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origCwd) })

	got := FindConfigDir()
	want, err := filepath.EvalSymlinks(configDir)
	if err != nil {
		t.Fatalf("eval configDir: %v", err)
	}
	gotEval, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("eval got: %v", err)
	}
	if gotEval != want {
		t.Errorf("FindConfigDir() = %q, want %q", gotEval, want)
	}
}

func TestFindConfigDir_FallsBackToExec(t *testing.T) {
	// When cwd has no parent with a config/ directory, FindConfigDir
	// falls back to checking next to the running executable. We don't
	// verify the exact fallback path (depends on how go test runs the
	// binary), only that the function returns *something* (non-empty)
	// without panicking.
	got := FindConfigDir()
	if got == "" {
		t.Error("FindConfigDir() returned empty string — fallback chain broken")
	}
}

func TestFindConfigDir_FallsBackToUsrShare(t *testing.T) {
	// Build a temp tree with NO config/ anywhere. Then run FindConfigDir
	// from a chdir that has no such ancestor. The function should
	// eventually return the /usr/share/remote-studio fallback OR
	// the executable-relative path. We accept either — the test just
	// verifies no panic and a non-empty return.
	tmp := t.TempDir()
	noConfig := filepath.Join(tmp, "noConfigAnywhere")
	if err := os.MkdirAll(noConfig, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	origCwd, _ := os.Getwd()
	_ = os.Chdir(noConfig)
	t.Cleanup(func() { _ = os.Chdir(origCwd) })

	got := FindConfigDir()
	if got == "" {
		t.Error("FindConfigDir() returned empty string")
	}
}
