#! /bin/bash

usage() {
    exitCode=${1:-0}
    cat << EndOfMessage
Usage: ${0##*/} [-n <image name>] [-p <image name in prodcution env>] [-l <docker image latest tag>]

Push the images to repository

 Flags:
  -n    Docker image name
  -p    Docker image name for the production env
  -l    Docker image latest tag
  -h    Usage
EndOfMessage
    exit $exitCode
}

#Print the usage
if [ $# -eq 0 ] || [ $# -eq 1 -a "A$1" = "A-h" ]; then
        usage
fi

while getopts 'n:p:l:h:' OPT; do
    case $OPT in
        n)
            IMAGE="$OPTARG";;
        p)
            IMAGE_PROD="$OPTARG";;
        l)
            IMAGE_LATEST="$OPTARG";;
        h)
            usage;;
        ?)
            echo "Usage: `basename $0` [options]"
    esac
done

shift $(($OPTIND - 1))

# Check the required vars if is defined
if [ "A${IMAGE}" == "A" ];then
        echo "Please define the image name."
fi
if [ "A${IMAGE_PROD}" == "A" ];then
        echo "Please define image name in the production env."
fi
if [ "A${IMAGE_LATEST}" == "A" ];then
        echo "Please define docker image latest tag."
fi

# push the images to repository
docker tag ${IMAGE} ${IMAGE_PROD}
docker push ${IMAGE_PROD}
docker tag ${IMAGE} ${IMAGE_LATEST}
docker push ${IMAGE_LATEST}
