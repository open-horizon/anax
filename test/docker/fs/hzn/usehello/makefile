# Makefile for workload2 container

VER = v1.0
BASE = openhorizon
IMAGE = amd64_usehello

default: $(IMAGE)-$(VER).tar

$(IMAGE)-$(VER).tar: Dockerfile start.sh
	-docker rmi $(BASE)/$(IMAGE):$(VER)
	docker build --no-cache -t $(BASE)/$(IMAGE):$(VER) .
	touch $(IMAGE)-$(VER).tar

horizonstart: $(IMAGE)-$(VER).tar
	hzn -v dev workload start

horizonstop: $(IMAGE)-$(VER).tar
	hzn -v dev workload stop

run: $(IMAGE)-$(VER).tar
	docker run --name $(IMAGE) -e MY_VAR1='outside' -d -t $(BASE)/$(IMAGE):$(VER)

stop:
	-docker stop $(IMAGE)
	-docker rm $(IMAGE)

clean:
	rm -rf $(IMAGE)-$(VER).tar

dockerclean:
	-docker rmi $(BASE)/$(IMAGE):$(VER)

realclean: clean dockerclean
