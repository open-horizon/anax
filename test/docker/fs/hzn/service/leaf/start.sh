#!/bin/sh

socat TCP4-LISTEN:8347,fork EXEC:./service.sh

