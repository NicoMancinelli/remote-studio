## 2026-06-18T08:30:00Z

You are a test developer. In the Remote-Studio repository, implement the Tier 1 (Feature Coverage) and Tier 2 (Boundary & Corner Cases) E2E test cases inside `tests/e2e/test_cases_tier1_test.go` and `tests/e2e/test_cases_tier2_test.go`.

Read `TEST_INFRA.md` and `tests/e2e/e2e_test.go` to understand the 9 features, the helper functions (`executeCmd`, `executeDaemon`), and the environment configuration.

Specifically, write:
1. `tests/e2e/test_cases_tier1_test.go`:
   - Must contain 45 tests (5 per feature, F1 to F9).
   - Prefix test functions with `TestTier1_F1_`, `TestTier1_F2_`, ..., up to `TestTier1_F9_`.
   - Examples of tests:
     - F1 (CLI): Help outputs, version matches pattern, status prints text, status --json returns valid json matching the schema, profile selection CLI dispatch.
     - F2 (Display): custom command calculation, xrandr mode registration, mode activation, gsettings UI scaling, gsettings text-scaling.
     - F3 (Session): session start creates state/wallpaper backups, session stop restores state, session start sets power profile, session stop restores power profile, speed toggles Cinnamon effects.
     - F4 (Watcher): watcher detects connection in ss table, trusted loopback connection, Tailscale status peer OS query, session start auto-trigger, session stop auto-trigger.
     - F5 (D-Bus): service registration on session bus, Status property query (returns status JSON), Refresh method call, StatusChanged signal emission, concurrent property accesses.
     - F6 (Web/WS): Web dashboard server port 9999 response, WebSocket listener port 9998 connect, WS status_full broadcast, WS command execution, WS scale command adjust.
     - F7 (RustDesk): backup command copy, config safe merger (identity keys preservation), quality/balanced presets apply, diff command show, telemetry log parsing.
     - F8 (Diagnostics): doctor command check, doctor software rendering warning (llvmpipe), doctor-fix symlink links, self-test verification runs, init wizard execution.
     - F9 (Xorg): lspci GPU nvidia probe, lspci GPU amd probe, xorg config generator, backup configurations rotation (cap at 10), xorg rollback config restore.

2. `tests/e2e/test_cases_tier2_test.go`:
   - Must contain 45 tests (5 per feature, F1 to F9).
   - Prefix test functions with `TestTier2_F1_`, `TestTier2_F2_`, ..., up to `TestTier2_F9_`.
   - Implement the boundary & corner cases:
     - F1: CLI unknown command exits non-zero, empty/no args fallback, invalid custom resolution parameters (negative/string), log command line count handling, config command invalid key handling.
     - F2: exceeding virtual framebuffer limits (e.g. 5000x5000) rejected/capped, running in headless without DISPLAY/WAYLAND_DISPLAY error, non-positive scaling factor rejected, dynamic switch handles missing display outputs, custom mode duplicate name resolution.
     - F3: double session start handling (idempotency check), session stop when inactive, stop when wallpaper backup missing, skip power profile if command missing, corrupted session.state parsing.
     - F4: untrusted network IP rejected, tailscale down handling, multiple concurrent connections tracking, non-RustDesk socket ports ignored, corrupted tailscale JSON handling.
     - F5: dbus session address missing error, properties return error JSON on backend failure, duplicate interface registration, rapid properties querying rate limit, signal broadcast queueing.
     - F6: port conflict handling, malformed WS input JSON, sudden WS client connection drop, WS high-frequency commands, HTTP 404 response on missing file.
     - F7: empty template merger safety, corrupted active TOML recovery, reload triggered only when files differ, missing options config creation, diff for invalid preset error.
     - F8: doctor handles all missing dependencies, strict doctor timeouts on network checks, doctor-fix handles read-only errors, self-test with disk full simulation, wizard onboarding SIGINT handling.
     - F9: write to xorg.conf without permissions error, fallback to modesetting on unknown GPU, rotating backup pruning (deletes oldest when >10 backups), xorg config file size capping, rollback error when backup is missing.

DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A Forensic Auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Make sure all tests compile successfully. Tests should execute, verify outputs, and fail cleanly when run against the current unimplemented/stub `res` binary. Verify the compilation and test run by running `go test ./tests/e2e/...` inside your workspace, and report the test run results back.

## 2026-06-18T12:30:41Z

Resume work in /Users/nico/Library/Mobile Documents/com~apple~CloudDocs/Dev/Code/Github/Remote-Studio/.agents/worker_5. Read ORIGINAL_REQUEST.md and BRIEFING.md. Implement 45 Tier 1 and 45 Tier 2 E2E test cases inside tests/e2e/test_cases_tier1_test.go and tests/e2e/test_cases_tier2_test.go. Make sure all tests compile successfully, verify compilation and test execution by running 'go test ./tests/e2e/...', and send a completion message with your handoff.
