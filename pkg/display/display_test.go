package display

import "testing"

func TestDetectBackend(t *testing.T) {
	cases := []struct {
		name     string
		setEnv   func(t *testing.T)
		expected string
	}{
		{
			name: "XDG_SESSION_TYPE=x11 wins over DISPLAY",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_SESSION_TYPE", "x11")
				t.Setenv("DISPLAY", ":0")
			},
			expected: BackendX11,
		},
		{
			name: "XDG_SESSION_TYPE=wayland wins over DISPLAY",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_SESSION_TYPE", "wayland")
				t.Setenv("DISPLAY", ":0")
			},
			expected: BackendWayland,
		},
		{
			name: "case-insensitive XDG_SESSION_TYPE",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_SESSION_TYPE", "X11")
			},
			expected: BackendX11,
		},
		{
			name: "lowercase wayland",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_SESSION_TYPE", "wayland")
			},
			expected: BackendWayland,
		},
		{
			name: "empty XDG falls back to WAYLAND_DISPLAY",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_SESSION_TYPE", "")
				t.Setenv("WAYLAND_DISPLAY", "wayland-0")
			},
			expected: BackendWayland,
		},
		{
			name: "empty XDG falls back to DISPLAY (X11)",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_SESSION_TYPE", "")
				t.Setenv("WAYLAND_DISPLAY", "")
				t.Setenv("DISPLAY", ":0")
			},
			expected: BackendX11,
		},
		{
			name: "WAYLAND_DISPLAY takes priority over DISPLAY",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_SESSION_TYPE", "")
				t.Setenv("WAYLAND_DISPLAY", "wayland-0")
				t.Setenv("DISPLAY", ":0")
			},
			expected: BackendWayland,
		},
		{
			name: "no env vars → unknown",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_SESSION_TYPE", "")
				t.Setenv("WAYLAND_DISPLAY", "")
				t.Setenv("DISPLAY", "")
			},
			expected: BackendUnknown,
		},
		{
			name: "unknown XDG_SESSION_TYPE value",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_SESSION_TYPE", "tty") // not x11, not wayland
			},
			expected: BackendUnknown,
		},
		{
			name: "XDG_SESSION_TYPE with whitespace",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_SESSION_TYPE", "  x11  ")
			},
			expected: BackendX11,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Clear all relevant env vars at the start of every subtest
			// so we get a clean slate. t.Setenv handles restore.
			t.Setenv("XDG_SESSION_TYPE", "")
			t.Setenv("WAYLAND_DISPLAY", "")
			t.Setenv("DISPLAY", "")
			c.setEnv(t)

			got := DetectBackend()
			if got != c.expected {
				t.Errorf("DetectBackend() = %q, want %q", got, c.expected)
			}
		})
	}
}

func TestBackendConstants(t *testing.T) {
	// Catch accidental renames of the public backend strings.
	if BackendX11 != "x11" {
		t.Errorf("BackendX11 = %q, want \"x11\"", BackendX11)
	}
	if BackendWayland != "wayland" {
		t.Errorf("BackendWayland = %q, want \"wayland\"", BackendWayland)
	}
	if BackendUnknown != "unknown" {
		t.Errorf("BackendUnknown = %q, want \"unknown\"", BackendUnknown)
	}
}
