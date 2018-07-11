FROM ubuntu:latest

LABEL generated_by="e2edev"
RUN apt-get update
RUN apt-get -y install curl

COPY start.sh /root/
COPY server /usr/local/bin/

RUN alias dir='ls -la'

WORKDIR /tmp
CMD ["/root/start.sh"]
