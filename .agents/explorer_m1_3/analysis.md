# Analysis and Test Strategy: Go Rewrite Foundation Modules (`pkg/config` and `pkg/status`)

This document outlines the design and test strategy for unit testing the `pkg/config` and `pkg/status` packages in the Go rewrite of the Remote Studio control plane. 

---

## 1. Package `pkg/config` Unit Test Design

### 1.1 Responsibilities & Requirements
The `pkg/config` package is responsible for:
1. **Config Path Resolution**: Locating the standard configurations at `$HOME/.config/remote-studio/` and `/etc/remote-studio/`.
2. **Local Startup Configuration Parsing (`remote-studio.conf`)**:
   - Parsing simple key-value lines (`KEY=VALUE`).
   - Supporting comment lines starting with `#` and empty/blank lines.
   - Trimming whitespace around keys and values.
   - Stripping surrounding single or double quotes from values (e.g. `"mac"` or `'mac'` becomes `mac`).
   - Restricting configuration parsing or retrieval to a whitelist of supported keys (`DEFAULT_PROFILE`, `DEFAULT_SESSION_PROFILE`, `DEFAULT_RUSTDESK_PRESET`, `AUTO_SESSION`, `XORG_DRIVER`).
3. **Updating Configuration Value**:
   - Updating/replacing the value of an existing key in-place while keeping comments and the rest of the file layout intact.
   - Appending new configuration entries if they do not exist.
   - Restricting key format to `^[A-Z][A-Z0-9_]*$`.
   - Blocking value updates that contain newlines or carriage returns.
4. **Device Profile Parsing (`profiles.conf`)**:
   - Format: `key=label|width|height|scaling|text_scale|cursor`.
   - Validating line formats with a strict regex equivalent to: `^[^\|]+\|[0-9]+\|[0-9]+\|[0-9.]+\|[0-9.]+\|[0-9]+$`.
   - Parsing types: `width`, `height`, and `cursor` must be integers, while `scaling` and `text_scale` must be parsed as floating-point numbers (`float64`).
5. **Profile Sourcing & Merging**:
   - Loading default profiles from system paths (e.g. `/usr/share/remote-studio/profiles.conf` or a relative path in development).
   - Loading user-specific profiles from `$HOME/.config/remote-studio/profiles.conf`.
   - Ensuring user profiles override defaults with the same key.
6. **Profile Key Sorting**:
   - Keys must be sorted with a specific preferred order (`mac`, `mac15`, `ipad`, `ipad13`, `iphonel`, `iphonep`, `fallback`), followed by user-defined profiles in alphabetical order.
7. **Recent Profiles tracking (`recent_profiles`)**:
   - Maintaining a maximum of 5 lines, with the most recently used profile key at the top, removing duplicates, and writing the updated list.

### 1.2 Proposed Test Cases
The unit tests in `pkg/config/config_test.go` and `pkg/config/profiles_test.go` should verify the following:

#### A. Configuration Parsing (`remote-studio.conf`)
*   **Case 1: Standard Valid Configuration File**
    *   *Input*:
        ```
        # Default options
        DEFAULT_PROFILE=mac
        DEFAULT_SESSION_PROFILE="mac15"
        DEFAULT_RUSTDESK_PRESET='balanced'
        AUTO_SESSION=false
        ```
    *   *Verification*: Verify structure matches expected map/struct: `DEFAULT_PROFILE` is `"mac"`, `DEFAULT_SESSION_PROFILE` is `"mac15"`, `DEFAULT_RUSTDESK_PRESET` is `"balanced"`, `AUTO_SESSION` is `"false"`.
*   **Case 2: Extra Whitespace & Empty Lines**
    *   *Input*:
        ```
          
        DEFAULT_PROFILE   =    mac   
        
        # Sourced profile
        AUTO_SESSION = true
        ```
    *   *Verification*: Trimmed keys and values parse successfully. Empty lines and comments are correctly ignored.
*   **Case 3: Missing Config File**
    *   *Input*: File does not exist at resolved paths.
    *   *Verification*: The package does not return an error or panic; instead, it falls back to defaults (`DEFAULT_PROFILE="mac"`, `DEFAULT_RUSTDESK_PRESET="default"`, etc.).
*   **Case 4: Quoted Configuration Values**
    *   *Input*: Single-quoted and double-quoted values.
    *   *Verification*: Quotes are stripped. Ensure mismatched quotes are handled safely or left as-is depending on string format.

#### B. Configuration Modifying (`Set` operations)
*   **Case 5: Set Existing Key**
    *   *Input*: Modify `DEFAULT_PROFILE=mac` to `DEFAULT_PROFILE=ipad` in a config containing other keys and comments.
    *   *Verification*: The file is written with the updated value, preserving comments and other settings, avoiding duplication.
*   **Case 6: Set New Key**
    *   *Input*: Add `AUTO_SESSION=true` when it does not exist.
    *   *Verification*: Appends the line `AUTO_SESSION=true` to the end of the file.
*   **Case 7: Key Validation**
    *   *Input*: Attempt to set a key like `bad-key`, `DEFAULT.PROFILE`, or empty string.
    *   *Verification*: Exits with validation error.
*   **Case 8: Value Validation (Newline Guard)**
    *   *Input*: Attempt to write a value containing newlines (`value\nwith\nnewlines`).
    *   *Verification*: Returns validation error.

#### C. Profile Parsing (`profiles.conf`)
*   **Case 9: Valid Profiles File**
    *   *Input*:
        ```
        mac=MacBook Pro|2560|1664|2.0|1.0|24
        ipad=iPad Pro|2048|2732|2.0|1.2|32
        ```
    *   *Verification*: Successfully parsed into profile map/structs. Check numeric conversions: `mac.Width == 2560`, `mac.Scaling == 2.0`, `mac.Cursor == 24`.
*   **Case 10: Invalid/Malformed Lines**
    *   *Input*: Lines with too few fields (e.g. `mac=MacBook|2560|1664|2.0`), non-numeric dimensions (`mac=MacBook|abc|1664|2.0|1.0|24`), or floating-point values where integers are expected.
    *   *Verification*: Parse errors/warnings are returned or lines are skipped; it does not crash or panic.
*   **Case 11: System & User Profile Merging**
    *   *Input*: Mock system profile file with keys `mac`, `ipad`, `fallback`. Mock user profile file with keys `mac` (different settings) and `custom_monitor`.
    *   *Verification*: Merged profile set contains `mac` (user version), `ipad` (system version), `fallback` (system version), and `custom_monitor` (user version).

#### D. Profile Operations
*   **Case 12: Profile Sorter**
    *   *Input*: Profile map with keys `fallback`, `ipad`, `mac`, `iphonel`, `custom1`, `z_monitor`, `mac15`.
    *   *Verification*: Sorted result matches exactly: `mac`, `mac15`, `ipad`, `iphonel`, `fallback`, `custom1`, `z_monitor`.
*   **Case 13: Recent Profiles Lifecycle**
    *   *Input*: Adding a key to a list of recent profiles.
    *   *Verification*:
        - If new: prepended to list.
        - If existing: moved to the top.
        - Cap at 5: excess entries at the bottom are discarded.
        - File output matches exact structure.

### 1.3 Mock Files and Test Helpers
To test file operations without altering the developer's local environment, the tests will employ **Isolation through Temp Directories**:
*   Using `t.TempDir()`: Each test run gets a unique directory mimicking `$HOME` or `/etc`.
*   **Environment Injections**: The config package should read directory paths from an internal structure that can be overridden during tests, or from custom environment variables (e.g. `REMOTE_STUDIO_TEST_HOME` or `REMOTE_STUDIO_TEST_ETC`).
*   No external mock framework (like GoMock) is required. The standard filesystem operations via the `os` package can be fully validated using the local temp directory.

---

## 2. Package `pkg/status` Unit Test Design

### 2.1 Responsibilities & Requirements
The `pkg/status` package is responsible for:
1. **JSON Serialization & Deserialization**:
   - Matching the JSON schema of the `SessionStatus` struct (fields such as `session_active`, `session_pid`, `display`, `profile`, `network_status`, `cpu_usage`, `memory_usage`, `last_updated`).
   - Custom time formatting for `last_updated` (RFC3339 compatibility).
2. **Writable Path Resolution**:
   - Primary target: `/var/run/remote-studio/status.json`.
   - Fallback target: `/tmp/remote-studio/status.json` (if primary path is unwritable or doesn't exist and can't be created).
3. **Atomic Writing**:
   - Ensuring `status.json` is not read in a partially-written state.
   - This must write status to a temporary file in the target directory (e.g. `status.json.tmp`) and then atomically rename it to `status.json`.
4. **Read Status**:
   - Checking primary then fallback directories.
   - Graceful handling of missing files (returning inactive state or structured errors).

### 2.2 Proposed Test Cases
The unit tests in `pkg/status/status_test.go` should verify the following:

#### A. JSON Schema & Types
*   **Case 1: Correct Serialization Structure**
    *   *Input*: Struct with active session, specific PID, CPU/Memory percentages, and a defined time.
    *   *Verification*: Ensure output JSON matches the legacy API contract precisely. Key names must be snake_case (e.g. `session_active`, `cpu_usage`).
*   **Case 2: Correct Deserialization**
    *   *Input*: Raw valid JSON string.
    *   *Verification*: Verify struct fields are populated accurately (e.g. floating point numbers, booleans, and datetime).

#### B. Path Resolution & Writable Conventions
*   **Case 3: Primary Path Writable**
    *   *Setup*: Both simulated primary (`/var/run`) and fallback (`/tmp`) paths are writable (mocked in test temp directories).
    *   *Verification*: Status must be written to the primary path.
*   **Case 4: Primary Path Not Writable**
    *   *Setup*: Simulating a write-restricted primary path (e.g., directory permission `0400` or a non-existent directory that cannot be created).
    *   *Verification*: Status must fall back to the `/tmp` path directory. The operation must succeed.
*   **Case 5: Fallback Path Not Writable**
    *   *Setup*: Both simulated paths are write-restricted.
    *   *Verification*: Writing status returns an explicit, handled error.

#### C. Atomic File Operations
*   **Case 6: Atomic Rename Verification**
    *   *Verification*: Inspect execution of the write command to verify that `status.json` is written via a temp file followed by a rename (`os.Rename`), protecting concurrent readers from reading blank or partial files.
*   **Case 7: Directory Auto-Creation**
    *   *Setup*: The parent directories (like `/var/run/remote-studio`) do not exist yet.
    *   *Verification*: Ensure they are automatically created with proper permissions (e.g., `0755` or `0700` appropriate for runtime files).

#### D. Read Operations & Corrupted Data
*   **Case 8: Read When File is Missing**
    *   *Setup*: No status files exist.
    *   *Verification*: Return a structured status representing "no active session" (or an expected sentinel error) instead of crashing.
*   **Case 9: Read Corrupted/Malformed JSON**
    *   *Setup*: Write random, invalid bytes to the status file.
    *   *Verification*: Return a clear JSON parsing error.

### 2.3 Mock Files and Test Helpers
*   **Path Injection Support**: In `status.go`, define a variable for candidate status directories that can be overridden in testing:
    ```go
    // pkg/status/status.go
    var statusDirCandidates = []string{
        "/var/run/remote-studio",
        "/tmp/remote-studio",
    }
    ```
*   In `status_test.go`, override these candidates using folders inside `t.TempDir()`.
*   **Write Permission Mocks**:
    - Under Unix/Mac, we can simulate an unwritable directory by creating it with permission `0400` (read-only) or `0000`. This allows native validation of the fallback logic without mock interfaces.

---

## 3. Execution & Validation Strategy

1. **Test Commands**:
   - Run tests for config package: `go test -v ./pkg/config/...`
   - Run tests for status package: `go test -v ./pkg/status/...`
   - Run all package tests: `go test -v ./...`
2. **Lint Checks**:
   - Run `golangci-lint run` to ensure test files comply with code guidelines (e.g. error checks, unused variables).
3. **Parity Check**:
   - Unit tests are designed to explicitly verify the BATS CLI behavior, ensuring that empty configs, quoted strings, and sorting requirements behave identically in Go.
