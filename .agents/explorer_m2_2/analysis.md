# CLI Subcommands Analysis and Design — config & profiles

## Executive Summary
This report defines the design, validation rules, and integration details for the `config` and `profiles` CLI subcommands in the Go rewrite (`res`).
Key findings include a bug in the existing `pkg/config/profile.go` implementation where default profiles are completely ignored if a user profiles file exists. We design the corrections for this, along with strict validation rules for configuration keys (`^[A-Z][A-Z0-9_]*$`), profile keys (`^[a-z][a-z0-9_-]*$`), and robust atomic file writing protocols to prevent configuration corruption.

---

## 1. Requirements and Legacy Behavior Mapping

### A. Config Subcommand (`res config [show | get KEY | set KEY VALUE]`)
1. **`show`**:
   - Prints the effective configuration values.
   - Shows the 4 standard keys: `DEFAULT_PROFILE`, `DEFAULT_SESSION_PROFILE`, `DEFAULT_RUSTDESK_PRESET`, and `AUTO_SESSION`.
   - In Go, we also list any custom keys defined by the user in the config map.
   - Prints path info at the bottom: `# User config: <path>` or `# No user config file`.
2. **`get KEY`**:
   - Validates that `KEY` matches `^[A-Z][A-Z0-9_]*$`.
   - Prints the raw value to stdout without a trailing newline.
   - If the key is not found, it prints nothing and exits with code 0 (matching legacy bash behavior).
3. **`set KEY VALUE`**:
   - Validates that `KEY` matches `^[A-Z][A-Z0-9_]*$`.
   - Validates that `VALUE` has no newlines (`\n` or `\r`).
   - Writes key-value atomically to `$USER_CONFIG`.
   - Outputs: `Set KEY=VALUE in <path>` and appends to event log.

### B. Profiles Subcommand (`res profiles [list | set KEY VALUE]`)
1. **`list`** (equivalent to legacy `res profiles`):
   - Reads default profiles and user profiles (overrides).
   - Resolves the currently active profile label by parsing `$HOME/.res_state`.
   - Prints the list of profiles in columns:
     - KEY (width 12)
     - LABEL (width 26)
     - RESOLUTION (width 14, format: `WidthxHeight@Scaling`)
     - SOURCE (either `default` or `user`)
     - Active marker (` *` appended if the profile's label matches the active mode).
2. **`set KEY VALUE`**:
   - Validates that `KEY` matches `^[a-z][a-z0-9_-]*$`.
   - `VALUE` can be passed as a single pipe-delimited string (`label|width|height|scaling|text_scale|cursor`) or via flags (`--label`, `--width`, etc.).
   - Values must contain exactly 6 fields, and numeric fields must be validated.
   - Saves profile atomically to the user's custom `profiles.conf`.

---

## 2. Key Constraints & Validations

1. **Config Key Regex**: `^[A-Z][A-Z0-9_]*$`
   - Verified via `regexp.MustCompile` in Go.
2. **Profile Key Regex**: `^[a-z][a-z0-9_-]*$`
   - Standard lowercase identifier format used to match built-in profiles.
3. **Config Value Constraint**: No newlines or carriage returns.
4. **Profile Value Constraints**:
   - Split by `|` must yield exactly 6 fields.
   - Field 0 (`label`): non-empty, no pipe characters.
   - Field 1 (`width`): positive integer.
   - Field 2 (`height`): positive integer.
   - Field 3 (`scaling`): positive float64.
   - Field 4 (`text_scale`): positive float64.
   - Field 5 (`cursor`): positive integer.

---

## 3. Atomic Writing Protocol

To prevent file corruption during write operations:
1. **Resolve Path**: Find the target config/profile file in `$HOME/.config/remote-studio/`.
2. **Mkdir**: Create parent directories with `0755` permissions if they do not exist.
3. **Temp File**: Create a temporary file in the same directory using `os.CreateTemp` (e.g. `remote-studio.conf.tmp.*`).
4. **Write**: Update or append keys, writing all lines to the temp file. Keep file permissions `0644`.
5. **Flush & Sync**: Flush any buffered writes and call `.Sync()` to commit to disk.
6. **Close**: Close the temp file.
7. **Atomic Rename**: Move the temp file to the target path using `os.Rename`.
8. **Defer Cleanup**: Ensure that the temp file is deleted on error/interrupt.

---

## 4. Go Integration Design & Proposed Changes

### A. Fix `pkg/config/profile.go` Load Logic
Currently, `pkg/config/paths.go` and `profile.go` resolve and load only a single profiles file. If user profiles exist, default profiles are completely lost.

**Proposed Changes to `pkg/config/profile.go`**:
```go
// Add IsUser field to Profile (ignored in JSON serialization)
type Profile struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Scaling   float64 `json:"scaling"`
	TextScale float64 `json:"text_scale"`
	Cursor    int     `json:"cursor"`
	IsUser    bool    `json:"-"`
}

// ResolveDefaultProfilesPath returns the default system/exec path
func ResolveDefaultProfilesPath() string {
	execPath, err := os.Executable()
	if err == nil {
		p := filepath.Join(filepath.Dir(execPath), "config", "profiles.conf")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "/usr/share/remote-studio/profiles.conf"
}

// ResolveUserProfilesPath returns the user-level path in $HOME
func ResolveUserProfilesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "remote-studio", "profiles.conf"), nil
}

// LoadAllProfiles loads default profiles and merges user overrides
func LoadAllProfiles() (*ProfileRegistry, error) {
	reg := NewProfileRegistry()
	
	// Load default system profiles
	defaultPath := ResolveDefaultProfilesPath()
	_ = reg.LoadProfiles(defaultPath) // isUser remains false

	// Merge user overrides
	userPath, err := ResolveUserProfilesPath()
	if err == nil {
		userReg := NewProfileRegistry()
		if err := userReg.LoadProfiles(userPath); err == nil {
			for k, p := range userReg.Profiles {
				p.IsUser = true
				reg.Profiles[k] = p
			}
		}
	}
	
	return reg, nil
}
```

### B. Add Atomic Write Helpers to `pkg/config`
```go
// In pkg/config/config.go

func IsValidConfigKey(key string) bool {
	return keyRegex.MatchString(key)
}

func WriteConfig(path string, key, value string) error {
	if !IsValidConfigKey(key) {
		return fmt.Errorf("invalid config key: %s", key)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("invalid config value: newlines are not supported")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var lines []string
	found := false
	keyUpper := strings.ToUpper(key)

	file, err := os.Open(path)
	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				lines = append(lines, line)
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				lines = append(lines, line)
				continue
			}
			k := strings.TrimSpace(parts[0])
			if strings.ToUpper(k) == keyUpper {
				lines = append(lines, fmt.Sprintf("%s=%s", key, value))
				found = true
			} else {
				lines = append(lines, line)
			}
		}
		file.Close()
	}

	if !found {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	// Atomic write
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	if err := tmpFile.Chmod(0644); err != nil {
		return err
	}

	writer := bufio.NewWriter(tmpFile)
	for _, line := range lines {
		writer.WriteString(line + "\n")
	}
	writer.Flush()
	tmpFile.Sync()
	tmpFile.Close()

	return os.Rename(tmpPath, path)
}
```

```go
// In pkg/config/profile.go

var profileKeyRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func IsValidProfileKey(key string) bool {
	return profileKeyRegex.MatchString(key)
}

func (p *Profile) Validate() error {
	if !IsValidProfileKey(p.Key) {
		return fmt.Errorf("invalid key — use only a-z, 0-9, _, - and start with a letter.")
	}
	if p.Label == "" || strings.Contains(p.Label, "|") {
		return fmt.Errorf("label cannot be empty or contain pipe '|' character")
	}
	if p.Width <= 0 || p.Height <= 0 || p.Scaling <= 0 || p.TextScale <= 0 || p.Cursor <= 0 {
		return fmt.Errorf("numeric dimensions and scale must be greater than 0")
	}
	return nil
}

func WriteProfile(path string, p Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var lines []string
	found := false
	profileLine := fmt.Sprintf("%s=%s|%d|%d|%g|%g|%d", p.Key, p.Label, p.Width, p.Height, p.Scaling, p.TextScale, p.Cursor)

	file, err := os.Open(path)
	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				lines = append(lines, line)
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				lines = append(lines, line)
				continue
			}
			k := strings.TrimSpace(parts[0])
			if k == p.Key {
				lines = append(lines, profileLine)
				found = true
			} else {
				lines = append(lines, line)
			}
		}
		file.Close()
	}

	if !found {
		lines = append(lines, profileLine)
	}

	// Atomic write
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	if err := tmpFile.Chmod(0644); err != nil {
		return err
	}

	writer := bufio.NewWriter(tmpFile)
	for _, line := range lines {
		writer.WriteString(line + "\n")
	}
	writer.Flush()
	tmpFile.Sync()
	tmpFile.Close()

	return os.Rename(tmpPath, path)
}
```

### C. Add Active Mode / State Parsing and Logging helper
```go
// In pkg/config/paths.go

// GetCurrentMode reads the active profile label from the state file
func GetCurrentMode() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "None"
	}
	stateFilePath := filepath.Join(home, ".res_state")
	
	file, err := os.Open(stateFilePath)
	if err != nil {
		return "None"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()
		start := strings.Index(line, "'")
		end := strings.LastIndex(line, "'")
		if start >= 0 && end > start {
			return line[start+1 : end]
		}
	}
	return "None"
}

// LogEvent logs a custom event to ~/.remote_studio.log, rotating if size > 1MB
func LogEvent(message string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	logPath := filepath.Join(home, ".remote_studio.log")

	info, err := os.Stat(logPath)
	if err == nil && info.Size() > 1048576 {
		_ = os.Rename(logPath, logPath+".1")
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err = fmt.Fprintf(file, "[%s] %s\n", timestamp, message)
	return err
}
```
