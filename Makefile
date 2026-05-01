.PHONY: install doctor test release deb

install:
	./install.sh install

doctor:
	./res.sh doctor

test:
	shellcheck *.sh

release:
	./res.sh xorg config/xorg.conf

deb:
	bash package/build-deb.sh
