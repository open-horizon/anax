# Makefile for hello service container

ARCH = $(ARCH)
VER = 1.0
BASE = localhost:443
IMAGE = $(ARCH)_helloservice

HZN_DEV_HOST_IP ?=127.0.0.1

default: build run check

build: clean Dockerfile start.sh server
	-docker rmi $(BASE)/$(IMAGE):$(VER)
	docker build --no-cache -t $(BASE)/$(IMAGE):$(VER) .

server: server.go
	CGO_ENABLED=0 go build server.go

horizonstart:
	hzn -v dev service start

horizonstop:
	hzn -v dev service stop

run: stop build
	docker run --name $(IMAGE) -e MY_S_VAR1='outside' -e HZN_ARCH=$(ARCH) -p 8000:8000 -d -t "$(BASE)/$(IMAGE):$(VER)"
	sleep 3

check:
	docker ps -a --filter name=$(DOCKER_NAME)
	docker logs $(DOCKER_NAME)
	curl -sSLv "http://${HZN_DEV_HOST_IP}:8000/movie"

stop:
	-docker stop $(IMAGE)
	-docker rm $(IMAGE)

clean:
	rm -f server

dockerclean:
	-docker rmi $(BASE)/$(IMAGE):$(VER)

realclean: clean dockerclean
