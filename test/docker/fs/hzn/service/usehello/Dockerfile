FROM alpine:latest

LABEL generated_by="e2edev"
RUN apk --no-cache --update add curl jq shadow

RUN adduser --disabled-password "e2edevuser"

COPY start.sh /tmp/

RUN alias dir='ls -la'

RUN mkdir /e2edevuser \
    && chown -R e2edevuser /tmp /e2edevuser

USER e2edevuser

ENV UNIX_SOCKET_FILE_PERMISSIONS="0777"

WORKDIR /tmp
CMD ["/tmp/start.sh"]
