# Analysis & Design: Go Foundation Status Management

## 1. Executive Summary
This document designs the JSON status structures, path resolution conventions, and status file read/write methods for the Go rewrite of Remote Studio (`res`). It ensures backwards compatibility with the Cinnamon Applet and legacy tools while providing a highly robust, race-condition-free, and permissions-aware implementation.

---

## 2. JSON Status Structures

The Go status module (`pkg/status`) must represent and manipulate session state. The primary representation is the `SessionStatus` structure, which matches the new system requirements.

### Go Struct Design (`pkg/status/status.go`)
```go
package status

import (
	"time"
)

// SessionStatus represents the current state of a Remote Studio session.
// It maps directly to the JSON structure expected by the daemon, WebSocket,
// and D-Bus interfaces.
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

// NewSessionStatus initializes a SessionStatus with a current timestamp.
func NewSessionStatus() *SessionStatus {
	return &SessionStatus{
		LastUpdated: time.Now().UTC(),
	}
}
```

### Validation Rules
Before writing to the status file, the system should validate fields:
*   `SessionPID`: Must be `0` if `SessionActive` is false; otherwise, must be > 0.
*   `Display`: Must follow the X11 display format (e.g., `:99` or `:1`) if active.
*   `NetworkStatus`: Enum values like `"connected"`, `"disconnected"`, `"checking"`.
*   `CPUUsage` & `MemoryUsage`: Floats bounded between `0.0` and `100.0`.

---

## 3. Path Resolution Conventions

The requirements specify resolving `/var/run/remote-studio/status.json` with a fallback to `/tmp/remote-studio/status.json`. 

### The Multi-User Collision Challenge
In multi-user Linux/macOS environments, `/tmp/remote-studio/status.json` creates a collision vulnerability. If User A runs `res` and creates `/tmp/remote-studio/status.json` with `0644` permissions, User B running `res` subsequently will fail to overwrite or write to this file, causing daemon or CLI crashes.

### Proposed Path Resolution Protocol
To resolve this cleanly:
1.  **Primary Path**: `/var/run/remote-studio/status.json` (typical daemon path).
2.  **Global Fallback Path**: `/tmp/remote-studio/status.json`.
3.  **User-Specific Fallback Path**: `/tmp/remote-studio-<uid>/status.json` (recreating legacy BATS test and CLI behavior to prevent multi-user write permissions locks).

#### Go Implementation (`pkg/status/path.go`)
```go
package status

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	PrimaryStatusPath  = "/var/run/remote-studio/status.json"
	FallbackStatusPath = "/tmp/remote-studio/status.json"
)

// GetStatusPathForRead resolves the status file path for reading.
// It checks paths in priority order:
// 1. /var/run/remote-studio/status.json
// 2. /tmp/remote-studio-<uid>/status.json
// 3. /tmp/remote-studio/status.json
func GetStatusPathForRead() string {
	if _, err := os.Stat(PrimaryStatusPath); err == nil {
		return PrimaryStatusPath
	}
	
	userSpecific := getUserSpecificFallbackPath()
	if _, err := os.Stat(userSpecific); err == nil {
		return userSpecific
	}
	
	if _, err := os.Stat(FallbackStatusPath); err == nil {
		return FallbackStatusPath
	}
	
	return PrimaryStatusPath // default return if none exists
}

// GetStatusPathForWrite resolves the status file path for writing.
// It tests write permissions dynamically to determine the best path.
func GetStatusPathForWrite() string {
	// 1. Try primary path
	if isWritable(PrimaryStatusPath) {
		return PrimaryStatusPath
	}
	
	// 2. Try global fallback
	if isWritable(FallbackStatusPath) {
		return FallbackStatusPath
	}
	
	// 3. Fall back to user-isolated path to prevent permission denial from other users
	return getUserSpecificFallbackPath()
}

// getUserSpecificFallbackPath constructs a user-isolated path similar to legacy res.sh
func getUserSpecificFallbackPath() string {
	uid := os.Getuid()
	return filepath.Join("/tmp", fmt.Sprintf("remote-studio-%d", uid), "status.json")
}

// isWritable checks if the process can write to the specified path by verifying
// directory presence and try-creating a lock/test file.
func isWritable(path string) bool {
	dir := filepath.Dir(path)
	
	// Try creating the directory if it does not exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false
	}
	
	// Test creating a file in the directory
	testFile := filepath.Join(dir, ".write_test")
	f, err := os.OpenFile(testFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	
	return true
}
```

---

## 4. Status File Read/Write Methods

Writing status files must be atomic to prevent concurrent readers (e.g. applets or CLI commands) from parsing partially written JSON.

### Atomic Write Protocol
To guarantee atomicity:
1.  Serialize the JSON to a temporary file in the same parent directory.
2.  Flush changes to the disk using `Sync()`.
3.  Close the temporary file.
4.  Rename the temporary file to the final destination path. Renaming on POSIX filesystems within the same mount point is atomic.

#### Go Implementation (`pkg/status/persistence.go`)
```go
package status

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteStatus serializes and atomically writes the SessionStatus.
func WriteStatus(status *SessionStatus) error {
	targetPath := GetStatusPathForWrite()
	dir := filepath.Dir(targetPath)

	// Ensure target directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create temp file in same directory for atomic rename
	tmpFile, err := os.CreateTemp(dir, "status-*.json.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	
	// Ensure cleanup if rename fails
	defer func() {
		if _, err := os.Stat(tmpName); err == nil {
			os.Remove(tmpName)
		}
	}()

	// Encode with indentation for human readability
	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(status); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	// Flush and sync metadata/content to disk
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set public read permissions (0644) so all users can query status via CLI/applet
	if err := os.Chmod(tmpName, 0644); err != nil {
		return fmt.Errorf("failed to set status file permissions: %w", err)
	}

	// Perform atomic replace
	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("failed to replace status file: %w", err)
	}

	return nil
}

// ReadStatus reads and deserializes the SessionStatus.
func ReadStatus() (*SessionStatus, error) {
	readPath := GetStatusPathForRead()
	
	file, err := os.Open(readPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open status file at %s: %w", readPath, err)
	}
	defer file.Close()

	var status SessionStatus
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode JSON from %s: %w", readPath, err)
	}

	return &status, nil
}
```

---

## 5. Backward Compatibility & System Integration

### Legacy Compatibility Strategy
Our investigation of `applet/applet.js` and `lib/diagnostics.sh` revealed that:
1.  The Cinnamon Applet expects a file at `$STATUS_DIR/status` (not `.json`).
2.  The applet parses both a JSON block and a pipe-separated string (`label | temp | latency | users...`).
3.  The fields in the legacy status are diagnostic/environmental (e.g. `temperature`, `warnings`, `resolution`, `codec`), whereas `SessionStatus` contains runtime performance telemetry (e.g. `cpu_usage`, `memory_usage`, `session_pid`).

To avoid breaking the Cinnamon Applet:
1.  **Twin-Write Mode**: The Go daemon should update two locations:
    - `/var/run/remote-studio/status.json` (the new `SessionStatus` for Go CLI/DBus/WebSockets).
    - `$STATUS_DIR/status` in the legacy schema format (updating either JSON or pipe-delimited format as defined in `applet/applet.js` line 199).
2.  **Unified Struct Option**: Extend `SessionStatus` or implement a separate structure `LegacyStatus` inside `pkg/status` containing all necessary fields for the Applet.

#### Proposed Legacy Status Struct
```go
type LegacyStatus struct {
	Mode          string   `json:"mode"`
	Temperature   string   `json:"temperature"`
	Latency       string   `json:"latency"`
	Users         int      `json:"users"`
	RAM           string   `json:"ram"`
	Warnings      Warnings `json:"warnings"`
	Network       string   `json:"network"`
	IP            string   `json:"ip"`
	Connection    string   `json:"connection"`
	Resolution    string   `json:"resolution"`
	DirectAddress string   `json:"direct_address"`
	Codec         string   `json:"codec"`
	StatusFile    string   `json:"status_file"`
	ActiveIPs     []string `json:"active_ips"`
}

type Warnings struct {
	Count   int    `json:"count"`
	Summary string `json:"summary"`
}
```

By providing helper write operations for both, the Go Foundation will compile and run seamlessly without interrupting applet visualization.
