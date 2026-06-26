package display

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// X11 backend — delegates to xrandr
// ---------------------------------------------------------------------------

// x11ListOutputs parses `xrandr` output to discover monitors.
func x11ListOutputs() ([]Output, error) {
	out, err := exec.Command("xrandr").Output()
	if err != nil {
		return nil, fmt.Errorf("xrandr failed: %w", err)
	}

	var outputs []Output
	// Match lines like:
	//   HDMI-1 connected 1920x1080+0+0 ...
	//   VGA-1 disconnected
	re := regexp.MustCompile(`^(\S+)\s+(connected|disconnected)\s*(.*)`)
	// Match a resolution with a star (current mode) e.g. "1920x1080     60.00*+"
	resCurrent := regexp.MustCompile(`^\s+(\d+x\d+)\s+.*\*`)

	lines := strings.Split(string(out), "\n")
	var current *Output
	for _, line := range lines {
		if m := re.FindStringSubmatch(line); m != nil {
			o := Output{
				Name:      m[1],
				Connected: m[2] == "connected",
			}
			// Try to grab inline resolution (e.g. "1920x1080+0+0")
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

		// If we haven't found a resolution yet for the current output,
		// look for the starred mode line.
		if current != nil && current.CurrentResolution == "" && current.Connected {
			if m := resCurrent.FindStringSubmatch(line); m != nil {
				current.CurrentResolution = m[1]
			}
		}
	}

	return outputs, nil
}

// x11GetConnectedOutput returns the first connected output name via xrandr.
func x11GetConnectedOutput() (string, error) {
	outputs, err := x11ListOutputs()
	if err != nil {
		return "", err
	}
	for _, o := range outputs {
		if o.Connected {
			return o.Name, nil
		}
	}
	return "", fmt.Errorf("x11: no connected display found")
}

// x11SetResolution creates (if necessary) and applies a custom mode via
// xrandr + cvt on the first connected output.
func x11SetResolution(width, height int) error {
	output, err := x11GetConnectedOutput()
	if err != nil {
		return err
	}

	modeName := fmt.Sprintf("rs-%dx%d-60", width, height)

	// Generate modeline with cvt.
	cvtOut, err := exec.Command("cvt",
		fmt.Sprintf("%d", width),
		fmt.Sprintf("%d", height),
		"60",
	).Output()
	if err != nil {
		return fmt.Errorf("cvt failed: %w", err)
	}

	var modelineParams string
	for _, line := range strings.Split(string(cvtOut), "\n") {
		if strings.Contains(line, "Modeline") {
			parts := strings.SplitN(line, "\"", 3)
			if len(parts) == 3 {
				modelineParams = strings.TrimSpace(parts[2])
				break
			}
		}
	}
	if modelineParams == "" {
		return fmt.Errorf("x11: cvt did not produce a Modeline for %dx%d", width, height)
	}

	// --newmode (ignore error if mode already exists)
	args := append([]string{"--newmode", modeName}, strings.Fields(modelineParams)...)
	_ = exec.Command("xrandr", args...).Run()

	// --addmode
	addOut, err := exec.Command("xrandr", "--addmode", output, modeName).CombinedOutput()
	if err != nil && !strings.Contains(string(addOut), "already exists") {
		return fmt.Errorf("xrandr --addmode failed: %s", string(addOut))
	}

	// --output --mode
	modeOut, err := exec.Command("xrandr", "--output", output, "--mode", modeName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("xrandr --output --mode failed: %s", string(modeOut))
	}

	return nil
}

// x11SetScale sets UI scaling via xrandr --scale. X11 does not have native
// fractional scaling, so we approximate via the scale transform and DPI.
func x11SetScale(factor float64) error {
	output, err := x11GetConnectedOutput()
	if err != nil {
		return err
	}

	// xrandr --output <out> --scale <factor>x<factor>
	scaleStr := fmt.Sprintf("%.2fx%.2f", factor, factor)
	out, err := exec.Command("xrandr", "--output", output, "--scale", scaleStr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("xrandr --scale failed: %s", string(out))
	}
	return nil
}

// x11Rotate rotates the first connected output.
func x11Rotate(direction string) error {
	output, err := x11GetConnectedOutput()
	if err != nil {
		return err
	}

	out, err := exec.Command("xrandr", "--output", output, "--rotate", direction).CombinedOutput()
	if err != nil {
		return fmt.Errorf("xrandr --rotate failed: %s", string(out))
	}
	return nil
}
