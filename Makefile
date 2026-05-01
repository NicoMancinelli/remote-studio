.PHONY: install doctor test release

install:
	./install.sh install

doctor:
	./res.sh doctor

test:
	shellcheck *.sh

release:
	./res.sh xorg config/xorg.conf
