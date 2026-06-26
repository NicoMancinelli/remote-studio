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

func (c *Config) GetConfigValue(key string) string {
	return c.Values[key]
}

func (c *Config) SetConfigValue(key, value string) {
	c.Values[key] = value
}
