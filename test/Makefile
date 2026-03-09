# Makefile for E2E Test Execution
#
# This Makefile orchestrates the Open Horizon E2E test environment, including:
# - Management hub components (Exchange, Agbot, CSS/ESS, databases)
# - Test infrastructure (Docker networks, registries, certificates)
# - Anax agent deployment (host mode and Kubernetes mode)
# - Test execution with various configurations
#
# Quick Start:
#   make e2e-test-noloop    # Fast test without loop tests (recommended for development)
#   make e2e-test-patterns  # Test with all service patterns
#   make e2e-test-full      # Complete test suite (both modes)
#   make e2e-quick          # Fastest test (no Kubernetes, no cleanup)
#
# Cleanup Levels:
#   make clean              # Light cleanup (stops services, keeps infrastructure)
#   make cleaner            # Medium cleanup (removes containers, keeps images)
#   make cleanest           # Full cleanup (removes everything)
#
# Environment Verification:
#   make e2e-verify         # Check environment readiness (images, containers, binaries)

SHELL := /bin/bash

# ============================================================================
# Architecture Configuration
# ============================================================================
# Detect target architecture for cross-platform testing
# Converts Debian-style arch names (ppc64el) to Go-style (ppc64le)
arch_tag := $(shell ../tools/arch-tag)
arch ?= $(arch_tag)
ifeq ($(arch),ppc64el)
	arch := ppc64le
endif

# ============================================================================
# Test Configuration Variables
# ============================================================================
# TEST_PATTERNS: Comma-separated list of service patterns to test
#   Service patterns: sall, sns, spws, susehello, sgps, sloc, cpu2msghub
#   Legacy patterns: all, ns, loc, gps, ns-keytest (workloads/microservices)
#   Empty string = policy-based deployment (no patterns)
#
# TEST_VARS: Space-separated test control variables
#   NOLOOP=1          - Skip loop/agreement verification tests (faster)
#   NOCANCEL=1        - Skip agreement cancellation tests
#   NOKUBE=1          - Skip Kubernetes cluster agent tests
#   NOVAULT=1         - Skip Vault/secrets manager tests
#   NOCOMPCHECK=1     - Skip compatibility check tests
#   NOUPGRADE=1       - Skip service upgrade/downgrade tests
#   NOSVC_CONFIGSTATE=1 - Skip service configuration state tests
#   NORETRY=1         - Skip service retry tests
#   NOHZNREG=1        - Skip hzn registration tests
#   OLDAGBOT=1        - Use latest agbot image instead of e2edev
#
# Special Requirements:
#   cpu2msghub pattern requires: MSGHUB_BROKER_URL, MSGHUB_API_KEY

TEST_VARS ?= NOLOOP=1 TEST_PATTERNS=sall
# Alternative: TEST_VARS ?= NOLOOP=1 NOCANCEL=1

# ============================================================================
# Prebuilt Image Configuration (for test-remote-prebuilt target)
# ============================================================================
# Used when testing with prebuilt images from a Docker registry
# instead of building locally
export PREBUILT_DOCKER_REG_URL ?=
export PREBUILT_DOCKER_REG_USER ?=
export PREBUILT_DOCKER_REG_PW ?=
export PREBUILT_ANAX_VERSION ?= nightly
export PREBUILT_ESS_VERSION ?= nightly

# ============================================================================
# Docker Image Tag Configuration Strategy
# ============================================================================
# All Docker images support three configuration scenarios:
#
# 1. DEFAULT TAGS (Production Defaults):
#    - Use the default tag values defined below
#    - Example: DOCKER_EXCH_TAG defaults to "testing"
#
# 2. USER-SET TAGS (Environment Variable Override):
#    - Override any image tag via environment variable
#    - Example: export DOCKER_EXCH_TAG=latest
#    - All image variables use ?= to allow override
#
# 3. CUSTOM LOCAL BUILD TAGS:
#    - For locally built images from this repository
#    - Anax images: openhorizon/$(arch)_anax:testing
#    - Agbot images: openhorizon/$(arch)_agbot:e2edev (or :testing)
#    - CSS/ESS images: openhorizon/$(arch)_cloud-sync-service:testing
#
# Management Hub Images (External):
#   - Exchange API: quay.io/openhorizon/$(arch)_exchange-api:testing
#   - MongoDB: mongo:4.0.6 (ppc64le: ppc64le/mongodb:2.6.10)
#   - PostgreSQL: postgres:17
#   - OpenBao (Vault): quay.io/openbao/openbao-ubi:2.0
#
# Anax Repository Images (Built Locally):
#   - Anax Agent: openhorizon/$(arch)_anax:testing
#   - Anax K8s: openhorizon/$(arch)_anax_k8s:testing
#   - Agbot: openhorizon/$(arch)_agbot:e2edev (testing build)
#   - CSS: openhorizon/$(arch)_cloud-sync-service:latest
#   - ESS: openhorizon/$(arch)_edge-sync-service:testing

# ============================================================================
# Build Configuration
# ============================================================================
BRANCH_NAME ?=
ANAX_SOURCE ?= $(shell dirname $(CURDIR))
DOCKER_DEV_OPTS :=  --rm --no-cache --build-arg ARCH=$(arch)
E2EDEVTEST_TEMPFS := $(CURDIR)/docker/tempfs/e2edevtest

# Certificate configuration
# CERT_LOC=0: HTTP mode (no certificates, faster for testing)
# CERT_LOC=1: HTTPS mode (with certificates, production-like)
export CERT_LOC ?= 0

# Transport protocol (currently HTTP only, HTTPS support commented out)
export HZN_TRANSPORT=http

# ============================================================================
# Anax Agent Image Configuration
# ============================================================================
# Anax images for different deployment modes
DOCKER_ANAX_INAME ?= openhorizon/$(arch)_anax:testing
DOCKER_ANAX_K8S_INAME ?= openhorizon/$(arch)_anax_k8s:testing

# ============================================================================
# E2E Test Container Configuration
# ============================================================================
# Legacy containerized test environment (being phased out in favor of localhost execution)
DOCKER_TEST_CNAME ?= e2edevtest
DOCKER_TEST_INAME ?= openhorizon/e2edev-test
DOCKER_TEST_TAG ?= latest
DOCKER_TEST_NETWORK ?= hzn_horizonnet
DOCKER_TEST_ADD_HOST ?= "localhost:127.0.0.1"

# ============================================================================
# Network Configuration
# ============================================================================
# E2EDEV_HOST_IP: IP address for service binding (0.0.0.0 = all interfaces)
# Services bind to 0.0.0.0 to accept connections from:
# - localhost (127.0.0.1) for local testing
# - actual network IP (e.g., 192.168.1.100) for Kubernetes pods
# - Docker bridge network for containerized tests
E2EDEV_HOST_IP ?= 0.0.0.0

# E2EDEV_CLIENT_IP: IP address for client connections (NEVER 0.0.0.0)
# Clients must connect to a specific IP address, not 0.0.0.0
# Used for: CSS_URL, DOCKER_EXCH, AGBOT_API, and other service URLs
# Detection order: default route IP → hostname -I → fallback to 127.0.0.1
# Note: run_kube.sh has its own IP detection for Kubernetes pod configuration
# For VM environments: Override with E2EDEV_CLIENT_IP=127.0.0.1 if services bind to localhost
E2EDEV_CLIENT_IP ?= $(shell ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K\S+' || hostname -I 2>/dev/null | awk '{print $$1}' || echo "127.0.0.1")

# ============================================================================
# Test Service Configuration
# ============================================================================
# Unix domain socket directory for agent communication
UDS ?= /var/run/horizon/

# ============================================================================
# Temporary Directory Configuration
# ============================================================================
# Base directory for all E2E test temporary files
# Follows Linux FHS convention: /var/tmp/$UID for user-specific temporary files
# Falls back to /tmp if /var/tmp is not available (backward compatibility)
E2E_TMP_BASE ?= $(shell if [ -d /var/tmp ] && [ -w /var/tmp ]; then echo "/var/tmp/$(shell id -u)"; else echo "/tmp"; fi)

# Individual temporary directories for test components
E2E_TEST_DIR := $(E2E_TMP_BASE)/e2edevtest
ESS_STORE_DIR := $(E2E_TMP_BASE)/ess-store
ESS_AUTH_DIR := $(E2E_TMP_BASE)/ess-auth
SERVICE_STORAGE_DIR := $(E2E_TMP_BASE)/service_storage
HZNDEV_DIR := $(E2E_TMP_BASE)/hzndev

# API key mode (0=disabled, 1=enabled)
API_KEY ?= 0

# CPU service image configuration (example service for testing)
export DOCKER_CPU_TAG := 1.2.7
ifeq ($(arch),ppc64le)
	export DOCKER_CPU_INAME ?= openhorizon/ibm.cpu_$(arch)
else
	export DOCKER_CPU_INAME ?= openhorizon/ibm.cpu_amd64
endif

# Legacy ICP configuration (deprecated)
ICP_HOST_IP ?= 0

# Organization ID for testing (can be overridden)
ORG_ID ?=

# ============================================================================
# Agreement Bot (Agbot) Configuration
# ============================================================================
# Agbot negotiates agreements with edge nodes for service deployment
DOCKER_AGBOT_CNAME ?= agbot
DOCKER_AGBOT2_CNAME ?= agbot2
AGBOT_IMAGE_NAME ?= docker.io/openhorizon/$(arch)_agbot
AGBOT_IMAGE_TAG ?= e2edev

# Use latest agbot image if OLDAGBOT=1 in TEST_VARS
old_agbot := $(findstring OLDAGBOT=1,$(TEST_VARS))
ifneq ($(old_agbot),)
	AGBOT_IMAGE_TAG = latest
endif

# Agbot API endpoints (computed from network configuration)
AGBOT_API := http://$(E2EDEV_CLIENT_IP):3110
AGBOT2_API := http://agbot2:8080
AGBOT_SAPI_URL := $(HZN_TRANSPORT)://$(E2EDEV_CLIENT_IP):3111

# Agbot authentication
AGBOT_NAME ?= agbot1
AGBOT_TOKEN ?= Abcdefghijklmno1

# ============================================================================
# Exchange API Configuration
# ============================================================================
# Exchange is the central management hub for Open Horizon
# Manages: nodes, services, patterns, policies, agreements
DOCKER_EXCH_CNAME := exchange-api
DOCKER_EXCH_INAME ?= quay.io/openhorizon/$(arch)_exchange-api
DOCKER_EXCH_TAG ?= testing
DOCKER_EXCH := $(HZN_TRANSPORT)://$(E2EDEV_CLIENT_IP):3090/v1
EXCHANGE_LOG_LEVEL ?= DEBUG

# Exchange authentication credentials
EXCH_ROOTPW ?= Horizon-Rul3s
EXCHANGE_HUB_ADMIN_PW ?= hubadminpw
EXCHANGE_SYSTEM_ADMIN_PW ?= ibmadminpw

# ============================================================================
# Exchange Database (PostgreSQL) Configuration
# ============================================================================
DOCKER_EXCHDB_CNAME ?= postgres
DOCKER_EXCHDB_INAME ?= postgres
DOCKER_EXCHDB_TAG ?= 17                         # PostgreSQL 17 for all architectures
EXCHDB_USER ?= admin
EXCHDB_PORT ?= 5432
EXCHDB_NAME ?= exchange
EXCHANGE_DB_PW ?= some_rand0mdat@basepassWord

# ============================================================================
# Docker Registry Configuration
# ============================================================================
# Local Docker registry for testing image pull/push operations
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

# ============================================================================
# Edge Sync Service (ESS) Configuration
# ============================================================================
# ESS runs on edge nodes to sync models and files from CSS
UNIX_SOCKET_FILE_PERMISSIONS ?= "0777"

# ============================================================================
# Cloud Sync Service (CSS) Configuration
# ============================================================================
# CSS manages model and file distribution to edge nodes
DOCKER_CSS_CNAME ?= css-api
DOCKER_CSS_INAME ?= docker.io/openhorizon/$(arch)_cloud-sync-service
DOCKER_CSS_TAG ?= latest
CSS_INTERNAL_PORT ?= 8080
CSS_LOG_LEVEL ?= DEBUG
CSS_TRACE_LEVEL ?= DEBUG
CSS_URL ?= $(HZN_TRANSPORT)://$(E2EDEV_CLIENT_IP):9443

# ============================================================================
# CSS Database (MongoDB) Configuration
# ============================================================================
DOCKER_CSSDB_CNAME := mongo
ifeq ($(arch),ppc64le)
	DOCKER_CSSDB_INAME ?= ppc64le/mongodb
	DOCKER_CSSDB_TAG ?= 2.6.10
else
	DOCKER_CSSDB_INAME ?= mongo
	DOCKER_CSSDB_TAG ?= 4.0.6
endif

# ============================================================================
# OpenBao (Vault Replacement) Configuration
# ============================================================================
# OpenBao provides secrets management for edge services
# OpenBao is the open-source continuation of HashiCorp Vault
# Official image: quay.io/openbao/openbao-ubi
DOCKER_VAULT_CNAME ?= bao
DOCKER_VAULT_INAME ?= quay.io/openbao/openbao-ubi
DOCKER_VAULT_TAG ?= 2.0
VAULT_PORT ?= 8200

# Conditionally set VAULT_ADDR based on NOVAULT test variable
no_vault := $(findstring NOVAULT=1,$(TEST_VARS))
ifeq ($(no_vault),)
	VAULT_ADDR = $(HZN_TRANSPORT)://bao:$(VAULT_PORT)
endif

# Vault development mode configuration
VAULT_DEV_ROOT_TOKEN_ID ?= vault_dev_root_token_id
VAULT_DEV_LISTEN_ADDRESS ?= 0.0.0.0:$(VAULT_PORT)

# ============================================================================
# Exported Environment Variables
# ============================================================================
# These variables are exported to make them available to test scripts and subprocesses.
# Note: Some variables are also exported in the run-mgmthub target (lines 857-891) to
# ensure they are available in the management hub startup context. This duplication is
# intentional - the exports here provide defaults for test scripts, while the run-mgmthub
# exports ensure proper environment setup for the management hub components.

export EXCH_APP_HOST ?= $(DOCKER_EXCH)
export HZN_SSL_SKIP_VERIFY ?= "1"
export CSS_URL ?= $(CSS_URL)
export EXCH_ROOTPW ?= $(EXCH_ROOTPW)
export EXCHANGE_HUB_ADMIN_PW ?= $(EXCHANGE_HUB_ADMIN_PW)
export EXCHANGE_SYSTEM_ADMIN_PW ?= $(EXCHANGE_SYSTEM_ADMIN_PW)
export AGBOT_API ?= $(AGBOT_API)
export AGBOT_NAME ?= $(AGBOT_NAME)
export AGBOT_SAPI_URL ?= $(AGBOT_SAPI_URL)
export AGBOT2_API ?= $(AGBOT2_API)
export EXCHANGE_DB_PW ?= $(EXCHANGE_DB_PW)
export DOCKER_REG_USER ?= $(DOCKER_REG_USER)
export DOCKER_REG_PW ?= $(DOCKER_REG_PW)
export HZN_DEV_HOST_IP ?= $(E2EDEV_HOST_IP)
export HZN_DEV_FSS_IMAGE_TAG ?= testing$(BRANCH_NAME)
export ARCH ?= amd64
export HZN_AGBOT_URL ?= $(AGBOT_SAPI_URL)

# ============================================================================
# Make Configuration
# ============================================================================
ifndef verbose
.SILENT:
endif

# Disable parallel execution - targets have dependencies on shared resources
.NOTPARALLEL:

# ============================================================================
# Target Aliases
# ============================================================================
# Convenience aliases for common operations
all: default
default: build
stop: clean
build: test-image
build-remote: test-image
run: test
distclean: cleanest
mostlyclean: cleanest
realclean: cleanest

# ============================================================================
# Build and Test Targets
# ============================================================================

# Progressive cleanup targets for E2E environment
# clean    - Light cleanup: stops services, removes workloads, keeps infrastructure
# cleaner  - Medium cleanup: clean + removes test containers and temp files
# cleanest - Full cleanup: cleaner + removes all images and infrastructure (equivalent to old realclean)

# run-test: Start E2E test container with all required mounts and environment
run-test: test-image e2edevtest-docker-prereqs test-network
	mkdir -p $(E2E_TEST_DIR)
	@echo "Handling $(DOCKER_TEST_CNAME)"
	docker/docker_run.bash "$(DOCKER_TEST_CNAME)" \
		docker run -d \
		--privileged \
		-p 8510:8510 \
		-p 8511:8511 \
		--name "$(DOCKER_TEST_CNAME)" \
		--network "$(DOCKER_TEST_NETWORK)" \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(UDS):$(UDS):rw \
		-v /var/tmp/horizon/:/var/tmp/horizon/:rw \
		-v ~/.docker:/home/agbotuser/.docker \
		-v ~/.docker:/root/.docker \
		-v /etc/wiotp-edge/:/etc/wiotp-edge/ \
		-v /var/wiotp-edge/persist/:/var/wiotp-edge/persist/ \
		-v $(ESS_AUTH_DIR):$(ESS_AUTH_DIR):rw \
		-v $(HZNDEV_DIR):$(HZNDEV_DIR) \
		-v $(E2EDEVTEST_TEMPFS)/certs:/certs \
		-v /var/log/:/var/log/ \
		-v $(ANAX_SOURCE)/anax-in-container:/tmp/anax-in-container \
		-v $(E2E_TEST_DIR):$(E2E_TEST_DIR):rw \
		-e "DOCKER_TEST_NETWORK=$(DOCKER_TEST_NETWORK)" \
		-e "EXCH_APP_HOST=$(DOCKER_EXCH)" \
		-e "HZN_SSL_SKIP_VERIFY=1" \
		-e "CSS_URL=$(CSS_URL)" \
		-e "CSS_LOG_LEVEL=$(CSS_LOG_LEVEL)" \
		-e "CSS_TRACE_LEVEL=$(CSS_TRACE_LEVEL)" \
		-e "API_KEY=$(API_KEY)" \
		-e "CERT_LOC=$(CERT_LOC)" \
		-e "EXCHANGE_LOG_LEVEL=$(EXCHANGE_LOG_LEVEL)" \
		-e "EXCH_ROOTPW=$(EXCH_ROOTPW)" \
		-e "EXCHANGE_HUB_ADMIN_PW=$(EXCHANGE_HUB_ADMIN_PW)" \
		-e "EXCHANGE_SYSTEM_ADMIN_PW=$(EXCHANGE_SYSTEM_ADMIN_PW)" \
		-e "AGBOT_API=$(AGBOT_API)" \
		-e "AGBOT_NAME=$(AGBOT_NAME)" \
		-e "AGBOT_SAPI_URL=$(AGBOT_SAPI_URL)" \
		-e "AGBOT2_API=$(AGBOT2_API)" \
		-e "ORG_ID=$(ORG_ID)" \
		-e "ICP_HOST_IP=$(ICP_HOST_IP)" \
		-e "EXCHANGE_DB_PW=$(EXCHANGE_DB_PW)" \
		-e "DOCKER_REG_USER=$(DOCKER_REG_USER)" \
		-e "DOCKER_REG_PW=$(DOCKER_REG_PW)" \
		-e "HZN_EXCHANGE_URL=$(DOCKER_EXCH)" \
		-e "HZN_DEV_HOST_IP=$(E2EDEV_HOST_IP)" \
		-e "HZN_DEV_FSS_IMAGE_TAG"="testing$(BRANCH_NAME)" \
		-e "DOCKER_CPU_INAME=$(DOCKER_CPU_INAME)" \
		-e "DOCKER_CPU_TAG=$(DOCKER_CPU_TAG)" \
		-e "ARCH=$(arch)" \
		-e "VAULT_TOKEN"="$(VAULT_DEV_ROOT_TOKEN_ID)" \
		-e "VAULT_ADDR"="$(VAULT_ADDR)" \
		-e "HZN_AGBOT_URL"="$(AGBOT_SAPI_URL)" \
		-e "TEST_PATTERNS"="$(TEST_PATTERNS)" \
		-e "UNIX_SOCKET_FILE_PERMISSIONS=$(UNIX_SOCKET_FILE_PERMISSIONS)" \
		-t $(DOCKER_TEST_INAME):$(DOCKER_TEST_TAG)
	docker cp -a $(E2EDEVTEST_TEMPFS)/. $(DOCKER_TEST_CNAME):/
	docker exec --user root $(DOCKER_TEST_CNAME) chown -R 1000:1000 /certs

# run-dockerreg: Start local Docker registry with TLS for testing
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
		--health-cmd="nc -z localhost 443 || exit 1" \
		--health-interval=10s \
		--health-timeout=5s \
		--health-retries=3 \
		--health-start-period=10s \
		-v $(E2EDEVTEST_TEMPFS)/certs:/certs \
		-v $(E2EDEVTEST_TEMPFS)/auth:/auth \
		-v ~/.docker:/root/.docker \
		-e REGISTRY_HTTP_ADDR=0.0.0.0:443 \
		-e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt \
		-e REGISTRY_HTTP_TLS_KEY=/certs/domain.key \
		-e REGISTRY_AUTH=htpasswd \
		-e REGISTRY_AUTH_HTPASSWD_REALM=RegistryRealm \
		-e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
		-e "TEST_PATTERNS"="$(TEST_PATTERNS)" \
		-t $(DOCKER_REG_INAME)@$(DOCKER_REG_TAG)
	@echo "Docker $(DOCKER_REG_CNAME) container running with TLS"
	docker run --name $(DOCKER_REG_PW_CNAME) \
		--entrypoint htpasswd \
		-t $(DOCKER_REG_INAME)@$(DOCKER_REG_TAG) \
		-Bbn $(DOCKER_REG_USER) $(DOCKER_REG_PW) > $(E2EDEVTEST_TEMPFS)/auth/htpasswd
	docker rm -v $(DOCKER_REG_PW_CNAME)

# Light cleanup: stops agents and workloads, keeps infrastructure (Exchange, CSS, images)
# Use this for quick cleanup between test runs when you want to keep the management hub running
# Preserves: Management hub (Exchange, CSS, Agbot), base images, networks
clean:
	@echo -e "\n========================================="
	@echo "LIGHT cleanup (Level 1)"
	@echo "Preserves: Management hub, base images"
	@echo "========================================="
	@echo "Clean up kube environment"
	-@ARCH=$(arch) gov/stop_kube.sh 2>/dev/null || true
	@echo "Shutdown anax cleanly"
	-@curl -sSLX DELETE http://localhost:8510/node 2>/dev/null || true
	-@curl -sSLX DELETE http://localhost:8511/node 2>/dev/null || true
	@echo "Removing agent containers"
	-@HC_BASE=$(ANAX_SOURCE)/anax-in-container $(ANAX_SOURCE)/test/gov/stop_multiple_agents.sh 2>/dev/null || true
	@echo "Cleaning up any left over workloads"
	-@docker stop $$(docker ps -a | egrep "openhorizon/|wiotp-|localhost|hashicorp/" | egrep -v "e2edev-test|$(arch)_agbot|$(arch)_exchange-api|$(arch)_cloud-sync-service|$(arch)_edge-sync-service|$(arch)_vault|REPOSITORY" | awk '{print $$1}') 2>/dev/null || true
	-@docker rm $$(docker ps -a | egrep "openhorizon/|wiotp-|localhost|hashicorp/" | egrep -v "e2edev-test|$(arch)_agbot|$(arch)_exchange-api|$(arch)_cloud-sync-service|$(arch)_edge-sync-service|$(arch)_vault|REPOSITORY" | awk '{print $$1}') 2>/dev/null || true
	@echo "========================================="
	@echo "Light cleanup complete"
	@echo "========================================="

# Medium cleanup: clean + removes management hub and test containers
# Use this for a more thorough cleanup while preserving base images
# Preserves: Base images (Exchange, CSS, Agbot, database images)
cleaner: clean
	@echo -e "\n========================================="
	@echo "MEDIUM cleanup (Level 2)"
	@echo "Preserves: Base images"
	@echo "========================================="
	@echo "Removing the all-in-one management hub"
	-@ANAX_SOURCE=$(ANAX_SOURCE) $(ANAX_SOURCE)/test/gov/stop_mgmt_hub.sh 2>/dev/null || true
	@echo "Removing the $(DOCKER_VAULT_CNAME) container"
	-@docker rm -f "$(DOCKER_VAULT_CNAME)" 2>/dev/null || true
	@echo "Removing e2edevtest container"
	-@docker rm -f "$(DOCKER_TEST_CNAME)" 2>/dev/null || true
	@echo "Removing the $(DOCKER_REG_CNAME) container"
	-@docker rm -v -f "$(DOCKER_REG_CNAME)" 2>/dev/null || true
	@echo "Removing working directories"
	-@rm -rf $(CURDIR)/docker/tempfs 2>/dev/null || true
	-@sudo rm -fr $(UDS) 2>/dev/null || true
	-@sudo rm -r $(E2E_TEST_DIR) $(HZNDEV_DIR) $(ESS_AUTH_DIR) $(ESS_STORE_DIR) $(SERVICE_STORAGE_DIR) 2>/dev/null || true
	@echo "Removing stale networks"
	-@docker network prune -f 2>/dev/null || true
	@echo "========================================="
	@echo "Medium cleanup complete"
	@echo "========================================="

# Full cleanup: cleaner + removes management hub, anax binaries, and all test-generated images
# Use this to completely reset the localhost to a clean state
# Preserves: Base infrastructure images (Exchange, CSS, Agbot, database base images)
cleanest: cleaner
	@echo -e "\n========================================="
	@echo "FULL cleanup (Level 3)"
	@echo "Preserves: Base infrastructure images"
	@echo "========================================="
	@echo "Removing the all-in-one management hub"
	-@ANAX_SOURCE=$(ANAX_SOURCE) $(ANAX_SOURCE)/test/gov/stop_mgmt_hub.sh 2>/dev/null || true
	@echo "Removing the $(DOCKER_VAULT_CNAME) container"
	-@docker rm -f "$(DOCKER_VAULT_CNAME)" 2>/dev/null || true
	@echo "Removing any leftover images generated by e2edev"
	-@docker rmi $$(docker images | egrep "openhorizon/|wiotp-|localhost" | egrep -v "e2edev-test|$(arch)_agbot|$(arch)_exchange-api|$(arch)_cloud-sync-service|$(arch)_edge-sync-service|$(arch)_anax_k8s|$(arch)_vault|REPOSITORY" | awk '{print $$3}') 2>/dev/null || true
	@echo "Cleaning anax binaries and configs"
	-@cd $(ANAX_SOURCE) && make clean
	@echo "Removing all infrastructure containers"
	-@docker rm -f "$(DOCKER_EXCHDB_CNAME)" "$(DOCKER_EXCH_CNAME)" "$(DOCKER_CSSDB_CNAME)" "$(DOCKER_CSS_CNAME)" 2>/dev/null || true
	@echo "Removing all infrastructure images"
	-@docker rmi \
		$(AGBOT_IMAGE_NAME):$(AGBOT_IMAGE_TAG) \
		$(DOCKER_TEST_INAME):$(DOCKER_TEST_TAG) \
		$(DOCKER_TEST_INAME):latest \
		$(DOCKER_EXCH_INAME):$(DOCKER_EXCH_TAG) \
		$(DOCKER_CSS_INAME):$(DOCKER_CSS_TAG) \
		$(DOCKER_VAULT_INAME):$(DOCKER_VAULT_TAG) \
		$(DOCKER_ANAX_INAME) \
		$(DOCKER_ANAX_K8S_INAME) \
		$(DOCKER_EXCHDB_INAME):$(DOCKER_EXCHDB_TAG) \
		$(DOCKER_CSSDB_INAME):$(DOCKER_CSSDB_TAG) \
		2>/dev/null || true
	@echo "Removing test network"
	-@docker network rm $(DOCKER_TEST_NETWORK) 2>/dev/null || true
	@echo "Removing all dangling images"
	-@docker rmi $$(docker images -qf "dangling=true") 2>/dev/null || true
	@echo "Pruning all unused networks"
	-@docker network prune -f 2>/dev/null || true
	@echo "Pruning all unused volumes"
	-@docker volume prune -f 2>/dev/null || true
	@echo "========================================="
	@echo "Full cleanup complete"
	@echo "Localhost reset to default state"
	@echo "========================================="

# ============================================================================
# Infrastructure Setup Targets
# ============================================================================

# test-network: Create Docker network for E2E testing
# NOTE: Network creation is handled by management hub scripts (start_mgmt_hub.sh)
# TODO: Remove this target if network creation is permanently delegated to management hub
test-network:
	@echo "Creating e2edev environment network"
	# Network creation delegated to start_mgmt_hub.sh to ensure proper initialization order
	# If manual network creation is needed, uncomment:
	# if [[ "$(shell docker network ls -qf name=$(DOCKER_TEST_NETWORK))" == "" ]]; then \
	#     docker network create "$(DOCKER_TEST_NETWORK)"; \
	# fi

# $(ANAX_SOURCE)/anax: Build anax binaries and Docker images
# Builds: anax daemon, hzn CLI, ESS image, CSS image
$(ANAX_SOURCE)/anax:
	@echo "Building anax with arch=$(arch)"
	cd $(ANAX_SOURCE) && \
		$(MAKE) clean && \
		$(MAKE) arch=$(arch) && \
		$(MAKE) arch=$(arch) ess-docker-image && \
		$(MAKE) arch=$(arch) css-docker-image

# e2edevtest-docker-prereqs: Prepare directories and configuration for legacy containerized tests
# Creates required directories and processes configuration templates
# TODO: Legacy containerized test approach - consider removing if fully replaced by localhost execution
# e2edevtest-docker-prereqs: Create required temporary directories for E2E tests
e2edevtest-docker-prereqs:
	mkdir -p $(ESS_STORE_DIR) $(ESS_AUTH_DIR) $(SERVICE_STORAGE_DIR) $(HZNDEV_DIR)
	# Legacy container setup (commented out - now using localhost execution):
	# mkdir -p $(E2EDEVTEST_TEMPFS)/usr/local/bin $(E2EDEVTEST_TEMPFS)/root/.colonus
	# cp $(ANAX_SOURCE)/anax $(ANAX_SOURCE)/cli/hzn $(E2EDEVTEST_TEMPFS)/usr/local/bin
	export PATH="$PATH,$(ANAX_SOURCE),$(ANAX_SOURCE)/cli"
	# cp -r $(CURDIR)/docker/fs/etc $(E2EDEVTEST_TEMPFS)
	# cp -r $(CURDIR)/docker/fs/hzn $(CURDIR)/gov/* $(CURDIR)/docker/fs/helm $(CURDIR)/docker/fs/resources $(CURDIR)/docker/fs/objects $(E2EDEVTEST_TEMPFS)/root
	for f in $$(find $(CURDIR)/docker/fs/etc/colonus/ -maxdepth 1 -name '*.tmpl'); do EXCH_APP_HOST=$(DOCKER_EXCH) CSS_URL=$(CSS_URL) HZN_AGBOT_URL=$(AGBOT_SAPI_URL) envsubst < $$f > "$${f%.tmpl}"; done

# test-image: Build e2edevtest container image (legacy)
# Skips build if image already exists (run 'make clean && make' to rebuild)
test-image:
	@echo "Handling $(DOCKER_TEST_INAME)"
	@if docker image inspect $(DOCKER_TEST_INAME):$(DOCKER_TEST_TAG) > /dev/null 2>&1; then \
		echo "Skipping since $(DOCKER_TEST_INAME):$(DOCKER_TEST_TAG) image exists, run 'make clean && make' if a rebuild is desired"; \
	else \
		echo "Building container image $(DOCKER_TEST_INAME):$(DOCKER_TEST_TAG)"; \
		docker build $(DOCKER_DEV_OPTS) -t $(DOCKER_TEST_INAME):$(DOCKER_TEST_TAG) -f docker/Dockerfile ./docker || exit 1; \
	fi

# ============================================================================
# Legacy Test Execution Targets
# ============================================================================
# NOTE: These targets use the old test execution approach
# For modern testing, use e2e-* targets instead

# test-no-clean: Run tests without initial cleanup (assumes clean environment)
# Executes: build → start management hub → bootstrap Exchange → setup Vault → run Kubernetes → run tests
test-no-clean: $(ANAX_SOURCE)/anax run-mgmthub run-dockerreg
	@echo -e  "\nBootstrapping the exchange"
	# docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) EXCHANGE_HUB_ADMIN_PW=$(EXCHANGE_HUB_ADMIN_PW) EXCHANGE_SYSTEM_ADMIN_PW=$(EXCHANGE_SYSTEM_ADMIN_PW); /root/init_exchange.sh"
	./gov/init_exchange.sh
	@echo -e  "\nSetting up secrets in the vault"
	# docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) DOCKER_VAULT_CNAME=$(DOCKER_VAULT_CNAME); /root/setup_secrets.sh"
	./gov/setup_secrets.sh
	@echo -e  "\nSetting up agent in kube"
	$(TEST_VARS) ARCH=$(arch) CERT_LOC=$(CERT_LOC) ./gov/run_kube.sh $(E2EDEVTEST_TEMPFS) $(ANAX_SOURCE) $(EXCH_ROOTPW) $(DOCKER_TEST_NETWORK) $(E2EDEV_HOST_IP)
	@echo -e  "\nStarting tests"
	# docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS); /root/gov-combined.sh"
	export $(TEST_VARS); ./gov/framework/gov-combined-new.sh

# Localhost E2E workflow targets - replicate GitHub Actions workflow for local development
# These targets provide a complete E2E test workflow optimized for localhost VMs

# e2e-setup: Prepare localhost environment for E2E testing
# Equivalent to GitHub Actions setup steps (checkout, build, image prep)
e2e-setup: $(ANAX_SOURCE)/anax
	@echo -e "\n========================================="
	@echo "E2E Setup: Preparing localhost environment"
	@echo "========================================="
	@echo "Anax binaries built successfully"
	@echo "Verifying Docker is available..."
	@docker --version
	@echo "Setup complete - ready for E2E testing"

# e2e-test-noloop: Run E2E tests without loop tests (faster, equivalent to NOLOOP=1)
# Equivalent to GitHub Actions matrix test with NOLOOP=1
# Uses cleaner (removes infrastructure but preserves images/binaries for caching)
e2e-test-noloop: cleaner e2e-setup run-mgmthub run-dockerreg
	@echo -e "\n========================================="
	@echo "E2E Test: Running without loop tests"
	@echo "Test mode: NOLOOP=1"
	@echo "Caching: Docker images and binaries preserved"
	@echo "========================================="
	@echo "Bootstrapping the exchange"
	./gov/init_exchange.sh
	@echo "Setting up secrets in the vault"
	./gov/setup_secrets.sh
	@echo "Setting up agent in kube"
	NOLOOP=1 ARCH=$(arch) CERT_LOC=$(CERT_LOC) ./gov/run_kube.sh $(E2EDEVTEST_TEMPFS) $(ANAX_SOURCE) $(EXCH_ROOTPW) $(DOCKER_TEST_NETWORK) $(E2EDEV_HOST_IP)
	@echo "Starting tests"
	export NOLOOP=1; ./gov/framework/gov-combined-new.sh
	@echo -e "\n========================================="
	@echo "Test complete - cleaning infrastructure"
	@echo "Docker images and binaries preserved for next run"
	@echo "========================================="
	$(MAKE) cleaner

# e2e-test-patterns: Run E2E tests with all patterns (equivalent to TEST_PATTERNS=sall)
# Equivalent to GitHub Actions matrix test with NOLOOP=1 TEST_PATTERNS=sall
# Uses cleaner (removes infrastructure but preserves images/binaries for caching)
e2e-test-patterns: cleaner e2e-setup run-mgmthub run-dockerreg
	@echo -e "\n========================================="
	@echo "E2E Test: Running with all patterns"
	@echo "Test mode: NOLOOP=1 TEST_PATTERNS=sall"
	@echo "Caching: Docker images and binaries preserved"
	@echo "========================================="
	@echo "Bootstrapping the exchange"
	./gov/init_exchange.sh
	@echo "Setting up secrets in the vault"
	./gov/setup_secrets.sh
	@echo "Setting up agent in kube"
	NOLOOP=1 TEST_PATTERNS=sall ARCH=$(arch) CERT_LOC=$(CERT_LOC) ./gov/run_kube.sh $(E2EDEVTEST_TEMPFS) $(ANAX_SOURCE) $(EXCH_ROOTPW) $(DOCKER_TEST_NETWORK) $(E2EDEV_HOST_IP)
	@echo "Starting tests"
	export NOLOOP=1 TEST_PATTERNS=sall; ./gov/framework/gov-combined-new.sh
	@echo -e "\n========================================="
	@echo "Test complete - cleaning infrastructure"
	@echo "Docker images and binaries preserved for next run"
	@echo "========================================="
	$(MAKE) cleaner

# e2e-test-full: Run complete E2E test suite (both test modes)
# Equivalent to running both GitHub Actions matrix tests sequentially
# Uses cleaner between tests (removes infrastructure but preserves images/binaries)
e2e-test-full:
	@echo -e "\n========================================="
	@echo "E2E Full Test Suite: Running both test modes"
	@echo "Caching: Docker images and binaries preserved between tests"
	@echo "========================================="
	$(MAKE) e2e-test-noloop
	@echo -e "\n========================================="
	@echo "First test complete, preparing for second test"
	@echo "========================================="
	$(MAKE) e2e-test-patterns
	@echo -e "\n========================================="
	@echo "Both tests complete"
	@echo "Run 'make cleaner' to remove infrastructure"
	@echo "Run 'make cleanest' for complete cleanup"
	@echo "========================================="

# e2e-test-sterile: Run E2E test with complete cleanup (no caching)
# For testing from a completely clean state
# Uses cleanest before and after (removes all images/binaries/infrastructure)
e2e-test-sterile: cleanest
	@echo -e "\n========================================="
	@echo "E2E Sterile Test: Complete cleanup mode"
	@echo "Caching: DISABLED - full rebuild required"
	@echo "========================================="
	$(MAKE) e2e-test-noloop
	@echo -e "\n========================================="
	@echo "Sterile test complete - performing full cleanup"
	@echo "========================================="
	$(MAKE) cleanest

# e2e-quick: Quick E2E test for rapid iteration (no kube, no patterns)
# Optimized for fast feedback during development
# Uses cleaner before test, no cleanup after (fastest iteration)
e2e-quick: cleaner e2e-setup run-mgmthub run-dockerreg
	@echo -e "\n========================================="
	@echo "E2E Quick Test: Fast iteration mode"
	@echo "Test mode: NOLOOP=1 NOKUBE=1"
	@echo "Caching: Docker images and binaries preserved"
	@echo "========================================="
	@echo "Bootstrapping the exchange"
	./gov/init_exchange.sh
	@echo "Setting up secrets in the vault"
	./gov/setup_secrets.sh
	@echo "Starting tests (no Kubernetes)"
	export NOLOOP=1 NOKUBE=1; ./gov/framework/gov-combined-new.sh
	@echo -e "\n========================================="
	@echo "Quick test complete (no cleanup for faster iteration)"
	@echo "Run 'make cleaner' manually when done iterating"
	@echo "========================================="

# e2e-verify: Verify E2E environment is ready (check images, containers, services)
# Equivalent to GitHub Actions verification steps
e2e-verify:
	@echo -e "\n========================================="
	@echo "E2E Verify: Checking environment readiness"
	@echo "========================================="
	@echo ""
	@echo "=== Docker Images ==="
	@docker images
	@echo ""
	@echo "=== Docker Containers ==="
	@docker ps -a
	@echo ""
	@echo "=== Checking for required management hub images ==="
	@for img in "quay.io/openhorizon/$(arch)_exchange-api:testing" \
	            "openhorizon/$(arch)_agbot:e2edev" \
	            "openhorizon/$(arch)_cloud-sync-service:latest" \
	            "mongo:4.0.6" \
	            "postgres:17" \
	            "openhorizon/$(arch)_vault:testing"; do \
	    if docker image inspect "$$img" > /dev/null 2>&1; then \
	        echo "✓ Found: $$img"; \
	    else \
	        echo "✗ Missing: $$img (will be pulled/built during test)"; \
	    fi; \
	done
	@echo ""
	@echo "=== Checking for base images ==="
	@for img in "alpine:latest" \
	            "registry.access.redhat.com/ubi9-minimal:latest"; do \
	    if docker image inspect "$$img" > /dev/null 2>&1; then \
	        echo "✓ Found: $$img"; \
	    else \
	        echo "✗ Missing: $$img (will be pulled during test)"; \
	    fi; \
	done
	@echo ""
	@echo "=== Anax Binaries ==="
	@if [ -f "$(ANAX_SOURCE)/anax" ]; then \
	    echo "✓ Found: $(ANAX_SOURCE)/anax"; \
	    $(ANAX_SOURCE)/anax -v 2>/dev/null || echo "  (version check failed)"; \
	else \
	    echo "✗ Missing: $(ANAX_SOURCE)/anax (run 'make' to build)"; \
	fi
	@if [ -f "$(ANAX_SOURCE)/cli/hzn" ]; then \
	    echo "✓ Found: $(ANAX_SOURCE)/cli/hzn"; \
	    $(ANAX_SOURCE)/cli/hzn version 2>/dev/null || echo "  (version check failed)"; \
	else \
	    echo "✗ Missing: $(ANAX_SOURCE)/cli/hzn (run 'make' to build)"; \
	fi
	@echo ""
	@echo "========================================="
	@echo "Environment verification complete"
	@echo "========================================="

# test: Run tests with initial cleanup
# Equivalent to: clean → test-no-clean
test: clean test-no-clean

# ============================================================================
# Remote/Containerized Test Targets (Legacy)
# ============================================================================
# These targets run tests inside Docker containers
# Maintained for compatibility but prefer e2e-* targets for new development

# test-remote: Run tests in container with locally built images
test-remote: clean $(ANAX_SOURCE)/anax run-dockerreg run-test copy-cert
	@echo -e  "\nBootstrapping the exchange"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) NOKUBE=1; /root/init_exchange.sh"
	@echo -e  "\nSetting up secrets in the vault"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) NOKUBE=1 DOCKER_VAULT_CNAME=$(DOCKER_VAULT_CNAME); /root/setup_secrets.sh"
	@echo -e  "\nStarting tests"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) NOKUBE=1; /root/gov-combined.sh"

# test-remote-prebuilt: Run tests in container with prebuilt images from registry
test-remote-prebuilt: clean run-dockerreg download-agbot-image run-test copy-cert
	@echo -e  "\nBootstrapping the exchange"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) NOKUBE=1; /root/init_exchange.sh"
	@echo -e  "\nSetting up secrets in the vault"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) NOKUBE=1 DOCKER_VAULT_CNAME=$(DOCKER_VAULT_CNAME); /root/setup_secrets.sh"
	@echo -e  "\nStarting tests"
	docker exec $(DOCKER_TEST_CNAME) bash -c "export $(TEST_VARS) NOKUBE=1; /root/gov-combined.sh"

# ============================================================================
# Image Management Targets
# ============================================================================

# copy-cert: Copy TLS certificates to test environment for containerized tests
copy-cert:
	cp css.crt $(E2EDEVTEST_TEMPFS)/certs/css.crt
	cp agbotapi.crt $(E2EDEVTEST_TEMPFS)/certs/agbotapi.crt

# download-agbot-image: Download prebuilt agbot image from registry
# Extracts anax and hzn binaries from the image for local use
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

# get-anax-images: Build or verify anax container images exist
# Builds anax and anax-k8s images if not present (required for Kubernetes tests)
get-anax-images:
	@echo "Configuring agbot prerequisites"
	@docker image inspect $(DOCKER_ANAX_K8S_INAME) >/dev/null 2>&1 || \
		(cd $(ANAX_SOURCE) && $(MAKE) arch=$(arch) anax-k8s-image)
	@docker image inspect $(DOCKER_ANAX_INAME) >/dev/null 2>&1 || \
		(cd $(ANAX_SOURCE) && $(MAKE) arch=$(arch) anax-image)

# get-agbot-image: Build or update agbot image
# If image exists, updates it with latest anax and hzn binaries
# If image doesn't exist, builds it from scratch
get-agbot-image:
	if [[ "$(AGBOT_IMAGE_TAG)" == "e2edev" ]]; then \
		if ! docker image inspect $(AGBOT_IMAGE_NAME):$(AGBOT_IMAGE_TAG) >/dev/null 2>&1; then \
	    	echo -e "Building agbot image $(AGBOT_IMAGE_NAME):$(AGBOT_IMAGE_TAG)"; \
	  cd $(ANAX_SOURCE) && $(MAKE) arch=$(arch) agbot-image; \
	  docker tag $(AGBOT_IMAGE_NAME):testing $(AGBOT_IMAGE_NAME):e2edev; \
			docker images; \
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

# get-css-image: Verify CSS image exists (placeholder for future use)
get-css-image:
	if [[ "$(DOCKER_CSS_TAG)" == "e2edev" ]]; then \
		if ! docker image inspect $(DOCKER_CSS_INAME):$(DOCKER_CSS_TAG) >/dev/null 2>&1; then \
	    	echo -e "Building css image $(DOCKER_CSS_INAME):$(DOCKER_CSS_TAG)"; \
			docker images; \
	    fi \
	fi

# get-vault-image: Verify Vault image exists
# TODO: Image pull logic commented out - vault image now pulled by start_mgmt_hub.sh
get-vault-image:
	# Vault image pull delegated to start_mgmt_hub.sh
	# If manual pull is needed, uncomment:
	# if [[ "$(NOVAULT)" != "1" ]]; then \
	#     echo "\nPulling vault image $(DOCKER_VAULT_INAME)"; \
	#     docker pull $(DOCKER_VAULT_INAME):$(DOCKER_VAULT_TAG); \
	# else
	echo "Skipping vault image $(DOCKER_VAULT_INAME)";
	# fi

# ============================================================================
# Management Hub Target
# ============================================================================

# run-mgmthub: Start the all-in-one management hub
# Starts: Exchange, Agbot, CSS, databases (PostgreSQL, MongoDB), Vault
# This is the core infrastructure required for E2E testing
run-mgmthub: get-agbot-image get-css-image get-vault-image get-anax-images
	@echo "Starting all-in-one management hub"
	echo "\nHZN_LISTEN_IP is $(E2EDEV_HOST_IP)"; \
	export \
		EXCHANGE_ROOT_PW=$(EXCH_ROOTPW) \
		EXCH_ROOTPW=$(EXCH_ROOTPW) \
		EXCHANGE_IMAGE_TAG=$(DOCKER_EXCH_TAG) \
		EXCHANGE_LOG_LEVEL=$(EXCHANGE_LOG_LEVEL) \
		EXCHANGE_DATABASE=$(EXCHDB_NAME); \
	export \
		EXCHANGE_HUB_ADMIN_PW=$(EXCHANGE_HUB_ADMIN_PW) \
		EXCHANGE_SYSTEM_ADMIN_PW=$(EXCHANGE_SYSTEM_ADMIN_PW); \
	export \
		AGBOT_ID=$(AGBOT_NAME) \
		AGBOT_TOKEN=$(AGBOT_TOKEN) \
		AGBOT_IMAGE_NAME=$(AGBOT_IMAGE_NAME) \
		AGBOT_IMAGE_TAG=$(AGBOT_IMAGE_TAG); \
	export \
		CSS_INTERNAL_PORT=$(CSS_INTERNAL_PORT) \
		CSS_LOG_LEVEL=$(CSS_LOG_LEVEL) \
		CSS_TRACE_LEVEL=$(CSS_TRACE_LEVEL) \
		UNIX_SOCKET_FILE_PERMISSIONS=$(UNIX_SOCKET_FILE_PERMISSIONS); \
	export MONGO_IMAGE_TAG=$(DOCKER_CSSDB_TAG); \
	export \
		EXCHANGE_DB_PW=$(EXCHANGE_DB_PW) \
		POSTGRES_IMAGE_TAG=$(DOCKER_EXCHDB_TAG) \
		POSTGRES_USER=$(EXCHDB_USER); \
	export \
		VAULT_IMAGE_TAG=$(DOCKER_VAULT_TAG) \
		VAULT_ROOT_TOKEN=$(VAULT_DEV_ROOT_TOKEN_ID); \
		# Vault port and listen address now configured in start_mgmt_hub.sh:
		# VAULT_PORT=$(VAULT_PORT) \
		# VAULT_DEV_LISTEN_ADDRESS=$(VAULT_DEV_LISTEN_ADDRESS)
	export \
		ANAX_SOURCE=$(ANAX_SOURCE) \
		ANAX_LOG_LEVEL=5; \
	export HZN_TRANSPORT=$(HZN_TRANSPORT); \
	export HZN_LISTEN_IP=$(E2EDEV_HOST_IP); \
	if docker ps >/dev/null 2>&1; then \
		$(TEST_VARS) $(ANAX_SOURCE)/test/gov/start_mgmt_hub.sh; \
	else \
		echo "Docker requires root permissions. Using sudo for management hub deployment."; \
		echo "Note: You may be prompted for your password."; \
		sudo -E $(TEST_VARS) $(ANAX_SOURCE)/test/gov/start_mgmt_hub.sh; \
	fi
	if [ -f "/etc/horizon/keys/horizonMgmtHub.crt" ]; then \
		cp /etc/horizon/keys/horizonMgmtHub.crt $(E2EDEVTEST_TEMPFS)/certs/css.crt; \
		cp /etc/horizon/keys/horizonMgmtHub.crt $(E2EDEVTEST_TEMPFS)/certs/agbotapi.crt; \
	fi

# ============================================================================
# .PHONY Declaration
# ============================================================================
# Declare all targets as phony to ensure they always run

.PHONY: all default stop build run distclean clean cleaner cleanest mostlyclean realclean run-test run-dockerreg test-network test test-no-clean e2edevtest-docker-prereqs test-image copy-cert get-anax-images get-agbot-image get-css-image get-vault-image run-mgmthub download-agbot-image test-remote test-remote-prebuilt e2e-setup e2e-test-noloop e2e-test-patterns e2e-test-full e2e-test-sterile e2e-quick e2e-verify
