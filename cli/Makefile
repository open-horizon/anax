SHELL = /bin/bash -e
BINARY = hzn

default: run

# Build hzn
#TODO: where does the go get really belong?
$(BINARY): $(BINARY).go */*.go
	go get gopkg.in/alecthomas/kingpin.v2
	go build $<

run: $(BINARY)
	./$?

# Am i doing this right?
lint:
	go get -u github.com/golang/lint/golint
	golint

#TODO: add automated tests

clean:
	go clean

.PHONY: default run clean
