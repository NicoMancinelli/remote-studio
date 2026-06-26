// Package encoding detects and reports hardware video encoding capabilities
// (VA-API, NVENC) on Linux systems. It inspects device nodes, runs system
// utilities, and returns structured results that the CLI layer can display.
package encoding

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EncoderType represents a hardware (or software) encoder backend.
type EncoderType int

const (
	EncoderSoftware EncoderType = iota
	EncoderVAAPI
	EncoderNVENC
)

// String returns a human-readable label for an EncoderType.
func (e EncoderType) String() string {
	switch e {
	case EncoderVAAPI:
		return "VA-API"
	case EncoderNVENC:
		return "NVENC"
	default:
		return "Software"
	}
}

// Codec describes a single supported encoding codec.
type Codec struct {
	Name      string // e.g. "H.264", "H.265", "AV1", "VP9"
	Profile   string // e.g. "Main", "High", parsed from vainfo
	Encode    bool   // true if the codec supports encoding (not just decode)
}

// VAAPIInfo holds the result of VA-API hardware probe.
type VAAPIInfo struct {
	Available   bool
	Driver      string   // e.g. "iHD" or "radeonsi"
	DevicePath  string   // e.g. "/dev/dri/renderD128"
	RawOutput   string   // full vainfo output for debugging
	Codecs      []Codec
}

// NVENCInfo holds the result of NVENC hardware probe.
type NVENCInfo struct {
	Available   bool
	GPUName     string   // e.g. "NVIDIA GeForce RTX 3070"
	DriverVer   string   // e.g. "535.183.01"
	DevicePaths []string // e.g. ["/dev/nvidia0", "/dev/nvidiactl"]
	RawOutput   string   // full nvidia-smi output for debugging
	Codecs      []Codec
}

// EncodingStatus is the aggregate result shown by `res encoding`.
type EncodingStatus struct {
	VAAPI       *VAAPIInfo
	NVENC       *NVENCInfo
	Recommended EncoderType
	RecommendedCodec string
}

// ---------- VA-API detection ----------

// DetectVAAPI checks for VA-API support by looking for a DRI render node and
// running `vainfo`.
func DetectVAAPI() (*VAAPIInfo, error) {
	info := &VAAPIInfo{}

	// Look for render node
	matches, _ := filepath.Glob("/dev/dri/renderD*")
	if len(matches) > 0 {
		info.DevicePath = matches[0]
	}

	// Run vainfo
	out, err := exec.Command("vainfo").CombinedOutput()
	if err != nil {
		return info, fmt.Errorf("vainfo not available: %w", err)
	}
	info.RawOutput = string(out)
	info.Available = true

	// Parse driver
	for _, line := range strings.Split(info.RawOutput, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Driver version") || strings.Contains(line, "vainfo:  Driver version") {
			info.Driver = line
		}
	}

	// Parse supported codecs from vainfo output.
	// Encoding profiles show up as "VAProfileH264Main   : VAEntrypointEncSlice" etc.
	info.Codecs = parseVAAPICodecs(info.RawOutput)

	return info, nil
}

// parseVAAPICodecs extracts codecs from vainfo output by scanning for encode
// entrypoints: VAEntrypointEncSlice, VAEntrypointEncSliceLP, VAEntrypointEncPicture.
func parseVAAPICodecs(raw string) []Codec {
	encEntrypoints := []string{
		"VAEntrypointEncSlice",
		"VAEntrypointEncSliceLP",
		"VAEntrypointEncPicture",
	}

	seen := make(map[string]bool)
	var codecs []Codec

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)

		isEncode := false
		for _, ep := range encEntrypoints {
			if strings.Contains(line, ep) {
				isEncode = true
				break
			}
		}
		if !isEncode {
			continue
		}

		name, profile := mapVAProfileToCodec(line)
		if name == "" {
			continue
		}
		key := name + "/" + profile
		if seen[key] {
			continue
		}
		seen[key] = true
		codecs = append(codecs, Codec{Name: name, Profile: profile, Encode: true})
	}
	return codecs
}

// mapVAProfileToCodec maps a VAProfile line to a human-readable codec name
// and profile. Returns ("", "") for unrecognised entries.
func mapVAProfileToCodec(line string) (string, string) {
	type mapping struct {
		prefix  string
		codec   string
		profile string
	}
	table := []mapping{
		{"VAProfileH264High", "H.264", "High"},
		{"VAProfileH264Main", "H.264", "Main"},
		{"VAProfileH264Baseline", "H.264", "Baseline"},
		{"VAProfileH264Constrained", "H.264", "Constrained Baseline"},
		{"VAProfileHEVCMain10", "H.265", "Main 10"},
		{"VAProfileHEVCMain444", "H.265", "Main 444"},
		{"VAProfileHEVCMain", "H.265", "Main"},
		{"VAProfileVP9Profile0", "VP9", "Profile 0"},
		{"VAProfileVP9Profile2", "VP9", "Profile 2"},
		{"VAProfileAV1Profile0", "AV1", "Profile 0"},
		{"VAProfileAV1Profile1", "AV1", "Profile 1"},
	}

	for _, m := range table {
		if strings.Contains(line, m.prefix) {
			return m.codec, m.profile
		}
	}
	return "", ""
}

// ---------- NVENC detection ----------

// DetectNVENC checks for NVIDIA GPU encoding support by scanning /dev/nvidia*
// device nodes and running `nvidia-smi`.
func DetectNVENC() (*NVENCInfo, error) {
	info := &NVENCInfo{}

	// Check for device nodes
	matches, _ := filepath.Glob("/dev/nvidia*")
	if len(matches) == 0 {
		return info, fmt.Errorf("no /dev/nvidia* devices found")
	}
	info.DevicePaths = matches

	// Run nvidia-smi to get GPU info
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=gpu_name,driver_version",
		"--format=csv,noheader,nounits").CombinedOutput()
	if err != nil {
		return info, fmt.Errorf("nvidia-smi not available: %w", err)
	}
	info.RawOutput = string(out)
	info.Available = true

	// Parse first line: "NVIDIA GeForce RTX 3070, 535.183.01"
	firstLine := strings.TrimSpace(strings.Split(info.RawOutput, "\n")[0])
	parts := strings.SplitN(firstLine, ",", 2)
	if len(parts) >= 1 {
		info.GPUName = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		info.DriverVer = strings.TrimSpace(parts[1])
	}

	// NVENC typically supports these codecs (presence of the GPU device
	// is sufficient; individual codec support depends on GPU generation,
	// but we assume modern GPUs).
	info.Codecs = inferNVENCCodecs(info.GPUName)

	return info, nil
}

// inferNVENCCodecs returns a conservative list of codecs based on the GPU name.
// All modern NVIDIA GPUs support H.264/H.265. AV1 is Lovelace (RTX 40) and later.
func inferNVENCCodecs(gpuName string) []Codec {
	codecs := []Codec{
		{Name: "H.264", Profile: "Main", Encode: true},
		{Name: "H.265", Profile: "Main", Encode: true},
	}

	upper := strings.ToUpper(gpuName)
	// RTX 40xx / 50xx have AV1 encode
	if strings.Contains(upper, "RTX 40") || strings.Contains(upper, "RTX 50") ||
		strings.Contains(upper, "L40") || strings.Contains(upper, "A100") {
		codecs = append(codecs, Codec{Name: "AV1", Profile: "Main", Encode: true})
	}
	return codecs
}

// ---------- Best-encoder selection ----------

// GetBestEncoder evaluates available hardware and returns the recommended
// encoder type and codec string (e.g. "h264_vaapi").
func GetBestEncoder() (EncoderType, string) {
	// Prefer NVENC when available — generally higher throughput
	nvenc, err := DetectNVENC()
	if err == nil && nvenc.Available {
		codec := bestCodecName(nvenc.Codecs)
		return EncoderNVENC, ffmpegEncoder(EncoderNVENC, codec)
	}

	vaapi, err := DetectVAAPI()
	if err == nil && vaapi.Available && len(vaapi.Codecs) > 0 {
		codec := bestCodecName(vaapi.Codecs)
		return EncoderVAAPI, ffmpegEncoder(EncoderVAAPI, codec)
	}

	return EncoderSoftware, "libx264"
}

// bestCodecName picks the highest-quality encode codec from a list, preferring
// H.265 > AV1 > H.264 > VP9 > whatever-is-first.
func bestCodecName(codecs []Codec) string {
	priority := map[string]int{"H.265": 4, "AV1": 3, "H.264": 2, "VP9": 1}
	best := ""
	bestP := -1
	for _, c := range codecs {
		if !c.Encode {
			continue
		}
		if p, ok := priority[c.Name]; ok && p > bestP {
			best = c.Name
			bestP = p
		}
	}
	if best == "" && len(codecs) > 0 {
		best = codecs[0].Name
	}
	if best == "" {
		best = "H.264"
	}
	return best
}

// ffmpegEncoder maps (type, codec) to the FFmpeg encoder name.
func ffmpegEncoder(t EncoderType, codec string) string {
	table := map[EncoderType]map[string]string{
		EncoderVAAPI: {
			"H.264": "h264_vaapi",
			"H.265": "hevc_vaapi",
			"AV1":   "av1_vaapi",
			"VP9":   "vp9_vaapi",
		},
		EncoderNVENC: {
			"H.264": "h264_nvenc",
			"H.265": "hevc_nvenc",
			"AV1":   "av1_nvenc",
		},
	}
	if m, ok := table[t]; ok {
		if s, ok := m[codec]; ok {
			return s
		}
	}
	return "libx264"
}

// ---------- Aggregate status ----------

// GetEncodingStatus probes all backends and returns a single status struct
// suitable for display.
func GetEncodingStatus() *EncodingStatus {
	s := &EncodingStatus{}

	vaapi, _ := DetectVAAPI()
	s.VAAPI = vaapi

	nvenc, _ := DetectNVENC()
	s.NVENC = nvenc

	s.Recommended, s.RecommendedCodec = GetBestEncoder()
	return s
}

// ---------- helpers ----------

// HasDevice returns true if the given device path exists and is a device file
// (char or block).
func HasDevice(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeDevice != 0
}
