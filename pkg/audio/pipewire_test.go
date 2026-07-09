package audio

import (
	"reflect"
	"testing"
)

func TestParseModuleID(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		moduleName string
		sinkName   string
		want       string
	}{
		{
			name:       "empty input",
			input:      "",
			moduleName: "module-null-sink",
			sinkName:   "RemoteStudio-Virtual",
			want:       "",
		},
		{
			name: "happy path",
			input: `0	module-null-sink	sink_name=RemoteStudio-Virtual
1	module-native-protocol-unix
2	module-cli-protocol-unix
`,
			moduleName: "module-null-sink",
			sinkName:   "RemoteStudio-Virtual",
			want:       "0",
		},
		{
			name: "module present but sink name doesn't match",
			input: `0	module-null-sink	sink_name=OtherSink
1	module-native-protocol-unix
`,
			moduleName: "module-null-sink",
			sinkName:   "RemoteStudio-Virtual",
			want:       "",
		},
		{
			name: "no module-null-sink in output",
			input: `0	module-native-protocol-unix
1	module-cli-protocol-unix
`,
			moduleName: "module-null-sink",
			sinkName:   "RemoteStudio-Virtual",
			want:       "",
		},
		{
			name:       "empty sink name matches any module-null-sink",
			input:      "0	module-null-sink\n",
			moduleName: "module-null-sink",
			sinkName:   "",
			want:       "0",
		},
		{
			name: "module name is a substring of another module",
			input: `0	module-null-sink-something
1	module-null-sink
`,
			moduleName: "module-null-sink",
			sinkName:   "",
			// Both lines contain "module-null-sink" as a substring.
			// First match wins — that's the first one, ID 0.
			want: "0",
		},
		{
			name: "different module, no overlap",
			input: `0	module-dummy
1	module-combine-sink
`,
			moduleName: "module-null-sink",
			sinkName:   "",
			want:       "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseModuleID(c.input, c.moduleName, c.sinkName)
			if got != c.want {
				t.Errorf("parseModuleID() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseSinkList(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		excludeSink string
		want        []string
	}{
		{
			name:        "empty input",
			input:       "",
			excludeSink: VirtualSinkName,
			want:        nil,
		},
		{
			name: "single physical sink",
			input: `0	alsa_output.pci-0000_00_1f.3.analog-stereo	module-alsa-card.c	s16le 2ch 44100Hz	RUNNING
1	alsa_output.usb-Generic_Webcam_200901010001-02.analog-stereo	module-alsa-card.c	s16le 2ch 48000Hz	IDLE
`,
			excludeSink: VirtualSinkName,
			want:        []string{"alsa_output.pci-0000_00_1f.3.analog-stereo", "alsa_output.usb-Generic_Webcam_200901010001-02.analog-stereo"},
		},
		{
			name: "excludes the virtual sink",
			input: `0	alsa_output.pci-0000_00_1f.3.analog-stereo	module-alsa-card.c	s16le 2ch 44100Hz	RUNNING
1	RemoteStudio-Virtual	module-null-sink.c	float32le 2ch 48000Hz	RUNNING
2	alsa_output.hdmi-stereo	module-alsa-card.c	s16le 2ch 48000Hz	IDLE
`,
			excludeSink: VirtualSinkName,
			want:        []string{"alsa_output.pci-0000_00_1f.3.analog-stereo", "alsa_output.hdmi-stereo"},
		},
		{
			name: "skips malformed lines",
			input: `0	first-sink	module-alsa-card.c	RUNNING
only-one-field
1	second-sink	module-alsa-card.c	IDLE
`,
			excludeSink: VirtualSinkName,
			want:        []string{"first-sink", "second-sink"},
		},
		{
			name: "empty exclude means no exclusion",
			input: `0	keep-this	module-alsa-card.c	RUNNING
1	also-keep	module-alsa-card.c	IDLE
`,
			excludeSink: "",
			want:        []string{"keep-this", "also-keep"},
		},
		{
			name: "only virtual sink present returns empty",
			input: `0	RemoteStudio-Virtual	module-null-sink.c	float32le 2ch 48000Hz	RUNNING
`,
			excludeSink: VirtualSinkName,
			want:        []string(nil), // nil slice, len 0
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseSinkList(c.input, c.excludeSink)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("parseSinkList() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	// Public constants: catch accidental renames.
	if VirtualSinkName != "RemoteStudio-Virtual" {
		t.Errorf("VirtualSinkName = %q", VirtualSinkName)
	}
	if VirtualSinkDescription != "Remote Studio Virtual Output" {
		t.Errorf("VirtualSinkDescription = %q", VirtualSinkDescription)
	}
}
