SHELL = /bin/bash -e

ARCH = $(shell ./tools/arch-tag)
DOCKER_REGISTRY ?= summit.hovitos.engineering
DOCKER_TAG ?= v2.0.0
DOCKER_OPTS ?= --no-cache
COMPILE_CLEAN ?= clean

image-string = $(DOCKER_REGISTRY)/$(ARCH)/agreementbot

clean: -rem-docker-bld clean-image
	rm -f Dockerfile-{bld,exec}
	rm -rf ./docker-{bld,exec}
	docker rmi $(call image-string):bld 2> /dev/null || :
	cd ../.. && $(MAKE) clean

clean-image:
	docker rmi $(call image-string):{$(DOCKER_TAG),latest} 2> /dev/null || :

-rem-docker-bld:
	- docker stop agbot_bld; docker rm -f agbot_bld 2> /dev/null || :

prep-sd-image: clean docker-bld docker-exec

docker: docker-bld docker-exec

docker-bld: Dockerfile-bld -rem-docker-bld
	 mkdir -p ./docker-bld; \
		rsync -avz ./docker/* ./docker-bld/; \
		rsync -avz ${CURDIR}/docker/* ./docker-bld/; \
		docker build -t $(call image-string):bld $(DOCKER_OPTS) -f Dockerfile-bld .
	docker run --name agbot_bld -d -v ${GOPATH}/src/github.com/open-horizon/anax:/go/src/github.com/open-horizon/anax -t $(call image-string):bld /bin/bash

# TODO: find a way to re-fetch deps in this step w/out killing the local copy changes to agbot source; also find a way to include local source for other libs in the container
docker-compile:
	rm -rf ./docker-exec
	docker exec -t agbot_bld /bin/bash -c "cd /go/src/github.com/open-horizon/anax/ && export DESTDIR="/bld" && $(MAKE) $(COMPILE_CLEAN) installagbot"
	tools/extract-artifacts agbot_bld

Dockerfile-bld:
	 tools/Dockerfile-render $(ARCH) "Dockerfile-bld.tmpl"

docker-exec: docker-compile Dockerfile-exec
		mkdir -p ./docker-exec; rsync -avz ./docker/* ./docker-exec/; \
		docker build -t $(call image-string):$(DOCKER_TAG) $(DOCKER_OPTS) -f Dockerfile-exec .
		docker tag $(call image-string):$(DOCKER_TAG) $(call image-string):latest

Dockerfile-exec:
	    tools/Dockerfile-render $(ARCH) "Dockerfile-exec.tmpl"

docker-push: docker
	docker push $(call image-string):$(DOCKER_TAG)
	docker push $(call image-string):latest

repo-fork-sync:
	tools/repo-fork-sync

.PHONY: clean repo-fork-sync
