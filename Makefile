.PHONY: install doctor test release deb rollback

install:
	./install.sh install

doctor:
	./res.sh doctor

test:
	shellcheck *.sh
	@command -v bats >/dev/null 2>&1 && bats tests/ || echo "bats not installed — skipping unit tests"

release:
	./res.sh xorg config/xorg.conf

deb:
	bash package/build-deb.sh
