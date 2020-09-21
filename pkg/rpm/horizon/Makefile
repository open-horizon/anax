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
RPMNAME ?= horizon

# rpmbuild options may contain target platform/architecture, global params 
# configurations using 'define' macros etc.
RPMBUILD_OPTS ?= --target $(rpm_arch)-linux

# Variables for build OS/platform specific opts helpers
BUILD_OS_ID =
BUILD_OS_KERNEL_NAME := $(shell uname -s)
IS_AWK_TOOL_INSTALLED := $(shell which awk >/dev/null && echo true || echo false)

# Recipe for setting up any extra rpmbuild opts depending on build OS/platform
rpmbuild-opts:
	@# OS release check available at Linux platforms, grabbing OS ID with awk
ifeq ($(IS_AWK_TOOL_INSTALLED),true)
ifeq ($(BUILD_OS_KERNEL_NAME),Linux)
	$(eval BUILD_OS_ID = $(shell awk -F= '$$1=="ID" { print $$2 ;}' /etc/os-release 2>/dev/null))
	@# For Fedora-based distros (Fedora/RHEL/Centos) we should switch off 
	@# debuginfo package generation, otherwise rpmbuild will throw an error.
	@# See https://forums.fedoraforum.org/showthread.php?182293-RPM-Build-Error
	@if test "$(BUILD_OS_ID)" = "rhel"; then \
		echo "Build OS is '$(BUILD_OS_ID)', switching off debuginfo package ..."; \
		$(eval RPMBUILD_OPTS += --define "debug_package %{nil}") \
	fi
endif
endif

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
rpmbuild: require-version rpmbuild-opts
	opsys=Linux arch=$(arch) $(MAKE) -C ../../.. anax
	mkdir -p fs/usr/horizon/bin
	@# If you add files copied, add them to .gitignore too
	cp ../../../anax fs/usr/horizon/bin
	cp -a ../../../cli/samples fs/usr/horizon
	cp -a ../../../LICENSE.txt fs/usr/horizon
	mkdir -p $(RPMROOT)/SOURCES
	mkdir -p $(RPMROOT)/RPMS
	mkdir -p $(RPMROOT)/SRPMS
	rm -f $(RPMNAME)-$(VERSION)
	rm -f $(RPMROOT)/SOURCES/$(RPMNAME)-*.tar.gz
	#tar --exclude '.git*' -X .tarignore -H -czf $(RPMROOT)/SOURCES/$(RPMNAME)-$(VERSION).tar.gz $(RPMNAME)-$(VERSION)
	tar -czf $(RPMROOT)/SOURCES/$(RPMNAME)-$(VERSION).tar.gz .
	rm -rf $(RPMROOT)/BUILD/horizon-*
	rm -f $(RPMROOT)/SRPMS/$(RPMNAME)-$${VERSION%%.*}*rpm \
		$(RPMROOT)/RPMS/$(rpm_arch)/$(RPMNAME)-$${VERSION%%.*}*rpm \
		$(RPMROOT)/RPMS/$(rpm_arch)/$(RPMNAME)-$${VERSION%%.*}*rpm.gz
	rpmbuild $(RPMBUILD_OPTS) -ba $(RPMNAME).spec
	$(eval target_rpm_path := $(RPMROOT)/RPMS/$(rpm_arch))
	$(eval target_rpm_name := $(RPMNAME)-$(VERSION)-$(BUILD_NUMBER).$(rpm_arch).rpm)
	@# To save compatibility with previous versions of gzip (older than 1.6), it's
	@#  recommended to use simple shell redirects for keeping original file (.rpm)
	@# See https://unix.stackexchange.com/a/58814
	gzip < $(target_rpm_path)/$(target_rpm_name) > $(target_rpm_path)/$(target_rpm_name).gz
	rm -f $(RPMNAME)-$(VERSION)   # remove the sym link

.PHONY: rpmbuild require-version rpmbuild-opts
