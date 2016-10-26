ifeq ($(TMPDIR),)
	TMPDIR := /tmp
endif

export TMPGOPATH := $(TMPDIR)/anax-gopath
export PKGPATH := $(TMPGOPATH)/src/github.com/open-horizon
export PATH := $(TMPGOPATH)/bin:$(PATH)

SHELL := /bin/bash
ARCH = $(shell uname -m)
PKGS=$(shell cd $(PKGPATH)/anax; GOPATH=$(TMPGOPATH) go list ./... | gawk '$$1 !~ /vendor\// {print $$1}')

COMPILE_ARGS := CGO_ENABLED=0 GOOS=linux
# TODO: handle other ARM architectures on build boxes too
ifeq ($(ARCH),armv7l)
	COMPILE_ARGS +=  GOARCH=arm GOARM=7
endif

# this makefile is assumed to be run AFTER the anax main makefile has been run

all: bhgovconfig

bhgovconfig: $(shell find . -name '*.go' -not -path './vendor/*')
	cd $(PKGPATH)/anax/policy/tools/bhgovconfig && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o bhgovconfig

clean:
	rm -f bhgovconfig

install: bhgovconfig
	mkdir -p $(DESTDIR)/{bin,srv}
	cp bhgovconfig $(DESTDIR)/bin/bhgovconfig

# only unit tests
test:
	cd $(PKGPATH)/anax/policy/tools/bhgovconfig && \
		GOPATH=$(TMPGOPATH) go test -v -cover $(PKGS)

test-integration:
	cd $(PKGPATH)/anax/policy/tools/bhgovconfig && \
		GOPATH=$(TMPGOPATH) go test -v -cover -tags=integration $(PKGS)

.PHONY: clean gopathlinks install test test-integration
