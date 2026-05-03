.PHONY: install doctor test release deb

install:
	./install.sh install

doctor:
	./res.sh doctor

test:
	@command -v shellcheck >/dev/null 2>&1 \
		&& shellcheck -x res.sh install.sh install-remote-studio.sh lib/*.sh \
		|| echo "shellcheck not installed — skipping lint"
	@command -v bats >/dev/null 2>&1 \
		&& bats tests/ \
		|| echo "bats not installed — skipping unit tests"

release:
	./res.sh xorg config/xorg.conf

deb:
	bash package/build-deb.sh
