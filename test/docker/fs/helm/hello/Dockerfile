FROM alpine:latest
RUN apk --no-cache --update add gawk bc socat curl
COPY *.sh /
WORKDIR /

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:${HELLO_PORT:-8080}/ || exit 1

CMD /start.sh

