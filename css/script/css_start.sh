#!/bin/bash

/usr/bin/envsubst < /etc/edge-sync-service/sync.conf.tmpl > /etc/edge-sync-service/sync.conf
export UNIX_SOCKET_FILE_PERMISSIONS ?= 0o777
/home/cssuser/cloud-sync-service
