ifeq ($(TMPDIR),)
	TMPDIR := /tmp
endif

export TMPGOPATH := $(TMPDIR)/anax-gopath
export PKGPATH := $(TMPGOPATH)/src/github.com/open-horizon
export PATH := $(TMPGOPATH)/bin:$(PATH)

SHELL := /bin/bash
ARCH = $(shell uname -m)
PKGS=$(shell cd $(PKGPATH)/anax; GOPATH=$(TMPGOPATH) go list ./... | gawk '$$1 !~ /vendor\// {print $$1}')

COMPILE_ARGS := CGO_ENABLED=0
# TODO: handle other ARM architectures on build boxes too
ifeq ($(ARCH),armv7l)
	COMPILE_ARGS +=  GOARCH=arm GOARM=7
endif

all: anax

# will always run b/c deps target is PHONY
anax: $(shell find . -name '*.go' -not -path './vendor/*' -and -not -path './policy/tools/bhgovconfig/*') deps
	cd $(PKGPATH)/anax && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o anax

bhgovconfig: $(shell find ./policy/tools/bhgovconfig/ -name '*.go') deps
	cd $(PKGPATH)/anax/policy/tools/bhgovconfig && \
	  $(MAKE)

clean:
	find ./vendor -maxdepth 1 -not -path ./vendor -and -not -iname "vendor.json" -print0 | xargs -0 rm -Rf
	rm -f anax
ifneq ($(TMPGOPATH),$(GOPATH))
	rm -rf $(TMPGOPATH)
endif
	rm -rf ./contracts
	cd ./policy/tools/bhgovconfig && \
	  $(MAKE) clean

# let this run on every build to ensure newest deps are pulled
deps: $(TMPGOPATH)/bin/govendor
	cd $(PKGPATH)/anax && \
	  export GOPATH=$(TMPGOPATH); \
			govendor sync

$(TMPGOPATH)/bin/govendor: gopathlinks
	mkdir -p $(TMPGOPATH)/bin
	-export GOPATH=$(TMPGOPATH); \
	  go get -u github.com/kardianos/govendor

# this is a symlink to facilitate building outside of user's GOPATH
gopathlinks:
ifneq ($(GOPATH),$(TMPGOPATH))
	mkdir -p $(PKGPATH)
	-ln -s $(CURDIR) $(PKGPATH)/anax
endif

CDIR=$(DESTDIR)/go/src/github.com/open-horizon/go-solidity/contracts
install: anax
	mkdir -p $(DESTDIR)/bin && cp anax $(DESTDIR)/bin
	mkdir -p $(CDIR) && \
		cp -apv ./vendor/github.com/open-horizon/go-solidity/contracts/. $(CDIR)/
	find $(CDIR)/ \( -name "Makefile" -or -iname ".git*" \) -exec rm {} \;

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

# build sequence diagrams
diagrams:
	java -jar $(plantuml_path)/plantuml.jar ./citizenscientist/diagrams/horizonSequenceDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./citizenscientist/diagrams/horizonActivityDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./citizenscientist/diagrams/protocolSequenceDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./citizenscientist/diagrams/protocolActivityDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./messaging/diagrams/senderEncryption.txt
	java -jar $(plantuml_path)/plantuml.jar ./messaging/diagrams/receiverEncryption.txt

.PHONY: clean deps gopathlinks install lint pull test test-integration
