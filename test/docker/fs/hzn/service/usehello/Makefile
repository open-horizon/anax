# Makefile for usehello service container

ARCH = $(ARCH)
VER = 1.0
BASE = localhost:443
IMAGE = $(ARCH)_usehello

HZN_DEV_HOST_IP ?=127.0.0.1

default: build run check

build: Dockerfile start.sh
	docker build -t $(BASE)/$(IMAGE):$(VER) .

horizonstart:
	hzn -v dev service start

horizonstop:
	hzn -v dev service stop

run:
	docker run --name $(IMAGE) --network=hzn_horizonnet -e MY_VAR1='outside' -e HZN_ARCH=$(ARCH) -d -t $(BASE)/$(IMAGE):$(VER)

check:
	docker ps -a --filter name=$(IMAGE)
	docker logs $(IMAGE)

stop:
	-docker stop $(IMAGE)
	-docker rm $(IMAGE)

dockerclean:
	-docker rmi $(BASE)/$(IMAGE):$(VER)

realclean: clean dockerclean
