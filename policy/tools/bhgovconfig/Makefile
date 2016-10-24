SHELL = /bin/bash -e

ARCH = $(shell ../arch-tag)
DOCKER_REGISTRY ?= summit.hovitos.engineering
DOCKER_TAG ?= bld
DOCKER_OPTS ?= --no-cache

IMAGE_STRING = $(DOCKER_REGISTRY)/$(ARCH)/policy-bld

# These are targets you can use while working on the bhgovconfig tool
default: install
	go test

install:
	go install

clean:
	rm -f bhgovconfig 2> /dev/null || :

# These are targets used to make an official build of the bhgovconfig tool through
# the use of a build container. It works the same way on x86 as it does on arm.
# A docker file for the build container is generated from a template and customized for your
# environment and then the container is started. The container has the source of this project
# mounted to its file system, so the build will occur based on whatever source is in the
# git sandbox that you're working in.

realclean: clean-image clean
	rm -f Dockerfile-bld 2> /dev/null || :
	rm -rf ./docker-bld 2> /dev/null || :
	rm -f docker-build-container

clean-image: -rem-docker-bld
	- docker rmi $(IMAGE_STRING):$(DOCKER_TAG)

-rem-docker-bld:
	- docker stop policy-bld
	- docker rm -f policy-bld

docker: docker-bld docker-compile

docker-bld: Dockerfile-bld
	mkdir -p ./docker-bld; \
		rsync -avz ./docker/* ./docker-bld/; \
		rsync -avz ${CURDIR}/docker/* ./docker-bld/; \
		docker build -t $(IMAGE_STRING):$(DOCKER_TAG) $(DOCKER_OPTS) -f Dockerfile-bld .
	docker run --name policy-bld -d -v ${GOPATH}/src/repo.hovitos.engineering/MTN/go-policy:/go/src/repo.hovitos.engineering/MTN/go-policy -t $(IMAGE_STRING):$(DOCKER_TAG) /bin/bash
	touch docker-build-container

Dockerfile-bld:
	../Dockerfile-render $(ARCH) "Dockerfile-bld.tmpl"

docker-compile: docker-build-container
	docker exec -t policy-bld /bin/bash -c "cd /go/src/repo.hovitos.engineering/MTN/go-policy/tools/bhgovconfig/ && make docker-container-build"

docker-upload: docker-build-container
	docker exec -t policy-bld /bin/bash -c "cd /tmp/src/repo.hovitos.engineering/MTN/go-policy/tools/bhgovconfig/ && make docker-container-upload"

# The following targets only run inside the build container.
#
# This target actually builds the bhgovconfig tool for whatever hardward platform you're running on
docker-container-build:
	rm -fr /tmp/src
	cp -r /go/src /tmp
	cd /tmp/src/repo.hovitos.engineering/MTN/go-policy/tools/bhgovconfig && $(MAKE) clean && GOPATH=/tmp govendor sync
ifeq ($(ARCH),armhf)
	cd /tmp/src/repo.hovitos.engineering/MTN/go-policy/tools/bhgovconfig && GOPATH=/tmp CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -a
else
	cd /tmp/src/repo.hovitos.engineering/MTN/go-policy/tools/bhgovconfig && GOPATH=/tmp CGO_ENABLED=0 GOOS=linux GO15VENDOREXPERIMENT=1 go build -a
endif

# This target will upload the bhgovconfig executable to objectstore. It assumes that your softlayer user name and
# API key have been set in the SL_USERNAME and SL_API_KEY environment variables. These env vars are set into the
# build container by the build container's docker file.
docker-container-upload: bhgovconfig
	export
ifeq ($(ARCH),armhf)
	cd /tmp/src/repo.hovitos.engineering/MTN/go-policy/tools/bhgovconfig && mv bhgovconfig bhgovconfig-$(ARCH)-$(shell date +%s)
else
	cd /tmp/src/repo.hovitos.engineering/MTN/go-policy/tools/bhgovconfig && mv bhgovconfig bhgovconfig-amd64-$(shell date +%s)
endif
	source /tmp/venv/bin/activate && cd /tmp/src/repo.hovitos.engineering/MTN/go-policy/tools/bhgovconfig && swiftup govconfig bhgovconfig-* && deactivate

.PHONY: clean outdir
