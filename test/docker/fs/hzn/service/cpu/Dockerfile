FROM alpine:latest
LABEL generated_by="e2edev"
RUN apk --no-cache --update add gawk bc socat curl
COPY *.sh /
WORKDIR /
CMD /start.sh
