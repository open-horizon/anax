FROM openhorizon/amd64_anax:2.25.0

#RUN chmod -R 777 /root /var /etc /usr

COPY horizon /etc/default/horizon
COPY anax.config /etc/horizon/anax.json
COPY anax /usr/horion/bin/anax
COPY hzn /usr/horion/bin/hzn
COPY hub.crt /root/hub.crt
RUN chmod 755 /usr/horion/bin/anax
RUN chmod 755 /usr/horion/bin/hzn

ENV HZN_ORG_ID=userdev
ENV HZN_EXCHANGE_USER_AUTH=userdev/userdevadmin:userdevadminpw
ENV HZN_SSL_SKIP_VERIFY=1

ENTRYPOINT ["/root/anax.service", "start"]