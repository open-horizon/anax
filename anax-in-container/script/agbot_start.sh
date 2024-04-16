#!/bin/bash

# Script to start agbot from inside the container

if [ -z "${ANAX_LOG_LEVEL}" ]; then
	ANAX_LOG_LEVEL=3
fi

/usr/bin/envsubst < /etc/horizon/anax.json.tmpl > /etc/horizon/anax.json
/usr/horizon/bin/anax -v "${ANAX_LOG_LEVEL}" -logtostderr -config /etc/horizon/anax.json &
ANAX_PID=$!

# If we receive SIGTERM, forward it to anax to start graceful termination
send_sigterm() {
        kill "${ANAX_PID}" 2>/dev/null
}
trap send_sigterm TERM

# Wait for anax termination
wait "${ANAX_PID}"
