FROM ubuntu:latest

RUN apt-get update
RUN apt-get -y install curl

COPY start.sh /root/

RUN alias dir='ls -la'

WORKDIR /tmp
CMD ["/root/start.sh"]
