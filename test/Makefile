# Makefile for e2e execution
SHELL := /bin/bash

#Versioning
DOCKER_EXCH_TAG := 2.15.1
PKG_URL_BASE := http://169.45.88.181/linux/ubuntu
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
EXCH_ROOTPW ?= Horizon-Rul3s
AGBOT_NAME ?= ag12345
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
DOCKER_ANAX_K8S_INAME := openhorizon/amd64_anax_k8s:testing
DOCKER_DEV_OPTS :=  --rm --no-cache
E2EDEV_HOST_IP := $(shell hostname -I | awk '{print $$1}')
AGBOT_SAPI_URL ?= https://localhost:8083
ICP_HOST_IP ?= 0

ifndef verbose
.SILENT:
endif

all: default
default: build
stop: clean
build: version-check agbot-image exchange-db-image exchange-image css-mongo-db-image
build-remote: version-check agbot-image
run: test
distclean: realclean

DOCKER_AGBOT_CNAME := agbot
DOCKER_EXCH ?= http://exchange-api:8080/v1
CSS_URL ?= https://css-api:9443
API_KEY ?= 0
CERT_LOC ?= 1
DOCKER_EXCH_CNAME = exchange-api
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
		-p 127.0.0.1:8510:8510 \
		-p 127.0.0.1:8511:8511 \
		-p 127.0.0.1:8082:8082 \
		-p 127.0.0.1:8083:8083 \
		-p 127.0.0.1:8084:8084 \
		-p 127.0.0.1:8085:8085 \
		--name "$(DOCKER_AGBOT_CNAME)" \
		--network "$(DOCKER_TEST_NETWORK)" \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(UDS):$(UDS):rw \
		-v ~/.docker:/home/agbotuser/.docker \
		-v /etc/wiotp-edge/:/etc/wiotp-edge/ \
		-v /var/wiotp-edge/persist/:/var/wiotp-edge/persist/ \
		-v /tmp/ess-auth/:/tmp/ess-auth/:rw \
		-v /tmp/hzndev/:/tmp/hzndev/ \
		-v $(AGBOT_TEMPFS)/certs:/certs \
		-v /var/log/:/var/log/ \
		-e "HZN_DISABLE_IPTABLES_MANIPULATION=true" \
		-e "EXCH_APP_HOST=$(DOCKER_EXCH)" \
		-e "HZN_SSL_SKIP_VERIFY=1" \
		-e "CSS_URL=$(CSS_URL)" \
		-e "API_KEY=$(API_KEY)" \
		-e "CERT_LOC=$(CERT_LOC)" \
		-e "EXCH_ROOTPW=$(EXCH_ROOTPW)" \
		-e "AGBOT_NAME=$(AGBOT_NAME)" \
		-e "AGBOT_SAPI_URL=$(AGBOT_SAPI_URL)" \
		-e "ICP_HOST_IP=$(ICP_HOST_IP)" \
		-e "DOCKER_REG_CNAME=$(DOCKER_REG_CNAME)" \
		-e "DOCKER_REG_USER=$(DOCKER_REG_USER)" \
		-e "DOCKER_REG_PW=$(DOCKER_REG_PW)" \
		-e "HZN_AGBOT_API=http://localhost:8082" \
		-e "HZN_EXCHANGE_URL=$(DOCKER_EXCH)" \
		-e "HZN_DEV_HOST_IP=$(E2EDEV_HOST_IP)" \
		-e "HZN_DEV_FSS_IMAGE_TAG"="testing$(BRANCH_NAME)" \
		-e "HZN_MGMT_HUB_CERT_PATH"="/certs/css.crt" \
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
		-p 127.0.0.1:8090:8080 \
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
		-e POSTGRES_HOST_AUTH_METHOD=trust \
		-t $(DOCKER_EXCHDB_INAME):$(DOCKER_EXCHDB_TAG)

clean:
	@echo -e "\nStarting cleanup"
	@echo "Clean up kube environment"
	gov/stop_kube.sh
	@echo "Shutdown anax cleanly"
	-@curl -sSLX DELETE http://localhost:8510/node 2>/dev/null || true
	@echo "Removing agbot container"
	-@docker rm -f "$(DOCKER_AGBOT_CNAME)" 2>/dev/null || true
	@echo "Dropping and re-initializing the exchange DB"
	@$(eval DROP_DB_TOKEN := $(shell curl -sLX GET -u "root/root:$(EXCH_ROOTPW)" "http://localhost:8080/v1/admin/dropdb/token" | jq -r '.token')) 2>/dev/null || true
	-@curl -sLX POST -u "root/root:$(DROP_DB_TOKEN)" "http://localhost:8080/v1/admin/dropdb" | jq -r '.msg' 2>/dev/null || true
	-@curl -sLX POST -u "root/root:$(EXCH_ROOTPW)" "http://localhost:8080/v1/admin/initdb" | jq -r '.msg' 2>/dev/null || true
	@echo "Removing stale networks"
	-@docker network prune -f 2>/dev/null || true
	@echo "Removing unused images"
	-@docker rmi $$(docker images -qf "dangling=true") 2>/dev/null || true
	@echo "Removing any leftover images generated by e2edev"
	-@docker rmi $$(docker images | egrep "openhorizon/|wiotp-|localhost" | egrep -v "e2edev-agbot|amd64_exchange-api|amd64_cloud-sync-service|amd64_edge-sync-service|amd64_anax_k8s|REPOSITORY" | awk '{print $$3}') 2>/dev/null || true
	@echo "Removing the $(DOCKER_EXCH_CNAME) container"
	-@docker rm -f "$(DOCKER_EXCH_CNAME)" 2>/dev/null || true
	@echo "Removing the $(DOCKER_EXCHDB_CNAME) container"
	-@docker rm -v -f "$(DOCKER_EXCHDB_CNAME)" 2>/dev/null || true
	@echo "Removing the $(DOCKER_REG_CNAME) container"
	-@docker rm -v -f "$(DOCKER_REG_CNAME)" 2>/dev/null || true
	@echo "Removing the $(DOCKER_CSSDB_CNAME) container"
	-@docker rm -v -f "$(DOCKER_CSSDB_CNAME)" 2>/dev/null || true
	@echo "Removing the $(DOCKER_CSS_CNAME) container"
	-@docker rm -f "$(DOCKER_CSS_CNAME)" 2>/dev/null || true
	@echo "Cleaning up any left over workloads"
	-@docker stop $(docker ps -a | egrep "openhorizon/|wiotp-|localhost" | egrep -v "e2edev-agbot|amd64_exchange-api|amd64_cloud-sync-service|amd64_edge-sync-service|REPOSITORY" | awk '{print $$1}') 2>/dev/null || true
	-@docker rm $(docker ps -a | egrep "openhorizon/|wiotp-|localhost" | egrep -v "e2edev-agbot|amd64_exchange-api|amd64_cloud-sync-service|amd64_edge-sync-service|REPOSITORY" | awk '{print $$1}') 2>/dev/null || true
	@echo "Removing working directories"
	-@rm -rf $(CURDIR)/docker/tempfs 2>/dev/null || true
	-@sudo rm -fr $(UDS) 2>/dev/null || true

mostlyclean: clean
	@echo "Cleaning anax binaries and configs"
	-cd $(ANAX_SOURCE) && make clean

realclean: mostlyclean
	-@docker rm -f "$(DOCKER_EXCHDB_CNAME)" "$(DOCKER_EXCH_CNAME)" 2>/dev/null || true
	-@docker rmi $(DOCKER_AGBOT_INAME):$(DOCKER_DEV_TAG) $(DOCKER_AGBOT_INAME):latest 2>/dev/null || true
	-@docker rmi openhorizon/amd64_exchange-api:$(DOCKER_EXCH_TAG) 2>/dev/null || true
	-@docker rmi $(DOCKER_CSS_INAME):$(DOCKER_CSS_TAG) 2>/dev/null || true
	-@docker network rm $(DOCKER_TEST_NETWORK) 2>/dev/null || true
	-@docker rmi $$(docker images -qf "dangling=true") 2>/dev/null || true

test-network:
	@echo "Creating e2edev environment network"
	if [[ "$(shell docker network ls -qf name=$(DOCKER_TEST_NETWORK))" == "" ]]; then \
		docker network create "$(DOCKER_TEST_NETWORK)"; \
	fi

$(ANAX_SOURCE)/anax:
	@echo "Building anax"
	cd $(ANAX_SOURCE) && make clean && make && make ess-docker-image && make css-docker-image

agbot-docker-prereqs: $(ANAX_SOURCE)/anax
	@echo "Configuring agbot prerequisites"
	if [[ "$(shell docker images -q $(DOCKER_ANAX_K8S_INAME) 2>/dev/null)" == "" ]]; then \
		cd $(ANAX_SOURCE) && make anax-k8s-image; \
	fi
	mkdir -p /tmp/ess-store/ /tmp/ess-auth/ /tmp/service_storage/ /tmp/hzndev/
	mkdir -p $(AGBOT_TEMPFS)/usr/local/bin $(AGBOT_TEMPFS)/root/.colonus
	cp $(ANAX_SOURCE)/anax $(ANAX_SOURCE)/cli/hzn $(AGBOT_TEMPFS)/usr/local/bin
	cp -r $(CURDIR)/docker/fs/etc $(AGBOT_TEMPFS)
	cp -r $(CURDIR)/docker/fs/hzn $(CURDIR)/gov/* $(CURDIR)/docker/fs/helm $(CURDIR)/docker/fs/resources $(CURDIR)/docker/fs/objects $(AGBOT_TEMPFS)/root
	for f in $$(find $(AGBOT_TEMPFS)/etc/agbot/ -maxdepth 1 -name '*.tmpl'); do EXCH_APP_HOST=$(DOCKER_EXCH) CSS_URL=$(CSS_URL) envsubst < $$f > "$$(echo $$f | sed 's/.tmpl//')"; done
	for f in $$(find $(AGBOT_TEMPFS)/etc/colonus/ -maxdepth 1 -name '*.tmpl'); do EXCH_APP_HOST=$(DOCKER_EXCH) CSS_URL=$(CSS_URL) envsubst < $$f > "$$(echo $$f | sed 's/.tmpl//')"; done
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

test: clean $(ANAX_SOURCE)/anax run-dockerreg run-exchange run-css run-agbot
	@echo -e  "\nSetting up agent in kube"
	gov/run_kube.sh $(AGBOT_TEMPFS) $(ANAX_SOURCE) $(EXCH_ROOTPW)
	@echo -e  "\nStarting tests"
	docker exec $(DOCKER_AGBOT_CNAME) bash -c "export $(TEST_VARS); export $(MSGHUB_VARS); /root/gov-combined.sh"

test-remote: clean $(ANAX_SOURCE)/anax run-dockerreg run-agbot copy-cert
	@echo -e  "\nStarting tests"
	docker exec $(DOCKER_AGBOT_CNAME) bash -c "export $(TEST_VARS); export $(MSGHUB_VARS); /root/gov-combined.sh"

copy-cert:
	cp css.crt $(AGBOT_TEMPFS)/certs/css.crt
	cp agbotapi.crt $(AGBOT_TEMPFS)/certs/agbotapi.crt

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
	chmod +r $(AGBOT_TEMPFS)/certs/css.key
	@echo "Starting $(DOCKER_CSS_CNAME)"
	docker/docker_run.bash "$(DOCKER_CSS_CNAME)" \
		docker run -d \
		--name "$(DOCKER_CSS_CNAME)" \
		-p 127.0.0.1:9443:9443 \
		--network "$(DOCKER_TEST_NETWORK)" \
		-e "HZN_EXCHANGE_URL=$(DOCKER_EXCH)" \
		-e "HZN_EXCHANGE_CA_CERT=/certs/domain.crt" \
		-v $(CSS_CONFIG):/etc/edge-sync-service/ \
		-v $(AGBOT_TEMPFS)/certs/:/certs/ \
		-t $(DOCKER_CSS_INAME):$(DOCKER_CSS_TAG)

.PHONY: all default stop build run distclean clean mostlyclean realclean run-agbot run-dockerreg run-exchange run-exchange-db test-network test agbot-docker-prereqs version-check agbot-image exchange-db-image exchange-image css-mongo-db-image run-mongo-db run-css copy-cert
#.SILENT: clean
