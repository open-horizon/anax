# Makefile for e2e execution
SHELL := /bin/bash

# Get Arch for tag and hardware (Golang style) to run test
arch_tag ?= $(shell ../tools/arch-tag)
arch ?= $(arch_tag)
ifeq ($(arch),ppc64el)
	arch := ppc64le
endif

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

export PREBUILT_DOCKER_REG_URL ?= ""
export PREBUILT_DOCKER_REG_USER ?= ""
export PREBUILT_DOCKER_REG_PW ?= ""
export PREBUILT_ANAX_VERSION ?= nightly
export PREBUILT_ESS_VERSION ?= nightly

BRANCH_NAME ?= ""
ANAX_SOURCE ?= $(shell dirname $(CURDIR))
DOCKER_DEV_OPTS :=  --rm --no-cache --build-arg ARCH=$(arch)
E2EDEVTEST_TEMPFS := $(CURDIR)/docker/tempfs/e2edevtest
CERT_LOC ?= 1

# anax
DOCKER_ANAX_INAME := openhorizon/$(arch)_anax:testing
DOCKER_ANAX_K8S_INAME := openhorizon/$(arch)_anax_k8s:testing

# e2edevtest
DOCKER_TEST_CNAME := e2edevtest
DOCKER_TEST_INAME := openhorizon/e2edev-test
DOCKER_TEST_TAG := latest
DOCKER_TEST_NETWORK := hzn_horizonnet
DOCKER_TEST_ADD_HOST ?= "localhost:127.0.0.1"
E2EDEV_HOST_IP := $(shell hostname -I | awk '{print $$1}')
UDS := /var/run/horizon/
API_KEY ?= 0
DOCKER_CPU_TAG := 1.2.2
ICP_HOST_IP ?= 0
ORG_ID ?= ""
ifeq ($(arch),ppc64le)
	DOCKER_CPU_INAME ?= openhorizon/example_ms_$(arch)_cpu
else
	DOCKER_CPU_INAME ?= openhorizon/example_ms_x86_cpu
endif

# agbot
DOCKER_AGBOT_CNAME := agbot
DOCKER_AGBOT2_CNAME := agbot2
AGBOT_IMAGE_NAME ?= openhorizon/$(arch)_agbot
AGBOT_IMAGE_TAG ?= e2edev
old_agbot := $(shell echo -e $(TEST_VARS) | grep OLDAGBOT=1)
ifneq ($(old_agbot),)
	AGBOT_IMAGE_TAG = latest
endif
AGBOT_API ?= http://agbot:8080
AGBOT2_API ?= http://agbot2:8080
AGBOT_SAPI_URL ?= https://agbot:8083
AGBOT_NAME ?= agbot
AGBOT_TOKEN ?= Abcdefghijklmno1

# exchange
DOCKER_EXCH_CNAME = exchange-api
DOCKER_EXCH_INAME := openhorizon/$(arch)_exchange-api
DOCKER_EXCH_TAG := testing
DOCKER_EXCH ?= http://exchange-api:8081/v1
EXCH_ROOTPW ?= Horizon-Rul3s
EXCHANGE_HUB_ADMIN_PW ?= hubadminpw
EXCHANGE_SYSTEM_ADMIN_PW ?= ibmadminpw

# exchange DB
DOCKER_EXCHDB_CNAME := postgres
DOCKER_EXCHDB_INAME ?= postgres
ifeq ($(arch),ppc64le)
	DOCKER_EXCHDB_TAG ?= 13.2
else
	DOCKER_EXCHDB_TAG ?= 9
endif
EXCHDB_USER := admin
EXCHDB_PORT := 5432
EXCHDB_NAME := exchange

# registry
DOCKER_REG_CNAME := e2edevregistry
ifeq ($(arch),ppc64le)
	DOCKER_REG_INAME ?= ibmcom/registry-ppc64le
	DOCKER_REG_TAG ?= sha256:b95d9a4abc64bef338b71836ac6876d59281f34f0f03be1023673d52ba1f40e0
else
	DOCKER_REG_INAME ?= registry
	DOCKER_REG_TAG ?= sha256:7d081088e4bfd632a88e3f3bcd9e007ef44a796fddfe3261407a3f9f04abe1e7
endif
DOCKER_REG_PW_CNAME := htregistry
DOCKER_REG_USER := testuser
DOCKER_REG_PW := testpassword

# css
DOCKER_CSS_CNAME := css-api
DOCKER_CSS_INAME := openhorizon/$(arch)_cloud-sync-service
DOCKER_CSS_TAG := e2edev
CSS_URL ?= https://css-api:8080

#css DB
DOCKER_CSSDB_CNAME := mongo
ifeq ($(arch),ppc64le)
	DOCKER_CSSDB_INAME ?= ppc64le/mongodb
	DOCKER_CSSDB_TAG ?= 2.6.10
else
	DOCKER_CSSDB_INAME ?= mongo
	DOCKER_CSSDB_TAG ?= 4.0.6
endif

#vault
DOCKER_VAULT_CNAME := vault
DOCKER_VAULT_INAME := openhorizon/$(arch)_vault
DOCKER_VAULT_TAG := testing
no_vault := $(shell echo -e $(TEST_VARS) | grep NOVAULT=1)
ifeq ($(no_vault),)
	VAULT_ADDR = http://vault:8200
endif
VAULT_CONFIG := $(CURDIR)/docker/fs/etc/vault/config.json
VAULT_DEV_ROOT_TOKEN_ID := $(shell jq ".VAULT_DEV_ROOT_TOKEN_ID" $(VAULT_CONFIG))
VAULT_DEV_LISTEN_ADDRESS := $(shell jq ".VAULT_DEV_LISTEN_ADDRESS" $(VAULT_CONFIG))

#sdo
DOCKER_SDO_CNAME := sdo-owner-services
HZN_SDO_SVC_URL ?= https://sdo-owner-services:9008/api

ifndef verbose
.SILENT:
endif

all: default
default: build
stop: clean
build: test-image vault-image
build-remote: test-image
run: test
distclean: realclean

run-test: test-image e2edevtest-docker-prereqs test-network
	mkdir -p /tmp/e2edevtest
	@echo "Handling $(DOCKER_TEST_CNAME)"
	docker/docker_run.bash "$(DOCKER_TEST_CNAME)" \
		docker run -d \
		--privileged \
		-p 127.0.0.1:8510:8510 \
		-p 127.0.0.1:8511:8511 \
		-p 127.0.0.1:8082:8082 \
		-p 127.0.0.1:8083:8083 \
		-p 127.0.0.1:8084:8084 \
		-p 127.0.0.1:8085:8085 \
		--name "$(DOCKER_TEST_CNAME)" \
		--network "$(DOCKER_TEST_NETWORK)" \
		--add-host "$(DOCKER_TEST_ADD_HOST)" \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(UDS):$(UDS):rw \
		-v /var/tmp/horizon/:/var/tmp/horizon/:rw \
		-v ~/.docker:/home/agbotuser/.docker \
		-v ~/.docker:/root/.docker \
		-v /etc/wiotp-edge/:/etc/wiotp-edge/ \
		-v /var/wiotp-edge/persist/:/var/wiotp-edge/persist/ \
		-v /tmp/ess-auth/:/tmp/ess-auth/:rw \
		-v /tmp/hzndev/:/tmp/hzndev/ \
		-v $(E2EDEVTEST_TEMPFS)/certs:/certs \
		-v /var/log/:/var/log/ \
		-v $(ANAX_SOURCE)/anax-in-container:/tmp/anax-in-container \
		-v /tmp/e2edevtest:/tmp/e2edevtest:rw \
		-e "DOCKER_TEST_NETWORK=$(DOCKER_TEST_NETWORK)" \
		-e "EXCH_APP_HOST=$(DOCKER_EXCH)" \
		-e "HZN_SSL_SKIP_VERIFY=1" \
		-e "CSS_URL=$(CSS_URL)" \
		-e "API_KEY=$(API_KEY)" \
		-e "CERT_LOC=$(CERT_LOC)" \
		-e "EXCH_ROOTPW=$(EXCH_ROOTPW)" \
		-e "EXCHANGE_HUB_ADMIN_PW=$(EXCHANGE_HUB_ADMIN_PW)" \
		-e "EXCHANGE_SYSTEM_ADMIN_PW=$(EXCHANGE_SYSTEM_ADMIN_PW)" \
		-e "AGBOT_API=$(AGBOT_API)" \
		-e "AGBOT_NAME=$(AGBOT_NAME)" \
		-e "AGBOT_SAPI_URL=$(AGBOT_SAPI_URL)" \
		-e "AGBOT2_API=$(AGBOT2_API)" \
		-e "ORG_ID=$(ORG_ID)" \
		-e "ICP_HOST_IP=$(ICP_HOST_IP)" \
		-e "DOCKER_REG_USER=$(DOCKER_REG_USER)" \
		-e "DOCKER_REG_PW=$(DOCKER_REG_PW)" \
		-e "HZN_EXCHANGE_URL=$(DOCKER_EXCH)" \
		-e "HZN_DEV_HOST_IP=$(E2EDEV_HOST_IP)" \
		-e "HZN_DEV_FSS_IMAGE_TAG"="testing$(BRANCH_NAME)" \
		-e "DOCKER_CPU_INAME=$(DOCKER_CPU_INAME)" \
		-e "DOCKER_CPU_TAG=$(DOCKER_CPU_TAG)" \
		-e "ARCH=$(arch)" \
		-e "HZN_VAULT"="$(HZN_VAULT)" \
		-e "HZN_VAULT_ADDR"="$(VAULT_ADDR)" \
		-e "VAULT_TOKEN"="$(VAULT_DEV_ROOT_TOKEN_ID)" \
		-e "VAULT_ADDR"="$(VAULT_ADDR)" \
		-e "HZN_SDO_SVC_URL"="$(HZN_SDO_SVC_URL)" \
		-e "HZN_AGBOT_URL"="$(AGBOT_SAPI_URL)" \
		-t $(DOCKER_TEST_INAME):$(DOCKER_TEST_TAG)
	docker cp $(E2EDEVTEST_TEMPFS)/. $(DOCKER_TEST_CNAME):/

run-dockerreg: test-network
	@echo "Handling Docker $(DOCKER_REG_CNAME)"
	mkdir -p $(E2EDEVTEST_TEMPFS)/certs
	@echo "Generating TLS cert for docker registry $(DOCKER_REG_CNAME)"
	openssl req \
		-newkey rsa:4096 \
		-nodes -sha256 -x509 \
		-keyout $(E2EDEVTEST_TEMPFS)/certs/domain.key \
		-days 365 \
		-out $(E2EDEVTEST_TEMPFS)/certs/domain.crt \
		-subj "/C=US/ST=e2edev@somecomp.com/L=e2edev@somecomp.com/O=e2edev@somecomp.com/CN=localhost"
	mkdir -p $(E2EDEVTEST_TEMPFS)/auth
	touch $(E2EDEVTEST_TEMPFS)/auth/htpasswd
	docker/docker_run.bash "$(DOCKER_REG_CNAME)" \
		docker run -d \
		-p 127.0.0.1:443:443 \
		--name $(DOCKER_REG_CNAME) \
		--network "$(DOCKER_TEST_NETWORK)" \
		-v $(E2EDEVTEST_TEMPFS)/certs:/certs \
		-v $(E2EDEVTEST_TEMPFS)/auth:/auth \
		-v ~/.docker:/root/.docker \
		-e REGISTRY_HTTP_ADDR=0.0.0.0:443 \
		-e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt \
		-e REGISTRY_HTTP_TLS_KEY=/certs/domain.key \
		-e REGISTRY_AUTH=htpasswd \
		-e REGISTRY_AUTH_HTPASSWD_REALM=RegistryRealm \
		-e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
		-t $(DOCKER_REG_INAME)@$(DOCKER_REG_TAG)
	@echo "Docker $(DOCKER_REG_CNAME) container running with TLS"
	docker run --name $(DOCKER_REG_PW_CNAME) \
		--entrypoint htpasswd \
		-t $(DOCKER_REG_INAME)@$(DOCKER_REG_TAG) \
		-Bbn $(DOCKER_REG_USER) $(DOCKER_REG_PW) > $(E2EDEVTEST_TEMPFS)/auth/htpasswd
	docker rm -v $(DOCKER_REG_PW_CNAME)

run-vault: test-network
	if [[ "$(NOVAULT)" != "1" ]]; then \
		# Start the vault container \
		echo "Handling $(DOCKER_VAULT_CNAME)"; \
		docker/docker_run.bash "$(DOCKER_VAULT_CNAME)" \
			docker run -d \
			--name "$(DOCKER_VAULT_CNAME)" \
			--network "$(DOCKER_TEST_NETWORK)" \
			-p 127.0.0.1:8200:8200 \
			-e VAULT_DEV_LISTEN_ADDRESS=$(VAULT_DEV_LISTEN_ADDRESS) \
			-e VAULT_DEV_ROOT_TOKEN_ID=$(VAULT_DEV_ROOT_TOKEN_ID) \
			-e VAULT_LOG_LEVEL=$(shell jq ".VAULT_LOG_LEVEL" $(VAULT_CONFIG)) \
			-v $(E2EDEVTEST_TEMPFS)/certs/:/openhorizon/certs/ \
			-t $(DOCKER_VAULT_INAME):$(DOCKER_VAULT_TAG); \
	else \
		echo "Skipping vault container $(DOCKER_VAULT_CNAME)"; \
	fi

clean:
	@echo -e "\nStarting cleanup"
	@echo "Clean up kube environment"
	ARCH=$(arch) gov/stop_kube.sh
	@echo "Shutdown anax cleanly"
	-@curl -sSLX DELETE http://localhost:8510/node 2>/dev/null || true
	@echo "Removing agent containers"
	-@HC_BASE=$(ANAX_SOURCE)/anax-in-container $(ANAX_SOURCE)/test/gov/stop_multiple_agents.sh
	@echo "Removing e2edevtest container"
	-@docker rm -f "$(DOCKER_TEST_CNAME)" 2>/dev/null || true
	@echo "Removing the $(DOCKER_REG_CNAME) container"
	-@docker rm -v -f "$(DOCKER_REG_CNAME)" 2>/dev/null || true
	@echo "Removing the all-in-one management hub"
	ANAX_SOURCE=$(ANAX_SOURCE) $(ANAX_SOURCE)/test/gov/stop_mgmt_hub.sh
	@echo "Removing unused images"
	-@docker rmi $$(docker images -qf "dangling=true") 2>/dev/null || true
	@echo "Removing any leftover images generated by e2edev"
	-@docker rmi $$(docker images | egrep "openhorizon/|wiotp-|localhost" | egrep -v "e2edev-test|$(arch)_agbot|$(arch)_exchange-api|$(arch)_cloud-sync-service|$(arch)_edge-sync-service|$(arch)_anax_k8s|$(arch)_vault|REPOSITORY" | awk '{print $$3}') 2>/dev/null || true
	@echo "Removing the $(DOCKER_VAULT_CNAME) container"
	-@docker rm -f "$(DOCKER_VAULT_CNAME)" 2>/dev/null || true
	@echo "Cleaning up any left over workloads"
	-@docker stop $(docker ps -a | egrep "openhorizon/|wiotp-|localhost|hashicorp/" | egrep -v "e2edev-test|amd64_exchange-api|amd64_cloud-sync-service|amd64_edge-sync-service|amd64_vault|REPOSITORY" | awk '{print $$1}') 2>/dev/null || true
	-@docker rm $(docker ps -a | egrep "openhorizon/|wiotp-|localhost|hashicorp/" | egrep -v "e2edev-test|amd64_exchange-api|amd64_cloud-sync-service|amd64_edge-sync-service|amd64_vault|REPOSITORY" | awk '{print $$1}') 2>/dev/null || true
	@echo "Removing working directories"
	-@rm -rf $(CURDIR)/docker/tempfs 2>/dev/null || true
	-@sudo rm -fr $(UDS) 2>/dev/null || true
	@echo "Removing stale networks"
	-@docker network prune -f 2>/dev/null || true

mostlyclean: clean
	@echo "Cleaning anax binaries and configs"
	-cd $(ANAX_SOURCE) && make clean

realclean: mostlyclean
	-@docker rm -f "$(DOCKER_EXCHDB_CNAME)" "$(DOCKER_EXCH_CNAME)" 2>/dev/null || true
	-@docker rmi $(AGBOT_IMAGE_NAME):$(AGBOT_IMAGE_TAG) 2>/dev/null || true
	-@docker rmi $(DOCKER_TEST_INAME):$(DOCKER_TEST_TAG) $(DOCKER_TEST_INAME):latest 2>/dev/null || true
	-@docker rmi $(DOCKER_EXCH_INAME):$(DOCKER_EXCH_TAG) 2>/dev/null || true
	-@docker rmi $(DOCKER_CSS_INAME):$(DOCKER_CSS_TAG) 2>/dev/null || true
	-@docker rmi $(DOCKER_VAULT_INAME):$(DOCKER_VAULT_TAG) 2>/dev/null || true
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

e2edevtest-docker-prereqs: 
	mkdir -p /tmp/ess-store/ /tmp/ess-auth/ /tmp/service_storage/ /tmp/hzndev/
	mkdir -p $(E2EDEVTEST_TEMPFS)/usr/local/bin $(E2EDEVTEST_TEMPFS)/root/.colonus
	cp $(ANAX_SOURCE)/anax $(ANAX_SOURCE)/cli/hzn $(E2EDEVTEST_TEMPFS)/usr/local/bin
	cp -r $(CURDIR)/docker/fs/etc $(E2EDEVTEST_TEMPFS)
	rm -r $(E2EDEVTEST_TEMPFS)/etc/vault
	cp -r $(CURDIR)/docker/fs/hzn $(CURDIR)/gov/* $(CURDIR)/docker/fs/helm $(CURDIR)/docker/fs/resources $(CURDIR)/docker/fs/objects $(E2EDEVTEST_TEMPFS)/root
	for f in $$(find $(E2EDEVTEST_TEMPFS)/etc/agbot/ -maxdepth 1 -name '*.tmpl'); do EXCH_APP_HOST=$(DOCKER_EXCH) CSS_URL=$(CSS_URL) VAULT_ADDR=${VAULT_ADDR} envsubst < $$f > "$$(echo $$f | sed 's/.tmpl//')"; done
	for f in $$(find $(E2EDEVTEST_TEMPFS)/etc/colonus/ -maxdepth 1 -name '*.tmpl'); do EXCH_APP_HOST=$(DOCKER_EXCH) CSS_URL=$(CSS_URL) AGBOT_SAPI_URL=$(AGBOT_SAPI_URL) VAULT_ADDR=${VAULT_ADDR} envsubst < $$f > "$$(echo $$f | sed 's/.tmpl//')"; done
	cp $(ANAX_SOURCE)/pkg/deb/horizon/etc/horizon/trust/horizon.pem $(E2EDEVTEST_TEMPFS)/root/.colonus/.
	cp $(ANAX_SOURCE)/pkg/deb/horizon/etc/horizon/trust/mtn-publicKey.pem $(E2EDEVTEST_TEMPFS)/root/.colonus/.

test-image:
	@echo "Handling $(DOCKER_TEST_INAME)"
	if [ -n "$(shell docker images | grep '$(DOCKER_TEST_INAME)')" ]; then \
		echo "Skipping since $(DOCKER_TEST_INAME) image exists, run 'make clean && make' if a rebuild is desired"; \
	else \
		echo "Building container image $(DOCKER_TEST_INAME)"; \
		docker build $(DOCKER_DEV_OPTS) -t $(DOCKER_TEST_INAME) -f docker/Dockerfile ./docker && docker tag $(DOCKER_TEST_INAME) $(DOCKER_TEST_INAME):$(DOCKER_TEST_TAG); \
	fi

vault-image:
	if [[ "$(NOVAULT)" != "1" ]]; then \
		echo "\nPulling vault image $(DOCKER_VAULT_INAME)"; \
		docker pull $(DOCKER_VAULT_INAME):$(DOCKER_VAULT_TAG); \
	else \
		echo "Skipping vault image $(DOCKER_VAULT_INAME)"; \
	fi

test: clean $(ANAX_SOURCE)/anax run-dockerreg run-mgmthub run-vault run-test
	@echo -e  "\nTesting and bootstrapping the vault"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) DOCKER_VAULT_CNAME=$(DOCKER_VAULT_CNAME); /root/init_vault.sh"
	@echo -e  "\nBootstrapping the exchange"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) EXCHANGE_HUB_ADMIN_PW=$(EXCHANGE_HUB_ADMIN_PW) EXCHANGE_SYSTEM_ADMIN_PW=$(EXCHANGE_SYSTEM_ADMIN_PW); /root/init_exchange.sh"
	@echo -e  "\nSetting up secrets in the vault"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) DOCKER_VAULT_CNAME=$(DOCKER_VAULT_CNAME); /root/setup_secrets.sh"
	@echo -e  "\nSetting up agent in kube"
	$(TEST_VARS) ARCH=$(arch) CERT_LOC=$(CERT_LOC) gov/run_kube.sh $(E2EDEVTEST_TEMPFS) $(ANAX_SOURCE) $(EXCH_ROOTPW) $(DOCKER_TEST_NETWORK)
	@echo -e  "\nStarting tests"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS); /root/gov-combined.sh"

test-remote: clean $(ANAX_SOURCE)/anax run-dockerreg run-test copy-cert
	@echo -e  "\nBootstrapping the exchange"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS); /root/init_exchange.sh"
	@echo -e  "\nSetting up secrets in the vault"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) DOCKER_VAULT_CNAME=$(DOCKER_VAULT_CNAME); /root/setup_secrets.sh"
	@echo -e  "\nStarting tests"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS); /root/gov-combined.sh"

test-remote-prebuilt: clean run-dockerreg download-agbot-image run-test copy-cert
	@echo -e  "\nBootstrapping the exchange"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS); /root/init_exchange.sh"
	@echo -e  "\nSetting up secrets in the vault"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) DOCKER_VAULT_CNAME=$(DOCKER_VAULT_CNAME); /root/setup_secrets.sh"
	@echo -e  "\nStarting tests"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS); /root/gov-combined.sh"

copy-cert:
	cp css.crt $(E2EDEVTEST_TEMPFS)/certs/css.crt
	cp agbotapi.crt $(E2EDEVTEST_TEMPFS)/certs/agbotapi.crt

download-agbot-image:
	@echo -e "\nLogging into Docker registry $(PREBUILT_DOCKER_REG_URL)"
	docker login -u=$(PREBUILT_DOCKER_REG_USER) -p=$(PREBUILT_DOCKER_REG_PW) $(PREBUILT_DOCKER_REG_URL)
	@echo -e  "\nPulling latest Agbot Image"
	docker pull $(PREBUILT_DOCKER_REG_URL)/amd64_agbot:$(PREBUILT_ANAX_VERSION) 
	docker pull $(PREBUILT_DOCKER_REG_URL)/amd64_edge-sync-service:$(PREBUILT_ESS_VERSION)
	docker tag $(PREBUILT_DOCKER_REG_URL)/amd64_edge-sync-service:$(PREBUILT_ESS_VERSION) openhorizon/amd64_edge-sync-service:testing
	@echo -e "\nCreating (but not starting/running) anax container..."
	docker create --name="agbot_temp" $(PREBUILT_DOCKER_REG_URL)/amd64_agbot:$(PREBUILT_ANAX_VERSION)
	@echo -e "\nCopying out anax and hzn binaries"
	docker cp agbot_temp:/usr/bin/hzn $(ANAX_SOURCE)/cli/.
	docker cp agbot_temp:/usr/horizon/bin/anax $(ANAX_SOURCE)/.
	@echo -e "\nRemoving agbot container"
	docker rm -f agbot_temp

get-anax-images:
	@echo "Configuring agbot prerequisites"
	if [[ "$(shell docker images -q $(DOCKER_ANAX_K8S_INAME) 2>/dev/null)" == "" ]]; then \
		cd $(ANAX_SOURCE) && make anax-k8s-image; \
	fi
	if [[ "$(shell docker images -q $(DOCKER_ANAX_INAME) 2>/dev/null)" == "" ]]; then \
		cd $(ANAX_SOURCE) && make anax-image; \
	fi

get-agbot-image:
	if [[ "$(AGBOT_IMAGE_TAG)" == "e2edev" ]]; then \
		if [[ "$(shell docker images -q $(AGBOT_IMAGE_NAME):$(AGBOT_IMAGE_TAG) 2>/dev/null)" == "" ]]; then \
	    	echo -e "Building agbot image $(AGBOT_IMAGE_NAME):$(AGBOT_IMAGE_TAG)"; \
			cd $(ANAX_SOURCE) && make agbot-image; \
			docker tag $(AGBOT_IMAGE_NAME):testing $(AGBOT_IMAGE_NAME):e2edev; \
	    else \
	    	echo "Update $(AGBOT_IMAGE_NAME):$(AGBOT_IMAGE_TAG) image with latest anax and hzn."; \
	    	docker run -it --name="agbot_temp" $(AGBOT_IMAGE_NAME):$(AGBOT_IMAGE_TAG) /bin/bash >/dev/null 2>&1  || true; \
	    	docker cp $(ANAX_SOURCE)/anax agbot_temp:/usr/horizon/bin/anax; \
	    	docker cp $(ANAX_SOURCE)/cli/hzn agbot_temp:/usr/bin/hzn; \
	    	docker rmi -f $(AGBOT_IMAGE_NAME):tmp >/dev/null 2>&1 || true; \
	    	docker commit agbot_temp $(AGBOT_IMAGE_NAME):tmp; \
	    	docker rm -f agbot_temp; \
	    	docker rmi -f $(AGBOT_IMAGE_NAME):$(AGBOT_IMAGE_TAG) /bin/bash >/dev/null 2>&1  || true; \
	    	docker tag $(AGBOT_IMAGE_NAME):tmp $(AGBOT_IMAGE_NAME):$(AGBOT_IMAGE_TAG); \
	    	docker rmi -f $(AGBOT_IMAGE_NAME):tmp /bin/bash >/dev/null 2>&1  || true; \
	    fi \
	fi

get-css-image:
	if [[ "$(DOCKER_CSS_TAG)" == "e2edev" ]]; then \
		if [[ "$(shell docker images -q $(DOCKER_CSS_INAME):$(DOCKER_CSS_TAG) 2>/dev/null)" == "" ]]; then \
	    	echo -e "Building css image $(DOCKER_CSS_INAME):$(DOCKER_CSS_TAG)"; \
			cd $(ANAX_SOURCE) && make css-docker-image; \
			docker tag $(DOCKER_CSS_INAME):testing $(DOCKER_CSS_INAME):e2edev; \
	    fi \
	fi


run-mgmthub: get-agbot-image get-css-image get-anax-images
	@echo "Starting all-in-one management hub"
	export EXCHANGE_ROOT_PW=$(EXCH_ROOTPW) EXCHANGE_IMAGE_TAG=$(DOCKER_EXCH_TAG) EXCHANGE_DATABASE=$(EXCHDB_NAME); \
	export EXCHANGE_HUB_ADMIN_PW=$(EXCHANGE_HUB_ADMIN_PW) EXCHANGE_SYSTEM_ADMIN_PW=$(EXCHANGE_SYSTEM_ADMIN_PW); \
	export AGBOT_ID=$(AGBOT_NAME) AGBOT_TOKEN=$(AGBOT_TOKEN) AGBOT_IMAGE_TAG=$(AGBOT_IMAGE_TAG) HZN_VAULT_URL=$(VAULT_ADDR); \
	export CSS_IMAGE_TAG=$(DOCKER_CSS_TAG); \
	export MONGO_IMAGE_TAG=$(DOCKER_CSSDB_TAG); \
	export POSTGRES_IMAGE_TAG=$(DOCKER_EXCHDB_TAG) POSTGRES_USER=$(EXCHDB_USER); \
	export HZN_TRANSPORT=https; \
	export ANAX_SOURCE=$(ANAX_SOURCE) ANAX_LOG_LEVEL=5; \
	$(TEST_VARS) $(ANAX_SOURCE)/test/gov/start_mgmt_hub.sh
	cp /etc/horizon/keys/horizonMgmtHub.crt $(E2EDEVTEST_TEMPFS)/certs/css.crt
	cp /etc/horizon/keys/horizonMgmtHub.crt $(E2EDEVTEST_TEMPFS)/certs/agbotapi.crt

.PHONY: all default stop build run distclean clean mostlyclean realclean run-test run-dockerreg test-network test e2edevtest-docker-prereqs test-image copy-cert run-vault vault-image
#.SILENT: clean
