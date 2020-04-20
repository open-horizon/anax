ifeq ($(TMPDIR),)
	TMPDIR := /tmp/
endif

ifneq ("$(wildcard ./rules.env)","")
	include rules.env
	export $(shell sed 's/=.*//' rules.env)
endif

SHELL := /bin/bash

# Set branch name to the name of the branch to which this file belongs. This must be updated
# when a new branch is created. Development on the main (master) branch should leave this variable empty.
# DO NOT set this variable to the branch in which you are doing development work.
BRANCH_NAME ?= ""

EXECUTABLE := $(shell basename $$PWD)
CLI_EXECUTABLE := cli/hzn
CLI_CONFIG_FILE := cli/hzn.json
CLI_HORIZON_CONTAINER := anax-in-container/horizon-container
CLI_MAN_DIR := cli/man1
CLI_COMPLETION_DIR := cli/bash_completion
DEFAULT_UI = api/static/index.html

# used for creating hzn man pages
CLI_TEMP_EXECUTABLE := cli/hzn.tmp

IMAGE_REPO ?= openhorizon
IMAGE_OVERRIDE ?= ""

ANAX_CONTAINER_DIR := anax-in-container
ANAX_IMAGE_VERSION ?= localbuild
ANAX_IMAGE_BASE = $(IMAGE_REPO)/$(arch)_anax
ANAX_IMAGE = $(ANAX_IMAGE_BASE):$(ANAX_IMAGE_VERSION)
ANAX_IMAGE_STG = $(ANAX_IMAGE_BASE):testing$(BRANCH_NAME)
ANAX_IMAGE_PROD = $(ANAX_IMAGE_BASE):stable$(BRANCH_NAME)
# the latest tag is the same as stable
ANAX_IMAGE_LATEST = $(ANAX_IMAGE_BASE):latest$(BRANCH_NAME)
# By default we do not use cache for the anax container build, so it picks up the latest horizon deb pkgs. If you do want to use the cache: DOCKER_MAYBE_CACHE='' make docker-image
DOCKER_MAYBE_CACHE ?= --no-cache

I18N_OUT_GOTEXT_FILES := locales/*/out.gotext.json
I18N_CATALOG_FILE := i18n_messages/catalog.go

AGBOT_IMAGE_BASE=$(IMAGE_REPO)/$(arch)_agbot
AGBOT_IMAGE_VERSION ?= $(ANAX_IMAGE_VERSION)
AGBOT_IMAGE = $(AGBOT_IMAGE_BASE):$(AGBOT_IMAGE_VERSION)
AGBOT_IMAGE_STG = $(AGBOT_IMAGE_BASE):testing$(BRANCH_NAME)
AGBOT_IMAGE_PROD = $(AGBOT_IMAGE_BASE):stable$(BRANCH_NAME)
# the latest tag is the same as stable
AGBOT_IMAGE_LATEST = $(AGBOT_IMAGE_BASE):latest$(BRANCH_NAME)

# anax container running in kubernetes
ANAX_K8S_CONTAINER_DIR := anax-in-k8s
ANAX_K8S_IMAGE_BASE = $(ANAX_IMAGE_BASE)_k8s
ANAX_K8S_IMAGE_VERSION ?= $(ANAX_IMAGE_VERSION)
ANAX_K8S_IMAGE = $(ANAX_K8S_IMAGE_BASE):$(ANAX_K8S_IMAGE_VERSION)
ANAX_K8S_IMAGE_STG = $(ANAX_K8S_IMAGE_BASE):testing$(BRANCH_NAME)
ANAX_K8S_IMAGE_PROD = $(ANAX_K8S_IMAGE_BASE):stable$(BRANCH_NAME)
ANAX_K8S_IMAGE_LATEST = $(ANAX_K8S_IMAGE_BASE):latest$(BRANCH_NAME)

# Variables that control packaging the file sync service containers
DOCKER_REGISTRY ?= "dockerhub"
ANAX_K8S_REGISTRY ?= $(DOCKER_REGISTRY)
FSS_REGISTRY ?= $(DOCKER_REGISTRY)
ANAX_REGISTRY ?= $(DOCKER_REGISTRY)
AGBOT_REGISTRY ?= $(DOCKER_REGISTRY)

# The CSS and its production container. This container is NOT used by hzn dev.
CSS_EXECUTABLE := css/cloud-sync-service
CSS_CONTAINER_DIR := css
CSS_IMAGE_VERSION ?= 1.2.0$(BRANCH_NAME)
CSS_IMAGE_BASE = image/cloud-sync-service
CSS_IMAGE_NAME = $(IMAGE_REPO)/$(arch)_cloud-sync-service
CSS_IMAGE = $(CSS_IMAGE_NAME):$(CSS_IMAGE_VERSION)
CSS_IMAGE_STG = $(CSS_IMAGE_NAME):testing$(BRANCH_NAME)
CSS_IMAGE_PROD = $(CSS_IMAGE_NAME):stable$(BRANCH_NAME)
# the latest tag is the same as stable
CSS_IMAGE_LATEST = $(CSS_IMAGE_NAME):latest$(BRANCH_NAME)

# for redhat ubi
CSS_UBI_IMAGE_NAME = $(IMAGE_REPO)/$(arch)_cloud-sync-service_ubi
CSS_UBI_IMAGE = $(CSS_UBI_IMAGE_NAME):$(CSS_IMAGE_VERSION)
CSS_UBI_IMAGE_STG = $(CSS_UBI_IMAGE_NAME):testing$(BRANCH_NAME)
CSS_UBI_IMAGE_PROD = $(CSS_UBI_IMAGE_NAME):stable$(BRANCH_NAME)
# the latest tag is the same as stable
CSS_UBI_IMAGE_LATEST = $(CSS_UBI_IMAGE_NAME):latest$(BRANCH_NAME)


# The hzn dev ESS/CSS and its container.
ESS_EXECUTABLE := ess/edge-sync-service
ESS_CONTAINER_DIR := ess
ESS_IMAGE_VERSION ?= 1.2.0$(BRANCH_NAME)
ESS_IMAGE_BASE = image/edge-sync-service
ESS_IMAGE_NAME = $(IMAGE_REPO)/$(arch)_edge-sync-service
ESS_IMAGE = $(ESS_IMAGE_NAME):$(ESS_IMAGE_VERSION)
ESS_IMAGE_STG = $(ESS_IMAGE_NAME):testing$(BRANCH_NAME)
ESS_IMAGE_PROD = $(ESS_IMAGE_NAME):stable$(BRANCH_NAME)
# the latest tag is the same as stable
ESS_IMAGE_LATEST = $(ESS_IMAGE_NAME):latest$(BRANCH_NAME)

# for redhat ubi
ESS_UBI_IMAGE_NAME = $(IMAGE_REPO)/$(arch)_edge-sync-service_ubi
ESS_UBI_IMAGE = $(ESS_UBI_IMAGE_NAME):$(ESS_IMAGE_VERSION)
ESS_UBI_IMAGE_STG = $(ESS_UBI_IMAGE_NAME):testing$(BRANCH_NAME)
ESS_UBI_IMAGE_PROD = $(ESS_UBI_IMAGE_NAME):stable$(BRANCH_NAME)
# the latest tag is the same as stable
ESS_UBI_IMAGE_LATEST = $(ESS_UBI_IMAGE_NAME):latest$(BRANCH_NAME)

# supported locales
SUPPORTED_LOCALES ?= de  es  fr  it  ja  ko  pt_BR  zh_CN  zh_TW


export TMPGOPATH ?= $(TMPDIR)$(EXECUTABLE)-gopath
export PKGPATH := $(TMPGOPATH)/src/github.com/open-horizon/$(EXECUTABLE)
export PATH := $(TMPGOPATH)/bin:$(PATH)

export EXCHANGE_URL ?=
export FSS_CSS_URL ?=

# we use a script that will give us the debian arch version since that's what the packaging system inputs
arch ?= $(shell tools/arch-tag)

COMPILE_ARGS := CGO_ENABLED=0
# TODO: handle other ARM architectures on build boxes too
ifeq ($(arch),armhf)
	COMPILE_ARGS +=  GOARCH=arm GOARM=6
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

COMPILE_ARGS_LOCAL := CGO_ENABLED=0
arch_local = $(shell tools/arch-tag)
# TODO: handle other ARM architectures on build boxes too
ifeq ($(arch_local),armhf)
	COMPILE_ARGS_LOCAL +=  GOARCH=arm GOARM=6
else ifeq ($(arch_local),arm64)
	COMPILE_ARGS_LOCAL +=  GOARCH=arm64
else ifeq ($(arch_local),amd64)
	COMPILE_ARGS_LOCAL +=  GOARCH=amd64
else ifeq ($(arch_local),ppc64el)
	COMPILE_ARGS_LOCAL +=  GOARCH=ppc64le
endif

opsys_local = $(shell uname -s)
ifeq ($(opsys_local),Linux)
	COMPILE_ARGS_LOCAL += GOOS=linux
else ifeq ($(opsys_local),Darwin)
	COMPILE_ARGS_LOCAL += GOOS=darwin
endif


ifndef verbose
.SILENT:
endif

all: deps all-nodeps
all-nodeps: gopathlinks gofolders $(EXECUTABLE) $(CLI_EXECUTABLE) $(CSS_EXECUTABLE) $(ESS_EXECUTABLE)

deps: gofolders i18n-catalog

noi18n: all-nodeps

$(EXECUTABLE): $(shell find . -name '*.go') gopathlinks
	@echo "Producing $(EXECUTABLE) given arch: $(arch)"
	cd $(PKGPATH) && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o $(EXECUTABLE);
	exch_min_ver=$(shell grep "MINIMUM_EXCHANGE_VERSION =" $(PKGPATH)/version/version.go | awk -F '"' '{print $$2}') && \
	    echo "The required minimum exchange version is $$exch_min_ver";
	exch_pref_ver=$(shell grep "PREFERRED_EXCHANGE_VERSION =" $(PKGPATH)/version/version.go | awk -F '"' '{print $$2}') && \
	    echo "The preferred exchange version is $$exch_pref_ver"

$(CLI_EXECUTABLE): $(shell find . -name '*.go') gopathlinks
	@echo "Producing $(CLI_EXECUTABLE) given arch: $(arch)"
	cd $(PKGPATH) && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o $(CLI_EXECUTABLE) $(CLI_EXECUTABLE).go && \
	    envsubst < cli/cliconfig/hzn.json.tmpl > $(CLI_CONFIG_FILE)
	if [[ $(arch) == $(shell tools/arch-tag) && $(opsys) == $(shell uname -s) ]]; then \
	  	mkdir -p $(CLI_MAN_DIR) && $(CLI_EXECUTABLE) --help-man > $(CLI_MAN_DIR)/hzn.1 && \
		for loc in $(SUPPORTED_LOCALES) ; do \
			HZN_LANG=$$loc $(CLI_EXECUTABLE) --help-man > $(CLI_MAN_DIR)/hzn.1.$$loc; \
		done && \
	  	mkdir -p $(CLI_COMPLETION_DIR) && $(CLI_EXECUTABLE) --completion-script-bash > $(CLI_COMPLETION_DIR)/hzn_bash_autocomplete.sh; \
	else \
		echo "Producing $(CLI_TEMP_EXECUTABLE) under $(arch_local) for generating hzn man pages"; \
		cd $(PKGPATH) && \
	  	  export GOPATH=$(TMPGOPATH); \
	    	$(COMPILE_ARGS_LOCAL) go build -o $(CLI_TEMP_EXECUTABLE) $(CLI_EXECUTABLE).go; \
	  		mkdir -p $(CLI_MAN_DIR) && $(CLI_TEMP_EXECUTABLE) --help-man > $(CLI_MAN_DIR)/hzn.1 && \
			for loc in $(SUPPORTED_LOCALES) ; do \
				HZN_LANG=$$loc $(CLI_TEMP_EXECUTABLE) --help-man > $(CLI_MAN_DIR)/hzn.1.$$loc; \
			done && \
	  		rm $(CLI_TEMP_EXECUTABLE); \
	fi

$(CSS_EXECUTABLE): $(shell find . -name '*.go') gopathlinks
	@echo "Producing $(CSS_EXECUTABLE) given arch: $(arch)"
	cd $(PKGPATH) && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o $(CSS_EXECUTABLE) css/cmd/cloud-sync-service/main.go;

$(ESS_EXECUTABLE): $(shell find . -name '*.go') gopathlinks
	@echo "Producing $(ESS_EXECUTABLE) given arch: $(arch)"
	cd $(PKGPATH) && \
	  export GOPATH=$(TMPGOPATH); \
	    $(COMPILE_ARGS) go build -o $(ESS_EXECUTABLE) ess/cmd/edge-sync-service/main.go;

# Build the horizon-cli pkg for mac
#todo: these targets should be moved into the official horizon build process
export MAC_PKG_VERSION ?= 2.22.7
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
	openssl req -x509 -days 3650 -new -config pkg/mac/key-gen/horizon-cli-key.conf -nodes -key pkg/mac/build/horizon-cli.key -extensions extensions -sha256 -out pkg/mac/build/horizon-cli.crt  # create self-signed cert
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
	mkdir -p pkg/mac/build pkg/mac/horizon-cli/bin pkg/mac/horizon-cli/share/horizon pkg/mac/horizon-cli/share/man/man1 pkg/mac/horizon-cli/etc/horizon
	cp $(CLI_EXECUTABLE) pkg/mac/horizon-cli/bin
	cp -Rapv cli/samples pkg/mac/horizon-cli/
	cp anax-in-container/horizon-container pkg/mac/horizon-cli/bin
	cp LICENSE.txt pkg/mac/horizon-cli/share/horizon
	cp $(CLI_MAN_DIR)/hzn.1 pkg/mac/horizon-cli/share/man/man1
	for loc in $(SUPPORTED_LOCALES) ; do \
		mkdir -p pkg/mac/horizon-cli/share/man/$$loc/man1 && \
		cp $(CLI_MAN_DIR)/hzn.1.$$loc pkg/mac/horizon-cli/share/man/$$loc/man1/hzn.1; \
	done
	cp $(CLI_COMPLETION_DIR)/hzn_bash_autocomplete.sh pkg/mac/horizon-cli/share/horizon
	cp $(CLI_CONFIG_FILE) pkg/mac/horizon-cli/etc/horizon
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

anax-image:
	@echo "Producing anax docker image $(ANAX_IMAGE)"
	if [[ $(arch) == "amd64" ]]; then \
	  rm -rf $(ANAX_CONTAINER_DIR)/anax; \
	  rm -rf $(ANAX_CONTAINER_DIR)/hzn; \
	  cp $(EXECUTABLE) $(ANAX_CONTAINER_DIR); \
	  cp $(CLI_EXECUTABLE) $(ANAX_CONTAINER_DIR); \
	  cd $(ANAX_CONTAINER_DIR) && docker build $(DOCKER_MAYBE_CACHE) -t $(ANAX_IMAGE) -f Dockerfile.ubi . && \
	  docker tag $(ANAX_IMAGE) $(ANAX_IMAGE_STG); \
	else echo "Building the anax docker image is not supported on $(arch)"; fi

agbot-image:
	@echo "Producing agbot docker image $(AGBOT_IMAGE)"
	if [[ $(arch) == "amd64" ]]; then \
	  rm -rf $(ANAX_CONTAINER_DIR)/anax; \
	  rm -rf $(ANAX_CONTAINER_DIR)/hzn; \
	  cp $(EXECUTABLE) $(ANAX_CONTAINER_DIR); \
	  cp $(CLI_EXECUTABLE) $(ANAX_CONTAINER_DIR); \
	  cd $(ANAX_CONTAINER_DIR) && docker build $(DOCKER_MAYBE_CACHE) -t $(AGBOT_IMAGE) -f Dockerfile_agbot.ubi . && \
	  docker tag $(AGBOT_IMAGE) $(AGBOT_IMAGE_STG); \
	else echo "Building the agbot docker image is not supported on $(arch)"; fi

# Pushes the docker image with the staging tag
docker-push-only:
	@echo "Pushing anax docker image $(ANAX_IMAGE)"
	docker push $(ANAX_IMAGE)
	docker push $(ANAX_IMAGE_STG)

agbot-push-only:
	@echo "Pushing agbot docker image $(AGBOT_IMAGE)"
	docker push $(AGBOT_IMAGE)
	docker push $(AGBOT_IMAGE_STG)

docker-push: docker-image docker-push-only agbot-image agbot-push-only

# you must set ANAX_IMAGE_VERSION to the correct version for promotion to production
promote-anax:
	@echo "Promoting $(ANAX_IMAGE)"
	docker pull $(ANAX_IMAGE)
	docker tag $(ANAX_IMAGE) $(ANAX_IMAGE_PROD)
	docker push $(ANAX_IMAGE_PROD)
	docker tag $(ANAX_IMAGE) $(ANAX_IMAGE_LATEST)
	docker push $(ANAX_IMAGE_LATEST)

# you must set ANAX_IMAGE_VERSION to the correct version for promotion to production
promote-agbot:
	@echo "Promoting $(AGBOT_IMAGE)"
	docker pull $(AGBOT_IMAGE)
	docker tag $(AGBOT_IMAGE) $(AGBOT_IMAGE_PROD)
	docker push $(AGBOT_IMAGE_PROD)
	docker tag $(AGBOT_IMAGE) $(AGBOT_IMAGE_LATEST)
	docker push $(AGBOT_IMAGE_LATEST)

promote-anax-k8s:
	@echo "Promoting $(ANAX_K8S_IMAGE)"
	docker pull $(ANAX_K8S_IMAGE)
	docker tag $(ANAX_K8S_IMAGE) $(ANAX_K8S_IMAGE_PROD)
	docker push $(ANAX_K8S_IMAGE_PROD)
	docker tag $(ANAX_K8S_IMAGE) $(ANAX_K8S_IMAGE_LATEST)
	docker push $(ANAX_K8S_IMAGE_LATEST)

promote-mac-pkg-and-docker: promote-mac-pkg promote-anax promote-agbot promote-css promote-anax-k8s

anax-package: anax-image
	@echo "Packaging anax image"
	if [[ $(shell tools/image-exists $(ANAX_REGISTRY) $(ANAX_IMAGE_BASE) $(ANAX_IMAGE_VERSION) 2> /dev/null) == "0" || $(IMAGE_OVERRIDE) != "" ]]; then \
		echo "Pushing anax docker image $(ANAX_IMAGE_BASE):$(ANAX_IMAGE_VERSION)"; \
		docker push $(ANAX_IMAGE_BASE):$(ANAX_IMAGE_VERSION); \
		docker push $(ANAX_IMAGE_STG); \
	else \
		echo "anax-k8s container $(ANAX_IMAGE_STG):$(ANAX_IMAGE_VERSION) already present in $(IMAGE_REPO)"; \
	fi

agbot-package: agbot-image
	@echo "Packaging agbot image"
	if [[ $(shell tools/image-exists $(AGBOT_REGISTRY) $(AGBOT_IMAGE_BASE) $(AGBOT_IMAGE_VERSION) 2> /dev/null) == "0" || $(IMAGE_OVERRIDE) != "" ]]; then \
		echo "Pushing agbot docker image $(AGBOT_IMAGE_BASE):$(AGBOT_IMAGE_VERSION)"; \
		docker push $(AGBOT_IMAGE_BASE):$(AGBOT_IMAGE_VERSION); \
		docker push $(AGBOT_IMAGE_STG); \
	else \
		echo "anax-k8s container $(AGBOT_IMAGE_STG):$(AGBOT_IMAGE_VERSION) already present in $(IMAGE_REPO)"; \
	fi

anax-k8s-image: anax-k8s-clean
	cp $(EXECUTABLE) $(ANAX_K8S_CONTAINER_DIR)
	cp $(CLI_EXECUTABLE) $(ANAX_K8S_CONTAINER_DIR)
	@echo "Producing ANAX K8S docker image $(ANAX_K8S_IMAGE_STG)"
	cd $(ANAX_K8S_CONTAINER_DIR) && docker build $(DOCKER_MAYBE_CACHE) -t $(ANAX_K8S_IMAGE_STG) -f Dockerfile.ubi . && \
	docker tag $(ANAX_K8S_IMAGE_STG) $(ANAX_K8S_IMAGE_BASE):$(ANAX_K8S_IMAGE_VERSION)

anax-k8s-package: anax-k8s-image
	@echo "Packaging anax-k8s container"
	if [[ $(shell tools/image-exists $(ANAX_K8S_REGISTRY) $(ANAX_K8S_IMAGE_BASE) $(ANAX_K8S_IMAGE_VERSION) 2> /dev/null) == "0" || $(IMAGE_OVERRIDE) != "" ]]; then \
		echo "Pushing anax-k8s docker image $(ANAX_K8S_IMAGE_BASE):$(ANAX_K8S_IMAGE_VERSION)"; \
		docker push $(ANAX_K8S_IMAGE_BASE):$(ANAX_K8S_IMAGE_VERSION); \
		docker push $(ANAX_K8S_IMAGE_STG); \
	else \
		echo "anax-k8s container $(ANAX_K8S_IMAGE_STG):$(ANAX_K8S_IMAGE_VERSION) already present in $(IMAGE_REPO)"; \
	fi

css-docker-image: css-clean
	@echo "Producing CSS docker image $(CSS_IMAGE)"
	cd $(CSS_CONTAINER_DIR) && docker build $(DOCKER_MAYBE_CACHE) -t $(CSS_IMAGE) -f ./$(CSS_IMAGE_BASE)-$(arch)/Dockerfile . && \
	docker tag $(CSS_IMAGE) $(CSS_IMAGE_STG);
	@echo "Producing CSS docker image $(CSS_UBI_IMAGE)"
	cd $(CSS_CONTAINER_DIR) && docker build $(DOCKER_MAYBE_CACHE) -t $(CSS_UBI_IMAGE) -f ./$(CSS_IMAGE_BASE)-$(arch)/Dockerfile.ubi . && \
	docker tag $(CSS_UBI_IMAGE) $(CSS_UBI_IMAGE_STG); \

promote-css:
	@echo "Promoting $(CSS_IMAGE)"
	docker pull $(CSS_IMAGE)
	docker tag $(CSS_IMAGE) $(CSS_IMAGE_PROD)
	docker push $(CSS_IMAGE_PROD)
	docker tag $(CSS_IMAGE) $(CSS_IMAGE_LATEST)
	docker push $(CSS_IMAGE_LATEST)
	@echo "Promoting $(CSS_UBI_IMAGE)"
	docker pull $(CSS_UBI_IMAGE)
	docker tag $(CSS_UBI_IMAGE) $(CSS_UBI_IMAGE_PROD)
	docker push $(CSS_UBI_IMAGE_PROD)
	docker tag $(CSS_UBI_IMAGE) $(CSS_UBI_IMAGE_LATEST)
	docker push $(CSS_UBI_IMAGE_LATEST)

ess-docker-image: ess-clean
	@echo "Producing ESS docker image $(ESS_IMAGE)"
	cd $(ESS_CONTAINER_DIR) && docker build $(DOCKER_MAYBE_CACHE) -t $(ESS_IMAGE) -f ./$(ESS_IMAGE_BASE)-$(arch)/Dockerfile . && \
	docker tag $(ESS_IMAGE) $(ESS_IMAGE_STG);
	@echo "Producing ESS docker image $(ESS_UBI_IMAGE)"
	cd $(ESS_CONTAINER_DIR) && docker build $(DOCKER_MAYBE_CACHE) -t $(ESS_UBI_IMAGE) -f ./$(ESS_IMAGE_BASE)-$(arch)/Dockerfile.ubi . && \
	docker tag $(ESS_UBI_IMAGE) $(ESS_UBI_IMAGE_STG); \

ess-promote:
	@echo "Promoting $(ESS_IMAGE)"
	docker pull $(ESS_IMAGE)
	docker tag $(ESS_IMAGE) $(ESS_IMAGE_PROD)
	docker push $(ESS_IMAGE_PROD)
	docker tag $(ESS_IMAGE) $(ESS_IMAGE_LATEST)
	docker push $(ESS_IMAGE_LATEST)
	@echo "Promoting $(ESS_UBI_IMAGE)"
	docker pull $(ESS_UBI_IMAGE)
	docker tag $(ESS_UBI_IMAGE) $(ESS_UBI_IMAGE_PROD)
	docker push $(ESS_UBI_IMAGE_PROD)
	docker tag $(ESS_UBI_IMAGE) $(ESS_UBI_IMAGE_LATEST)
	docker push $(ESS_UBI_IMAGE_LATEST)

# This target should be used by developers working on anax, to build the ESS and CSS containers for the anax test environment.
# These containers only need to be rebuilt when either the authentication plugin changes or when anax rebases on a new level/tag
# of the edge-sync-service.
fss: ess-docker-image css-docker-image
	@echo "Built file sync service containers"

# This is a target that is ONLY called by the deb packager system. The ESS and CSS containers are built and published by that process
# when new versions are created. Developers should not use this target.
fss-package: ess-docker-image css-docker-image
	@echo "Packaging file sync service containers"
	if [[ $(shell tools/image-exists $(FSS_REGISTRY) $(ESS_IMAGE_NAME) $(ESS_IMAGE_VERSION) 2> /dev/null) == "0" || $(IMAGE_OVERRIDE) != "" ]]; then \
		echo "Pushing ESS docker image $(ESS_IMAGE)"; \
		docker push $(ESS_IMAGE); \
		docker push $(ESS_IMAGE_STG); \
	else \
		echo "File sync service container $(ESS_IMAGE_NAME):$(ESS_IMAGE_VERSION) already present in $(FSS_REGISTRY)"; \
	fi
	if [[ $(shell tools/image-exists $(FSS_REGISTRY) $(ESS_UBI_IMAGE_NAME) $(ESS_IMAGE_VERSION) 2> /dev/null) == "0" || $(IMAGE_OVERRIDE) != "" ]]; then \
		echo "Pushing ESS docker image $(ESS_UBI_IMAGE)"; \
		docker push $(ESS_UBI_IMAGE); \
		docker push $(ESS_UBI_IMAGE_STG); \
	else \
		echo "File sync service container $(ESS_UBI_IMAGE_NAME):$(ESS_IMAGE_VERSION) already present in $(FSS_REGISTRY)"; \
	fi
	if [[ $(shell tools/image-exists $(FSS_REGISTRY) $(CSS_IMAGE_NAME) $(CSS_IMAGE_VERSION) 2> /dev/null) == "0" || $(IMAGE_OVERRIDE) != "" ]]; then \
		echo "Pushing CSS docker image $(CSS_IMAGE)"; \
		docker push $(CSS_IMAGE); \
		docker push $(CSS_IMAGE_STG); \
	else \
		echo "File sync service container $(CSS_IMAGE_NAME):$(CSS_IMAGE_VERSION) already present in $(FSS_REGISTRY)"; \
	fi
	if [[ $(shell tools/image-exists $(FSS_REGISTRY) $(CSS_UBI_IMAGE_NAME) $(CSS_IMAGE_VERSION) 2> /dev/null) == "0" || $(IMAGE_OVERRIDE) != "" ]]; then \
		echo "Pushing CSS docker image $(CSS_UBI_IMAGE)"; \
		docker push $(CSS_UBI_IMAGE); \
		docker push $(CSS_UBI_IMAGE_STG); \
	else \
		echo "File sync service container $(CSS_UBI_IMAGE_NAME):$(CSS_IMAGE_VERSION) already present in $(FSS_REGISTRY)"; \
	fi

clean: mostlyclean i18n-clean anax-k8s-clean
	@echo "Clean"
	rm -f ./go.sum
ifneq ($(TMPGOPATH),$(GOPATH))
	rm -rf $(TMPGOPATH)
endif
	rm -rf ./contracts

mostlyclean: css-clean ess-clean
	@echo "Mostlyclean"
	rm -f $(EXECUTABLE) $(CLI_EXECUTABLE) $(CSS_EXECUTABLE) $(ESS_EXECUTABLE) $(CLI_CONFIG_FILE)
	-docker rmi $(ANAX_IMAGE) 2> /dev/null || :
	-docker rmi $(AGBOT_IMAGE) 2> /dev/null || :

i18n-clean:
	rm -f $(I18N_OUT_GOTEXT_FILES) cli/$(I18N_OUT_GOTEXT_FILES) $(I18N_CATALOG_FILE) cli/$(I18N_CATALOG_FILE)

css-clean:
	-docker rmi $(CSS_IMAGE) 2> /dev/null || :
	-docker rmi $(CSS_IMAGE_STG) 2> /dev/null || :
	-docker rmi $(CSS_UBI_IMAGE) 2> /dev/null || :
	-docker rmi $(CSS_UBI_IMAGE_STG) 2> /dev/null || :

ess-clean:
	-docker rmi $(ESS_IMAGE) 2> /dev/null || :
	-docker rmi $(ESS_IMAGE_STG) 2> /dev/null || :
	-docker rmi $(ESS_UBI_IMAGE) 2> /dev/null || :
	-docker rmi $(ESS_UBI_IMAGE_STG) 2> /dev/null || :

anax-k8s-clean:
	rm -f $(ANAX_K8S_CONTAINER_DIR)/hzn
	rm -f $(ANAX_K8S_CONTAINER_DIR)/anax
	-docker rmi $(ANAX_K8S_IMAGE_STG) 2> /dev/null || :

gofolders:
ifneq ($(GOPATH),$(TMPGOPATH))
	if [ ! -z $(GOPATH) ] && [ -w $(GOPATH) ] && [ -d $(GOPATH) ]; then \
		mkdir -p $(GOPATH)/pkg $(GOPATH)/bin; \
	fi
endif

i18n-catalog: $(TMPGOPATH)/bin/gotext
	@echo "Creating message catalogs"
	cd $(PKGPATH) && \
		export GOPATH=$(TMPGOPATH); export PATH=$(TMPGOPATH)/bin:$$PATH; \
			tools/update-i18n-messages

i18n-translation: deps i18n-catalog all-nodeps
	@echo "Copying message files for translation"
	cd $(PKGPATH) && \
		export PKGPATH=$(PKGPATH); export PATH=$(TMPGOPATH)/bin:$$PATH; \
			tools/copy-i18n-messages


$(TMPGOPATH)/bin/gotext: gopathlinks
	if [ ! -e "$(TMPGOPATH)/bin/gotext" ]; then \
		echo "Fetching gotext"; \
		export GOPATH=$(TMPGOPATH); export PATH=$(TMPGOPATH)/bin:$$PATH; \
			go get -u golang.org/x/text/cmd/gotext; \
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
	find . -name '*.go' -exec gofmt -l -w {} \;

lint: gopathlinks
	@echo "Checking source code for style issues and statically-determinable errors"
	-golint ./...
	-cd $(PKGPATH) && \
		GOPATH=$(TMPGOPATH) $(COMPILE_ARGS) go vet $(shell find . -iname '*.go' -print | xargs dirname | sort | uniq | xargs) 2>&1 | grep -vP "^exit.*"

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

check-noi18n: lint test test-integration


# build sequence diagrams
diagrams:
	java -jar $(plantuml_path)/plantuml.jar ./citizenscientist/diagrams/horizonSequenceDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./citizenscientist/diagrams/protocolSequenceDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./messaging/diagrams/senderEncryption.txt
	java -jar $(plantuml_path)/plantuml.jar ./messaging/diagrams/receiverEncryption.txt
	java -jar $(plantuml_path)/plantuml.jar ./basicprotocol/diagrams/protocolSequenceDiagram.txt
	java -jar $(plantuml_path)/plantuml.jar ./basicprotocol/diagrams/horizonSequenceDiagram.txt

.PHONY: check clean deps format gopathlinks install lint mostlyclean pull i18n-catalog i18n-translation test test-integration docker-image docker-push promote-mac-pkg-and-docker promote-mac-pkg promote-anax gen-mac-key install-mac-key css-docker-image ess-promote css-docker-image ess-promote
