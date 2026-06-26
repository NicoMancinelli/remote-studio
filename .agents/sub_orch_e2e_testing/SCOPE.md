# Scope: E2E Testing Track

## Architecture
The E2E test suite is designed as an opaque-box verification system written in Go (`tests/e2e/`).
It verifies the modernized `res` Go binary by executing it in a subprocess under controlled environments.

### Mocking Strategy (Command-level Opaque Box)
To achieve isolation and multi-platform compatibility (e.g. running on macOS development machines):
1. **Mock Binaries**: A dedicated directory `tests/e2e/mocks/bin/` will contain stub scripts/binaries for Linux/system utilities (`xrandr`, `gsettings`, `tailscale`, `systemctl`, `powerprofilesctl`, `wpctl`, `lspci`, `xset`, etc.).
2. **Path Manipulation**: The test runner runs the `res` binary with a modified `PATH` environment variable prepending the mock directory.
3. **Isolated State**: The tests configure `$HOME` and `$XDG_RUNTIME_DIR` to temporary directories to prevent altering user settings.
4. **Isolated D-Bus Session**: The tests spin up a local `dbus-daemon` session (if available) or mock the D-Bus communication to verify properties, methods, and signals of the daemon.

---

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|---|---|---|---|
| 1 | Write TEST_INFRA.md | Document the test suite design, features inventory, test tiers, and expected inputs/outputs. | None | DONE (aee5cad7) |
| 2 | Set up Test Infrastructure | Implement mock commands in `tests/e2e/mocks/bin/` and the E2E test runner framework. | M1 | DONE (2b86c135) |
| 3 | Implement Tier 1 & Tier 2 Tests | Implement Feature Coverage (Tier 1) and Boundary & Corner cases (Tier 2) for all 9 features. | M2 | IN_PROGRESS (2f56257c) |
| 4 | Implement Tier 3 & Tier 4 Tests | Implement Cross-Feature Combinations (Tier 3) and Real-world Workload Scenarios (Tier 4). | M3 | PLANNED |
| 5 | Verify and Publish | Validate tests run and correctly fail on the unimplemented Go features, then publish `TEST_READY.md`. | M4 | PLANNED |

---

## Interface Contracts & Test Verification Channels
- **CLI Commands**: Checked via exit codes and standard stdout/stderr matching.
- **D-Bus Daemon**: Verified by querying properties/methods via a test D-Bus client.
- **WebSocket / Web UI**: Tested by opening websocket connections to port 9998 and HTTP requests to port 9999.
- **Status File**: Verified by reading `$XDG_RUNTIME_DIR/remote-studio/status` and checking layout.
- **RustDesk Configurations**: Verified by comparing the generated TOML files before and after merges.
