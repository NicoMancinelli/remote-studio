//go:build linux

// Package audio provides PipeWire/PulseAudio virtual audio sink management
// for Remote Studio. It creates a virtual audio sink that captures all desktop
// audio during remote sessions, and can mute/unmute physical outputs so the
// remote operator doesn't leak sound to the local speakers.
package audio

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

const (
	// VirtualSinkName is the PipeWire/PulseAudio sink name.
	VirtualSinkName = "RemoteStudio-Virtual"

	// VirtualSinkDescription is the human-readable description shown in
	// audio mixer UIs (pavucontrol, GNOME Settings, etc.).
	VirtualSinkDescription = "Remote Studio Virtual Output"

	// moduleNullSink is the PulseAudio module used to create virtual sinks.
	moduleNullSink = "module-null-sink"
)

// mutedSinks tracks which physical sinks were muted by us so we can
// selectively unmute only those we touched.
var (
	mutedSinks   []string
	mutedSinksMu sync.Mutex
)

// --------------------------------------------------------------------------
// Public API
// --------------------------------------------------------------------------

// CreateVirtualSink creates a PipeWire/PulseAudio virtual sink named
// "RemoteStudio-Virtual" using pactl. If the sink already exists the call
// is a no-op.
func CreateVirtualSink() error {
	if IsVirtualSinkActive() {
		return nil // already exists
	}

	// pactl load-module module-null-sink sink_name=... sink_properties=...
	cmd := exec.Command("pactl", "load-module", moduleNullSink,
		fmt.Sprintf("sink_name=%s", VirtualSinkName),
		fmt.Sprintf("sink_properties=device.description=\"%s\"", VirtualSinkDescription),
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create virtual sink: %w: %s", err, stderr.String())
	}

	return nil
}

// RemoveVirtualSink removes the virtual sink by unloading its PulseAudio module.
func RemoveVirtualSink() error {
	moduleID, err := findModuleID()
	if err != nil {
		return err
	}
	if moduleID == "" {
		return nil // nothing to remove
	}

	cmd := exec.Command("pactl", "unload-module", moduleID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unload virtual sink module %s: %w: %s", moduleID, err, stderr.String())
	}

	return nil
}

// MutePhysicalOutputs mutes every physical (non-virtual) sink so audio
// doesn't leak out of local speakers during a remote session.
func MutePhysicalOutputs() error {
	sinks, err := listPhysicalSinks()
	if err != nil {
		return err
	}

	mutedSinksMu.Lock()
	defer mutedSinksMu.Unlock()

	mutedSinks = nil
	for _, sink := range sinks {
		cmd := exec.Command("pactl", "set-sink-mute", sink, "1")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to mute sink %s: %w: %s", sink, err, stderr.String())
		}
		mutedSinks = append(mutedSinks, sink)
	}

	return nil
}

// UnmutePhysicalOutputs unmutes the physical sinks that were previously
// muted by MutePhysicalOutputs.
func UnmutePhysicalOutputs() error {
	mutedSinksMu.Lock()
	targets := make([]string, len(mutedSinks))
	copy(targets, mutedSinks)
	mutedSinks = nil
	mutedSinksMu.Unlock()

	var firstErr error
	for _, sink := range targets {
		cmd := exec.Command("pactl", "set-sink-mute", sink, "0")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to unmute sink %s: %w: %s", sink, err, stderr.String())
		}
	}

	return firstErr
}

// IsVirtualSinkActive returns true if the RemoteStudio-Virtual sink is
// currently loaded in PipeWire/PulseAudio.
func IsVirtualSinkActive() bool {
	cmd := exec.Command("pactl", "list", "short", "sinks")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, VirtualSinkName) {
			return true
		}
	}
	return false
}

// Status returns a human-readable status string for CLI display.
func Status() string {
	if IsVirtualSinkActive() {
		return fmt.Sprintf("✓ Virtual sink '%s' is active", VirtualSinkName)
	}
	return fmt.Sprintf("✗ Virtual sink '%s' is not loaded", VirtualSinkName)
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// findModuleID returns the PulseAudio module ID for our null-sink module,
// or "" if not loaded.
func findModuleID() (string, error) {
	cmd := exec.Command("pactl", "list", "short", "modules")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list modules: %w", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, moduleNullSink) && strings.Contains(line, VirtualSinkName) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return fields[0], nil
			}
		}
	}

	return "", nil
}

// listPhysicalSinks returns the names of all loaded sinks that are NOT
// our virtual sink.
func listPhysicalSinks() ([]string, error) {
	cmd := exec.Command("pactl", "list", "short", "sinks")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list sinks: %w", err)
	}

	var sinks []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[1]
		if name == VirtualSinkName {
			continue
		}
		sinks = append(sinks, name)
	}

	return sinks, nil
}
