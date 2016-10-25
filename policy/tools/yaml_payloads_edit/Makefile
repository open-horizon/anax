# payload_edit make

default:
ifeq ($(ARCH),armhf)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -a
else
	CGO_ENABLED=0 GOOS=linux GO15VENDOREXPERIMENT=1 go build -a
endif

install:
	go install

clean:
	rm -f yaml_payloads_edit

test: install
	go test

