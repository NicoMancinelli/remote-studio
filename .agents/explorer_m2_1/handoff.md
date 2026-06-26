# Handoff Report — explorer_m2_1

## 1. Observation
The following legacy behaviors and requirements were observed:
- **Command Dispatch**: `res.sh` handles CLI commands via a `case` statement (lines 103-174), routing `version` (line 162), `info` (line 134), and `log` (line 135).
- **Version output**: Running `res version` prints the value of `$VERSION` (defined as `"9.0"` in `res.sh` line 8).
- **Log output**:
  - `show_log()` (defined in `lib/diagnostics.sh` line 285) reads `$LOG_FILE` (`~/.remote_studio.log`).
  - If the file is missing, it prints `"No log file yet."`.
  - It prints the last `N` lines of the file, defaulting to 20 lines, or using a positional numeric argument (e.g. `res log 50`).
  - Log testing contracts are defined in `tests/test_log.bats` (lines 14-78), verifying line boundaries and exit statuses.
- **Info output**:
  - `show_info()` (defined in `lib/diagnostics.sh` lines 204-224) gathers system data and outputs colorized terminal text (using cyan, green, yellow, dim, and bold ANSI codes).
  - Mode and resolution are read from `~/.res_state` (format is space-separated fields, e.g., `width height scaling text_scale cursor 'label'`).
  - Cinnamon toggle states are queried using `gsettings get` commands for `desktop-effects`, `lock-enabled`, and `gtk-theme`.
  - Night Shift is queried via `xgamma 2>&1 | awk '{print $4}'`.
  - IP is checked via Tailscale IP (`tailscale ip -4`) with a fallback to `hostname -I` (LAN IP).
  - Temperature is read from `sensors` (looking for `"Package id 0"`), triggering a `"⚠️ "` emoji prefix if the temperature is > 80.
  - RAM usage is calculated from `free -m`.
  - Latency is read from a cache file (`.ping_cache` under status directory) that triggers background refreshes via `ping -c 1 -W 1 8.8.8.8 &`.
  - RustDesk connections are counted by parsing established TCP connections in `ss -tnp`.

## 2. Logic Chain
1. To modernize `res` into a single statically-linked Go binary, we need a CLI framework that supports command routing and flags. **Cobra** (`github.com/spf13/cobra`) is the industry standard for Go.
2. The root command of the Cobra hierarchy should execute the interactive TUI when no subcommand is specified to match the legacy fallback behavior of `res.sh`.
3. To meet legacy contracts:
   - `res version` must output a hardcoded string `9.0\n`.
   - `res log` must look for `~/.remote_studio.log`, print `"No log file yet."` if it doesn't exist (exiting 0), and tail the last 20 (or custom positional count) lines.
   - `res info` must perform the same system queries as the legacy shell script, using the exact ANSI escape sequences to match legacy terminal coloring.
4. Spawning external commands in Go (like `free -m`) has higher overhead and dependency requirements. Therefore, we should leverage native Go packages (like `net.InterfaceAddrs()` for IPs and reading `/proc/meminfo` for memory stats) where possible, while keeping shell command executions (`gsettings`, `xgamma`, `sensors`) for desktop-specific configurations.

## 3. Caveats
- No Go files have been written or modified, as per the strict read-only constraint.
- The design assumes standard Linux Mint Cinnamon tools (`gsettings`, `xgamma`, `sensors`, `ss`) are available on the target execution host.
- Background latency caching in Go will run inside a goroutine rather than a background subshell.

## 4. Conclusion
We have completed a detailed CLI root command and subcommand (`version`, `info`, `log`) design for Cobra. The design details state file parsing, log reading, log rotation integration, system queries, and exact ANSI color formats. The full blueprint is recorded in `analysis.md`.

## 5. Verification Method
1. **Inspection**: Verify that `analysis.md` exists and contains detailed Cobra commands, state file structure, log-tailing logic, and ANSI colors.
2. **Go Unit Testing**: Once implemented, run `go test ./pkg/cli/...` to verify the state file and log file parsing logic.
3. **E2E Bats Integration**: Replace `SCRIPT` in `tests/test_log.bats` and `tests/test_diagnostics.bats` to point to the compiled `res` Go binary, and execute the tests using BATS:
   ```bash
   bats tests/test_log.bats
   bats tests/test_diagnostics.bats
   ```
   All tests should pass.
