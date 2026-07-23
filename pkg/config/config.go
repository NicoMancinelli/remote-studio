package config

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

type Config struct {
	Values map[string]string
}

func NewConfig() *Config {
	return &Config{Values: make(map[string]string)}
}

var keyRegex = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := NewConfig()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if !keyRegex.MatchString(key) {
			continue
		}
		if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
			(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			val = val[1 : len(val)-1]
		}
		cfg.Values[key] = val
	}
	return cfg, scanner.Err()
}

func FindAndLoadConfig() (*Config, string, error) {
	path := ResolveConfigPath()
	cfg, err := LoadConfig(path)
	if err != nil {
		return NewConfig(), path, nil
	}
	return cfg, path, nil
}

// Version is the remote-studio release version. The default ("9.1") is
// used when the binary is built without the `-ldflags` injection; the
// release Makefile target injects the same value from res.sh so the
// two sources can't drift.
//
// To inject at build time:
//
//   go build -ldflags "-X remote-studio/pkg/config.Version=9.1.2" .
//
// `cmd/version` reads this so `./res version` and `make release-check`
// agree on the number; `pkg/diagnostics` uses it as the expected tag
// when comparing against the latest GitHub release.
var Version = "9.1"

func (c *Config) GetConfigValue(key string) string {
	return c.Values[key]
}

func (c *Config) SetConfigValue(key, value string) {
	c.Values[key] = value
}
