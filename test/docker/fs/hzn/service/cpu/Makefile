DOCKER_NAME = amd64_cpu
DOCKER_TAG = 1.0

DOCKER_HUB_ID = localhost:443

default: all

all: build run check

build: clean
	docker build -t $(DOCKER_HUB_ID)/$(DOCKER_NAME):$(DOCKER_TAG) .

run: stop
	docker run -d --name $(DOCKER_NAME) --publish=8347:8347 --volume `pwd`:/outside $(DOCKER_HUB_ID)/$(DOCKER_NAME):$(DOCKER_TAG)

check:
	contId=$(shell docker ps -q | head -1) && \
		cpuip=$$(docker inspect --format "{{ .NetworkSettings.Gateway }}" $$contId) && \
			curl -sSL http://$$cpuip:8347/v1/cpu | jq .

stop:
	-docker rm -f $(DOCKER_NAME) 2> /dev/null || :

publish:
	docker push $(DOCKER_HUB_ID)/$(DOCKER_NAME):$(DOCKER_TAG)

clean: stop
	-docker rmi $(DOCKER_HUB_ID)/$(DOCKER_NAME):$(DOCKER_TAG)

.PHONY: default all build run check publish clean
