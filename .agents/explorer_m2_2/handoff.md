# Handoff Report — explorer_m2_2

## 1. Observation
- In `pkg/config/profile.go` (lines 94-99), `LoadAllProfiles` reads only one resolved path:
  ```go
  func LoadAllProfiles() (*ProfileRegistry, error) {
  	reg := NewProfileRegistry()
  	path, _ := ResolveProfilesPath()
  	_ = reg.LoadProfiles(path)
  	return reg, nil
  }
  ```
- In `pkg/config/paths.go` (lines 19-35), `ResolveProfilesPath` resolves a single path depending on file existence:
  ```go
  func ResolveProfilesPath() (string, error) {
  	home, err := os.UserHomeDir()
  	if err == nil {
  		p := filepath.Join(home, ".config", "remote-studio", "profiles.conf")
  		if _, err := os.Stat(p); err == nil {
  			return p, nil
  		}
  	}
  	...
  ```
- In `lib/config.sh` (lines 142-152), the legacy `profiles` subcommand iterates over all profiles and tags their source dynamically by checking user file existence:
  ```bash
      for k in "${sorted_keys[@]}"; do
          IFS='|' read -r label w h scale _ _ <<< "${PROFILES[$k]}"
          src="default"
          grep -q "^${k}=" "$USER_PROFILES" 2>/dev/null && src="user"
  ```
- In `lib/config.sh` (line 16), config key constraints are verified:
  ```bash
  [[ "$2" =~ ^[A-Z][A-Z0-9_]*$ ]] || { echo "Invalid config key: $2" >&2; return 1; }
  ```
- In `lib/tui.sh` (line 335), user profile key constraints are verified:
  ```bash
  if [[ "$pkey" =~ ^[a-z][a-z0-9_-]*$ ]]; then
  ```
- In `lib/config.sh` (lines 27-30), config set uses a temporary file and rename protocol:
  ```bash
                  tmp=$(mktemp "${USER_CONFIG}.XXXXXX")
                  awk -F= -v k="$key" -v v="$val" '$1 == k { print k "=" v; next } { print }' \
                      "$USER_CONFIG" > "$tmp" && mv "$tmp" "$USER_CONFIG"
  ```

## 2. Logic Chain
1. We observed that `LoadAllProfiles()` in `pkg/config/profile.go` resolves and loads only a single path returned by `ResolveProfilesPath()`.
2. This path resolution stops at the user profile if it exists. Consequently, if a user has custom profiles, none of the built-in default profiles (such as `mac` or `ipad`) will be loaded.
3. To resolve this functional gap, we must load default profiles first, and then overlay user profiles on top of the registry, merging the two collections.
4. We observed that the legacy shell scripts apply strict validation rules on config keys (`^[A-Z][A-Z0-9_]*$`) and user profile keys (`^[a-z][a-z0-9_-]*$`).
5. To maintain exact high-fidelity behavior, the Go Cobra subcommands must validate keys prior to write operations.
6. We observed the legacy write protocol copying to a temp file and renaming to avoid file truncation or corruption.
7. To make this robust in Go, we designed atomic write routines (`WriteConfig` and `WriteProfile`) that write to `os.CreateTemp`, flush buffers, execute a `.Sync()` call to guarantee disk write, close the file, and then perform `os.Rename`.

## 3. Caveats
- No code was written to or modified in `pkg/config/*.go` (per read-only rule).
- Integration into the CLI rootCmd via Cobra (in `cmd/`) was designed but was not applied, as it falls under the scope of `explorer_m2_1`.
- We assumed the user-level configuration path is `$HOME/.config/remote-studio/remote-studio.conf` and user profiles path is `$HOME/.config/remote-studio/profiles.conf`.

## 4. Conclusion
The requirements for the `config` and `profiles` subcommands have been analyzed and mapped to Go designs. The design resolves a critical bug in default/user profile loading, introduces the necessary regex validation checks, and implements the required atomic writing protocol. All details are documented in `analysis.md`.

## 5. Verification Method
- Execute the config unit tests: `go test -v ./pkg/config/...`
- Inspect `analysis.md` to review the proposed code structures for `WriteConfig`, `WriteProfile`, and the corrected `LoadAllProfiles` function.
