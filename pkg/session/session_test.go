package session

import (
	"fmt"
	"math"
	"testing"
)

func TestFirstConnectedOutput(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "single HDMI connected",
			input: `Screen 0: minimum 320 x 200, current 2560 x 1664, maximum 16384 x 16384
HDMI-1 connected primary 2560x1664+0+0 (normal left inverted right x axis y axis) 553mm x 344mm
   2560x1664     59.95*+`,
			want: "HDMI-1",
		},
		{
			name: "DP preferred",
			input: `Screen 0: minimum 320 x 200, current 3840 x 2160, maximum 16384 x 16384
DP-1 connected 3840x2160+0+0 (normal)
DP-2 disconnected (normal)
HDMI-1 disconnected (normal)`,
			want: "DP-1",
		},
		{
			name: "no connected output",
			input: `Screen 0: minimum 320 x 200
DP-1 disconnected (normal)
HDMI-1 disconnected (normal)`,
			want: "",
		},
		{
			name: "empty",
			input: "",
			want: "",
		},
		{
			name: "only modes, no connected line",
			input: `Screen 0: minimum 320 x 200, current 2560 x 1664
   2560x1664     59.95*+`,
			want: "",
		},
		{
			name: "disconnected before connected",
			input: `DP-1 disconnected (normal)
HDMI-1 connected 2560x1664+0+0 (normal)`,
			want: "HDMI-1",
		},
		{
			name: "eDP-1 connected (laptop internal)",
			input: `eDP-1 connected primary 1920x1080+0+0 (normal left inverted right x axis y axis) 344mm x 194mm
   1920x1080    60.01*+`,
			want: "eDP-1",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := firstConnectedOutput(c.input); got != c.want {
				t.Errorf("firstConnectedOutput() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseStateLine(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantW     int
		wantH     int
		wantS     float64
		wantTS    float64
		wantC     int
		wantLabel string
		wantOK    bool
	}{
		{
			name:      "typical 1x scale, simple label",
			input:     "2560 1664 1 1.5 48 'MacBook Air 13'",
			wantW:     2560,
			wantH:     1664,
			wantS:     1.0,
			wantTS:    1.5,
			wantC:     48,
			wantLabel: "MacBook Air 13",
			wantOK:    true,
		},
		{
			name:      "2x scale retina",
			input:     "3024 1964 2 1.2 64 'MacBook Air 13 (Retina)'",
			wantW:     3024,
			wantH:     1964,
			wantS:     2.0,
			wantTS:    1.2,
			wantC:     64,
			wantLabel: "MacBook Air 13 (Retina)",
			wantOK:    true,
		},
		{
			name:      "fallback resolution",
			input:     "1920 1200 1 1.1 32 'Fallback 1920x1200'",
			wantW:     1920,
			wantH:     1200,
			wantS:     1.0,
			wantTS:    1.1,
			wantC:     32,
			wantLabel: "Fallback 1920x1200",
			wantOK:    true,
		},
		{
			name:      "empty label",
			input:     "1920 1080 1 1.0 24 ''",
			wantW:     1920,
			wantH:     1080,
			wantS:     1.0,
			wantTS:    1.0,
			wantC:     24,
			wantLabel: "",
			wantOK:    true,
		},
		{
			name:   "missing fields",
			input:  "2560 1664 1 1.5 48",
			wantOK: false,
		},
		{
			name:   "non-numeric width",
			input:  "abc 1664 1 1.5 48 'Label'",
			wantOK: false,
		},
		{
			name:   "non-numeric scaling",
			input:  "2560 1664 xyz 1.5 48 'Label'",
			wantOK: false,
		},
		{
			name:   "label not quoted",
			input:  "2560 1664 1 1.5 48 unquoted-label",
			wantOK: false,
		},
		{
			name:   "empty input",
			input:  "",
			wantOK: false,
		},
		{
			name:   "trailing garbage after quoted label",
			input:  "2560 1664 1 1.5 48 'Label' extra-stuff",
			wantOK: true, // regex is anchored to ^ only, not $ — trailing junk ignored
			wantW: 2560, wantH: 1664, wantS: 1, wantTS: 1.5, wantC: 48, wantLabel: "Label",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, h, s, ts, cur, label, ok := parseStateLine(c.input)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v (w=%d h=%d s=%v ts=%v c=%d label=%q)",
					ok, c.wantOK, w, h, s, ts, cur, label)
			}
			if !ok {
				return
			}
			if w != c.wantW {
				t.Errorf("width = %d, want %d", w, c.wantW)
			}
			if h != c.wantH {
				t.Errorf("height = %d, want %d", h, c.wantH)
			}
			if math.Abs(s-c.wantS) > 1e-9 {
				t.Errorf("scaling = %v, want %v", s, c.wantS)
			}
			if math.Abs(ts-c.wantTS) > 1e-9 {
				t.Errorf("textScale = %v, want %v", ts, c.wantTS)
			}
			if cur != c.wantC {
				t.Errorf("cursor = %d, want %d", cur, c.wantC)
			}
			if label != c.wantLabel {
				t.Errorf("label = %q, want %q", label, c.wantLabel)
			}
		})
	}
}

func TestParseStateLine_RoundTrip(t *testing.T) {
	// Generated state line from ApplyAll:
	//   fmt.Sprintf("%d %d %g %g %d '%s'\n", width, height, scaling, textScale, cursor, label)
	// Verify parseStateLine recovers the same fields.
	cases := []struct {
		w, h, c int
		s, ts   float64
		label   string
	}{
		{2560, 1664, 48, 1.0, 1.5, "MacBook Air 13"},
		{1920, 1080, 24, 1.0, 1.0, "FHD"},
		{3024, 1964, 64, 2.0, 1.2, "MacBook Air 13 (Retina)"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			// match ApplyAll's format string
			line := sprintfState(c.w, c.h, c.c, c.s, c.ts, c.label)
			w, h, s, ts, cur, label, ok := parseStateLine(line)
			if !ok {
				t.Fatalf("parseStateLine(%q) failed", line)
			}
			if w != c.w || h != c.h || cur != c.c {
				t.Errorf("ints: got (%d,%d,%d), want (%d,%d,%d)", w, h, cur, c.w, c.h, c.c)
			}
			if math.Abs(s-c.s) > 1e-9 || math.Abs(ts-c.ts) > 1e-9 {
				t.Errorf("floats: got (%v,%v), want (%v,%v)", s, ts, c.s, c.ts)
			}
			if label != c.label {
				t.Errorf("label: got %q, want %q", label, c.label)
			}
		})
	}
}

// sprintfState mirrors ApplyAll's state-file format string. Kept here
// to avoid import cycles and ensure the test exercises the exact format
// ApplyAll writes.
func sprintfState(w, h, c int, s, ts float64, label string) string {
	return fmt.Sprintf("%d %d %g %g %d '%s'", w, h, s, ts, c, label)
}
