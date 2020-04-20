#!/bin/bash

# Script to start agbot from inside the container

if [ -z "$ANAX_LOG_LEVEL" ]; then
	ANAX_LOG_LEVEL=3
fi

/usr/bin/envsubst < /etc/horizon/anax.json.tmpl > /etc/horizon/anax.json
/usr/horizon/bin/anax -v $ANAX_LOG_LEVEL -logtostderr -config /etc/horizon/anax.json

