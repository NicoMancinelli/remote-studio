# Review Handoff Report

## 1. Observation
We ran the E2E tests using `go test -v ./tests/e2e/...` in the workspace `/Users/nico/Library/Mobile Documents/com~apple~CloudDocs/Dev/Code/Github/Remote-Studio`.

We observed the following test failures:

- **TestSanity Case Sensitivity**:
  ```
  e2e_test.go:189: Expected output to contain 'remote-studio', got: "Remote Studio CLI is a control plane for remote session management.\n\nUsage:\n  res [command]..."
  ```
  Line 188 in `tests/e2e/e2e_test.go` queries:
  ```go
  if !strings.Contains(stdout, "remote-studio")
  ```

- **TestTier1_F1_StatusText Formatting Mismatch**:
  ```
  test_cases_tier1_test.go:60: Expected stdout to contain "Mode:", but got: "None |  | … | 0 | 0.0% | 2 | tailscale,applet-symlink | n/a | 100.1.2.3/127.0.0.1 | None | N/A | 100.1.2.3:21118 | none\n"
  ```
  Line 60 in `tests/e2e/test_cases_tier1_test.go` asserts:
  ```go
  verifyCmdRun(t, []string{"status"}, nil, "Mode:", "", false)
  ```

- **Missing Test Setup for Dynamic Profiles**:
  ```
  test_cases_tier1_test.go:138: Expected command 'res session start mac' to succeed, but got error: exit status 1. Stdout: "failed to apply profile 'mac': profile 'mac' not found\n"
  ```
  And similarly:
  ```
  test_cases_tier1_test.go:95: Expected command 'res mac' to succeed, but got error: exit status 1. Stdout: "unknown command \"mac\" for \"res\"\n"
  ```
  The E2E tests redirect the `HOME` environment variable to `IsolatedHome` but do not create the `profiles.conf` file in `IsolatedHome/.config/remote-studio/profiles.conf`, nor copy the default `config/profiles.conf` to that location.

- **D-Bus Daemon Availability Mismatch**:
  Multiple tests skip or fail when running on macOS due to the absence of `dbus-daemon` in the system `PATH`:
  ```
  dbus-daemon not found in PATH, skipping private D-Bus daemon setup.
  ```
  And when starting the daemon, it fails with:
  ```
  Error: daemon failure: DBus session bus address is missing
  ```
  This causes all daemon-dependent tests (including Web/WebSocket tests) to fail with `connection refused` on `localhost:9999` and `localhost:9998`.

- **Incorrect Exit Code / Stderr Assertions**:
  ```
  test_cases_tier2_test.go:203: Expected command 'res tailnet' to fail, but it succeeded. Stdout: "Tailscale IPv4 unavailable.\n", Stderr: ""
  test_cases_tier2_test.go:203: Expected stderr to contain "unavailable", but got: ""
  ```
  And:
  ```
  test_cases_tier2_test.go:608: Expected command 'res rustdesk diff invalid_preset' to fail, but it succeeded. Stdout: "Missing files (preset: invalid_preset).\n", Stderr: ""
  ```

- **Platform-Specific Path Assumptions**:
  ```
  test_cases_tier2_test.go:680: Expected stderr to contain "permission denied", but got: "Error: open /etc/X11/xorg.conf.nonwritable.test: no such file or directory\n"
  ```

- **GPU Driver Logic Bug**:
  ```
  test_cases_tier2_test.go:705: Expected fallback modesetting driver, got: Section "Device"
          Identifier "Configured Video Device"
          Driver "amdgpu"
      EndSection
  ```
  Line 108 of `cmd/xorg.go` checks:
  ```go
  } else if strings.Contains(lspciLower, "amd") || strings.Contains(lspciLower, "ati") || strings.Contains(lspciLower, "radeon") {
  ```
  Since the test mock output contains `"Unknown GPU Corporation"`, and `"corporation"` contains `"ati"`, it incorrectly detects the AMD driver (`amdgpu`).

- **Missing Implementation Features**:
  `TestTier2_F9_RotateBackupPruning` fails because `cmd/xorg.go` lacks backup pruning logic (it found 18 backups instead of capping at 10).

---

## 2. Logic Chain
1. **Broken Test Setup**: Since `IsolatedHome` does not contain any `profiles.conf` file, `loadAllProfilesCombined` returns an empty profile registry. As a result, all tests verifying session start/stop and mode activation for `mac` profile fail because the profile is missing.
2. **Incorrect Test Assertions**: Several tests assert case-sensitive strings or format-specific markers (like `"remote-studio"` or `"Mode:"`) that do not match the CLI outputs.
3. **Flawed Exit Code Expectations**: The tests assume utility commands like `tailnet` and `rustdesk diff` exit with a non-zero status code on errors, but the actual implementation catches the error internally and exits with status 0.
4. **Platform Incompatibility**: The tests execute platform-specific assertions (like writing to `/etc/X11` or using D-Bus) on macOS, where they fail with OS-specific errors instead of the expected assertion failures.
5. **Logic Bug in Vendor Detection**: The implementation checks for `"ati"` inside the lowercased GPU info. Because `"corporation"` contains `"ati"`, any GPU vendor containing `"Corporation"` (which is extremely common) gets incorrectly classified as AMD (`amdgpu`).

---

## 3. Caveats
No caveats. All findings were directly verified by running the test suite under the isolation environment and investigating the code path.

---

## 4. Conclusion
The E2E test cases in `tests/e2e/test_cases_tier1_test.go` and `tests/e2e/test_cases_tier2_test.go` do not run cleanly due to multiple test design flaws (missing setup files, case-sensitivity issues, platform assumptions, and incorrect exit code expectations). Therefore, our verdict is a **VETO** on the tests.

Furthermore, the review uncovered two issues in the implementation code:
1. **GPU vendor detection logic bug** in `cmd/xorg.go` where any GPU string containing `"Corporation"` matches `"ati"` and gets set to `"amdgpu"`.
2. **Missing backup pruning logic** in `cmd/xorg.go`.

---

## 5. Verification Method
To independently verify:
1. Run `go test -v ./tests/e2e/...` in the root of the repository.
2. Observe the failures for `TestSanity`, `TestTier1_F1_StatusText`, `TestTier1_F2_ModeActivation`, `TestTier2_F4_TailscaleDown`, and `TestTier2_F9_FallbackModesetting`.
3. Check `cmd/xorg.go` line 108 to see how the case-insensitive substring search for `"ati"` matches `"Corporation"`.
