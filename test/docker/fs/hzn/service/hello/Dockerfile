FROM alpine:latest

LABEL generated_by="e2edev"
RUN apk --no-cache --update add curl

COPY start.sh /root/
COPY server /usr/local/bin/

RUN alias dir='ls -la'

WORKDIR /tmp

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8000/ || exit 1

CMD [ "/root/start.sh" ]
