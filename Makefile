.PHONY: install doctor lint test ci ci-strict test-python test-go release release-check deb

# Version is sourced from res.sh — single source of truth. Used at
# build time to inject the value into remote-studio/pkg/config.Version
# via -ldflags, so ./res version and the GitHub-release comparison in
# doctor can't drift from the VERSION= line.
VERSION := $(shell grep '^VERSION=' res.sh | head -1 | cut -d'"' -f2)
LDFLAGS := -X 'remote-studio/pkg/config.Version=$(VERSION)'

install:
	./install.sh install

doctor:
	./res.sh doctor

lint:
	@command -v shellcheck >/dev/null 2>&1 || { echo "shellcheck not installed"; exit 1; }
	shellcheck -x res.sh install.sh install-remote-studio.sh lib/*.sh package/build-deb.sh

test: lint
	@command -v bats >/dev/null 2>&1 || { echo "bats not installed"; exit 1; }
	bats tests/

# Python daemon tests (unittest discovery on daemon/test_*.py).
# Run after the bash tests so a busted bash change doesn't hide
# a Python regression.
test-python:
	@command -v python3 >/dev/null 2>&1 || { echo "python3 not installed"; exit 1; }
	python3 -m unittest discover -s daemon -p 'test_*.py' -v

# Go daemon tests (the bash-shim build skips these — there is no
# Go toolchain on a typical Linux Mint host — but the repo carries
# them so a build environment that does have Go can run them).
test-go:
	@command -v go >/dev/null 2>&1 || { echo "go not installed (skipping)"; exit 0; }
	go test ./pkg/...

ci: test
	bash -n res.sh install.sh install-remote-studio.sh lib/*.sh package/build-deb.sh
	node --check applet/applet.js
	./res.sh status --json >/dev/null
	./install.sh --dry-run install >/dev/null

# Strict local CI gate. Everything `make ci` runs, plus the Python
# daemon tests, the Go tests (if Go is available), and an npm audit
# on the web dashboard. This is the "is this safe to ship?" check
# for the v10 release line.
#
# Note: this does NOT replace GitHub Actions. GitHub's runner pool
# has been observed to stall for hours on this repo (see
# docs/handoff-2026-07-21.md), but Actions still gives a public
# cross-platform signal. Local make ci-strict is the source of truth
# per HANDOFF_FOR_AI.md.
ci-strict: ci test-python
	@command -v go >/dev/null 2>&1 && go test ./pkg/... || echo "go: skipped (no toolchain)"
	@command -v go >/dev/null 2>&1 && go test ./tests/e2e/... || echo "go e2e: skipped (no toolchain)"
	@command -v npm >/dev/null 2>&1 && (cd web && npm audit --omit=dev) || echo "npm audit: skipped (no npm)"

release:
	./res.sh xorg config/xorg.conf

release-check: ci
	# Rebuild the binary with the version injected so ./res version
	# matches res.sh's VERSION= line. This makes the .deb contents and
	# the doctor check agree on the same number.
	go build -ldflags "$(LDFLAGS)" -o remote-studio .
	./install.sh --dry-run system >/dev/null
	bash package/build-deb.sh >/dev/null
	deb="dist/remote-studio_$$(./res.sh version)_all.deb"; \
		contents=$$(mktemp); \
		dpkg-deb --contents "$$deb" > "$$contents"; \
		grep -q 'usr/share/remote-studio/res.sh' "$$contents"; \
		grep -q 'usr/share/remote-studio/install.sh' "$$contents"; \
		grep -q 'usr/share/remote-studio/config/RustDesk_default.toml' "$$contents"; \
		grep -q 'usr/share/remote-studio/config/xsessionrc' "$$contents"; \
		grep -q 'usr/local/bin/res -> /usr/share/remote-studio/res.sh' "$$contents"; \
		rm -f "$$contents"

deb:
	bash package/build-deb.sh
