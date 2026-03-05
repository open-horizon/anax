#!/bin/sh

# Example microservice that returns hello plus configured string

HEADERS="Content-Type: text/html; charset=ISO-8859-1"
BODY="{\"hello\":\"${HELLO_VAR}\"}"

# Emit the HTTP response
printf 'HTTP/1.1 200 OK\r\n%s\r\n\r\n%s\r\n' "${HEADERS}" "${BODY}"

