SHELL ?= /bin/bash -e

# can be overridden by the caller
export arch ?= $(shell ../../tools/arch-tag)

# Normally these variable values come from the top-level Makefile when it calls this Makefile
export VERSION
export BUILD_NUMBER
# only set DISTRO if the artifact needs to be built differently for this distro (not currently the case). Value can be like "ubuntu" or "ubuntu.bionic". Will be added to BUILD_NUMBER
export DISTRO

BUILD_NUMBER := $(BUILD_NUMBER)$(DISTRO)
ifdef BUILD_NUMBER
BUILD_NUMBER := -$(BUILD_NUMBER:-%=%)
endif

# Notes:
#  - This Makefile needs to be above the horizon and horizon-cli dirs, because dpkg-deb will put everything in those dirs into the pkg (except DEBIAN)
#  - There are many levels to the debian build tools. The highest level tool is debuild, which in turn uses many helper
#	 tools, which can automatically create many of the control files. We are using the lowest level tool is dpkg-deb

all: horizon-cli-deb horizon-deb

# The rest of the targets will fail oddly if VERSION is not set, so provide a target to check for it
require-version:
ifndef VERSION
	@echo "Error: VERSION must be passed into this Makefile for this target"
	@false
else
	@true
endif

# Remember to up VERSION above. If building the deb on mac, first: brew install dpkg
horizon-deb: require-version
	opsys=Linux arch=$(arch) $(MAKE) -C ../.. anax
	mkdir -p horizon/usr/horizon/bin
	@# If you add files copied, add them to .gitignore too
	cp ../../anax horizon/usr/horizon/bin
	cp -a ../../cli/samples horizon/usr/horizon
	cp -a ../../LICENSE.txt horizon/usr/horizon
	envsubst < horizon-control.tmpl > horizon/DEBIAN/control
	mkdir -p debs
	rm -f debs/horizon_*$(DISTRO)_$(arch).deb
	dpkg-deb --build horizon debs/horizon_$(VERSION)$(BUILD_NUMBER)_$(arch).deb

# Remember to up VERSION above. If building the deb on mac, first: brew install dpkg
horizon-cli-deb: require-version
	opsys=Linux arch=$(arch) $(MAKE) -C ../.. cli/hzn
	mkdir -p horizon-cli/usr/horizon/bin horizon-cli/usr/horizon/cluster horizon-cli/etc/bash_completion.d horizon-cli/usr/share/man/man1
	@# If you add files copied, add them to .gitignore too
	cp ../../cli/hzn horizon-cli/usr/horizon/bin
	cp ../../anax-in-container/horizon-container horizon-cli/usr/horizon/bin
	cp ../../agent-install/agent-install.sh horizon-cli/usr/horizon/bin
	cp ../../agent-install/agent-uninstall.sh horizon-cli/usr/horizon/bin
	cp ../../agent-install/edgeNodeFiles.sh horizon-cli/usr/horizon/bin
	cp ../../agent-install/k8s/deployment-template.yml horizon-cli/usr/horizon/cluster
	cp ../../agent-install/k8s/persistentClaim-template.yml horizon-cli/usr/horizon/cluster
	if [ -e ../../cli/bash_completion/hzn_bash_autocomplete.sh ]; then \
          cp ../../cli/bash_completion/hzn_bash_autocomplete.sh horizon-cli/etc/bash_completion.d; \
	fi
	mkdir -p horizon-cli/usr/share/man/man1
	gzip --stdout ../../cli/man1/hzn.1 > horizon-cli/usr/share/man/man1/hzn.1.gz
	for m in ../../cli/man1/hzn.1.*; do \
	  test -f $$m || continue; \
	  d="horizon-cli/usr/share/man/$${m##../../cli/man1/hzn.1.}/man1"; \
	  mkdir -p $$d; \
	  gzip --stdout $$m > "$$d/hzn.1.gz"; \
	done
	envsubst < horizon-cli-control.tmpl > horizon-cli/DEBIAN/control
	mkdir -p debs
	rm -f debs/horizon-cli_*$(DISTRO)_$(arch).deb
	dpkg-deb --build horizon-cli debs/horizon-cli_$(VERSION)$(BUILD_NUMBER)_$(arch).deb

.PHONY: all horizon-deb horizon-cli-deb
