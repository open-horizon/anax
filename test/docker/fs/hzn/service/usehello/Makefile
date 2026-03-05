# Makefile for usehello service container

ARCH = $(ARCH)
VER = 1.0
BASE = localhost:443
IMAGE = $(ARCH)_usehello

HZN_DEV_HOST_IP ?=127.0.0.1

default: build run check

build: Dockerfile start.sh
	@if ! docker image inspect $(BASE)/$(IMAGE):$(VER) > /dev/null 2>&1; then \
		docker build -t $(BASE)/$(IMAGE):$(VER) .; \
	else \
		echo "Image $(BASE)/$(IMAGE):$(VER) already exists, skipping build"; \
	fi

horizonstart:
	hzn -v dev service start

horizonstop:
	hzn -v dev service stop

run:
	@if docker ps -a --filter name=^$(IMAGE)$$ --format '{{.Names}}' | grep -q '^$(IMAGE)$$'; then \
		if docker ps --filter name=^$(IMAGE)$$ --format '{{.Names}}' | grep -q '^$(IMAGE)$$'; then \
			echo "Container $(IMAGE) already running, skipping"; \
		else \
			echo "Container $(IMAGE) exists but not running, restarting"; \
			docker start $(IMAGE); \
		fi \
	else \
		docker run --name $(IMAGE) --network=hzn_horizonnet -e MY_VAR1='outside' -e HZN_ARCH=$(ARCH) -d -t $(BASE)/$(IMAGE):$(VER); \
	fi

check:
	docker ps -a --filter name=$(IMAGE)
	docker logs $(IMAGE)

stop:
	-docker stop $(IMAGE)
	-docker rm $(IMAGE)

dockerclean:
	-docker rmi $(BASE)/$(IMAGE):$(VER)

rebuild: Dockerfile start.sh
	docker build -t $(BASE)/$(IMAGE):$(VER) .

realclean: clean dockerclean

.PHONY: default build rebuild horizonstart horizonstop run check stop dockerclean realclean
