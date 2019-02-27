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
CLI_HORIZON_CONTAINER := anax-in-container/horizon-container
CLI_MAN_DIR := cli/man1
CLI_COMPLETION_DIR := cli/bash_completion
DEFAULT_UI = api/static/index.html

ANAX_CONTAINER_DIR := anax-in-container
DOCKER_IMAGE_VERSION ?= 2.22.0
DOCKER_IMAGE_BASE = openhorizon/$(arch)_anax
DOCKER_IMAGE = $(DOCKER_IMAGE_BASE):$(DOCKER_IMAGE_VERSION)
DOCKER_IMAGE_STG = $(DOCKER_IMAGE_BASE):testing
DOCKER_IMAGE_PROD = $(DOCKER_IMAGE_BASE):stable
# the latest tag is the same as stable
DOCKER_IMAGE_LATEST = $(DOCKER_IMAGE_BASE):latest
# By default we do not use cache for the anax container build, so it picks up the latest horizon deb pkgs. If you do want to use the cache: DOCKER_MAYBE_CACHE='' make docker-image
DOCKER_MAYBE_CACHE ?= --no-cache

# The CSS and its production container. This container is NOT used by hzn dev.
CSS_EXECUTABLE := css/cloud-sync-service
CSS_CONTAINER_DIR := css
CSS_IMAGE_VERSION ?= 1.0.1
CSS_IMAGE_BASE = image/cloud-sync-service
CSS_IMAGE_NAME = openhorizon/$(arch)_cloud-sync-service
CSS_IMAGE = $(CSS_IMAGE_NAME):$(CSS_IMAGE_VERSION)
CSS_IMAGE_STG = $(CSS_IMAGE_NAME):testing
CSS_IMAGE_PROD = $(CSS_IMAGE_NAME):stable
# the latest tag is the same as stable
CSS_IMAGE_LATEST = $(CSS_IMAGE_NAME):latest

# The hzn dev ESS/CSS and its container.
ESS_EXECUTABLE := ess/edge-sync-service
ESS_CONTAINER_DIR := ess
ESS_IMAGE_VERSION ?= 1.0.1
ESS_IMAGE_BASE = image/edge-sync-service
ESS_IMAGE_NAME = openhorizon/$(arch)_edge-sync-service
ESS_IMAGE = $(ESS_IMAGE_NAME):$(ESS_IMAGE_VERSION)
ESS_IMAGE_STG = $(ESS_IMAGE_NAME):testing
ESS_IMAGE_PROD = $(ESS_IMAGE_NAME):stable
# the latest tag is the same as stable
ESS_IMAGE_LATEST = $(ESS_IMAGE_NAME):latest

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
all-nodeps: gopathlinks $(EXECUTABLE) $(CLI_EXECUTABLE) $(CSS_EXECUTABLE) $(ESS_EXECUTABLE)

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

$(CSS_EXECUTABLE): $(shell find . -name '*.go' -not -path './vendor/*') gopathlinks
	@echo "Producing $(CSS_EXECUTABLE) given arch: $(arch)"
	cd $(PKGPATH) && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o $(CSS_EXECUTABLE) css/cmd/cloud-sync-service/main.go;

$(ESS_EXECUTABLE): $(shell find . -name '*.go' -not -path './vendor/*') gopathlinks
	@echo "Producing $(ESS_EXECUTABLE) given arch: $(arch)"
	cd $(PKGPATH) && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o $(ESS_EXECUTABLE) ess/cmd/edge-sync-service/main.go;

# Build the horizon-cli pkg for mac
#todo: these targets should be moved into the official horizon build process
export MAC_PKG_VERSION ?= 2.22.0
MAC_PKG = pkg/mac/build/horizon-cli-$(MAC_PKG_VERSION).pkg
MAC_PKG_IDENTIFIER ?= com.github.open-horizon.pkg.horizon-cli
MAC_PKG_INSTALL_DIR ?= /Users/Shared/horizon-cli
# this is Softlayer hostname aptrepo-sjc03-1
APT_REPO_HOST ?= 169.45.88.181
APT_REPO_DIR ?= /vol/aptrepo-local/repositories/view-public

# This is a 1-time step to create the private signing key and public cert for the mac pkg.
# You must first set HORIZON_CLI_PRIV_KEY_PW to the passphrase to use the private key.
gen-mac-key:
	: $${HORIZON_CLI_PRIV_KEY_PW:?}
	@echo "Generating the horizon-cli mac pkg private key and public certificate, and putting them in the p12 archive"
	openssl genrsa -out pkg/mac/build/horizon-cli.key 2048  # create private key
	openssl req -x509 -new -config pkg/mac/key-gen/horizon-cli-key.conf -nodes -key pkg/mac/build/horizon-cli.key -extensions extensions -sha256 -out pkg/mac/build/horizon-cli.crt  # create self-signed cert
	openssl pkcs12 -export -inkey pkg/mac/build/horizon-cli.key -in pkg/mac/build/horizon-cli.crt -out pkg/mac/build/horizon-cli.p12 -password env:HORIZON_CLI_PRIV_KEY_PW  # wrap the key and certificate into PKCS#12 archive
	rm -f pkg/mac/build/horizon-cli.key  # clean up intermediate files
	@echo "Created pkg/mac/build/horizon-cli.crt and pkg/mac/build/horizon-cli.p12. Once you are sure that this the key/cert that should be used for all new staging mac packages:"
	@echo "  - Copy pkg/mac/build/horizon-cli.p12 to the horizon dev team's private key location."
	@echo "  - Make pkg/mac/build/horizon-cli.crt available to users via 'make macuploadcert' ."

# This is a 1-time step to install the mac pkg signing key on your mac so it can be used to sign the pkg.
# You must first set HORIZON_CLI_PRIV_KEY_PW to the passphrase to use the private key.
# If you did not just create the pkg/mac/build/horizon-cli.p12 file using the make target above, download it and put it there.
install-mac-key:
	: $${HORIZON_CLI_PRIV_KEY_PW:?}
	@echo "Importing the key/cert into your keychain. When prompted, enter your Mac admin password:"
	sudo security import pkg/mac/build/horizon-cli.p12 -k /Library/Keychains/System.keychain -P "$$HORIZON_CLI_PRIV_KEY_PW" -f pkcs12  # import key/cert into keychain
	#todo: the cmd above does not automatically set the cert to "Always Trust", tried the cmd below but it does not work.
	#sudo security add-trusted-cert -d -r trustAsRoot -p pkgSign -k /Library/Keychains/System.keychain pkg/mac/build/horizon-cli.p12
	@echo "pkg/mac/build/horizon-cli.p12 installed in the System keychain. Now set it to 'Always Trust' by doing:"
	@echo "Open Finder, click on Applications and then Utilities, and open the Keychain Access app."
	@echo "Click on the System keychain, find horizon-cli-installer in list, and open it."
	@echo "Expand the Trust section and for 'When using this certificate' select 'Always Trust'."

# Inserts the version into version.go in prep for the macpkg build
temp-mod-version:
	mv version/version.go version/version.go.bak   # preserve the time stamp
	cp version/version.go.bak version/version.go
	sed -i.bak2 's/local build/$(MAC_PKG_VERSION)/' version/version.go
	rm -f version/version.go.bak2	# this backup is necessary to make the above sed work on both linux and mac

# Undoes the above, so the source is unchanged
temp-mod-version-undo:
	mv version/version.go.bak version/version.go

# Build the pkg and put it in pkg/mac/build/
# Note: this will only run the dependencies sequentially (as we need) if make is run without --jobs
macpkg: $(MAC_PKG)

$(MAC_PKG): temp-mod-version $(CLI_EXECUTABLE) temp-mod-version-undo
	@echo "Producing Mac pkg horizon-cli"
	mkdir -p pkg/mac/build pkg/mac/horizon-cli/bin pkg/mac/horizon-cli/share/horizon pkg/mac/horizon-cli/share/man/man1
	cp $(CLI_EXECUTABLE) pkg/mac/horizon-cli/bin
	cp anax-in-container/horizon-container pkg/mac/horizon-cli/bin
	cp LICENSE.txt pkg/mac/horizon-cli/share/horizon
	cp $(CLI_MAN_DIR)/hzn.1 pkg/mac/horizon-cli/share/man/man1
	cp $(CLI_COMPLETION_DIR)/hzn_bash_autocomplete.sh pkg/mac/horizon-cli/share/horizon
	pkgbuild --sign "horizon-cli-installer" --root pkg/mac/horizon-cli --scripts pkg/mac/scripts --identifier $(MAC_PKG_IDENTIFIER) --version $(MAC_PKG_VERSION) --install-location $(MAC_PKG_INSTALL_DIR) $@

# Upload the pkg to the staging dir of our apt repo svr, so users can get to it at http://pkg.bluehorizon.network/macos/
#todo: For now, you must have ssh access to the apt repo svr for this to work
macupload: $(MAC_PKG)
	@echo "Uploading $< to http://pkg.bluehorizon.network/macos/testing/"
	rsync -avz $< root@$(APT_REPO_HOST):$(APT_REPO_DIR)/macos/testing

# Upload the pkg cert to the staging dir of our apt repo svr, so users can get to it at http://pkg.bluehorizon.network/testing/macos/
#todo: For now, you must have ssh access to the apt repo svr for this to work
macuploadcert:
	@echo "Uploading pkg/mac/build/horizon-cli.crt to http://pkg.bluehorizon.network/macos/testing/certs/"
	rsync -avz pkg/mac/build/horizon-cli.crt root@$(APT_REPO_HOST):$(APT_REPO_DIR)/macos/testing/certs

# This target promotes the last version you uploaded to staging, so assumes MAC_PKG_VERSION is still set to that version
promote-mac-pkg:
	@echo "Promoting horizon-cli.crt"
	ssh root@$(APT_REPO_HOST) 'cp $(APT_REPO_DIR)/macos/testing/certs/horizon-cli.crt $(APT_REPO_DIR)/macos/certs'
	@echo "Promoting horizon-cli-$(MAC_PKG_VERSION).pkg"
	ssh root@$(APT_REPO_HOST) 'cp $(APT_REPO_DIR)/macos/testing/horizon-cli-$(MAC_PKG_VERSION).pkg $(APT_REPO_DIR)/macos'

macinstall: $(MAC_PKG)
	sudo installer -pkg $< -target '/Volumes/Macintosh HD'

# This only works on the installed package
macpkginfo:
	pkgutil --pkg-info $(MAC_PKG_IDENTIFIER)
	pkgutil --only-files --files $(MAC_PKG_IDENTIFIER)

docker-image:
	@echo "Producing anax docker image $(DOCKER_IMAGE)"
	if [[ $(arch) == "amd64" ]]; then \
	  cd $(ANAX_CONTAINER_DIR) && docker build $(DOCKER_MAYBE_CACHE) -t $(DOCKER_IMAGE) -f ./Dockerfile.$(arch) . && \
	  docker tag $(DOCKER_IMAGE) $(DOCKER_IMAGE_STG); \
	else echo "Building the anax docker image is not supported on $(arch)"; fi

# Pushes the docker image with the staging tag
docker-push-only:
	@echo "Pushing anax docker image $(DOCKER_IMAGE)"
	docker push $(DOCKER_IMAGE)
	docker push $(DOCKER_IMAGE_STG)

docker-push: docker-image docker-push-only

# you must set DOCKER_IMAGE_VERSION to the correct version for promotion to production
promote-docker:
	@echo "Promoting $(DOCKER_IMAGE)"
	docker tag $(DOCKER_IMAGE) $(DOCKER_IMAGE_PROD)
	docker push $(DOCKER_IMAGE_PROD)
	docker tag $(DOCKER_IMAGE) $(DOCKER_IMAGE_LATEST)
	docker push $(DOCKER_IMAGE_LATEST)

promote-mac-pkg-and-docker: promote-mac-pkg promote-docker

css-docker-image: css-clean
	@echo "Producing CSS docker image $(CSS_IMAGE)"
	cd $(CSS_CONTAINER_DIR) && docker build $(DOCKER_MAYBE_CACHE) -t $(CSS_IMAGE) -f ./$(CSS_IMAGE_BASE)-$(arch)/Dockerfile . && \
	docker tag $(CSS_IMAGE) $(CSS_IMAGE_STG); \

css-docker-push-only:
	@echo "Pushing CSS docker image $(CSS_IMAGE)"
	docker push $(CSS_IMAGE)
	docker push $(CSS_IMAGE_STG)

css-promote:
	@echo "Promoting $(CSS_IMAGE_STG)"
	docker tag $(CSS_IMAGE_STG) $(CSS_IMAGE_PROD)
	docker push $(CSS_IMAGE_PROD)
	docker tag $(CSS_IMAGE_STG) $(CSS_IMAGE_LATEST)
	docker push $(CSS_IMAGE_LATEST)

ess-docker-image: ess-clean
	@echo "Producing ESS docker image $(ESS_IMAGE)"
	cd $(ESS_CONTAINER_DIR) && docker build $(DOCKER_MAYBE_CACHE) -t $(ESS_IMAGE) -f ./$(ESS_IMAGE_BASE)-$(arch)/Dockerfile . && \
	docker tag $(ESS_IMAGE) $(ESS_IMAGE_STG); \

ess-docker-push-only:
	@echo "Pushing ESS docker image $(ESS_IMAGE)"
	docker push $(ESS_IMAGE)
	docker push $(ESS_IMAGE_STG)

ess-promote:
	@echo "Promoting $(ESS_IMAGE_STG)"
	docker tag $(ESS_IMAGE_STG) $(ESS_IMAGE_PROD)
	docker push $(ESS_IMAGE_PROD)
	docker tag $(ESS_IMAGE_STG) $(ESS_IMAGE_LATEST)
	docker push $(ESS_IMAGE_LATEST)

clean: mostlyclean
	@echo "Clean"
	find ./vendor -maxdepth 1 -not -path ./vendor -and -not -iname "vendor.json" -print0 | xargs -0 rm -Rf
ifneq ($(TMPGOPATH),$(GOPATH))
	rm -rf $(TMPGOPATH)
endif
	rm -rf ./contracts

mostlyclean: css-clean ess-clean
	@echo "Mostlyclean"
	rm -f $(EXECUTABLE) $(CLI_EXECUTABLE) $(CSS_EXECUTABLE) $(ESS_EXECUTABLE)
	-docker rmi $(DOCKER_IMAGE) 2> /dev/null || :

css-clean:
	-docker rmi $(CSS_IMAGE) 2> /dev/null || :
	-docker rmi $(CSS_IMAGE_STG) 2> /dev/null || :

ess-clean:
	-docker rmi $(ESS_IMAGE) 2> /dev/null || :
	-docker rmi $(ESS_IMAGE_STG) 2> /dev/null || :

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
	@echo "Installing $(EXECUTABLE), $(CLI_EXECUTABLE) and $(CLI_HORIZON_CONTAINER) in $(DESTDIR)/bin"
	mkdir -p $(DESTDIR)/bin && \
		cp $(EXECUTABLE) $(DESTDIR)/bin && \
		cp $(CLI_EXECUTABLE) $(DESTDIR)/bin && \
		cp $(CLI_HORIZON_CONTAINER) $(DESTDIR)/bin
	# mkdir -p $(DESTDIR)/web && \
	#	cp $(DEFAULT_UI) $(DESTDIR)/web
	cp -Rapv cli/samples $(DESTDIR)
	mkdir -p $(CDIR) && \
	find $(CDIR)/ \( -name "Makefile" -or -iname ".git*" \) -exec rm {} \;

format:
	@echo "Formatting all Golang source code with gofmt"
	find . -name '*.go' -not -path './vendor/*' -exec gofmt -l -w {} \;

lint: gopathlinks
	@echo "Checking source code for style issues and statically-determinable errors"
	-golint ./... | grep -v "vendor/"
	-cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) $(COMPILE_ARGS) go vet $(shell find . -not -path './vendor/*' -iname '*.go' -print | xargs dirname | sort | uniq | xargs) 2>&1 | grep -vP "^exit.*"

pull: deps

# only unit tests
test: gopathlinks
	@echo "Executing unit tests"
	-@cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) $(COMPILE_ARGS) go test -cover -tags=unit $(PKGS)

test-integration: gopathlinks
	@echo "Executing integration tests"
	-@cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) $(COMPILE_ARGS) go test -cover -tags=integration $(PKGS)

test-ci: gopathlinks
	@echo "Executing integration tests intended for CI systems with special configuration"
	-@cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) $(COMPILE_ARGS) go test -cover -tags=ci $(PKGS)

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

.PHONY: check clean deps format gopathlinks install lint mostlyclean pull test test-integration docker-image docker-push promote-mac-pkg-and-docker promote-mac-pkg promote-docker gen-mac-key install-mac-key css-docker-image ess-promote css-docker-image ess-promote
