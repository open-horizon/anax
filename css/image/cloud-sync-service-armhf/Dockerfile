FROM docker.io/multiarch/alpine:armhf-v3.6

RUN apk --no-cache add libcrypto1.0 libssl1.0 ca-certificates \
    && adduser -D -H -u 10001 appuser \
    && mkdir -p /edge-sync-service

ADD edge-sync-service /edge-sync-service/

RUN chown -R appuser:appuser /edge-sync-service

USER appuser

CMD ["/cloud-sync-service/cloud-sync-service"]

