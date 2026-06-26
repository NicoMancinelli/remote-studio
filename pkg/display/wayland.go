package display

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Wayland backend — delegates to wlr-randr (wlroots compositors) or
// gnome-randr (GNOME/Mutter compositors).
//
// Tool resolution order:
//   1. gnome-randr  — works on GNOME Shell / Mutter (e.g. Ubuntu, Fedora).
//   2. wlr-randr    — works on wlroots-based compositors (Sway, Hyprland).
//
// If neither tool is found, functions return a descriptive error.
// ---------------------------------------------------------------------------

// waylandTool returns the path to the preferred Wayland randr tool.
// It prefers gnome-randr (Rust binary: gnome-randr) over wlr-randr.
func waylandTool() (string, error) {
	if p, err := exec.LookPath("gnome-randr"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("wlr-randr"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("wayland: neither gnome-randr nor wlr-randr found in $PATH — install one to manage Wayland displays")
}

// isGnomeRandr returns true when the resolved tool path is gnome-randr.
func isGnomeRandr(toolPath string) bool {
	return strings.Contains(toolPath, "gnome-randr")
}

// ---------------------------------------------------------------------------
// List outputs
// ---------------------------------------------------------------------------

// waylandListOutputs parses wlr-randr / gnome-randr output to discover monitors.
//
// wlr-randr output looks like:
//
//	HEADLESS-1 "..." (DP-1)
//	  Enabled: yes
//	  Modes:
//	    1920x1080 px, 60.000000 Hz (preferred, current)
//
// gnome-randr output looks like:
//
//	HDMI-1   connected   1920x1080+0+0
//	  1920x1080  60.00*+
//
// We handle both formats.
func waylandListOutputs() ([]Output, error) {
	tool, err := waylandTool()
	if err != nil {
		return nil, err
	}

	out, err := exec.Command(tool).Output()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", tool, err)
	}

	if isGnomeRandr(tool) {
		return parseGnomeRandrOutputs(string(out))
	}
	return parseWlrRandrOutputs(string(out))
}

// parseWlrRandrOutputs parses wlr-randr listing format.
func parseWlrRandrOutputs(raw string) ([]Output, error) {
	var outputs []Output

	// Each output block starts with a non-whitespace line containing the
	// output name, e.g.:
	//   HEADLESS-1 "Virtual display" (DP-1)
	reHead := regexp.MustCompile(`^(\S+)\s+`)
	reCurrent := regexp.MustCompile(`(\d+x\d+)\s+px.*current`)
	reEnabled := regexp.MustCompile(`^\s+Enabled:\s+(yes|no)`)

	var current *Output
	for _, line := range strings.Split(raw, "\n") {
		if m := reHead.FindStringSubmatch(line); m != nil && !strings.HasPrefix(line, " ") {
			o := Output{
				Name:      m[1],
				Connected: true, // wlr-randr only lists connected outputs
			}
			outputs = append(outputs, o)
			current = &outputs[len(outputs)-1]
			continue
		}

		if current == nil {
			continue
		}

		if m := reEnabled.FindStringSubmatch(line); m != nil {
			if m[1] == "no" {
				current.Connected = false
			}
		}

		if m := reCurrent.FindStringSubmatch(line); m != nil {
			current.CurrentResolution = m[1]
		}
	}

	return outputs, nil
}

// parseGnomeRandrOutputs parses gnome-randr listing format.
func parseGnomeRandrOutputs(raw string) ([]Output, error) {
	var outputs []Output

	// gnome-randr output is similar to xrandr:
	//   DP-1 connected 2560x1440+0+0
	//   DP-2 disconnected
	reHead := regexp.MustCompile(`^(\S+)\s+(connected|disconnected)\s*(.*)`)
	resCurrent := regexp.MustCompile(`^\s+(\d+x\d+)\s+.*\*`)

	var current *Output
	for _, line := range strings.Split(raw, "\n") {
		if m := reHead.FindStringSubmatch(line); m != nil {
			o := Output{
				Name:      m[1],
				Connected: m[2] == "connected",
			}
			if o.Connected {
				inlineRes := regexp.MustCompile(`(\d+x\d+)\+`)
				if rm := inlineRes.FindStringSubmatch(m[3]); rm != nil {
					o.CurrentResolution = rm[1]
				}
			}
			outputs = append(outputs, o)
			current = &outputs[len(outputs)-1]
			continue
		}

		if current != nil && current.CurrentResolution == "" && current.Connected {
			if m := resCurrent.FindStringSubmatch(line); m != nil {
				current.CurrentResolution = m[1]
			}
		}
	}

	return outputs, nil
}

// ---------------------------------------------------------------------------
// Set resolution
// ---------------------------------------------------------------------------

func waylandSetResolution(width, height int) error {
	tool, err := waylandTool()
	if err != nil {
		return err
	}

	output, err := waylandGetConnectedOutput()
	if err != nil {
		return err
	}

	modeStr := fmt.Sprintf("%dx%d", width, height)

	// Both wlr-randr and gnome-randr accept:
	//   <tool> --output <name> --mode <WxH>
	out, err := exec.Command(tool, "--output", output, "--mode", modeStr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s --mode failed: %s", tool, string(out))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Set scale
// ---------------------------------------------------------------------------

func waylandSetScale(factor float64) error {
	tool, err := waylandTool()
	if err != nil {
		return err
	}

	output, err := waylandGetConnectedOutput()
	if err != nil {
		return err
	}

	scaleStr := fmt.Sprintf("%.2f", factor)

	out, err := exec.Command(tool, "--output", output, "--scale", scaleStr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s --scale failed: %s", tool, string(out))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Rotate
// ---------------------------------------------------------------------------

// waylandRotate translates the xrandr-style direction name to a Wayland
// transform value and applies it.
//
// wlr-randr transform values: normal, 90, 180, 270, flipped, flipped-90, …
// gnome-randr: --rotate normal|left|right|inverted (same as xrandr)
func waylandRotate(direction string) error {
	tool, err := waylandTool()
	if err != nil {
		return err
	}

	output, err := waylandGetConnectedOutput()
	if err != nil {
		return err
	}

	if isGnomeRandr(tool) {
		// gnome-randr uses the same direction names as xrandr.
		out, err := exec.Command(tool, "--output", output, "--rotate", direction).CombinedOutput()
		if err != nil {
			return fmt.Errorf("gnome-randr --rotate failed: %s", string(out))
		}
		return nil
	}

	// wlr-randr uses --transform with degree values.
	transformMap := map[string]string{
		"normal":   "normal",
		"left":     "90",
		"inverted": "180",
		"right":    "270",
	}
	transform, ok := transformMap[direction]
	if !ok {
		return fmt.Errorf("wayland: unsupported rotation %q", direction)
	}

	out, err := exec.Command(tool, "--output", output, "--transform", transform).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wlr-randr --transform failed: %s", string(out))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func waylandGetConnectedOutput() (string, error) {
	outputs, err := waylandListOutputs()
	if err != nil {
		return "", err
	}
	for _, o := range outputs {
		if o.Connected {
			return o.Name, nil
		}
	}
	return "", fmt.Errorf("wayland: no connected output found")
}
