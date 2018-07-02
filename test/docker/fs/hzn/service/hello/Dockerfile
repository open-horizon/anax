FROM alpine:latest

LABEL generated_by="e2edev"
RUN apk --no-cache --update add curl

COPY start.sh /root/
COPY server /usr/local/bin/

RUN alias dir='ls -la'

WORKDIR /tmp
CMD ["/root/start.sh"]
