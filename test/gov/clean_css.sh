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
hzn mms object delete -t "test" -i "test-medium1"
hzn mms object delete -t "test" -i "test-large1"
hzn mms object delete -t "test" -i "test1"
hzn mms object delete -t "test" -i "test2"
hzn mms object delete -t "test" -i "test3"
hzn mms object delete -t "test" -i "test_user_access"
sleep 5
hzn mms object delete -t "test" -i "test-medium1"
hzn mms object delete -t "test" -i "test-large1"
hzn mms object delete -t "test" -i "test1"
hzn mms object delete -t "test" -i "test2"
hzn mms object delete -t "test" -i "test3"
hzn mms object delete -t "test" -i "test_user_access"

unset HZN_FSS_CSSURL
