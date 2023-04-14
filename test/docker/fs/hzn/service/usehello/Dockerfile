FROM alpine:latest

LABEL generated_by="e2edev"
RUN apk --no-cache --update add curl jq

RUN adduser --disabled-password "e2edevuser"

COPY start.sh /tmp/

RUN alias dir='ls -la'

RUN mkdir /e2edevuser \
    && chown -R e2edevuser /tmp /e2edevuser

USER e2edevuser

WORKDIR /tmp
CMD ["/tmp/start.sh"]
