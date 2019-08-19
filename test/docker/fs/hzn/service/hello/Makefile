# Makefile for hello service container

VER = 1.0
BASE = localhost:443
IMAGE = amd64_helloservice

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
	docker run --name $(IMAGE) -e MY_S_VAR1='outside' -p 8000:8000 -d -t $(BASE)/$(IMAGE):$(VER)
	sleep 3

check:
	contId=$(shell docker ps -q | head -1) && \
		helloip=$$(docker inspect --format "{{ .NetworkSettings.Gateway }}" $$contId) && \
			curl -sSL http://$$helloip:8000/movie

stop:
	-docker stop $(IMAGE)
	-docker rm $(IMAGE)

clean:
	rm -f server

dockerclean:
	-docker rmi $(BASE)/$(IMAGE):$(VER)

realclean: clean dockerclean
