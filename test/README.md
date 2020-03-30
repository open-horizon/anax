# End to end Horizon developer test

## Overview

The e2e project is used by Horizon developers to test an X86 device and X86 agbot together, thus the name End to End test.

The project will create 3 containers:
- exchange-db
  - A postgres container for the exchange-api
- exchange-api
  - Pulled from openhorizon/amd64_exchange-api:latest
- agbot
  - Where anax runs, built initially from openhorizon/anax source, uses local copy of source afterwards unless told otherwise
And depending on which PATTERN is chosen, a series of workload containers

## Building and running

### One time steps to setup your environment
- Install docker
  - `curl https://get.docker.com/ | sh`
- Install make and jq
  - `apt update && apt install -y make jq build-essential`
- Install golang version 1.14.* ...
  - `curl https://dl.google.com/go/go1.14.linux-amd64.tar.gz | tar -xzf- -C /usr/local/`
  - `export PATH=$PATH:/usr/local/go/bin` (and modify your ~/.bashrc file with the same)
- GOPATH cannot be set to the same path as GOROOT
  - `export GOPATH=</your/go/path>`
  - `export ANAX_SOURCE=</path/to/anax>`
- Set up a single node k8s for testing, follow the instructions here:
  - https://microk8s.io/docs/
  - make sure you install from the 1.14 channel: `sudo snap install microk8s --classic --channel=1.14/stable`
  - see also https://microk8s.io/docs/setting-snap-channel when deciding when to upgrade to a newer Kubernetes
- Clone this repo  
  - `git clone git@github.com:open-horizon/anax.git`
- Build the anax binary, the agbot base image, and pull the exchange images
  - `cd anax && make`

### Development Iterations - Basic
This is the most comprehensive 'Basic' test.

It creates the agbot, exchange-db, and exchange-api containers and copies all anax/exchange configs and binaries at that point.

It will make agreements and run all workloads, verifying agreements are made, survive, are successfully cancelled, and are remade.

It runs `make clean` to start which gives a fresh environment each time
- `make test`


### Development Iterations - Advanced
There are several env vars that you can specify on the make run-combined command to condition what happens in the e2edev environment.

A common way to run the environment during development is `make test TEST_VARS="NOLOOP=1 PATTERN=sloc"`

Light Test:
`make test TEST_VARS="NOLOOP=1 NOCANCEL=1 NOHZNREG=1 NORETRY=1 NOSVC_CONFIGSTATE=1 NOSURFERR=1 NOPATTERNCHANGE=1 NOCOMPCHECK=1"`

Here is a full description of all the variables you can use to setup the test the way you want it:
- NOLOOP=1 - turns off the loop that cancels agreements on the device and agbot (alternating), every 10 mins. Usually you want to specify NOLOOP=1 when actively iterating code.
- NOCANCEL=1 - when set with NOLOOP=1, skips the single round of cancellation tests for less log clutter and time when just interested in agreement formation.
- UNCONFIG=1 - turns on the unconfig/reconfig loop tests.
- PATTERN=name - specify the name of a configured pattern that you want the device to use. Builtin patterns are spws, sns, sloc, sgps, sall, cpu2msghub etc. If you specify PATTERN, but turn off one of the dependent services that the top service needs, the system will not work correctly. If you dont specify a PATTERN, the manually managed policy files will be used to run the workloads (unless you turn them off).
- NOHZNREG=1 - turns off the tests for registering/unregistering nodes with `hzn` commands.
- NORETRY=1 - turns off the service retry test.
- NOSVC_CONFIGSTATE=1 - turns off the service config state test.
- NOSURFERR=1 - turns off the node surface error test.
- NOPATTERNCHANGE=1 - turns off the node pattern change test.
- NOCOMPCHECK=1 - turns off the policy compatibility test.
- NONS=1 - dont register the netspeed service.
- NOGPS=1 - dont register the gpstest service.
- NOLOC=1 - dont register the location service.
- NOPWS=1 - dont register the weather service. This is a good workload to run when iterating code because it is simple and reliable, it wont get in your way.
- NOK8S=1 - dont register the k8s-service1.
- NOANAX=1 - anax is started for API tests but is then stopped and is NOT restarted to run workloads.
- NOAGBOT=1 - the agbot is never started.
- HA=1 - register 2 devices (and the workload services) as an HA pair. You will get 2 anax device processes in the container.
- OLD_ANAX=1 - run the anax device based on the current commit in github, i.e. the device before you made your changes. This is helpfiul for compatibility testing of new agbot with previous device.
- OLD_AGBOT=1 - run the agbot based on the current commit in github, i.e. the agbot before you made your changes. This is helpfiul for compatibility testing of new device with previous agbot.

### Debugging
- `docker exec -it agbot /bin/bash`
- All log files are in the container at /tmp; /tmp/anax.log for the device and /tmp/agbot.log for the agbot
- Important data files and scripts are in /root/ and /root/.colonus and /root/eth
- Config files are in /etc
- From the outside the container, on your development machine you can do the following
- `curl http://localhost/agreement | jq -r '.agreements'`
  - Will show you all current and archived agreements from the device's perspective
- `curl http://localhost:81/agreement | jq -r '.agreements'`
  - Will show you all current and archived agreements from the agbot's perspective
- Access the exchange API documentation at http://localhost:8080/v1

### Clean options/developer flow
- `make clean`
 - Removes workloads, the agbot/exchange-api/db containers, all data from the exchange, and all stale configs/scripts ... runs automatically on `make test`
- `make mostlyclean`
 - Does everything in `make clean` and also removes the anax binaries (for making anax changes)
- `make realclean`
 - Does all the above, plus removes the agbot and exchange base images, our docker test network, and all dangling docker images
 - NOTE: This is the only 'clean' command which requires re-running `make`

### Remote environment testing
-`export DOCKER_EXCH="Exchange's URL"`
-`export CSS_URL="CSS's URL"`
-`export EXCH_ROOTPW="Exchange Root PW"`
-`export AGBOT_NAME="Agbot Name"`
-`export API_KEY="Main Org API Key"`
-`export AGBOT_SAPI_URL="Agbot Secure API URL"`
-`export ICP_HOST_IP="IP address of ICP Host"` If required to be added to hosts file only!
-`export CERT_LOC=1` 1 for if cert is used. 0 if cert is not being used.
-put css.crt file in test directory if using cert with ICP or DEV
-put agbotapi.crt file in test directory
-`cd /anax`
-`make`
-`cd /test`
-`make build-remote`
-`make test-remote`
-`make stop` - used between runs
