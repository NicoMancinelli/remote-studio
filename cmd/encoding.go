package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"remote-studio/pkg/encoding"

	"github.com/spf13/cobra"
)

var encodingCmd = &cobra.Command{
	Use:   "encoding",
	Short: "Show hardware video encoding capabilities",
	Long:  `Detect and display available hardware video encoders (VA-API, NVENC) and the recommended encoder for this system.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")
		status := encoding.GetEncodingStatus()

		if jsonFlag {
			return printEncodingJSON(status)
		}
		printEncodingTable(status)
		return nil
	},
}

func printEncodingTable(s *encoding.EncodingStatus) {
	w := 60 // column width for separator
	sep := strings.Repeat("─", w)

	fmt.Println("Hardware Encoding Status")
	fmt.Println(sep)

	// VA-API
	fmt.Println()
	fmt.Println("  VA-API")
	if s.VAAPI != nil && s.VAAPI.Available {
		fmt.Printf("    Status:     ✓ available\n")
		if s.VAAPI.DevicePath != "" {
			fmt.Printf("    Device:     %s\n", s.VAAPI.DevicePath)
		}
		if s.VAAPI.Driver != "" {
			fmt.Printf("    Driver:     %s\n", s.VAAPI.Driver)
		}
		if len(s.VAAPI.Codecs) > 0 {
			fmt.Printf("    Codecs:     %s\n", formatCodecs(s.VAAPI.Codecs))
		} else {
			fmt.Printf("    Codecs:     (none with encode support)\n")
		}
	} else {
		fmt.Printf("    Status:     ✗ not detected\n")
	}

	// NVENC
	fmt.Println()
	fmt.Println("  NVENC")
	if s.NVENC != nil && s.NVENC.Available {
		fmt.Printf("    Status:     ✓ available\n")
		if s.NVENC.GPUName != "" {
			fmt.Printf("    GPU:        %s\n", s.NVENC.GPUName)
		}
		if s.NVENC.DriverVer != "" {
			fmt.Printf("    Driver:     %s\n", s.NVENC.DriverVer)
		}
		if len(s.NVENC.DevicePaths) > 0 {
			fmt.Printf("    Devices:    %s\n", strings.Join(s.NVENC.DevicePaths, ", "))
		}
		if len(s.NVENC.Codecs) > 0 {
			fmt.Printf("    Codecs:     %s\n", formatCodecs(s.NVENC.Codecs))
		}
	} else {
		fmt.Printf("    Status:     ✗ not detected\n")
	}

	// Recommendation
	fmt.Println()
	fmt.Println(sep)
	fmt.Printf("  Recommended:  %s (%s)\n", s.Recommended, s.RecommendedCodec)
	fmt.Println()
}

func formatCodecs(codecs []encoding.Codec) string {
	seen := make(map[string]bool)
	var names []string
	for _, c := range codecs {
		if !c.Encode {
			continue
		}
		if seen[c.Name] {
			continue
		}
		seen[c.Name] = true
		names = append(names, c.Name)
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}

func printEncodingJSON(s *encoding.EncodingStatus) error {
	type codecJSON struct {
		Name    string `json:"name"`
		Profile string `json:"profile"`
	}
	type vaapiJSON struct {
		Available  bool       `json:"available"`
		Device     string     `json:"device,omitempty"`
		Driver     string     `json:"driver,omitempty"`
		Codecs     []codecJSON `json:"codecs,omitempty"`
	}
	type nvencJSON struct {
		Available  bool       `json:"available"`
		GPU        string     `json:"gpu,omitempty"`
		Driver     string     `json:"driver,omitempty"`
		Devices    []string   `json:"devices,omitempty"`
		Codecs     []codecJSON `json:"codecs,omitempty"`
	}
	type outputJSON struct {
		VAAPI       vaapiJSON `json:"vaapi"`
		NVENC       nvencJSON `json:"nvenc"`
		Recommended string   `json:"recommended"`
		Encoder     string   `json:"encoder"`
	}

	out := outputJSON{
		Recommended: s.Recommended.String(),
		Encoder:     s.RecommendedCodec,
	}

	if s.VAAPI != nil {
		out.VAAPI.Available = s.VAAPI.Available
		out.VAAPI.Device = s.VAAPI.DevicePath
		out.VAAPI.Driver = s.VAAPI.Driver
		for _, c := range s.VAAPI.Codecs {
			out.VAAPI.Codecs = append(out.VAAPI.Codecs, codecJSON{Name: c.Name, Profile: c.Profile})
		}
	}
	if s.NVENC != nil {
		out.NVENC.Available = s.NVENC.Available
		out.NVENC.GPU = s.NVENC.GPUName
		out.NVENC.Driver = s.NVENC.DriverVer
		out.NVENC.Devices = s.NVENC.DevicePaths
		for _, c := range s.NVENC.Codecs {
			out.NVENC.Codecs = append(out.NVENC.Codecs, codecJSON{Name: c.Name, Profile: c.Profile})
		}
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func init() {
	encodingCmd.Flags().Bool("json", false, "Emit JSON format")
	RootCmd.AddCommand(encodingCmd)
}
