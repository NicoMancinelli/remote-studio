# Handoff Report - Go Foundation Unit Test Design

## 1. Observation
- **Scope File**: `SCOPE.md` (lines 5-6, 40-53) outlines the responsibilities of `pkg/config` and `pkg/status` packages, including the exact fields of the `SessionStatus` struct.
  - "Status Management (`pkg/status/`): Handles JSON serialization and file persistence under `/var/run/remote-studio/status.json` (falling back to `/tmp/remote-studio/status.json`)."
- **Design Proposal**: `design_proposal.md` (lines 51-67, 86-95) outlines configuration and profile paths (`$HOME/.config/remote-studio/` and `/etc/remote-studio/`), `remote-studio.conf` format, and `status.json` structure.
- **Legacy Files**:
  - `lib/config.sh` (lines 4-39) shows `show_config` logic (get, set, show), key validation `^[A-Z][A-Z0-9_]*$`, and values with no newlines.
  - `lib/core.sh` (lines 222-264) shows profile parsing, regex validation `^[^\|]+\|[0-9]+\|[0-9]+\|[0-9.]+\|[0-9.]+\|[0-9]+$`, and preferred profile sorting (`mac`, `mac15`, `ipad`, etc.).
  - `lib/core.sh` (lines 266-281) shows `recent_profiles` tracking, limiting to 5 unique items.
- **Existing BATS Tests**:
  - `tests/test_config.bats` (lines 33-58, 173-230) verifies quotes stripping, malformed files handling, set/get operations with symbols (=, /), key validation, and status file format.
  - `tests/test_profiles.bats` (lines 44-85, 154-170) verifies field count, numeric conversions, profile parsing, list outputs, and sorting.

## 2. Logic Chain
- To achieve a robust Go rewrite, the new implementation of `pkg/config` and `pkg/status` must match the legacy behavior.
- By studying `lib/config.sh`, `lib/core.sh`, `test_config.bats`, and `test_profiles.bats`, we extracted the exact requirements and test cases needed for Go unit tests.
- We designed test coverage across four dimensions for `pkg/config`:
  1. Parsing edge cases (comments, whitespaces, quotes).
  2. Modifying operations (overwrite vs append, key and value validation).
  3. Profile loading (system vs user profiles, validation, merging).
  4. Core profile operations (preferred sorting, recent profiles lifecycle).
- We designed test coverage across three dimensions for `pkg/status`:
  1. Serialization structure and JSON snake_case naming.
  2. Write path resolution (primary `/var/run/remote-studio/status.json` falling back to `/tmp/remote-studio/status.json` when primary is unwritable).
  3. Atomic file operations (write-then-rename) to prevent corruption/partial reads.
- We defined the mocking mechanism using standard Go testing primitives (`t.TempDir()`) and directory candidate variables in Go files that can be overridden in tests, avoiding external mock library overhead and preserving system files.

## 3. Caveats
- Since the Go source files have not been written yet, the exact signatures/API details might differ slightly when implemented. The tests are designed against the high-level architecture specified in the design proposal and interface contracts.
- Permission mocking for the `/var/run` write restriction is mocked by creating read-only test folders via file permissions (`0400`), which behaves differently under some restricted container environments but works on macOS and Linux standard environments.

## 4. Conclusion
- The test design and strategy are fully documented in `.agents/explorer_m1_3/analysis.md`.
- No Go files have been created or modified in this read-only design phase, complying with constraints.
- Implementing the unit tests according to the `analysis.md` strategy will ensure complete functional parity with the legacy bash control plane.

## 5. Verification Method
- Independent verification consists of reviewing the designed strategy in `.agents/explorer_m1_3/analysis.md`.
- Check that the proposed test cases in `analysis.md` map to the legacy constraints found in `lib/config.sh`, `lib/core.sh`, and the BATS test files.
- Ensure no Go files were created or modified during this turn (`git status` should be clean except for files inside `.agents/explorer_m1_3`).
