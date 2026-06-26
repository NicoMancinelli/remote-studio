# Go Rewrite Foundation — Analysis and Strategy Report

## 1. Executive Summary
This report defines the structural layout and core configuration loading logic for the Go rewrite of the Remote Studio control plane (`res`). The objective of this phase (Milestone 1) is to establish a robust, modern Go foundation, focusing specifically on:
* A scalable, clean package structure.
* Secure and correct config loading rules for `remote-studio.conf`.
* High-fidelity profile registry loading, validation, merging, and sorting for `profiles.conf`.
* Thread-safe session status management and applet status updates.

No Go code is implemented in this phase, preserving read-only investigation rules. All proposed structures and algorithms are designed to match legacy shell behavior exactly while eliminating shell-based security issues and fragility.

---

## 2. Proposed Go Package Structure
A single Go module `github.com/nicomancinelli/remote-studio` is proposed. The project directory structure follows the standard Go project layout, dividing code into `cmd/` (command-line interface parsing via Cobra) and `pkg/` (reusable library code).

```
remote-studio/
├── go.mod                # Module declaration (Go 1.21+)
├── go.sum
├── main.go               # Entry point (initializes CLI and runs cmd.Execute())
├── cmd/                  # CLI commands (Cobra definitions)
│   ├── root.go           # Base command, shared settings, persistent flags
│   ├── status.go         # res status [-j|--json]
│   ├── info.go           # res info
│   ├── log.go            # res log
│   ├── doctor.go         # res doctor
│   ├── session.go        # res session [start|stop|restart|attach]
│   ├── rotate.go         # res rotate
│   ├── profiles.go       # res profiles [list|set]
│   ├── config.go         # res config [show|get|set]
│   └── daemon.go         # res daemon
└── pkg/                  # Domain packages
    ├── config/           # Configuration and Profiles sub-system
    │   ├── config.go     # Parser and manager for remote-studio.conf
    │   ├── profile.go    # Parser and registry for profiles.conf
    │   └── paths.go      # Shared configuration and profiles path resolution
    ├── status/           # Persistence & serialization of session status
    │   ├── status.go     # SessionStatus management (/var/run/remote-studio/status.json)
    │   └── applet.go     # Applet status file updates ($XDG_RUNTIME_DIR/remote-studio/status)
    ├── diagnostics/      # Diagnostics check rules (res doctor equivalent)
    │   └── doctor.go     # Check implementations (command existence, ports, Tailscale status)
    ├── session/          # Session, displays, and process management
    │   ├── session.go    # Session processes (Xorg/Wayland), modeline generation
    │   └── manager.go    # Display-switch coordinator
    └── daemon/           # Long-running daemon backend services
        ├── dbus.go       # D-Bus org.remote_studio.Daemon listener
        ├── websocket.go  # Port 9998 telemetry broadcaster
        ├── http.go       # Port 9999 dashboard host
        └── poll.go       # Network/ping thread (pings 8.8.8.8)
```

---

## 3. Configuration Management (`remote-studio.conf`)

### A. Path Resolution Rules
When the application starts, it must search for the user configuration file at:
1. **User-level configuration**: `$HOME/.config/remote-studio/remote-studio.conf` (or resolved via `os.UserConfigDir()` to ensure compliance with XDG specifications).
2. **System-level configuration**: `/etc/remote-studio/remote-studio.conf` (fallback if the user configuration does not exist or is unreadable).
3. **Internal Default Fallbacks**: If neither file exists, default values are assigned at runtime.

### B. Format & Supported Keys
The configuration file is a simple line-based key-value file using the `KEY=VALUE` format. The following standard keys are recognized:
* `DEFAULT_PROFILE` (default: `"mac"`)
* `DEFAULT_SESSION_PROFILE` (default: same as `DEFAULT_PROFILE`)
* `DEFAULT_RUSTDESK_PRESET` (default: `"default"`)
* `AUTO_SESSION` (default: `false` - parsed as boolean)
* `XORG_DRIVER` (default: `"nvidia"`)

To allow for extensibility (e.g., custom variables set via `res config set KEY VALUE`), the system must parse and retain *all* valid keys in a raw configuration map.
* **Key Validation**: Keys must match the regular expression `^[A-Z][A-Z0-9_]*$`.
* **Value Constraints**: Values must not contain newline (`\n`) or carriage return (`\r`) characters.

### C. Safe Parsing Algorithm
Legacy bash configuration parsing was vulnerable to command injection if a configuration value contained subshells (e.g., `DEFAULT_PROFILE=$(touch flag)`). The Go parser must avoid execution of any external commands.
1. Read the file line-by-line using a scanner (e.g. `bufio.NewScanner`).
2. Trim leading/trailing whitespace from each line.
3. Skip empty lines or lines starting with `#` (comments).
4. Locate the first occurrence of `=` on the line. If not found, skip the line or log a warning.
5. Extract the key (before `=`) and value (after `=`), trimming whitespace from both.
6. Validate the key format using the regex: `^[A-Z][A-Z0-9_]*$`.
7. Clean up the value:
   * If the value is enclosed in double quotes (`"..."`) or single quotes (`'...'`), remove the outer pair.
   * If the key is `AUTO_SESSION`, parse it to a boolean: `"true"`, `"1"`, `"yes"`, `"on"` map to `true`; other values map to `false`.
8. Assign values to both the structured fields and a `RawSettings map[string]string` map.

### D. Safe Writing/Updating Mechanism
When setting a configuration key via `res config set KEY VALUE`:
1. Read the existing configuration file into memory line-by-line, preserving comments, empty lines, and order.
2. Search for a line starting with the target key followed by `=`.
3. If found, replace the line with `KEY=VALUE`.
4. If not found, append `KEY=VALUE` at the end of the line collection.
5. Create a temporary file in the same directory (e.g. `$HOME/.config/remote-studio/remote-studio.conf.tmp`).
6. Write all lines to the temporary file, ensuring permissions are set to `0644`.
7. Atomically rename the temporary file to `remote-studio.conf` (using `os.Rename`).
8. If the configuration directory does not exist, create it with `0755` permissions beforehand.

---

## 4. Device Profiles Registry (`profiles.conf`)

### A. Path Resolution Rules
Profiles must be loaded in the following order to implement "last-wins" customization:
1. **Built-in / Default Profiles**:
   - Check the directory where the current executable is running (via `os.Executable()`). If a subdirectory `config/profiles.conf` exists relative to the executable path, load it.
   - If not found, fall back to the system default path: `/usr/share/remote-studio/profiles.conf`.
2. **User Profiles**:
   - Load from `$HOME/.config/remote-studio/profiles.conf` (if it exists).
   - Any profile defined here with a key matching a built-in profile will override the built-in definition.

### B. Format & Parser Specification
Each line in `profiles.conf` represents a device profile in the format:
`key=label|width|height|scaling|text_scale|cursor`

The Go `Profile` struct is defined as:
```go
type Profile struct {
    Key       string  `json:"key"`
    Label     string  `json:"label"`
    Width     int     `json:"width"`
    Height    int     `json:"height"`
    Scaling   float64 `json:"scaling"`
    TextScale float64 `json:"text_scale"`
    Cursor    int     `json:"cursor"`
}
```

**Parsing Steps**:
1. Scan the file line-by-line, skipping blank lines and lines starting with `#`.
2. Split the line into `key` and `value` by the first `=` character.
3. Validate that `key` does not contain space or pipe `|` characters.
4. Split the `value` by the pipe symbol `|`. It must contain exactly **6 fields** (5 pipe delimiters).
5. Parse fields:
   - Field 0 (`label`): String. Must not be empty.
   - Field 1 (`width`): Parse as `int`. Must be `> 0`.
   - Field 2 (`height`): Parse as `int`. Must be `> 0`.
   - Field 3 (`scaling`): Parse as `float64`. Must be `> 0.0`.
   - Field 4 (`text_scale`): Parse as `float64`. Must be `> 0.0`.
   - Field 5 (`cursor`): Parse as `int`. Must be `> 0`.
6. If any line is invalid, print a warning to `stderr` with the malformed line content, but do not fail the overall profile loading process (log-and-skip).

### C. Registry Management
The `ProfileRegistry` manages the set of loaded profiles:
```go
type ProfileRegistry struct {
    Profiles map[string]Profile
    Keys     []string // Maintains insertion order for deduplication
}
```
* Built-in profiles are loaded first and populated in the registry.
* User-defined profiles are loaded next. If a user profile overrides an existing key, the registry updates the map value and keeps the original key order. If it's a new key, it is added to the map and appended to `Keys`.

### D. Preferred Ordering Rules
When displaying profiles in the CLI (`res profiles` or `res help`), they must follow a specific layout:
1. Hardcoded preferred keys order (if defined): `mac`, `mac15`, `ipad`, `ipad13`, `iphonel`, `iphonep`, `fallback`.
2. All other user-defined or additional profiles sorted alphabetically.

**Sorting Algorithm**:
```go
func SortProfileKeys(registry *ProfileRegistry) []string {
    preferred := []string{"mac", "mac15", "ipad", "ipad13", "iphonel", "iphonep", "fallback"}
    seen := make(map[string]bool)
    result := make([]string, 0)

    // First, append preferred keys that exist
    for _, k := range preferred {
        if _, exists := registry.Profiles[k]; exists {
            result = append(result, k)
            seen[k] = true
        }
    }

    // Collect all remaining keys
    remaining := make([]string, 0)
    for k := range registry.Profiles {
        if !seen[k] {
            remaining = append(remaining, k)
        }
    }
    // Sort remaining keys alphabetically
    sort.Strings(remaining)

    // Append remaining keys
    result = append(result, remaining...)
    return result
}
```

---

## 5. Status Persistence & Serialization (`pkg/status`)

To bridge the gap between CLI commands, the background daemon, and the desktop Cinnamon applet, two status components must be managed.

### A. Session Status (`SessionStatus`)
Represents the current virtual display session state. The structure matches the legacy daemon:
```go
type SessionStatus struct {
    SessionActive bool      `json:"session_active"`
    SessionPID    int       `json:"session_pid"`
    Display       string    `json:"display"`
    Profile       string    `json:"profile"`
    NetworkStatus string    `json:"network_status"`
    CPUUsage      float64   `json:"cpu_usage"`
    MemoryUsage   float64   `json:"memory_usage"`
    LastUpdated   time.Time `json:"last_updated"`
}
```

* **Storage Path**:
  1. `/var/run/remote-studio/status.json` (tried first; writable by daemon running as root or a system user).
  2. `/tmp/remote-studio/status.json` (fallback).
* **DBus Signal Integration**: When `SessionStatus` changes, the daemon broadcasts the JSON-serialized string of this struct via the `StatusChanged` signal on D-Bus interface `org.remote_studio.Daemon`.

### B. Applet TUI/Health Status (`STATUS_FILE`)
Used by the Cinnamon desktop applet for panel displaying.
* **Storage Path**:
  - If `$XDG_RUNTIME_DIR` is set and writable, use `$XDG_RUNTIME_DIR/remote-studio/status`.
  - Fallback: `/tmp/remote-studio-<uid>/status` (where `<uid>` is resolved via `os.Getuid()`).
* **Format**:
  Must be written as a single line containing pipe-delimited values (with `" | "` separators):
  `mode | temp | latency | users | ram | warnings | warningText | traffic | ip | connection | resolution | direct_address | codec`
  *Note: The Go implementation should also support writing this file as a JSON block starting with `{` if needed, as the Cinnamon applet supports parsing both formats.*

---

## 6. Implementation Strategy & Recommendations

1. **Third-Party Dependencies**:
   * Use `github.com/spf13/cobra` for CLI command and flag parsing.
   * Use `github.com/godbus/dbus/v5` for D-Bus communication.
2. **Concurrency Safety**:
   * The daemon and CLI commands will read/write the status and configuration files simultaneously. Use file locking (e.g. `syscall.Flock` or `github.com/gofrs/flock`) when editing `remote-studio.conf` or updating `status.json`.
3. **Environment Isolation**:
   * Ensure paths such as `$HOME` and `$XDG_RUNTIME_DIR` are queried at runtime. In test suites, tests can override `HOME` to a temporary directory to verify path resolution behavior.
4. **Error Handling**:
   * When loading files, check if an error is `os.ErrNotExist`. If so, handle it gracefully by falling back to the next path or defaults, rather than returning fatal errors.
