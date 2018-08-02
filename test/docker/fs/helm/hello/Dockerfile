FROM alpine:latest
RUN apk --no-cache --update add gawk bc socat curl
COPY *.sh /
WORKDIR /
CMD /start.sh

