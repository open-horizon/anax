FROM alpine:3.6

RUN addgroup -g 1000 -S cssuser && adduser -u 1000 -S cssuser -G cssuser \
        && apk --no-cache add libcrypto1.0 libssl1.0 ca-certificates

ADD cloud-sync-service /home/cssuser/cloud-sync-service

RUN mkdir /var/edge-sync-service && chown -R cssuser:cssuser /var/edge-sync-service

USER cssuser

CMD ["/home/cssuser/cloud-sync-service"]