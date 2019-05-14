# Makefile for e2e execution
SHELL := /bin/bash

#Versioning
DOCKER_EXCH_TAG := 1.81.0
PKG_URL_BASE := http://pkg.bluehorizon.network/linux/ubuntu
PKGS_URL := $(PKG_URL_BASE)/dists/xenial-testing/main/binary-amd64/Packages
DEB_PKGS_URL := $(PKG_URL_BASE)/pool/main/h/horizon
HORIZON_PKG_VER ?= $(shell curl -s $(PKGS_URL) | grep -A 2 'Package: horizon' | grep -m 1 'Version:' | awk -F ' |~' '{print $$2}')

# user configurable variables
# The TEST_PATTERNS is a comma separated list of following patterns.
# sall,sns,spws,susehello,sgps,sloc,cpu2msghub (These are pattern with services)
# all,ns,loc,gps,ns-keytest (These are patterns with workloads and microservices)
#
# If cpu2msghub pattern is in the TEST_PATTERNS, then 
# please export the following variables before calling 'make test'
#    MSGHUB_BROKER_URL
#    MSGHUB_API_KEY
#

TEST_VARS ?= NOLOOP=1 TEST_PATTERNS=sall
# TEST_VARS ?= NOLOOP=1 NOCANCEL=1
APPSTORE = $(shell echo $(TEST_VARS) | awk -F "APPSTORE=" '{print $$2}' | cut -c1)

# msghub
MSGHUB_VARS := MSGHUB_BROKER_URL='$(MSGHUB_BROKER_URL)' MSGHUB_API_KEY='$(MSGHUB_API_KEY)'

BRANCH_NAME ?= ""
ANAX_SOURCE ?= $(shell dirname $(CURDIR))
AGBOT_TEMPFS := $(CURDIR)/docker/tempfs/agbot
EXCH_TEMPFS := $(CURDIR)/docker/tempfs/exchange
CSS_CONFIG := $(CURDIR)/docker/fs/etc/edge-sync-service/
UDS := /var/run/horizon/
EXCHDB_USER := admin
EXCHDB_PORT := 5432
EXCHDB_NAME := exchange
EXCH_ROOTPW := Horizon-Rul3s
DOCKER_TEST_NETWORK := e2edev_test_network
DOCKER_AGBOT_INAME := openhorizon/e2edev-agbot
DOCKER_EXCH_INAME := openhorizon/amd64_exchange-api
DOCKER_DEV_TAG := latest
DOCKER_EXCHDB_INAME := postgres
DOCKER_EXCHDB_TAG := 9
DOCKER_REG_INAME := registry
DOCKER_REG_TAG := 2
DOCKER_CSSDB_INAME := mongo
DOCKER_CSSDB_TAG := 4.0.6
DOCKER_CSS_INAME := openhorizon/amd64_cloud-sync-service
DOCKER_CSS_TAG := testing$(BRANCH_NAME)
DOCKER_DEV_OPTS :=  --rm --no-cache
E2EDEV_HOST_IP := $(shell hostname -I | awk '{print $$1}')

ifndef verbose
.SILENT:
endif

all: default
default: build
stop: appstore-stop clean
build: version-check agbot-image exchange-db-image exchange-image minikube-setup css-mongo-db-image
run: test
distclean: realclean

DOCKER_AGBOT_CNAME := agbot
DOCKER_EXCH_CNAME := exchange-api
DOCKER_EXCHDB_CNAME := exchange-db
DOCKER_REG_CNAME := e2edevregistry
DOCKER_REG_PW_CNAME := htregistry
DOCKER_REG_USER := testuser
DOCKER_REG_PW := testpassword
DOCKER_CSS_CNAME := css-api
DOCKER_CSSDB_CNAME := css-db

run-agbot: agbot-docker-prereqs test-network
	@echo "Handling $(DOCKER_AGBOT_CNAME)"
	docker/docker_run.bash "$(DOCKER_AGBOT_CNAME)" \
		docker run -d \
		--privileged \
		-p 127.0.0.1:80:80 \
		-p 127.0.0.1:81:81 \
		-p 127.0.0.1:82:82 \
		--name "$(DOCKER_AGBOT_CNAME)" \
		--network "$(DOCKER_TEST_NETWORK)" \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(UDS):$(UDS):rw \
		-v ~/.docker:/root/.docker \
		-v ~/.kube:/root/.kube \
		-v ~/.minikube:$(HOME)/.minikube \
		-v /etc/wiotp-edge/:/etc/wiotp-edge/ \
		-v /var/wiotp-edge/persist/:/var/wiotp-edge/persist/ \
		-v /tmp/ess-auth/:/tmp/ess-auth/:rw \
		-v /tmp/hzndev/:/tmp/hzndev/ \
		-v $(AGBOT_TEMPFS)/certs:/certs \
		-e "HZN_DISABLE_IPTABLES_MANIPULATION=true" \
		-e "EXCH_APP_HOST=$(DOCKER_EXCH_CNAME)" \
		-e "DOCKER_REG_CNAME=$(DOCKER_REG_CNAME)" \
		-e "DOCKER_REG_USER=$(DOCKER_REG_USER)" \
		-e "DOCKER_REG_PW=$(DOCKER_REG_PW)" \
		-e "AGBOT_HZN_API=http://localhost:81" \
		-e "HZN_DEV_HOST_IP=$(E2EDEV_HOST_IP)" \
		-e "HZN_DEV_FSS_IMAGE_TAG"="testing$(BRANCH_NAME)" \
		-t $(DOCKER_AGBOT_INAME):$(DOCKER_DEV_TAG)
	docker cp $(AGBOT_TEMPFS)/. $(DOCKER_AGBOT_CNAME):/

run-dockerreg: test-network
	@echo "Handling Docker $(DOCKER_REG_CNAME)"
	mkdir -p $(AGBOT_TEMPFS)/certs
	@echo "Generating TLS cert for docker registry $(DOCKER_REG_CNAME)"
	openssl req \
		-newkey rsa:4096 \
		-nodes -sha256 -x509 \
		-keyout $(AGBOT_TEMPFS)/certs/domain.key \
		-days 365 \
		-out $(AGBOT_TEMPFS)/certs/domain.crt \
		-subj "/C=US/ST=e2edev@somecomp.com/L=e2edev@somecomp.com/O=e2edev@somecomp.com/CN=localhost"
	mkdir -p $(AGBOT_TEMPFS)/auth
	touch $(AGBOT_TEMPFS)/auth/htpasswd
	docker/docker_run.bash "$(DOCKER_REG_CNAME)" \
		docker run -d \
		-p 127.0.0.1:443:443 \
		--name $(DOCKER_REG_CNAME) \
		--network "$(DOCKER_TEST_NETWORK)" \
		-v $(AGBOT_TEMPFS)/certs:/certs \
		-e REGISTRY_HTTP_ADDR=0.0.0.0:443 \
		-e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt \
		-e REGISTRY_HTTP_TLS_KEY=/certs/domain.key \
		-v $(AGBOT_TEMPFS)/auth:/auth \
		-e REGISTRY_AUTH=htpasswd \
		-e REGISTRY_AUTH_HTPASSWD_REALM=RegistryRealm \
		-e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
		-t $(DOCKER_REG_INAME):$(DOCKER_REG_TAG)
	@echo "Docker $(DOCKER_REG_CNAME) container running with TLS"
	docker run --name $(DOCKER_REG_PW_CNAME) \
		--entrypoint htpasswd \
		-t $(DOCKER_REG_INAME):$(DOCKER_REG_TAG) \
		-Bbn $(DOCKER_REG_USER) $(DOCKER_REG_PW) > $(AGBOT_TEMPFS)/auth/htpasswd
	docker rm -v $(DOCKER_REG_PW_CNAME)

run-exchange: run-exchange-db test-network $(EXCH_TEMPFS)/config.json
	@echo "Handling $(DOCKER_EXCH_CNAME)"
	docker/docker_run.bash "$(DOCKER_EXCH_CNAME)" \
		docker run -d \
		-p 127.0.0.1:8080:8080 \
		--name "$(DOCKER_EXCH_CNAME)" \
		--network "$(DOCKER_TEST_NETWORK)" \
		-v $(EXCH_TEMPFS)/config.json:/etc/horizon/exchange/config.json:ro \
		-t openhorizon/amd64_exchange-api:$(DOCKER_EXCH_TAG)

$(EXCH_TEMPFS)/config.json:
	@echo "Populating the exchange configuration"
	mkdir -p $(EXCH_TEMPFS)
	echo "{ \"api\": { \"db\": { \"jdbcUrl\": \"jdbc:postgresql://$(DOCKER_EXCHDB_CNAME):$(EXCHDB_PORT)/$(EXCHDB_NAME)\"," \
                "\"user\": \"$(EXCHDB_USER)\" }, \"root\": { \"password\": \"$(EXCH_ROOTPW)\" } } }" > $(EXCH_TEMPFS)/config.json

run-exchange-db: test-network
	@echo "Handling $(DOCKER_EXCHDB_CNAME)"
	docker/docker_run.bash "$(DOCKER_EXCHDB_CNAME)" \
		docker run -d \
		--name "$(DOCKER_EXCHDB_CNAME)" \
		--network "$(DOCKER_TEST_NETWORK)" \
		-e POSTGRES_DB=$(EXCHDB_NAME) \
		-e POSTGRES_USER=$(EXCHDB_USER) \
		-t $(DOCKER_EXCHDB_INAME):$(DOCKER_EXCHDB_TAG)

clean:
	@echo -e "\nStarting cleanup"
	@echo "Shutdown anax cleanly"
	-curl -sSLX DELETE http://localhost/node
	@echo "Removing agbot container"
	-docker rm -f "$(DOCKER_AGBOT_CNAME)"
	@echo "Dropping and re-initializing the exchange DB"
	$(eval DROP_DB_TOKEN := $(shell curl -sLX GET -H 'Authorization:Basic root/root:$(EXCH_ROOTPW)' "http://localhost:8080/v1/admin/dropdb/token" | jq -r '.token'))
	-curl -sLX POST -H 'Authorization:Basic root/root:$(DROP_DB_TOKEN)' "http://localhost:8080/v1/admin/dropdb" | jq -r '.msg'
	-curl -sLX POST -H "Authorization:Basic root/root:$(EXCH_ROOTPW)" "http://localhost:8080/v1/admin/initdb" | jq -r '.msg'
	@echo "Removing stale networks"
	-docker network prune -f
	@echo "Removing unused images"
	-docker rmi $$(docker images -qf "dangling=true")
	@echo "Removing any leftover images generated by e2edev"
	-docker rmi $$(docker images | egrep "openhorizon/|wiotp-|localhost" | egrep -v "e2edev-agbot|amd64_exchange-api|amd64_cloud-sync-service|amd64_edge-sync-service|REPOSITORY" | awk '{print $$3}')
	@echo "Removing the $(DOCKER_EXCH_CNAME) container"
	-docker rm -f "$(DOCKER_EXCH_CNAME)"
	@echo "Removing the $(DOCKER_EXCHDB_CNAME) container"
	-docker rm -v -f "$(DOCKER_EXCHDB_CNAME)"
	@echo "Removing the $(DOCKER_REG_CNAME) container"
	-docker rm -v -f "$(DOCKER_REG_CNAME)"
	@echo "Removing the $(DOCKER_CSSDB_CNAME) container"
	-docker rm -v -f "$(DOCKER_CSSDB_CNAME)"
	@echo "Removing the $(DOCKER_CSS_CNAME) container"
	-docker rm -f "$(DOCKER_CSS_CNAME)"
	-rm -rf $(CURDIR)/docker/tempfs
	-sudo rm -fr $(UDS)

mostlyclean: clean
	@echo "Cleaning anax binaries and configs"
	-cd $(ANAX_SOURCE) && make clean

realclean: mostlyclean
	-docker rm -f "$(DOCKER_EXCHDB_CNAME)" "$(DOCKER_EXCH_CNAME)"
	-docker rmi $(DOCKER_AGBOT_INAME):$(DOCKER_DEV_TAG) $(DOCKER_AGBOT_INAME):latest
	-docker rmi openhorizon/amd64_exchange-api:$(DOCKER_EXCH_TAG)
	-docker rmi $(DOCKER_CSS_INAME):$(DOCKER_CSS_TAG)
	-docker network rm $(DOCKER_TEST_NETWORK)
	-docker rmi $$(docker images -qf "dangling=true")

test-network:
	@echo "Creating e2edev environment network"
	if [[ "$(shell docker network ls -qf name=$(DOCKER_TEST_NETWORK))" == "" ]]; then \
		docker network create "$(DOCKER_TEST_NETWORK)"; \
	fi

$(ANAX_SOURCE)/anax:
	@echo "Building anax"
	cd $(ANAX_SOURCE) && make clean && make

agbot-docker-prereqs: $(ANAX_SOURCE)/anax
	@echo "Configuring agbot prerequisites"
	mkdir -p /tmp/ess-store/ /tmp/ess-auth/ /tmp/service_storage/ /tmp/hzndev/
	mkdir -p $(AGBOT_TEMPFS)/usr/local/bin $(AGBOT_TEMPFS)/root/.colonus
	cp $(ANAX_SOURCE)/anax $(ANAX_SOURCE)/cli/hzn $(AGBOT_TEMPFS)/usr/local/bin
	cp -r $(CURDIR)/docker/fs/etc $(AGBOT_TEMPFS)
	cp -r $(CURDIR)/docker/fs/hzn $(CURDIR)/gov/* $(CURDIR)/docker/fs/helm $(CURDIR)/docker/fs/resources $(AGBOT_TEMPFS)/root
	for f in $$(find $(AGBOT_TEMPFS)/etc/agbot/ -maxdepth 1 -name '*.tmpl'); do EXCH_APP_HOST=$(DOCKER_EXCH_CNAME) envsubst < $$f > "$$(echo $$f | sed 's/.tmpl//')"; done
	for f in $$(find $(AGBOT_TEMPFS)/etc/colonus/ -maxdepth 1 -name '*.tmpl'); do EXCH_APP_HOST=$(DOCKER_EXCH_CNAME) envsubst < $$f > "$$(echo $$f | sed 's/.tmpl//')"; done
	curl $(DEB_PKGS_URL)/horizon_$(HORIZON_PKG_VER)~ppa~ubuntu.xenial.orig.tar.gz \
	-o $(AGBOT_TEMPFS)/etc/colonus/horizon_$(HORIZON_PKG_VER)~ppa~ubuntu.xenial.orig.tar.gz
	tar --strip=6 -C $(AGBOT_TEMPFS)/root/.colonus -zxvf $(AGBOT_TEMPFS)/etc/colonus/horizon_$(HORIZON_PKG_VER)~ppa~ubuntu.xenial.orig.tar.gz \
	./debian/fs-bluehorizon/etc/horizon/trust/horizon.pem ./debian/fs-bluehorizon/etc/horizon/trust/mtn-publicKey.pem

version-check:
	@echo "Horizon version being used for public keys/configs: $(HORIZON_PKG_VER)"
ifeq ($(HORIZON_PKG_VER),)
        $(error Version not found. Likely the apt-repo is down; $(PKGS_URL))
endif

agbot-image:
	@echo "Handling $(DOCKER_AGBOT_INAME)"
	if [ -n "$(shell docker images | grep '$(DOCKER_AGBOT_INAME)')" ]; then \
		echo "Skipping since $(DOCKER_AGBOT_INAME) image exists, run 'make clean && make' if a rebuild is desired"; \
	else \
		echo "Building container image $(DOCKER_AGBOT_INAME)"; \
		docker build $(DOCKER_DEV_OPTS) -t $(DOCKER_AGBOT_INAME) -f docker/Dockerfile ./docker && docker tag $(DOCKER_AGBOT_INAME) $(DOCKER_AGBOT_INAME):$(DOCKER_DEV_TAG); \
	fi

exchange-db-image:
	docker pull $(DOCKER_EXCHDB_INAME):$(DOCKER_EXCHDB_TAG)

exchange-image:
	docker pull $(DOCKER_EXCH_INAME):$(DOCKER_EXCH_TAG)

test: clean run-dockerreg run-exchange run-css appstore-start run-agbot
	@echo -e  "\nStarting tests"
	docker exec $(DOCKER_AGBOT_CNAME) bash -c "export $(TEST_VARS); export $(MSGHUB_VARS); /root/gov-combined.sh"

minikube-setup:
	@echo "Setting up minikube"
	if [ ! -f /usr/local/bin/kubectl ]; then \
		touch minikube-setup-by-test; \
		./scripts/setup_minikube.sh; \
		./scripts/stop_minikube.sh; \
	else \
		echo "Skipping minikube setup, already done"; \
	fi

minikube-start: minikube-setup
	@echo "Starting minikube"
	./scripts/start_minikube.sh

minikube-stop:
	@echo "Stopping minikube"
	./scripts/stop_minikube.sh

minikube-clean: minikube-stop
	@echo "Cleaning up minikube"
	if [ -f minikube-setup-by-test ]; then \
		./scripts/remove_minikube.sh; \
		rm -f minikube-setup-by-test; \
	else \
		echo "Skipping minikube cleanup, minikube was not setup by the test environment"; \
	fi

css-mongo-db-image:
	docker pull $(DOCKER_CSSDB_INAME):$(DOCKER_CSSDB_TAG)

run-mongo-db: test-network
	@echo "Handling $(DOCKER_CSSDB_CNAME)"
	docker/docker_run.bash "$(DOCKER_CSSDB_CNAME)" \
		docker run -d \
		--name "$(DOCKER_CSSDB_CNAME)" \
		-p 127.0.0.1:27017:27017 \
		--network "$(DOCKER_TEST_NETWORK)" \
		-t $(DOCKER_CSSDB_INAME):$(DOCKER_CSSDB_TAG)

run-css: run-mongo-db
	@echo "Generating TLS cert for CSS $(DOCKER_CSS_CNAME)"
	openssl req \
		-newkey rsa:4096 \
		-nodes -sha256 -x509 \
		-keyout $(AGBOT_TEMPFS)/certs/css.key \
		-days 365 \
		-out $(AGBOT_TEMPFS)/certs/css.crt \
		-subj "/C=US/ST=e2edev@somecomp.com/L=e2edev@somecomp.com/O=e2edev@somecomp.com/CN=$(DOCKER_CSS_CNAME)"
	@echo "Starting $(DOCKER_CSS_CNAME)"
	docker/docker_run.bash "$(DOCKER_CSS_CNAME)" \
		docker run -d \
		--name "$(DOCKER_CSS_CNAME)" \
		-p 127.0.0.1:9443:9443 \
		--network "$(DOCKER_TEST_NETWORK)" \
		-e "HZN_EXCHANGE_URL=http://$(DOCKER_EXCH_CNAME):8080/v1" \
		-e "HZN_EXCHANGE_CA_CERT=/certs/domain.crt" \
		-v $(CSS_CONFIG):/etc/edge-sync-service/ \
		-v $(AGBOT_TEMPFS)/certs/:/certs/ \
		-t $(DOCKER_CSS_INAME):$(DOCKER_CSS_TAG)

appstore-start:
	if [ "$(APPSTORE)" = "1" ]; then \
		echo "Starting app store"; \
		docker run -d --rm --name app-store -p 9001:9001 -e EXCHANGE_URL=http://$(DOCKER_EXCH_CNAME):8080/v1 --network "$(DOCKER_TEST_NETWORK)" registry.ng.bluemix.net/rj-dev/app-store:0.7.0; \
		docker run -d --rm --name app-store-ui -p 9015:9015 --network "$(DOCKER_TEST_NETWORK)" -e NODE_ENV=production -e APP_STORE_CONTAINER_NAME=app-store -e APP_STORE_CONTAINER_PORT=9001 registry.ng.bluemix.net/rj-dev/app-store-ui:0.11.0; \
	fi

appstore-stop:
	@echo "Stopping app store"
	-docker stop app-store-ui
	-docker stop app-store

.PHONY: all default stop build run distclean clean mostlyclean realclean run-agbot run-dockerreg run-exchange run-exchange-db test-network test agbot-docker-prereqs version-check agbot-image exchange-db-image exchange-image minikube-setup minikube-start minikube-stop minikube-clean appstore-start appstore-stop css-mongo-db-image run-mongo-db run-css
