# Makefile for usehello service container

VER = 1.0
BASE = localhost:443
IMAGE = amd64_usehello

default: build run check

build: Dockerfile start.sh
	-docker rmi $(BASE)/$(IMAGE):$(VER)
	docker build --no-cache -t $(BASE)/$(IMAGE):$(VER) .

horizonstart:
	hzn -v dev service start

horizonstop:
	hzn -v dev service stop

run:
	docker run --name $(IMAGE) -e MY_VAR1='outside' -d -t $(BASE)/$(IMAGE):$(VER)

check:
	docker logs $(IMAGE)

stop:
	-docker stop $(IMAGE)
	-docker rm $(IMAGE)

dockerclean:
	-docker rmi $(BASE)/$(IMAGE):$(VER)

realclean: clean dockerclean
