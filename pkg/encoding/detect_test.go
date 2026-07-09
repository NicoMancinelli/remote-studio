// Tests for pure (non-exec) helpers in pkg/encoding. The IO-bound
// DetectVAAPI / DetectNVENC functions exec external tools (vainfo,
// nvidia-smi) and are out of scope for unit tests here — they are
// exercised by the e2e suite and would need a process-spawning mock
// to unit-test cleanly.
package encoding

import (
	"reflect"
	"strings"
	"testing"
)

func TestEncoderTypeString(t *testing.T) {
	cases := []struct {
		got  EncoderType
		want string
	}{
		{EncoderSoftware, "Software"},
		{EncoderVAAPI, "VA-API"},
		{EncoderNVENC, "NVENC"},
		{EncoderType(99), "Software"}, // unknown → fallback
	}
	for _, c := range cases {
		if got := c.got.String(); got != c.want {
			t.Errorf("EncoderType(%d).String() = %q, want %q", c.got, got, c.want)
		}
	}
}

func TestParseVAAPICodecs(t *testing.T) {
	// A canonical vainfo output snippet with three encoding entrypoints.
	raw := strings.Join([]string{
		"libva info: VA-API version 1.17.0",
		"vainfo: Driver version: Intel iHD driver for Intel(R) Gen Graphics - 23.1.0",
		"vainfo: Supported profile and entrypoints",
		"      VAProfileH264Main    : VAEntrypointVLD",
		"      VAProfileH264Main    : VAEntrypointEncSlice",
		"      VAProfileHEVCMain     : VAEntrypointVLD",
		"      VAProfileHEVCMain     : VAEntrypointEncSlice",
		"      VAProfileAV1Profile0  : VAEntrypointVLD",
		"      VAProfileAV1Profile0  : VAEntrypointEncSlice",
		"      VAProfileVP9Profile0  : VAEntrypointVLD",
		"",
	}, "\n")

	codecs := parseVAAPICodecs(raw)
	want := []Codec{
		{Name: "H.264", Profile: "Main", Encode: true},
		{Name: "H.265", Profile: "Main", Encode: true},
		{Name: "AV1", Profile: "Profile 0", Encode: true},
	}
	if !reflect.DeepEqual(codecs, want) {
		t.Errorf("parseVAAPICodecs() = %+v, want %+v", codecs, want)
	}
}

func TestParseVAAPICodecs_Empty(t *testing.T) {
	// No encode entrypoints → no codecs.
	codecs := parseVAAPICodecs("vainfo: nothing useful here")
	if len(codecs) != 0 {
		t.Errorf("expected empty, got %+v", codecs)
	}
}

func TestParseVAAPICodecs_Dedupes(t *testing.T) {
	// Same profile repeated (e.g. two enc entrypoints for one profile)
	// should appear only once in the output.
	raw := "" +
		"VAProfileH264Main  : VAEntrypointEncSlice\n" +
		"VAProfileH264Main  : VAEntrypointEncSliceLP\n"
	codecs := parseVAAPICodecs(raw)
	if len(codecs) != 1 {
		t.Errorf("expected 1 codec after dedup, got %d: %+v", len(codecs), codecs)
	}
	if codecs[0].Name != "H.264" || codecs[0].Profile != "Main" {
		t.Errorf("unexpected codec: %+v", codecs[0])
	}
}

func TestMapVAProfileToCodec(t *testing.T) {
	cases := []struct {
		line      string
		wantName  string
		wantProf  string
	}{
		{"VAProfileH264High           : VAEntrypointEncSlice", "H.264", "High"},
		{"VAProfileH264Main           : VAEntrypointEncSlice", "H.264", "Main"},
		{"VAProfileH264Baseline       : VAEntrypointEncSlice", "H.264", "Baseline"},
		{"VAProfileH264Constrained    : VAEntrypointEncSlice", "H.264", "Constrained Baseline"},
		{"VAProfileHEVCMain           : VAEntrypointEncSlice", "H.265", "Main"},
		{"VAProfileHEVCMain10         : VAEntrypointEncSlice", "H.265", "Main 10"},
		{"VAProfileHEVCMain444        : VAEntrypointEncSlice", "H.265", "Main 444"},
		{"VAProfileVP9Profile0        : VAEntrypointEncSlice", "VP9", "Profile 0"},
		{"VAProfileVP9Profile2        : VAEntrypointEncSlice", "VP9", "Profile 2"},
		{"VAProfileAV1Profile0        : VAEntrypointEncSlice", "AV1", "Profile 0"},
		{"VAProfileAV1Profile1        : VAEntrypointEncSlice", "AV1", "Profile 1"},
		{"VAProfileJPEG               : VAEntrypointVLD", "", ""}, // decode-only, no map
		{"something completely unrelated", "", ""},
	}
	for _, c := range cases {
		gotName, gotProf := mapVAProfileToCodec(c.line)
		if gotName != c.wantName || gotProf != c.wantProf {
			t.Errorf("mapVAProfileToCodec(%q) = (%q, %q), want (%q, %q)",
				c.line, gotName, gotProf, c.wantName, c.wantProf)
		}
	}
}

func TestInferNVENCCodecs(t *testing.T) {
	cases := []struct {
		gpu  string
		want []Codec
	}{
		{
			gpu: "NVIDIA GeForce RTX 3070",
			want: []Codec{
				{Name: "H.264", Profile: "Main", Encode: true},
				{Name: "H.265", Profile: "Main", Encode: true},
			},
		},
		{
			gpu: "NVIDIA GeForce RTX 4090", // has AV1
			want: []Codec{
				{Name: "H.264", Profile: "Main", Encode: true},
				{Name: "H.265", Profile: "Main", Encode: true},
				{Name: "AV1", Profile: "Main", Encode: true},
			},
		},
		{
			gpu: "NVIDIA RTX 5000", // RTX 50xx
			want: []Codec{
				{Name: "H.264", Profile: "Main", Encode: true},
				{Name: "H.265", Profile: "Main", Encode: true},
				{Name: "AV1", Profile: "Main", Encode: true},
			},
		},
		{
			gpu: "NVIDIA L40",
			want: []Codec{
				{Name: "H.264", Profile: "Main", Encode: true},
				{Name: "H.265", Profile: "Main", Encode: true},
				{Name: "AV1", Profile: "Main", Encode: true},
			},
		},
		{
			gpu: "NVIDIA A100",
			want: []Codec{
				{Name: "H.264", Profile: "Main", Encode: true},
				{Name: "H.265", Profile: "Main", Encode: true},
				{Name: "AV1", Profile: "Main", Encode: true},
			},
		},
		{
			// lowercase: function does strings.ToUpper, so this should
			// still match.
			gpu: "nvidia geforce rtx 4090",
			want: []Codec{
				{Name: "H.264", Profile: "Main", Encode: true},
				{Name: "H.265", Profile: "Main", Encode: true},
				{Name: "AV1", Profile: "Main", Encode: true},
			},
		},
		{
			// Quadro / older Tesla: just H.264/H.265.
			gpu: "NVIDIA Tesla K80",
			want: []Codec{
				{Name: "H.264", Profile: "Main", Encode: true},
				{Name: "H.265", Profile: "Main", Encode: true},
			},
		},
	}
	for _, c := range cases {
		got := inferNVENCCodecs(c.gpu)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("inferNVENCCodecs(%q) = %+v, want %+v", c.gpu, got, c.want)
		}
	}
}

func TestBestCodecName(t *testing.T) {
	cases := []struct {
		name   string
		input  []Codec
		expect string
	}{
		{
			name:   "empty",
			input:  nil,
			expect: "H.264",
		},
		{
			name:   "single H264",
			input:  []Codec{{Name: "H.264", Encode: true}},
			expect: "H.264",
		},
		{
			name:   "prefers H265 over H264 over VP9",
			input:  []Codec{{Name: "VP9", Encode: true}, {Name: "H.264", Encode: true}, {Name: "H.265", Encode: true}},
			expect: "H.265",
		},
		{
			name:   "prefers AV1 over H264 but not over H265",
			input:  []Codec{{Name: "AV1", Encode: true}, {Name: "H.264", Encode: true}},
			expect: "AV1",
		},
		{
			name:   "skips decode-only codecs",
			input:  []Codec{{Name: "H.265", Encode: false}, {Name: "H.264", Encode: true}},
			expect: "H.264",
		},
		{
			name:   "falls back to first codec if no priority match",
			input:  []Codec{{Name: "WeirdCodec", Encode: true}},
			expect: "WeirdCodec",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := bestCodecName(c.input); got != c.expect {
				t.Errorf("bestCodecName(%+v) = %q, want %q", c.input, got, c.expect)
			}
		})
	}
}

func TestFFmpegEncoder(t *testing.T) {
	cases := []struct {
		typ   EncoderType
		codec string
		want  string
	}{
		{EncoderVAAPI, "H.264", "h264_vaapi"},
		{EncoderVAAPI, "H.265", "hevc_vaapi"},
		{EncoderVAAPI, "AV1", "av1_vaapi"},
		{EncoderVAAPI, "VP9", "vp9_vaapi"},
		{EncoderVAAPI, "WeirdCodec", "libx264"}, // fallback
		{EncoderNVENC, "H.264", "h264_nvenc"},
		{EncoderNVENC, "H.265", "hevc_nvenc"},
		{EncoderNVENC, "AV1", "av1_nvenc"},
		{EncoderNVENC, "VP9", "libx264"}, // NVENC table has no VP9
		{EncoderNVENC, "WeirdCodec", "libx264"},
		{EncoderSoftware, "H.264", "libx264"},
		{EncoderSoftware, "H.265", "libx264"},
		{EncoderType(99), "H.264", "libx264"},
	}
	for _, c := range cases {
		if got := ffmpegEncoder(c.typ, c.codec); got != c.want {
			t.Errorf("ffmpegEncoder(%d, %q) = %q, want %q", c.typ, c.codec, got, c.want)
		}
	}
}

func TestHasDevice(t *testing.T) {
	// /dev/null is always a device file in Linux.
	if !HasDevice("/dev/null") {
		t.Error("expected /dev/null to be a device file")
	}
	// Non-existent path
	if HasDevice("/this/path/does/not/exist/anywhere") {
		t.Error("expected non-existent path to return false")
	}
	// A regular file (this test source itself, or /etc/hostname)
	if HasDevice("/etc/hostname") {
		t.Error("expected regular file to return false (only char/block devices qualify)")
	}
}
