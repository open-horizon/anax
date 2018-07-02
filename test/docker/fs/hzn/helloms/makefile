# Makefile for microservice container

VER = v1.0
BASE = openhorizon
IMAGE = amd64_hellomicroservice

default: $(IMAGE)-$(VER).tar

$(IMAGE)-$(VER).tar: Dockerfile start.sh server
	-docker rmi $(BASE)/$(IMAGE):$(VER)
	docker build --no-cache -t $(BASE)/$(IMAGE):$(VER) .
	touch $(IMAGE)-$(VER).tar

server: server.go
	go build server.go

horizonstart: $(IMAGE)-$(VER).tar
	hzn -v dev microservice start

horizonstop: $(IMAGE)-$(VER).tar
	hzn -v dev microservice stop

run: $(IMAGE)-$(VER).tar
	docker run --name $(IMAGE) -e MY_MS_VAR1='outside' -p 8000:8000 -d -t $(BASE)/$(IMAGE):$(VER)

stop:
	-docker stop $(IMAGE)
	-docker rm $(IMAGE)

clean:
	rm -f server
	rm -rf $(IMAGE)-$(VER).tar

dockerclean:
	-docker rmi $(BASE)/$(IMAGE):$(VER)

realclean: clean dockerclean
