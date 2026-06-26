## 2026-06-15T14:20:56Z
Please set up the E2E testing infrastructure. In the Remote-Studio repository, perform the following tasks:

1. Create a `tests/e2e/mocks/bin/` folder.
2. In `tests/e2e/mocks/bin/`, write the following mock bash scripts and make them executable (ensure they are executable with `chmod +x`):
   - `xrandr`: Returns `HDMI-1 connected primary 2560x1664+0+0` by default. If called with `--output` or other options, it should print a success message to stderr and exit 0.
   - `gsettings`: Stores/retrieves keys from a text file `${XDG_RUNTIME_DIR}/gsettings.mock`. If called with `set SCHEMA KEY VALUE`, it saves/updates `SCHEMA.KEY=VALUE` in the mock file. If called with `get SCHEMA KEY`, it parses the mock file and prints the value (falling back to a default value if unset).
   - `tailscale`: If called with `ip`, prints `100.1.2.3`. If called with `status --json`, prints a JSON object containing a few peers (`node1` with macOS at IP `100.64.0.5`, `node2` with iOS at IP `100.64.0.6`, `node3` with windows at IP `100.64.0.7`).
   - `systemctl`: Simulates systemctl behavior. Tracks service state in `${XDG_RUNTIME_DIR}/systemctl.mock`. Supports `is-active`, `start`, `stop`, `restart` for services like `rustdesk`, `tailscaled`.
   - `powerprofilesctl`: Stores/retrieves CPU profile in `${XDG_RUNTIME_DIR}/powerprofile.mock`.
   - `cvt`: Generates modelines when called with width/height.
   - `lspci`: Prints standard lspci output identifying an NVIDIA GPU.
   - `xgamma`: Prints default gamma levels.
   - `wpctl`, `xset`: Stub scripts that simply exit 0.
3. Create `tests/e2e/e2e_test.go` with Go code that handles the `TestMain` entry point, compiles the `res` binary to a temp directory, isolates environment variables (HOME, PATH, XDG_RUNTIME_DIR), and optionally launches a private D-Bus daemon using `dbus-daemon` if available. Provide command execution helpers `executeCmd` and `executeDaemon`.

Ensure that you initialize a `go.mod` file at the project root using `go mod init remote-studio` if it does not already exist, to enable Go compilation and unit testing.

Confirm once the infrastructure is set up and files are created.
