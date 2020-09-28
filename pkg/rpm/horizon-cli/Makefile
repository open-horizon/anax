SHELL ?= /bin/bash -e

# can be overridden by the caller
export arch ?= $(shell ../../../tools/arch-tag)
# uname -m is used for RPMs: x86_64, ppc64le, aarch64, etc.
export rpm_arch ?= $(shell uname -m)


# When calling this Makefile you must pass in at least VERSION and BUILD_NUMBER for the artifact build targets
export VERSION
export BUILD_NUMBER
# only set DISTRO if the artifact needs to be built differently for this distro. Value can be like "el8". Will be added to BUILD_NUMBER
export DISTRO

BUILD_NUMBER := $(BUILD_NUMBER:-%=%)$(DISTRO)

# rpmbuild does not give us a good way to set topdir, so use the default location
RPMROOT ?= $(HOME)/rpmbuild
RPMNAME ?= horizon-cli


# The rest of the targets will fail oddly if VERSION and BUILD_NUMBER are not set, so provide a target to check for it
require-version:
ifndef VERSION
	@echo "Error: VERSION must be passed into this Makefile for this target"
	@false
else
	@true
endif
ifndef BUILD_NUMBER
	@echo "Error: BUILD_NUMBER must be passed into this Makefile for this target"
	@false
else
	@true
endif

NO_DEBUG_PKGS := $(shell ../../../tools/no-debug-pkg)
ifeq (${NO_DEBUG_PKGS},true)
	RPM_OPTS =--define="%debug_package %{nil}"
endif

# Remember to up VERSION above. If building the rpm on mac, first: brew install rpm
# Note: during rpmbuild on mac, you may get this benign msg: error: Couldn't exec /usr/local/Cellar/rpm/4.14.1_1/lib/rpm/elfdeps: No such file or directory
rpmbuild: require-version
	opsys=Linux arch=$(arch) $(MAKE) -C ../../.. cli/hzn
	mkdir -p fs/usr/horizon/bin fs/usr/horizon/cluster fs/etc/bash_completion.d fs/usr/share/man/man1
	@# If you add files copied, add them to .gitignore too
	cp ../../../cli/hzn fs/usr/horizon/bin/
	cp ../../../anax-in-container/horizon-container fs/usr/horizon/bin/
	cp ../../../agent-install/agent-install.sh fs/usr/horizon/bin/
	cp ../../../agent-install/agent-uninstall.sh fs/usr/horizon/bin/
	cp ../../../agent-install/edgeNodeFiles.sh fs/usr/horizon/bin/
	cp ../../../agent-install/k8s/deployment-template.yml fs/usr/horizon/cluster/
	cp ../../../agent-install/k8s/persistentClaim-template.yml fs/usr/horizon/cluster/
	cp ../../../cli/bash_completion/hzn_bash_autocomplete.sh fs/etc/bash_completion.d/
	mkdir -p fs/usr/share/man/man1
	gzip --stdout ../../../cli/man1/hzn.1 > fs/usr/share/man/man1/hzn.1.gz
	for m in ../../../cli/man1/hzn.1.*; do \
	  test -f $$m || continue; \
	  d="fs/usr/share/man/$${m##../../../cli/man1/hzn.1.}/man1"; \
	  mkdir -p $$d; \
	  gzip --stdout $$m > "$$d/hzn.1.gz"; \
	done
	mkdir -p $(RPMROOT)/SOURCES
	mkdir -p $(RPMROOT)/RPMS
	mkdir -p $(RPMROOT)/SRPMS
	rm -f $(RPMNAME)-$(VERSION)
	rm -f $(RPMROOT)/SOURCES/$(RPMNAME)-*.tar.gz
	#tar --exclude '.git*' -X .tarignore -H -czf $(RPMROOT)/SOURCES/$(RPMNAME)-$(VERSION).tar.gz $(RPMNAME)-$(VERSION)
	tar -czf $(RPMROOT)/SOURCES/$(RPMNAME)-$(VERSION).tar.gz .
	rm -rf $(RPMROOT)/BUILD/horizon-cli-*
	rm -f $(RPMROOT)/SRPMS/$(RPMNAME)*rpm $(RPMROOT)/RPMS/$(rpm_arch)/$(RPMNAME)*rpm $(RPMROOT)/RPMS/$(rpm_arch)/$(RPMNAME)*rpm.gz
	rpmbuild --target $(rpm_arch)-linux $(RPM_OPTS) -ba $(RPMNAME).spec
	gzip --keep $(RPMROOT)/RPMS/$(rpm_arch)/$(RPMNAME)-$(VERSION)-$(BUILD_NUMBER).$(rpm_arch).rpm
	rm -f $(RPMNAME)-$(VERSION)   # remove the sym link

.PHONY: rpmbuild
