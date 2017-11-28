ifeq ($(TMPDIR),)
	TMPDIR := /tmp/
endif

ifneq ("$(wildcard ./rules.env)","")
	include rules.env
	export $(shell sed 's/=.*//' rules.env)
endif

SHELL := /bin/bash
EXECUTABLE := $(shell basename $$PWD)
CLI_EXECUTABLE := cli/hzn
DEFAULT_UI = api/static/index.html

export TMPGOPATH ?= $(TMPDIR)$(EXECUTABLE)-gopath
export PKGPATH := $(TMPGOPATH)/src/github.com/open-horizon/$(EXECUTABLE)
export PATH := $(TMPGOPATH)/bin:$(PATH)

# we use a script that will give us the debian arch version since that's what the packaging system inputs
arch ?= $(shell tools/arch-tag)

COMPILE_ARGS := CGO_ENABLED=0 GOOS=linux
# TODO: handle other ARM architectures on build boxes too
ifeq ($(arch),armhf)
	COMPILE_ARGS +=  GOARCH=arm GOARM=7
else ifeq ($(arch),arm64)
	COMPILE_ARGS +=  GOARCH=arm64
else ifeq ($(arch),amd64)
	COMPILE_ARGS +=  GOARCH=amd64
else ifeq ($(arch),ppc64el)
	COMPILE_ARGS +=  GOARCH=ppc64le
endif

ifndef verbose
.SILENT:
endif

all: deps all-nodeps
all-nodeps: gopathlinks $(EXECUTABLE) $(CLI_EXECUTABLE)

$(EXECUTABLE): $(shell find . -name '*.go' -not -path './vendor/*') $(CLI_EXECUTABLE)
	@echo "Producing $(EXECUTABLE) given arch: $(arch)"
	cd $(PKGPATH) && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o $(EXECUTABLE)

$(CLI_EXECUTABLE): $(shell find cli -name '*.go')
	@echo "Producing $(CLI_EXECUTABLE) given arch: $(arch)"
	cd $(PKGPATH) && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o $(CLI_EXECUTABLE) $(CLI_EXECUTABLE).go

clean: mostlyclean
	@echo "Clean"
	find ./vendor -maxdepth 1 -not -path ./vendor -and -not -iname "vendor.json" -print0 | xargs -0 rm -Rf
ifneq ($(TMPGOPATH),$(GOPATH))
	rm -rf $(TMPGOPATH)
endif
	rm -rf ./contracts

mostlyclean:
	@echo "Mostlyclean"
	rm -f $(EXECUTABLE)

deps: $(TMPGOPATH)/bin/govendor
	@echo "Fetching dependencies"
	cd $(PKGPATH) && \
		export GOPATH=$(TMPGOPATH); export PATH=$(TMPGOPATH)/bin:$$PATH; \
			govendor sync

$(TMPGOPATH)/bin/govendor: gopathlinks
	if [ ! -e "$(TMPGOPATH)/bin/govendor" ]; then \
		echo "Fetching govendor"; \
		export GOPATH=$(TMPGOPATH); export PATH=$(TMPGOPATH)/bin:$$PATH; \
			go get -u github.com/kardianos/govendor; \
	fi

gopathlinks:
# unfortunately the .cache directory has to be copied b/c of https://github.com/kardianos/govendor/issues/274
ifneq ($(GOPATH),$(TMPGOPATH))
	if [ -d "$(PKGPATH)" ] && [ "$(readlink -- "$(PKGPATH)")" != "$(CURDIR)" ]; then \
		rm $(PKGPATH); \
	fi
	if [ ! -L "$(PKGPATH)" ]; then \
		mkdir -p $(shell dirname "$(PKGPATH)"); \
		ln -s "$(CURDIR)" "$(PKGPATH)"; \
	fi
	for d in bin pkg; do \
		if [ ! -L "$(TMPGOPATH)/$$d" ]; then \
			ln -s $(GOPATH)/$$d $(TMPGOPATH)/$$d; \
		fi; \
	done
	if [ ! -L "$(TMPGOPATH)/.cache" ] && [ -d "$(GOPATH)/.cache" ]; then \
		cp -Rfpa $(GOPATH)/.cache $(TMPGOPATH)/.cache; \
	fi
endif
PKGS=$(shell cd $(PKGPATH); GOPATH=$(TMPGOPATH) go list ./... | gawk '$$1 !~ /vendor\// {print $$1}')

CDIR=$(DESTDIR)/go/src/github.com/open-horizon/go-solidity/contracts
install:
	@echo "Installing $(EXECUTABLE) and $(CLI_EXECUTABLE) in $(DESTDIR)/bin"
	mkdir -p $(DESTDIR)/bin && \
		cp $(EXECUTABLE) $(DESTDIR)/bin && \
		cp $(CLI_EXECUTABLE) $(DESTDIR)/bin
	mkdir -p $(DESTDIR)/web && \
		cp $(DEFAULT_UI) $(DESTDIR)/web
	cp -Rapv cli/samples $(DESTDIR)
	mkdir -p $(CDIR) && \
		cp -apv ./vendor/github.com/open-horizon/go-solidity/contracts/. $(CDIR)/
	find $(CDIR)/ \( -name "Makefile" -or -iname ".git*" \) -exec rm {} \;

format:
	@echo "Formatting all Golang source code with gofmt"
	find . -name '*.go' -not -path './vendor/*' -exec gofmt -l -w {} \;

lint:
	@echo "Checking source code for style issues and statically-determinable errors"
	-golint ./... | grep -v "vendor/"
	-cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) go vet $(shell find . -not -path './vendor/*' -iname '*.go' -print | xargs dirname | sort | uniq | xargs) 2>&1 | grep -vP "^exit.*"

pull: deps

# only unit tests
test:
	@echo "Executing unit tests"
	-@cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) go test -cover -tags=unit $(PKGS)

test-integration:
	@echo "Executing integration tests"
	-@cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) go test -cover -tags=integration $(PKGS)

test-ci:
	@echo "Executing integration tests intended for CI systems with special configuration"
	-@cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) go test -cover -tags=ci $(PKGS)

check: deps lint test test-integration test-ci

# build sequence diagrams
diagrams:
	java -jar $(plantuml_path)/plantuml.jar ./citizenscientist/diagrams/horizonSequenceDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./citizenscientist/diagrams/protocolSequenceDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./messaging/diagrams/senderEncryption.txt
	java -jar $(plantuml_path)/plantuml.jar ./messaging/diagrams/receiverEncryption.txt
	java -jar $(plantuml_path)/plantuml.jar ./basicprotocol/diagrams/protocolSequenceDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./basicprotocol/diagrams/horizonSequenceDiagram.txt

.PHONY: check clean deps format gopathlinks install lint mostlyclean pull test test-integration
