FROM multiarch/alpine:arm64-v3.6

RUN apk --no-cache add libcrypto1.0 libssl1.0 ca-certificates 

ADD cloud-sync-service /cloud-sync-service/

CMD ["/cloud-sync-service/cloud-sync-service"]

