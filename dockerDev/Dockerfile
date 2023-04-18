FROM golang:1.20.1

ADD ./start.sh /start.sh

RUN apt-get update && apt-get upgrade -y && \
	apt-get install openssh-server nano -y && \
	systemctl enable ssh && \
	chmod +x /start.sh


# Should work in theory for vscode as well
ENV SSH_USERNAME={SSH_USERNAME:-jetbrains}
ENV SSH_PASSWORD={SSH_PASSWORD:-jetbrains}
EXPOSE 22

# pull from main repo, or choice of fork depending on config
# ToDo: later
RUN mkdir /opt/anax && cd /opt/anax && git clone https://github.com/open-horizon/anax.git .
WORKDIR /opt/anax
RUN go install && go clean

CMD [ "/start.sh" ]