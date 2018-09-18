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
CLI_MAN_DIR := cli/man1
CLI_COMPLETION_DIR := cli/bash_completion
DEFAULT_UI = api/static/index.html

ANAX_CONTAINER_DIR := anax-in-container
DOCKER_IMAGE_VERSION ?= 2.17.14
DOCKER_IMAGE = openhorizon/$(arch)_anax:$(DOCKER_IMAGE_VERSION)
DOCKER_IMAGE_LATEST = openhorizon/$(arch)_anax:latest
# To not use cache, so it picks up the latest horizon deb pkgs: DOCKER_MAYBE_CACHE='--no-cache' make docker-image
DOCKER_MAYBE_CACHE ?=

export TMPGOPATH ?= $(TMPDIR)$(EXECUTABLE)-gopath
export PKGPATH := $(TMPGOPATH)/src/github.com/open-horizon/$(EXECUTABLE)
export PATH := $(TMPGOPATH)/bin:$(PATH)

# we use a script that will give us the debian arch version since that's what the packaging system inputs
arch ?= $(shell tools/arch-tag)

COMPILE_ARGS := CGO_ENABLED=0
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

opsys ?= $(shell uname -s)
ifeq ($(opsys),Linux)
	COMPILE_ARGS += GOOS=linux
else ifeq ($(opsys),Darwin)
	COMPILE_ARGS += GOOS=darwin
endif

ifndef verbose
.SILENT:
endif

all: deps all-nodeps
all-nodeps: gopathlinks $(EXECUTABLE) $(CLI_EXECUTABLE)

$(EXECUTABLE): $(shell find . -name '*.go' -not -path './vendor/*') gopathlinks
	@echo "Producing $(EXECUTABLE) given arch: $(arch)"
	cd $(PKGPATH) && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o $(EXECUTABLE); 
	exch_min_ver=$(shell grep "MINIMUM_EXCHANGE_VERSION =" $(PKGPATH)/version/version.go | awk -F '"' '{print $$2}') && \
	    echo "The required minimum exchange version is $$exch_min_ver";
	exch_pref_ver=$(shell grep "PREFERRED_EXCHANGE_VERSION =" $(PKGPATH)/version/version.go | awk -F '"' '{print $$2}') && \
	    echo "The preferred exchange version is $$exch_pref_ver"

$(CLI_EXECUTABLE): $(shell find . -name '*.go' -not -path './vendor/*') gopathlinks
	@echo "Producing $(CLI_EXECUTABLE) given arch: $(arch)"
	cd $(PKGPATH) && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o $(CLI_EXECUTABLE) $(CLI_EXECUTABLE).go
	if [[ $(arch) == $(shell tools/arch-tag) && $(opsys) == $(shell uname -s) ]]; then \
	  mkdir -p $(CLI_MAN_DIR) && $(CLI_EXECUTABLE) --help-man > $(CLI_MAN_DIR)/hzn.1 && \
	  mkdir -p $(CLI_COMPLETION_DIR) && $(CLI_EXECUTABLE) --completion-script-bash > $(CLI_COMPLETION_DIR)/hzn_bash_autocomplete.sh; \
	fi


# Build an install pkg for horizon-cli for mac
#todo: these targets should probably be moved into the official horizon build process
export MAC_PKG_VERSION ?= 2.17.14
MAC_PKG_IDENTIFIER ?= com.github.open-horizon.pkg.horizon-cli
MAC_PKG_INSTALL_DIR ?= /Users/Shared/horizon-cli

macpkg: $(CLI_EXECUTABLE)
	mkdir -p pkg/mac/horizon-cli/bin pkg/mac/horizon-cli/share/horizon pkg/mac/horizon-cli/share/man/man1
	cp $(CLI_EXECUTABLE) pkg/mac/horizon-cli/bin
	cp anax-in-container/horizon-container pkg/mac/horizon-cli/bin
	cp LICENSE.txt pkg/mac/horizon-cli/share/horizon
	cp $(CLI_MAN_DIR)/hzn.1 pkg/mac/horizon-cli/share/man/man1
	cp $(CLI_COMPLETION_DIR)/hzn_bash_autocomplete.sh pkg/mac/horizon-cli/share/horizon
	pkgbuild --root pkg/mac/horizon-cli --scripts pkg/mac/scripts --identifier $(MAC_PKG_IDENTIFIER) --version $(MAC_PKG_VERSION) --install-location $(MAC_PKG_INSTALL_DIR) pkg/mac/build/horizon-cli-$(MAC_PKG_VERSION).pkg
	rm -f pkg/mac/build/horizon-cli-$(MAC_PKG_VERSION).pkg.zip
	cd pkg/mac/build; zip horizon-cli-$(MAC_PKG_VERSION).pkg.zip horizon-cli-$(MAC_PKG_VERSION).pkg; cd ../../..   # need to be in the same dir to zip

macinstall: macpkg
	sudo installer -pkg pkg/mac/build/horizon-cli-$(MAC_PKG_VERSION).pkg -target '/Volumes/Macintosh HD'

macpkginfo:
	pkgutil --pkg-info $(MAC_PKG_IDENTIFIER)
	pkgutil --only-files --files $(MAC_PKG_IDENTIFIER)

docker-image:
	@echo "Producing anax docker image $(DOCKER_IMAGE)"
	if [[ $(arch) == "amd64" ]]; then \
	  cd $(ANAX_CONTAINER_DIR) && docker build $(DOCKER_MAYBE_CACHE) -t $(DOCKER_IMAGE) -f ./Dockerfile.$(arch) . && \
	  docker tag $(DOCKER_IMAGE) $(DOCKER_IMAGE_LATEST); \
	else echo "Building the anax docker image is not supported on $(arch)"; fi

docker-push: docker-image
	@echo "Pushing anax docker image $(DOCKER_IMAGE)"
	docker push $(DOCKER_IMAGE)
	docker push $(DOCKER_IMAGE_LATEST)

clean: mostlyclean
	@echo "Clean"
	find ./vendor -maxdepth 1 -not -path ./vendor -and -not -iname "vendor.json" -print0 | xargs -0 rm -Rf
ifneq ($(TMPGOPATH),$(GOPATH))
	rm -rf $(TMPGOPATH)
endif
	rm -rf ./contracts

mostlyclean:
	@echo "Mostlyclean"
	rm -f $(EXECUTABLE) $(CLI_EXECUTABLE)
	-docker rmi $(DOCKER_IMAGE) 2> /dev/null || :

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
	# mkdir -p $(DESTDIR)/web && \
	#	cp $(DEFAULT_UI) $(DESTDIR)/web
	cp -Rapv cli/samples $(DESTDIR)
	mkdir -p $(CDIR) && \
		cp -apv ./vendor/github.com/open-horizon/go-solidity/contracts/. $(CDIR)/
	find $(CDIR)/ \( -name "Makefile" -or -iname ".git*" \) -exec rm {} \;

format:
	@echo "Formatting all Golang source code with gofmt"
	find . -name '*.go' -not -path './vendor/*' -exec gofmt -l -w {} \;

lint: gopathlinks
	@echo "Checking source code for style issues and statically-determinable errors"
	-golint ./... | grep -v "vendor/"
	-cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) go vet $(shell find . -not -path './vendor/*' -iname '*.go' -print | xargs dirname | sort | uniq | xargs) 2>&1 | grep -vP "^exit.*"

pull: deps

# only unit tests
test: gopathlinks
	@echo "Executing unit tests"
	-@cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) go test -cover -tags=unit $(PKGS)

test-integration: gopathlinks
	@echo "Executing integration tests"
	-@cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) go test -cover -tags=integration $(PKGS)

test-ci: gopathlinks
	@echo "Executing integration tests intended for CI systems with special configuration"
	-@cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) go test -cover -tags=ci $(PKGS)

# N.B. this doesn't run ci tests, the ones that require CI system setup
check: deps lint test test-integration

# build sequence diagrams
diagrams:
	java -jar $(plantuml_path)/plantuml.jar ./citizenscientist/diagrams/horizonSequenceDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./citizenscientist/diagrams/protocolSequenceDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./messaging/diagrams/senderEncryption.txt
	java -jar $(plantuml_path)/plantuml.jar ./messaging/diagrams/receiverEncryption.txt
	java -jar $(plantuml_path)/plantuml.jar ./basicprotocol/diagrams/protocolSequenceDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./basicprotocol/diagrams/horizonSequenceDiagram.txt

.PHONY: check clean deps format gopathlinks install lint mostlyclean pull test test-integration docker-image docker-push
