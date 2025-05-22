FROM docker.io/multiarch/alpine:arm64-v3.6

RUN apk --no-cache add libcrypto1.0 libssl1.0 ca-certificates 

ADD cloud-sync-service /cloud-sync-service/

CMD ["/cloud-sync-service/cloud-sync-service"]

HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD grep -q css_start.sh /proc/1/cmdline || exit 1
