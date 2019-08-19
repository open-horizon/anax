DOCKER_NAME = hello
DOCKER_TAG = 1.0

# was tagged as: docker tag hello:1.0 registry.ng.bluemix.net/booz_dev/amd64/hello:1.0
# was pushed as: docker push registry.ng.bluemix.net/booz_dev/amd64/hello:1.0
# 1.0: digest: sha256:79853b76b173fb8b6da6c1dfb38abc0beb7d71733d3825ab82c4f23811cc323f size: 946

default: all

all: build run

build: clean
	docker build -t $(DOCKER_NAME):$(DOCKER_TAG) .

run: stop
	docker run -d --name $(DOCKER_NAME) --publish=8347:8347 --volume `pwd`:/outside -e HELLO_VAR='test1' -e HELLO_PORT='8347' $(DOCKER_NAME):$(DOCKER_TAG)

check:
	contId=$(shell docker ps -q | head -1) && \
		helloip=$$(docker inspect --format "{{ .NetworkSettings.IPAddress }}" $$contId) && \
			curl -sSL http://$$helloip:8347 | jq .

stop:
	-docker rm -f $(DOCKER_NAME) 2> /dev/null || :

clean:
	-docker rmi $(DOCKER_NAME):$(DOCKER_TAG)

.PHONY: default all build run check publish clean

