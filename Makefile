ifeq ($(TMPDIR),)
	TMPDIR := /tmp
endif

export TMPGOPATH := $(TMPDIR)/anax-gopath
export PKGPATH := $(TMPGOPATH)/src/repo.hovitos.engineering/MTN
export PATH := $(TMPGOPATH)/bin:$(PATH)

SHELL := /bin/bash
ARCH = $(shell uname -m)
PKGS=$(shell cd $(PKGPATH)/anax; GOPATH=$(TMPGOPATH) go list ./... | gawk '$$1 !~ /vendor\// {print $$1}')

COMPILE_ARGS := CGO_ENABLED=0 GOOS=linux
# TODO: handle other ARM architectures on build boxes too
ifeq ($(ARCH),armv7l)
	COMPILE_ARGS +=  GOARCH=arm GOARM=7
endif

all: anax

# will always run b/c deps target is PHONY
anax: $(shell find . -name '*.go' -not -path './vendor/*') deps
	cd $(PKGPATH)/anax && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o anax

clean:
	find ./vendor -maxdepth 1 -not -path ./vendor -and -not -iname "vendor.json" -print0 | xargs -0 rm -Rf
	rm -f anax
	rm -rf $(TMPGOPATH)
	rm -rf ./contracts

# let this run on every build to ensure newest deps are pulled
deps: $(TMPGOPATH)/bin/govendor
	cd $(PKGPATH)/anax && \
	  export GOPATH=$(TMPGOPATH); \
			govendor sync

$(TMPGOPATH)/bin/govendor: gopathlinks
	mkdir -p $(TMPGOPATH)/bin
	export GOPATH=$(TMPGOPATH); \
	  go get -u github.com/kardianos/govendor

# this is a symlink to facilitate building outside of user's GOPATH
gopathlinks:
	mkdir -p $(PKGPATH)
	rm -f $(PKGPATH)/anax
	ln -s $(CURDIR) $(PKGPATH)/anax

install: anax
	mkdir -p $(DESTDIR)/{bin,srv}
	cp anax $(DESTDIR)/bin/anax

  # duplicate smart contracts from repo
	-git clone ssh://git@repo.hovitos.engineering:10022/MTN/go-solidity.git && \
		mv ./go-solidity/contracts $(DESTDIR)
	rm -Rf ./go-solidity

	# copy static content
	cp -rvfa ./api/static $(DESTDIR)/srv/

lint:
	-cd api/static && \
		jshint -c ./.jshintrc --verbose ./js/
	-golint ./... | grep -v "vendor/"
	-go vet ./... 2>&1 | grep -vP "exit\ status|vendor/"

pull: deps


# only unit tests
test: deps
	cd $(PKGPATH)/anax && \
		GOPATH=$(TMPGOPATH) go test -v -cover $(PKGS)

test-integration: deps
	cd $(PKGPATH)/anax && \
		GOPATH=$(TMPGOPATH) go test -v -cover -tags=integration $(PKGS)

.PHONY: clean deps gopathlinks install lint pull test test-integration
