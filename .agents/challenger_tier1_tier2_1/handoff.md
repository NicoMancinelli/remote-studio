# E2E Test Verification Handoff Report

## 1. Observation

During execution of `go test ./tests/e2e/...` in `/Users/nico/Library/Mobile Documents/com~apple~CloudDocs/Dev/Code/Github/Remote-Studio`, multiple compilation and runtime failures were observed.

### Verbatim Test Execution Logs (Runtime Failures)
```
=== RUN   TestSanity
    e2e_test.go:189: Expected output to contain 'remote-studio', got: "Remote Studio CLI is a control plane for remote session management.\n\nUsage:\n  res [command]\n..."
--- FAIL: TestSanity (0.18s)

--- FAIL: TestTier1_F1_StatusText (0.03s)
    test_cases_tier1_test.go:60: Expected stdout to contain "Mode:", but got: "None |  | … | 0 | 0.0% | 2 | tailscale,applet-symlink | n/a | 100.1.2.3/127.0.0.1 | None | N/A | 100.1.2.3:21118 | none\n"

--- FAIL: TestTier1_F2_ModeActivation (0.00s)
    test_cases_tier1_test.go:95: Expected command 'res mac' to succeed, but got error: exit status 1. Stdout: "unknown command \"mac\" for \"res\"\n..."

--- FAIL: TestTier1_F3_SessionStartBackups (0.02s)
    test_cases_tier1_test.go:138: Expected command 'res session start mac' to succeed, but got error: exit status 1. Stdout: "failed to apply profile 'mac': profile 'mac' not found\n"

--- FAIL: TestTier1_F6_WebServerPort (0.50s)
    test_cases_tier1_test.go:470: HTTP request to Web UI failed: Get "http://localhost:9999/": dial tcp [::1]:9999: connect: connection refused

--- FAIL: TestTier2_F1_InvalidResolutionParams (0.01s)
    test_cases_tier2_test.go:53: Expected stderr to contain "invalid", but got: "Error: unknown shorthand flag: '1' in -1920\nUsage:\n  res custom <width> <height> [scale] [flags]"

--- FAIL: TestTier2_F2_FramebufferLimit (0.04s)
    test_cases_tier2_test.go:74: Expected command 'res custom 6000 6000' to fail, but it succeeded. Stdout: "Save as profile? [y/N] "

--- FAIL: TestTier2_F9_FallbackModesetting (0.13s)
    test_cases_tier2_test.go:705: Expected fallback modesetting driver, got: Section "Device"
            Identifier "Configured Video Device"
            Driver "amdgpu"
        EndSection

--- FAIL: TestTier2_F9_RotateBackupPruning (0.01s)
    test_cases_tier2_test.go:737: Expected backups to be pruned and capped at 10, found: 18
```

### Key Source Code Inconsistencies
* **`cmd/tailnet.go:4`**: Unused import `"bytes"` is declared.
* **`cmd/xorg.go:108`**: Driver selection check:
  `else if strings.Contains(lspciLower, "amd") || strings.Contains(lspciLower, "ati") || strings.Contains(lspciLower, "radeon")`
* **`cmd/watch.go:135`**: Bypass mechanism in test environment:
  ```go
  if isTest {
      break
  }
  ```

---

## 2. Logic Chain

1. **Unused Import**: The Go compiler treats unused imports as compile-time errors. On strict/clean builds, `cmd/tailnet.go` fails to compile because `"bytes"` is imported at line 4 but not used in the file.
2. **Casing Mismatch (`TestSanity`)**: `TestSanity` verifies that the `res --help` output contains the hyphenated string `"remote-studio"`. However, the binary prints `"Remote Studio"` or `"res"`, causing the assertion to fail.
3. **Status Text Pattern (`TestTier1_F1_StatusText`)**: The test asserts that the status output contains `"Mode:"`. However, `res status` prints a pipe-delimited string (e.g. `None | ... | None`), leading to the mismatch.
4. **Missing Profile CLI Command (`TestTier1_F2_ModeActivation`)**: Dynamic profile subcommands (like `res mac`) are registered only if profile files exist. Because the E2E tests run in a blank, isolated `HOME` directory, no profiles are loaded. Consequently, executing `res mac` fails with `unknown command "mac"`.
5. **Session Command Failures**: Because profiles cannot be loaded in the isolated `HOME` environment, any session command targeting a profile (e.g., `res session start mac`) fails with `profile 'mac' not found`.
6. **Daemon and Socket Failures**: On macOS platforms, `dbus-daemon` is not found, so `DBUS_SESSION_BUS_ADDRESS` is empty. The `res daemon` command immediately aborts with `DBus session bus address is missing`. Because the daemon dies, the embedded HTTP/WebSocket servers on ports 9999/9998 are never started, causing all WebSocket/Web tests to fail with `connection refused`.
7. **Negative Option Parsing Defect**: Passing negative values (e.g., `-1920`, `-10`, `-1.0`) causes Cobra to parse them as a sequence of shorthand flags (e.g., `-1`, `-0`), producing `unknown shorthand flag: '1' in -1920` instead of trigger clean validation errors.
8. **Lack of Test Isolation**: The test harness uses a global `IsolatedHome` and `IsolatedXdgRuntime` directory without clean-up/re-creation between tests. This allows state files and backup directories to leak across tests, causing `TestTier2_F9_RotateBackupPruning` to find 18 backups instead of the expected pruned size.
9. **GPU Vendor ATI/Corporation Bug (`TestTier2_F9_FallbackModesetting`)**: The vendor check `strings.Contains(lspciLower, "ati")` returns `true` for `"unknown gpu corporation"` because `"corporation"` contains `"ati"`. This mistakenly selects the `"amdgpu"` driver instead of falling back to `"modesetting"`.
10. **Missing Pruning Implementation**: There is no pruning/rotation logic implemented inside `cmd/xorg.go`, so rotating backups are never capped at 10.

---

## 3. Caveats

* The D-Bus tests rely on `dbus-daemon` being present in the system `PATH`. On macOS, this tool is absent by default, causing D-Bus/Daemon dependent tests to fail or get skipped.
* No changes were made to the implementation or test code, in compliance with the "review-only" constraint.

---

## 4. Conclusion

The Remote-Studio E2E test suite currently fails to pass due to:
1. **Broken Test Environment Logic**: Lack of configuration files (profiles, RustDesk templates) in the isolated `HOME` dir.
2. **Implementation Logic Defects**: A GPU vendor matching bug, missing backup rotation/pruning implementation, and broken negative parameter parsing under Cobra.
3. **Weak Assertions and Bypasses**: The `res watch` loop contains an explicit test bypass (`if isTest { break }`), preventing verification of the autonomous watcher loop. WebSocket and Preset merge assertions are too weak (only doing surface-level `strings.Contains` checks).

---

## 5. Verification Method

To verify the test failures and bugs:
1. Run `go test ./tests/e2e/...` in the repository root.
2. Observe the runtime failures listed in Section 1.
3. Inspect `cmd/xorg.go` around line 108 to verify the GPU driver selection logic flaw (`"ati"` inside `"corporation"`).
4. Inspect `cmd/watch.go` around line 135 to verify the test bypass (`if isTest { break }`).

---

## 6. Adversarial Challenge Report

### Challenge Summary
* **Overall risk assessment**: **CRITICAL** (The test suite does not pass, contains numerous weak assertions, and the production code contains a test bypass that prevents verifying the core background polling loop).

### Challenges

#### [High] Challenge 1: Watcher Test Bypass
* **Assumption challenged**: The E2E tests successfully verify the autonomous watch loop.
* **Attack scenario**: The production watch command terminates after one iteration if it detects a test environment `HOME` or `XDG_RUNTIME_DIR`. This means we are not testing the actual infinite polling loop or event loop timing in production.
* **Blast radius**: The watcher could hang, leak resources, or enter CPU spikes in production, which the test suite would completely fail to catch.
* **Mitigation**: Implement an iteration cap flag `--max-iterations` or run the loop in a goroutine with a timeout context, rather than hardcoding environment-based test escapes in production code.

#### [High] Challenge 2: Lack of Test Isolation
* **Assumption challenged**: Tests are independent of each other.
* **Attack scenario**: Shared global folders (`IsolatedHome` and `IsolatedXdgRuntime`) leak state. If one test fails to clean up or creates extra files, it breaks subsequent tests (e.g., `RotateBackupPruning`).
* **Blast radius**: Flaky tests and false positives/negatives in CI pipelines.
* **Mitigation**: Re-create or isolate directory paths (e.g. using `t.TempDir()`) per-test.

#### [Medium] Challenge 3: Weak Assertions
* **Assumption challenged**: Presets and WebSockets are fully validated.
* **Attack scenario**: The WebSocket tests just check for socket connectivity and raw strings in header frames, and the RustDesk preset tests check that original keys exist. If the command does nothing or merges garbage data, the tests still pass.
* **Blast radius**: Broken preset mergers and incorrect WebSocket data formats deployed silently to production.
* **Mitigation**: Parse/decode the WebSocket frames and JSON payloads, and assert specific merged values in TOML configuration files.

#### [Medium] Challenge 4: ATI/Corporation GPU Driver Check Collision
* **Assumption challenged**: GPU vendor detection is robust.
* **Attack scenario**: Any system with a GPU description containing `"Corporation"` (but not AMD/ATI) matches the `"ati"` string match and is assigned the `"amdgpu"` driver, causing failure to boot or configure display in Xorg.
* **Blast radius**: Broken display configurations on systems with Intel, NVIDIA, or VM/Unknown GPUs that use "Corporation" in their vendor strings.
* **Mitigation**: Use precise regex word boundaries (e.g. `\bati\b`) or match vendor IDs from `lspci -n`.
