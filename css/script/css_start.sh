#!/bin/bash

/usr/bin/envsubst < /etc/edge-sync-service/sync.conf.tmpl > /etc/edge-sync-service/sync.conf
/home/cssuser/cloud-sync-service &
CSS_PID=$!

# If we receive SIGTERM, forward it to css to start graceful termination
send_sigterm() {
        kill "${CSS_PID}" 2>/dev/null
}
trap send_sigterm TERM

# Wait for css termination
wait "${CSS_PID}"
