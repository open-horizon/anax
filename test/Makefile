# Makefile for e2e execution
SHELL := /bin/bash

# Get Arch for tag and hardware (Golang style) to run test
arch_tag ?= $(shell ../tools/arch-tag)
arch ?= $(arch_tag)
ifeq ($(arch),ppc64el)
	arch := ppc64le
endif

#Versioning
DOCKER_EXCH_TAG := testing

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
VAULT_CONFIG := $(CURDIR)/docker/fs/etc/vault/config.json
UDS := /var/run/horizon/
EXCHDB_USER := admin
EXCHDB_PORT := 5432
EXCHDB_NAME := exchange
EXCH_ROOTPW ?= Horizon-Rul3s
AGBOT_NAME ?= ag12345
VAULT_DEV_ROOT_TOKEN_ID := $(shell jq ".VAULT_DEV_ROOT_TOKEN_ID" $(VAULT_CONFIG))
VAULT_DEV_LISTEN_ADDRESS := $(shell jq ".VAULT_DEV_LISTEN_ADDRESS" $(VAULT_CONFIG))
DOCKER_TEST_NETWORK := e2edev_test_network
DOCKER_AGBOT_INAME := openhorizon/e2edev-agbot
DOCKER_EXCH_INAME := openhorizon/$(arch)_exchange-api
DOCKER_DEV_TAG := latest
DOCKER_EXCHDB_INAME ?= postgres
ifeq ($(arch),ppc64le)
# ppc64le settings (new)
	DOCKER_EXCHDB_TAG ?= 13.2
	DOCKER_REG_INAME ?= ibmcom/registry-ppc64le
	DOCKER_REG_TAG ?= sha256:b95d9a4abc64bef338b71836ac6876d59281f34f0f03be1023673d52ba1f40e0
	DOCKER_CSSDB_INAME ?= ppc64le/mongodb
	DOCKER_CSSDB_TAG ?= 2.6.10
	DOCKER_CPU_INAME ?= openhorizon/example_ms_$(arch)_cpu
else
# amd64 settings (default)
	DOCKER_EXCHDB_TAG ?= 9
	DOCKER_REG_INAME ?= registry
	DOCKER_REG_TAG ?= sha256:7d081088e4bfd632a88e3f3bcd9e007ef44a796fddfe3261407a3f9f04abe1e7
	DOCKER_CSSDB_INAME ?= mongo
	DOCKER_CSSDB_TAG ?= 4.0.6
	DOCKER_CPU_INAME ?= openhorizon/example_ms_x86_cpu
endif
DOCKER_CPU_TAG := 1.2.2
DOCKER_CSS_INAME := openhorizon/$(arch)_cloud-sync-service
DOCKER_CSS_TAG := testing$(BRANCH_NAME)
DOCKER_HASHICORP_VAULT_INAME := hashicorp/vault
DOCKER_HASHICORP_VAULT_TAG := latest
DOCKER_ANAX_K8S_INAME := openhorizon/$(arch)_anax_k8s:testing
DOCKER_DEV_OPTS :=  --rm --no-cache --build-arg ARCH=$(arch)
DOCKER_ANAX_INAME := openhorizon/$(arch)_anax:testing
E2EDEV_HOST_IP := $(shell hostname -I | awk '{print $$1}')
AGBOT_SAPI_URL ?= https://localhost:8083
ICP_HOST_IP ?= 0
DOCKER_AGBOT_ADD_HOST ?= "localhost:127.0.0.1"
ORG_ID ?= ""

export PREBUILT_DOCKER_REG_URL ?= ""
export PREBUILT_DOCKER_REG_USER ?= ""
export PREBUILT_DOCKER_REG_PW ?= ""
export PREBUILT_ANAX_VERSION ?= nightly
export PREBUILT_ESS_VERSION ?= nightly

ifndef verbose
.SILENT:
endif

all: default
default: build
stop: clean
build: agbot-image exchange-db-image exchange-image css-mongo-db-image hashicorp-vault-image
build-remote: agbot-image
run: test
distclean: realclean

DOCKER_AGBOT_CNAME := agbot
DOCKER_EXCH ?= http://exchange-api:8080/v1
CSS_URL ?= https://css-api:9443
VAULT_ADDR = http://vault:8200
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
DOCKER_HASHICORP_VAULT_CNAME := vault
MUL_AGENTS ?= 0

run-agbot: agbot-docker-prereqs test-network
	touch /tmp/horizon
	touch /tmp/css.crt
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
		--add-host "$(DOCKER_AGBOT_ADD_HOST)" \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(UDS):$(UDS):rw \
		-v /var/tmp/horizon/:/var/tmp/horizon/:rw \
		-v ~/.docker:/home/agbotuser/.docker \
		-v ~/.docker:/root/.docker \
		-v /etc/wiotp-edge/:/etc/wiotp-edge/ \
		-v /var/wiotp-edge/persist/:/var/wiotp-edge/persist/ \
		-v /tmp/ess-auth/:/tmp/ess-auth/:rw \
		-v /tmp/hzndev/:/tmp/hzndev/ \
		-v $(AGBOT_TEMPFS)/certs:/certs \
		-v /var/log/:/var/log/ \
		-v $(ANAX_SOURCE)/anax-in-container:/tmp/anax-in-container \
		-v /tmp/horizon:/tmp/horizon:rw \
		-v /tmp/css.crt:/tmp/css.crt:rw \
		-e "HZN_DISABLE_IPTABLES_MANIPULATION=true" \
		-e "EXCH_APP_HOST=$(DOCKER_EXCH)" \
		-e "HZN_SSL_SKIP_VERIFY=1" \
		-e "CSS_URL=$(CSS_URL)" \
		-e "API_KEY=$(API_KEY)" \
		-e "CERT_LOC=$(CERT_LOC)" \
		-e "MUL_AGENTS=$(MUL_AGENTS)" \
		-e "EXCH_ROOTPW=$(EXCH_ROOTPW)" \
		-e "AGBOT_NAME=$(AGBOT_NAME)" \
		-e "AGBOT_SAPI_URL=$(AGBOT_SAPI_URL)" \
		-e "ORG_ID=$(ORG_ID)" \
		-e "ICP_HOST_IP=$(ICP_HOST_IP)" \
		-e "DOCKER_REG_CNAME=$(DOCKER_REG_CNAME)" \
		-e "DOCKER_REG_USER=$(DOCKER_REG_USER)" \
		-e "DOCKER_REG_PW=$(DOCKER_REG_PW)" \
		-e "DOCKER_EXCH_TAG=$(DOCKER_EXCH_TAG)" \
		-e "HZN_AGBOT_API=http://localhost:8082" \
		-e "HZN_EXCHANGE_URL=$(DOCKER_EXCH)" \
		-e "HZN_DEV_HOST_IP=$(E2EDEV_HOST_IP)" \
		-e "HZN_DEV_FSS_IMAGE_TAG"="testing$(BRANCH_NAME)" \
		-e "HZN_MGMT_HUB_CERT_PATH"="/certs/css.crt" \
		-e "DOCKER_CPU_INAME=$(DOCKER_CPU_INAME)" \
		-e "DOCKER_CPU_TAG=$(DOCKER_CPU_TAG)" \
		-e "ARCH=$(arch)" \
		-e "HZN_VAULT"="$(HZN_VAULT)" \
		-e "HZN_VAULT_ADDR"="$(VAULT_ADDR)" \
		-e "VAULT_TOKEN"="$(VAULT_DEV_ROOT_TOKEN_ID)" \
		-e "VAULT_ADDR"="$(VAULT_ADDR)" \
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
		-t $(DOCKER_REG_INAME)@$(DOCKER_REG_TAG)
	@echo "Docker $(DOCKER_REG_CNAME) container running with TLS"
	docker run --name $(DOCKER_REG_PW_CNAME) \
		--entrypoint htpasswd \
		-t $(DOCKER_REG_INAME)@$(DOCKER_REG_TAG) \
		-Bbn $(DOCKER_REG_USER) $(DOCKER_REG_PW) > $(AGBOT_TEMPFS)/auth/htpasswd
	docker rm -v $(DOCKER_REG_PW_CNAME)

run-exchange: run-exchange-db test-network $(EXCH_TEMPFS)/exchange-api.tmpl
	@echo "Handling $(DOCKER_EXCH_CNAME)"
	docker/docker_run.bash "$(DOCKER_EXCH_CNAME)" \
		docker run -d \
		-p 0.0.0.0:8090:8080 \
		--name "$(DOCKER_EXCH_CNAME)" \
		--network "$(DOCKER_TEST_NETWORK)" \
		-v $(EXCH_TEMPFS)/exchange-api.tmpl:/etc/horizon/exchange/exchange-api.tmpl:ro \
		-t $(DOCKER_EXCH_INAME):$(DOCKER_EXCH_TAG)

$(EXCH_TEMPFS)/exchange-api.tmpl:
	@echo "Populating the exchange configuration"
	mkdir -p $(EXCH_TEMPFS)
	echo "{ \"api\": { \"db\": { \"jdbcUrl\": \"jdbc:postgresql://$(DOCKER_EXCHDB_CNAME):$(EXCHDB_PORT)/$(EXCHDB_NAME)\"," \
                "\"user\": \"$(EXCHDB_USER)\" }, \"root\": { \"password\": \"$(EXCH_ROOTPW)\" } } }" > $(EXCH_TEMPFS)/exchange-api.tmpl

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

run-vault: test-network 
	if [[ "$(HZN_VAULT)" == "true" ]]; then \
		echo "Handling $(DOCKER_HASHICORP_VAULT_CNAME)"; \
		docker/docker_run.bash "$(DOCKER_HASHICORP_VAULT_CNAME)" \
			docker run -d \
			--name "$(DOCKER_HASHICORP_VAULT_CNAME)" \
			--network "$(DOCKER_TEST_NETWORK)" \
			-p 127.0.0.1:8200:8200 \
			-e VAULT_DEV_LISTEN_ADDRESS=$(VAULT_DEV_LISTEN_ADDRESS) \
			-e VAULT_DEV_ROOT_TOKEN_ID=$(VAULT_DEV_ROOT_TOKEN_ID) \
			-e VAULT_LOG_LEVEL=$(shell jq ".VAULT_LOG_LEVEL" $(VAULT_CONFIG)) \
			-t $(DOCKER_HASHICORP_VAULT_INAME):$(DOCKER_HASHICORP_VAULT_TAG); \
	else \
		echo "Skipping vault container $(DOCKER_HASHICORP_VAULT_CNAME)"; \
	fi

clean:
	@echo -e "\nStarting cleanup"
	@echo "Clean up kube environment"
	ARCH=$(arch) gov/stop_kube.sh
	@echo "Shutdown anax cleanly"
	-@curl -sSLX DELETE http://localhost:8510/node 2>/dev/null || true
	@echo "Removing agbot container"
	-@docker rm -f "$(DOCKER_AGBOT_CNAME)" 2>/dev/null || true
	@echo "Removing agent containers"
	-@HC_BASE=$(ANAX_SOURCE)/anax-in-container $(ANAX_SOURCE)/test/gov/stop_multiple_agents.sh
	@echo "Dropping and re-initializing the exchange DB"
	@$(eval DROP_DB_TOKEN := $(shell curl -sLX GET -u "root/root:$(EXCH_ROOTPW)" "http://localhost:8080/v1/admin/dropdb/token" | jq -r '.token')) 2>/dev/null || true
	-@curl -sLX POST -u "root/root:$(DROP_DB_TOKEN)" "http://localhost:8080/v1/admin/dropdb" | jq -r '.msg' 2>/dev/null || true
	-@curl -sLX POST -u "root/root:$(EXCH_ROOTPW)" "http://localhost:8080/v1/admin/initdb" | jq -r '.msg' 2>/dev/null || true
	@echo "Removing stale networks"
	-@docker network prune -f 2>/dev/null || true
	@echo "Removing unused images"
	-@docker rmi $$(docker images -qf "dangling=true") 2>/dev/null || true
	@echo "Removing any leftover images generated by e2edev"
	-@docker rmi $$(docker images | egrep "openhorizon/|wiotp-|localhost" | egrep -v "e2edev-agbot|$(arch)_exchange-api|$(arch)_cloud-sync-service|$(arch)_edge-sync-service|$(arch)_anax_k8s|REPOSITORY" | awk '{print $$3}') 2>/dev/null || true
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
	@echo "Removing the $(DOCKER_HASHICORP_VAULT_CNAME) container"
	-@docker rm -f "$(DOCKER_HASHICORP_VAULT_CNAME)" 2>/dev/null || true
	@echo "Cleaning up any left over workloads"
	-@docker stop $(docker ps -a | egrep "openhorizon/|wiotp-|localhost|hashicorp/" | egrep -v "e2edev-agbot|amd64_exchange-api|amd64_cloud-sync-service|amd64_edge-sync-service|vault|REPOSITORY" | awk '{print $$1}') 2>/dev/null || true
	-@docker rm $(docker ps -a | egrep "openhorizon/|wiotp-|localhost|hashicorp/" | egrep -v "e2edev-agbot|amd64_exchange-api|amd64_cloud-sync-service|amd64_edge-sync-service|vault|REPOSITORY" | awk '{print $$1}') 2>/dev/null || true
	@echo "Removing working directories"
	-@rm -rf $(CURDIR)/docker/tempfs 2>/dev/null || true
	-@sudo rm -fr $(UDS) 2>/dev/null || true

mostlyclean: clean
	@echo "Cleaning anax binaries and configs"
	-cd $(ANAX_SOURCE) && make clean

realclean: mostlyclean
	-@docker rm -f "$(DOCKER_EXCHDB_CNAME)" "$(DOCKER_EXCH_CNAME)" 2>/dev/null || true
	-@docker rmi $(DOCKER_AGBOT_INAME):$(DOCKER_DEV_TAG) $(DOCKER_AGBOT_INAME):latest 2>/dev/null || true
	-@docker rmi $(DOCKER_EXCH_INAME):$(DOCKER_EXCH_TAG) 2>/dev/null || true
	-@docker rmi $(DOCKER_CSS_INAME):$(DOCKER_CSS_TAG) 2>/dev/null || true
	-@docker rmi $(DOCKER_HASHICORP_VAULT_INAME):$(DOCKER_HASHICORP_VAULT_TAG) 2>/dev/null || true
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

get-anax:

ifeq ($(MAKECMDGOALS),test-remote-prebuilt)
get-anax: agbot-prereq-download-anax
else
get-anax: agbot-prereq-build-anax
endif

agbot-prereq-build-anax: $(ANAX_SOURCE)/anax
	@echo "Configuring agbot prerequisites"
	if [[ "$(shell docker images -q $(DOCKER_ANAX_K8S_INAME) 2>/dev/null)" == "" ]]; then \
		cd $(ANAX_SOURCE) && make anax-k8s-image; \
	fi
	if [[ "$(shell docker images -q $(DOCKER_ANAX_INAME) 2>/dev/null)" == "" ]]; then \
		cd $(ANAX_SOURCE) && make anax-image; \
	fi

agbot-prereq-download-anax:
	gov/setup_agbot.sh

agbot-docker-prereqs: get-anax
	mkdir -p /tmp/ess-store/ /tmp/ess-auth/ /tmp/service_storage/ /tmp/hzndev/
	mkdir -p $(AGBOT_TEMPFS)/usr/local/bin $(AGBOT_TEMPFS)/root/.colonus
	cp $(ANAX_SOURCE)/anax $(ANAX_SOURCE)/cli/hzn $(AGBOT_TEMPFS)/usr/local/bin
	cp -r $(CURDIR)/docker/fs/etc $(AGBOT_TEMPFS)
	rm -r $(AGBOT_TEMPFS)/etc/vault
	cp -r $(CURDIR)/docker/fs/hzn $(CURDIR)/gov/* $(CURDIR)/docker/fs/helm $(CURDIR)/docker/fs/resources $(CURDIR)/docker/fs/objects $(AGBOT_TEMPFS)/root
	for f in $$(find $(AGBOT_TEMPFS)/etc/agbot/ -maxdepth 1 -name '*.tmpl'); do EXCH_APP_HOST=$(DOCKER_EXCH) CSS_URL=$(CSS_URL) VAULT_ADDR=${VAULT_ADDR} envsubst < $$f > "$$(echo $$f | sed 's/.tmpl//')"; done
	for f in $$(find $(AGBOT_TEMPFS)/etc/colonus/ -maxdepth 1 -name '*.tmpl'); do EXCH_APP_HOST=$(DOCKER_EXCH) CSS_URL=$(CSS_URL) VAULT_ADDR=${VAULT_ADDR} envsubst < $$f > "$$(echo $$f | sed 's/.tmpl//')"; done
	cp $(ANAX_SOURCE)/pkg/deb/horizon/etc/horizon/trust/horizon.pem $(AGBOT_TEMPFS)/root/.colonus/.
	cp $(ANAX_SOURCE)/pkg/deb/horizon/etc/horizon/trust/mtn-publicKey.pem $(AGBOT_TEMPFS)/root/.colonus/.

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

hashicorp-vault-image:
	if [[ "$(HZN_VAULT)" == "true" ]]; then \
		echo "\nPulling hashicorp vault image $(DOCKER_HASHICORP_VAULT_INAME)"; \
		docker pull $(DOCKER_HASHICORP_VAULT_INAME):$(DOCKER_HASHICORP_VAULT_TAG); \
	else \
		echo "Skipping vault image $(DOCKER_HASHICORP_VAULT_INAME)"; \
	fi

test: clean $(ANAX_SOURCE)/anax run-dockerreg run-exchange run-css run-vault run-agbot
	@echo -e  "\nBootstrapping the exchange"
	docker exec $(DOCKER_AGBOT_CNAME) bash -c "export $(TEST_VARS); /root/init_exchange.sh"
	@echo -e  "\nSetting up agent in kube"
	$(TEST_VARS) ARCH=$(arch) gov/run_kube.sh $(AGBOT_TEMPFS) $(ANAX_SOURCE) $(EXCH_ROOTPW)
	@echo -e  "\nStarting tests"
	docker exec $(DOCKER_AGBOT_CNAME) bash -c "export $(TEST_VARS); export $(MSGHUB_VARS); /root/gov-combined.sh"

test-remote: clean $(ANAX_SOURCE)/anax run-dockerreg run-agbot copy-cert
	@echo -e  "\nBootstrapping the exchange"
	docker exec $(DOCKER_AGBOT_CNAME) bash -c "export $(TEST_VARS); /root/init_exchange.sh"
	@echo -e  "\nStarting tests"
	docker exec $(DOCKER_AGBOT_CNAME) bash -c "export $(TEST_VARS); export $(MSGHUB_VARS); /root/gov-combined.sh"

test-remote-prebuilt: clean run-dockerreg run-agbot copy-cert
	@echo -e  "\nBootstrapping the exchange"
	docker exec $(DOCKER_AGBOT_CNAME) bash -c "export $(TEST_VARS); /root/init_exchange.sh"
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

.PHONY: all default stop build run distclean clean mostlyclean realclean run-agbot run-dockerreg run-exchange run-exchange-db test-network test agbot-docker-prereqs agbot-image exchange-db-image exchange-image css-mongo-db-image run-mongo-db run-css copy-cert run-vault hashicorp-vault-image
#.SILENT: clean
