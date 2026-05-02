# Releasing Remote Studio

This document describes how to cut a new release.

## Versioning

Remote Studio uses simple `MAJOR.MINOR` versioning. Bump:
- **MAJOR** when there's a breaking change to the CLI, profile format, or applet API
- **MINOR** for new features, additions to the TUI, new profiles, or significant refactors

## Release checklist

### 1. Bump version

Edit the `VERSION=` line in `res.sh`:

    VERSION="9.0"

Edit `applet/metadata.json` and bump the matching version field.

### 2. Update CHANGELOG.md

Move everything under `## [Unreleased]` into a new dated section:

    ## [9.0] — 2026-04-30

    - feature ...

Leave a fresh `## [Unreleased]` section at the top.

### 3. Run the test suite

    make test

This runs `shellcheck` and the `bats` suite. Both must pass.

### 4. Commit and tag

    git add res.sh applet/metadata.json CHANGELOG.md
    git commit -m "chore: release v9.0"
    git tag v9.0
    git push origin master --tags

### 5. The CI does the rest

The `release-deb.yml` workflow triggers on `v*` tags and:

1. Builds `dist/remote-studio_9.0_all.deb` via `package/build-deb.sh`
2. Creates a GitHub Release for the tag
3. Attaches the `.deb` as a release asset

Verify the release at https://github.com/NicoMancinelli/remote-studio/releases/

### 6. Manual sanity check

On a Linux Mint machine, install the new `.deb`:

    sudo dpkg -i remote-studio_9.0_all.deb
    res doctor
    res self-test

Both should exit clean.

## Hotfix release

For an urgent fix on the latest release:

1. Branch from the latest tag: `git checkout -b hotfix/9.0.1 v9.0`
2. Apply the fix and bump VERSION to `9.0.1`
3. Update CHANGELOG.md
4. Tag `v9.0.1` and push — CI handles the rest
5. Cherry-pick or merge the fix back to master
