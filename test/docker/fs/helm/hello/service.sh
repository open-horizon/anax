#!/bin/sh

# Example microservice that returns hello plus configured string

HEADERS="Content-Type: text/html; charset=ISO-8859-1"
BODY="{\"hello\":\"${HELLO_VAR}\"}"
HTTP="HTTP/1.1 200 OK\r\n${HEADERS}\r\n\r\n${BODY}\r\n"

# Emit the HTTP response
echo -en $HTTP

