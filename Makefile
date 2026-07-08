.PHONY: install doctor lint test ci release release-check deb

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

ci: test
	bash -n res.sh install.sh install-remote-studio.sh lib/*.sh package/build-deb.sh
	node --check applet/applet.js
	node tests/applet/applet.test.js
	python3 -c "import json; json.load(open('applet/settings-schema.json')); json.load(open('applet/metadata.json'))"
	./res.sh status --json >/dev/null
	./install.sh --dry-run install >/dev/null

release:
	./res.sh xorg config/xorg.conf

release-check: ci
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
