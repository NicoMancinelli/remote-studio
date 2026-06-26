package config

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Profile struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Scaling   float64 `json:"scaling"`
	TextScale float64 `json:"text_scale"`
	Cursor    int     `json:"cursor"`
}

type ProfileRegistry struct {
	Profiles map[string]Profile
}

func NewProfileRegistry() *ProfileRegistry {
	return &ProfileRegistry{Profiles: make(map[string]Profile)}
}

func ParseProfileLine(line string) (*Profile, error) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid format")
	}
	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	fields := strings.Split(val, "|")
	if len(fields) != 6 {
		return nil, fmt.Errorf("invalid field count")
	}
	w, err := strconv.Atoi(fields[1])
	if err != nil || w <= 0 {
		return nil, fmt.Errorf("invalid width")
	}
	h, err := strconv.Atoi(fields[2])
	if err != nil || h <= 0 {
		return nil, fmt.Errorf("invalid height")
	}
	s, err := strconv.ParseFloat(fields[3], 64)
	if err != nil || s <= 0 {
		return nil, fmt.Errorf("invalid scaling")
	}
	ts, err := strconv.ParseFloat(fields[4], 64)
	if err != nil || ts <= 0 {
		return nil, fmt.Errorf("invalid text_scale")
	}
	c, err := strconv.Atoi(fields[5])
	if err != nil || c <= 0 {
		return nil, fmt.Errorf("invalid cursor")
	}
	return &Profile{
		Key:       key,
		Label:     fields[0],
		Width:     w,
		Height:    h,
		Scaling:   s,
		TextScale: ts,
		Cursor:    c,
	}, nil
}

func (r *ProfileRegistry) LoadProfiles(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p, err := ParseProfileLine(line)
		if err != nil {
			continue
		}
		r.Profiles[p.Key] = *p
	}
	return scanner.Err()
}

func LoadAllProfiles() (*ProfileRegistry, error) {
	reg := NewProfileRegistry()
	path, _ := ResolveProfilesPath()
	_ = reg.LoadProfiles(path)
	return reg, nil
}

func SortProfileKeys(registry *ProfileRegistry) []string {
	preferred := []string{"mac", "mac15", "ipad", "ipad13", "iphonel", "iphonep", "fallback"}
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, k := range preferred {
		if _, exists := registry.Profiles[k]; exists {
			result = append(result, k)
			seen[k] = true
		}
	}

	remaining := make([]string, 0)
	for k := range registry.Profiles {
		if !seen[k] {
			remaining = append(remaining, k)
		}
	}
	sort.Strings(remaining)
	result = append(result, remaining...)
	return result
}
