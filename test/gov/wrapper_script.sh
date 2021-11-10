#!/bin/bash
usage() { echo "Usage: e2edev [-v <exchange-version> ] [-p </path/to/anax/root>]" 1>&2; exit 1; }


while getopts ":hv:p:" o; do
	case "$o" in
		v)
			if [[ -z "${OPTARG}" ]]
			then
				usage
                exit
			else
				EXCHANGE_VERSION=$OPTARG
			fi
			;;
		p)
			if [ -z "${OPTARG}" ]
			then
                echo "Please set a valid path : eg. '$HOME/open-horizon/anax' "
				usage
			else
				ANAX_PATH=$OPTARG
			fi
			;;
		h)
			usage
			;;
		*)
			usage
			;;
	esac
done

# shift $((OPTIND-1))

if [ -z "${EXCHANGE_VERSION}" ] || [ -z "${ANAX_PATH}" ]; then
	usage
fi

echo "EXCHANGE_VERSION = ${EXCHANGE_VERSION}"
echo "ANAX_PATH = ${ANAX_PATH}"

echo "Testing exchange-api $EXCHANGE_VERSION in e2edev..."
docker pull openhorizon/amd64_exchange-api:$EXCHANGE_VERSION

echo "Stopping horizon.service..."
sudo systemctl stop horizon.service   # this is only needed if you normally use this machine as a horizon edge node

set -e
echo cd $ANAX_PATH
cd $ANAX_PATH

echo "Cleaning up from previous e2edev run..."
make -C test realclean

echo "Pulling latest anax repo updates..."
git pull

echo "Cleaning and then building executables and docker images..."
make clean
make

echo "Running tests..."
cd test

make DOCKER_EXCH_TAG=$EXCHANGE_VERSION

echo "make test TEST_VARS='NOLOOP=1 NOUPGRADE=1 NORETRY=1 NOK8S=1 NOKUBE=1 TEST_PATTERNS=sall' DOCKER_EXCH_TAG=$EXCHANGE_VERSION"
make test TEST_VARS="NOLOOP=1 NOUPGRADE=1 NORETRY=1 NOK8S=1 NOKUBE=1 TEST_PATTERNS=sall" DOCKER_EXCH_TAG=$EXCHANGE_VERSION
make stop

echo "make test TEST_VARS='NOLOOP=1 NOUPGRADE=1 NORETRY=1 NOK8S=1 NOKUBE=1' DOCKER_EXCH_TAG=$EXCHANGE_VERSION"
make test TEST_VARS="NOLOOP=1 NOUPGRADE=1 NORETRY=1 NOK8S=1 NOKUBE=1" DOCKER_EXCH_TAG=$EXCHANGE_VERSION
set +e

# need to do this even if the test fails, if we want to get back to using it as a node
echo 'Now run: cd ${ANAX_PATH}/test && make realclean && cd -'
cd ${ANAX_PATH}/test && make realclean && cd -
