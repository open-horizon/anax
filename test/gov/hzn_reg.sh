#!/bin/bash

echo -e "Registering microservice and workload with hzn dev"

E2EDEV_ADMIN_AUTH="e2edev/e2edevadmin:e2edevadminpw"
export HZN_EXCHANGE_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

KEY_TEST_DIR="/tmp/keytest"
mkdir -p $KEY_TEST_DIR

cd $KEY_TEST_DIR && rm -f *.pem *.key

echo -e "Generate signing keys:"
hzn key create -l 4096 e2edev e2edev@gmail.com
if [ $? -ne 0 ]
then
    echo -e "hzn key create failed."
    exit 1
fi

echo -e "Copy public key into anax folder:"
cp $KEY_TEST_DIR/*public.pem /root/.colonus/.

echo -e "Define microservice using hzn dev:"


cd /root/hzn/helloms
hzn dev microservice publish -vI -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn dev microservice publish failed."
    exit 1
fi

echo -e "Listing microservices:"
hzn exchange microservice list -o e2edev

echo -e "Define workload using hzn dev:"
cd /root/hzn/usehello
hzn dev workload publish -vI -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn dev workload publish failed."
    exit 1
fi

echo -e "Listing workloads:"
hzn exchange workload list -o e2edev

unset HZN_EXCHANGE_URL

echo -e "Success registering microservice and workload with hzn dev"
