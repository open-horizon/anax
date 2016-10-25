# makefile for governor clients

default: all

all: clients

clients: build_bhgovconfig

build_bhgovconfig: bhgovconfig/bhgovconfig.go
	$(MAKE) -C bhgovconfig docker

clean:
	$(MAKE) -C bhgovconfig realclean

upload: bhgovconfig/docker-build-container
	$(MAKE) -C bhgovconfig docker-upload
