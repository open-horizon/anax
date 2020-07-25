SHELL ?= /bin/bash -e

# can be overridden by the caller
export arch ?= $(shell ../../tools/arch-tag)
# uname -m is used for RPMs: x86_64, ppc64le, aarch64, etc.
export rpm_arch ?= $(shell uname -m)


# When calling this Makefile you must pass in at least VERSION and BUILD_NUMBER for the artifact build targets
export VERSION
export BUILD_NUMBER
# only set DISTRO if the artifact needs to be built differently for this distro. Value can be like "el8". Will be added to BUILD_NUMBER
export DISTRO

all:
	$(MAKE) -C horizon rpmbuild
	$(MAKE) -C horizon-cli rpmbuild

.PHONY: all
