// Package display provides a unified abstraction over X11 (xrandr) and
// Wayland (wlr-randr / gnome-randr) display management. It auto-detects the
// active session type and dispatches resolution, scale, and rotation commands
// to the correct backend.
package display

import (
	"fmt"
	"os"
	"strings"
)

// Backend identifies the display server protocol in use.
const (
	BackendX11     = "x11"
	BackendWayland = "wayland"
	BackendUnknown = "unknown"
)

// Output describes a single display output (monitor).
type Output struct {
	Name              string // e.g. "HDMI-1", "eDP-1", "HEADLESS-1"
	Connected         bool
	CurrentResolution string // e.g. "1920x1080" or "" if not active
}

// DetectBackend returns the display-server backend for the running session.
// It checks, in order:
//  1. $XDG_SESSION_TYPE (set by systemd-logind / display manager)
//  2. $WAYLAND_DISPLAY  (set when a Wayland compositor is running)
//  3. $DISPLAY          (set when an X11 server is running)
//
// Returns one of BackendX11, BackendWayland, or BackendUnknown.
func DetectBackend() string {
	// XDG_SESSION_TYPE is the canonical source on modern distros.
	sessionType := strings.TrimSpace(os.Getenv("XDG_SESSION_TYPE"))
	switch strings.ToLower(sessionType) {
	case "x11":
		return BackendX11
	case "wayland":
		return BackendWayland
	}

	// Fall back to environment probes.
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return BackendWayland
	}
	if os.Getenv("DISPLAY") != "" {
		return BackendX11
	}

	return BackendUnknown
}

// SetResolution sets the resolution on the first connected output.
// It delegates to the X11 or Wayland backend automatically.
func SetResolution(width, height int) error {
	switch DetectBackend() {
	case BackendX11:
		return x11SetResolution(width, height)
	case BackendWayland:
		return waylandSetResolution(width, height)
	default:
		return fmt.Errorf("display: cannot set resolution — unknown session type (set $XDG_SESSION_TYPE or $DISPLAY/$WAYLAND_DISPLAY)")
	}
}

// SetScale sets the display scale factor on the first connected output.
func SetScale(factor float64) error {
	switch DetectBackend() {
	case BackendX11:
		return x11SetScale(factor)
	case BackendWayland:
		return waylandSetScale(factor)
	default:
		return fmt.Errorf("display: cannot set scale — unknown session type")
	}
}

// Rotate changes the orientation of the first connected output.
// direction must be one of: normal, left, right, inverted.
func Rotate(direction string) error {
	valid := map[string]bool{
		"normal": true, "left": true, "right": true, "inverted": true,
	}
	if !valid[direction] {
		return fmt.Errorf("display: invalid rotation %q (must be normal|left|right|inverted)", direction)
	}

	switch DetectBackend() {
	case BackendX11:
		return x11Rotate(direction)
	case BackendWayland:
		return waylandRotate(direction)
	default:
		return fmt.Errorf("display: cannot rotate — unknown session type")
	}
}

// ListOutputs returns the list of known display outputs and their status.
func ListOutputs() ([]Output, error) {
	switch DetectBackend() {
	case BackendX11:
		return x11ListOutputs()
	case BackendWayland:
		return waylandListOutputs()
	default:
		return nil, fmt.Errorf("display: cannot list outputs — unknown session type")
	}
}

// GetConnectedOutput is a convenience helper that returns the name of the
// first connected output. Used by callers that only need the primary display.
func GetConnectedOutput() (string, error) {
	outputs, err := ListOutputs()
	if err != nil {
		return "", err
	}
	for _, o := range outputs {
		if o.Connected {
			return o.Name, nil
		}
	}
	return "", fmt.Errorf("display: no connected output found")
}
