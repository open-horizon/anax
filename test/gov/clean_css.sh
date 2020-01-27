#!/bin/bash

echo "Cleaning up CSS"

export HZN_FSS_CSSURL=${CSS_URL}

hzn mms object delete -t "model" -i "basicres.tgz"
hzn mms object delete -t "model" -i "multires.tgz"
hzn mms object delete -t "model" -i "policy-basicres.tgz"
hzn mms object delete -t "model" -i "policy-multires.tgz"
sleep 5
hzn mms object delete -t "model" -i "basicres.tgz"
hzn mms object delete -t "model" -i "multires.tgz"
hzn mms object delete -t "model" -i "policy-basicres.tgz"
hzn mms object delete -t "model" -i "policy-multires.tgz"
sleep 5

unset HZN_FSS_CSSURL
