#!/bin/bash

PREFIX="hzn sdo test:"

echo ""
echo -e "${PREFIX} Inspecting and importing vouchers with hzn command."

# preparing the voucher file
cat <<'EOF' > /tmp/sdo_voucher.json
{"sz":1,"oh":{"pv":113,"pe":1,"r":[1,[4,{"dn":"bruce-dev1.fyre.com","po":8040,"pow":8040,"pr":"http"}]],"g":"Vmkm97cJRpSHmLfbyMyyCw==","d":"SDO Java Device","pk":[13,1,[91,"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE1LBVR8bIcRAPcyGHtGBzfk0sJRAQ3pcwBaZo1EDDEHh6NvzmK8BC8DgJfYJj0xL8F3murSIlwK2CHEdOD3TzOA=="]],"hdc":[32,8,"MlpaJw6JrVY+esM8EHfgAKX9c1T8NWZktGb1tuW4CpY="]},"hmac":[32,108,"R72z5AGp30nSCMJN3K9cup2dJ9Uvx6rvIZ/Ydc+h5ts="],"dc":[1,2,[[315,"MIIBNzCB3aADAgECAgYBcYvR8p8wCgYIKoZIzj0EAwIwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMDA0MTgwNTQ1MjRaFw0yMDA1MTgwNTQ1MjRaMAAwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAR1o6FQfSCGPjJa6UnnnZwrk2wurYX/7pqyrzjqomddjHkIZIB4hKfKP4DNOBu7Y7OPKiFXTsldlz/0xhEvGf6zMAoGCCqGSM49BAMCA0kAMEYCIQD/MYOBodEL5F4zNewh5vdAEWvVlItLSKLqf35rSSoR/gIhAONzv20dG6to5knATHSaCGMIf08btwbk6w9r0RXs1tYP"],[483,"MIIB3zCCAYWgAwIBAgIUGe+4vTFELPtaQ9zTT2E7tgpb+J4wCgYIKoZIzj0EAwIwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0xOTEyMjAxOTU2MDNaFw0yMDEyMTkxOTU2MDNaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATUsFVHxshxEA9zIYe0YHN+TSwlEBDelzAFpmjUQMMQeHo2/OYrwELwOAl9gmPTEvwXea6tIiXArYIcR04PdPM4o1MwUTAdBgNVHQ4EFgQUJcffU8efi7VUyZ3NHZ7MXAIkbYYwHwYDVR0jBBgwFoAUJcffU8efi7VUyZ3NHZ7MXAIkbYYwDwYDVR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNIADBFAiEAjfCAWcXdHtLp837WftNVZhSvb1mrNTMNLGcopa+4zt0CIHdusGmDHTPcIk/lq1m91nKE4XCGUI7wD4V3zk5Kpb0r"]]],"en":[{"bo":{"hp":[32,8,"gzGJRwRWVT0IiK3xGaTo68LwK3juTdeEzzS+Tfsf7dg="],"hc":[32,8,"jwqcp/laj60ScXyQ843Czz2ilbYfLw3gfnoRFBWHNn8="],"pk":[13,1,[91,"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEWVUE2G0GLy8scmAOyQyhcBiF/fSUd3i/Og7XDShiJb2IsbCZSRqt1ek15IbeCI5z7BHea2GZGgaK63cyD15gNA=="]]},"pk":[0,0,[0]],"sg":[71,"MEUCICKiSEc05cHU3nQcr+lhN+gpCWUkjJlk/iexds/1TpqpAiEAnD9WaJo8P1WhSBZHvzFiyk0A0DneW6NQqm+bLhnjezQ="]}]}
EOF

# inspect output
read -r -d '' inspectOutput <<'EOF'
{
  "device": {
    "uuid": "566926f7-b709-4694-8798-b7dbc8ccb20b",
    "deviceType": "SDO Java Device"
  },
  "voucher": {
    "rendezvousUrls": [
      "http://bruce-dev1.fyre.com:8040"
    ]
  }
}
EOF

cat <<'EOF' > /tmp/sdo_voucher-ip.json
{"sz":1,"oh":{"pv":113,"pe":1,"r":[1,[4,{"ip":[4,"qS0yUg=="],"po":8040,"pow":8040,"pr":"http"}]],"g":"LuDNty+1QouPaf5Jd/aSQQ==","d":"SDO Java Device","pk":[13,1,[91,"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE1LBVR8bIcRAPcyGHtGBzfk0sJRAQ3pcwBaZo1EDDEHh6NvzmK8BC8DgJfYJj0xL8F3murSIlwK2CHEdOD3TzOA=="]],"hdc":[32,8,"sEQYXPYYSNh64ArkJyZJbcB2cHN93IExWyJJrbzl0cU="]},"hmac":[32,108,"94qpsFRrpgNrzXbQ0eCxF5SVaCekQeV6+l9cAnHlq8Q="],"dc":[1,2,[[313,"MIIBNTCB3aADAgECAgYBcffDJj8wCgYIKoZIzj0EAwIwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMDA1MDkwNDQ4MTNaFw0yMDA2MDgwNDQ4MTNaMAAwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAR1o6FQfSCGPjJa6UnnnZwrk2wurYX/7pqyrzjqomddjHkIZIB4hKfKP4DNOBu7Y7OPKiFXTsldlz/0xhEvGf6zMAoGCCqGSM49BAMCA0cAMEQCIHtTmstBEYxbJhQ5D7HxLebfdPH4AxJn3zcKOlDC7MXkAiAeMNCh7z22+CuS8Lyw0G2fYiJkXVVecNWkDfW09CZlSw=="],[483,"MIIB3zCCAYWgAwIBAgIUGe+4vTFELPtaQ9zTT2E7tgpb+J4wCgYIKoZIzj0EAwIwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0xOTEyMjAxOTU2MDNaFw0yMDEyMTkxOTU2MDNaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATUsFVHxshxEA9zIYe0YHN+TSwlEBDelzAFpmjUQMMQeHo2/OYrwELwOAl9gmPTEvwXea6tIiXArYIcR04PdPM4o1MwUTAdBgNVHQ4EFgQUJcffU8efi7VUyZ3NHZ7MXAIkbYYwHwYDVR0jBBgwFoAUJcffU8efi7VUyZ3NHZ7MXAIkbYYwDwYDVR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNIADBFAiEAjfCAWcXdHtLp837WftNVZhSvb1mrNTMNLGcopa+4zt0CIHdusGmDHTPcIk/lq1m91nKE4XCGUI7wD4V3zk5Kpb0r"]]],"en":[{"bo":{"hp":[32,8,"2mBiOIwl5GQJm0s+EeB8QY1jollIxGXZ4SYJnEcHCgA="],"hc":[32,8,"fD7IJ8Db5CNUmpdfZdEwd1HP1+ZgLO3SzpiOOD/DN9U="],"pk":[13,1,[91,"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEWVUE2G0GLy8scmAOyQyhcBiF/fSUd3i/Og7XDShiJb2IsbCZSRqt1ek15IbeCI5z7BHea2GZGgaK63cyD15gNA=="]]},"pk":[0,0,[0]],"sg":[72,"MEYCIQC762tS3zSaE68oIT4Ogf5MhjrvBRPRF9w2XiVvpH0LeAIhAKwSZLIPjm07oTrpCrCzc45jsQN5fVYa3b5g3dzl4Wh7"]}]}
EOF

read -r -d '' inspectOutputIp <<'EOF'
{
  "device": {
    "uuid": "2ee0cdb7-2fb5-428b-8f69-fe4977f69241",
    "deviceType": "SDO Java Device"
  },
  "voucher": {
    "rendezvousUrls": [
      "http://169.45.50.82:8040"
    ]
  }
}
EOF

cat <<'EOF' > /tmp/sdo_key.json
{
  "key_name": "test-sdo-key",
  "common_name": "test-key",
  "email_name": "user.email@domain.com",
  "company_name": "TestCo",
  "country_name": "XX",
  "state_name": "XX",
  "locale_name": "City"
}
EOF

cat <<'EOF' > /tmp/sdo_key2.json
{
  "key_name": "test-sdo-key2",
  "common_name": "test-key2",
  "email_name": "user.email@domain.com",
  "company_name": "TestCo",
  "country_name": "XX",
  "state_name": "XX",
  "locale_name": "City"
}
EOF

read -r -d '' inspectSampleKey <<'EOF'
{
  "key_name": "",
  "common_name": "",
  "email_name": "",
  "company_name": "",
  "country_name": "",
  "state_name": "",
  "locale_name": ""
}
EOF

read -r -d '' inspectSingleKeyList <<'EOF'
[
  {
    "name": "test-sdo-key",
    "orgid": "e2edev@somecomp.com",
    "owner": "e2edevadmin",
    "filename": "e2edev@somecomp.com_test-sdo-key_public-key.pem",
    "isExpired": false
  }
]
EOF

read -r -d '' inspectDoubleKeyList <<'EOF'
[
  {
    "name": "test-sdo-key2",
    "orgid": "e2edev@somecomp.com",
    "owner": "e2edevadmin",
    "filename": "e2edev@somecomp.com_test-sdo-key2_public-key.pem",
    "isExpired": false
  },
  {
    "name": "test-sdo-key",
    "orgid": "e2edev@somecomp.com",
    "owner": "e2edevadmin",
    "filename": "e2edev@somecomp.com_test-sdo-key_public-key.pem",
    "isExpired": false
  }
]
EOF

# save voucher device-id
VOUCHER_DEVICE_ID="566926f7-b709-4694-8798-b7dbc8ccb20b"
VOUCHER_IP_DEVICE_ID="2ee0cdb7-2fb5-428b-8f69-fe4977f69241"

# save key names
SDO_KEY_NAME="test-sdo-key"

# Test hzn sdo voucher inspect
echo -e "${PREFIX} Testing 'hzn sdo voucher inspect <voucher-file>'"
cmdOutput=$(hzn sdo voucher inspect /tmp/sdo_voucher.json 2>&1)
rc=$?
if [[ $rc -ne 0 ]]; then
	echo -e "${PREFIX} Failed: exit code $rc from 'hzn sdo voucher inspect': $cmdOutput."
	exit 1
elif [[ "$cmdOutput" != "$inspectOutput" ]]; then
	echo -e "${PREFIX} Failed: Wrong output for 'hzn sdo voucher inspect <voucher-file>': $cmdOutput."
	exit 1
fi

# Test hzn sdo voucher inspect with a voucher that has an IP address for the rendezvous svr
echo -e "${PREFIX} Testing 'hzn sdo voucher inspect <voucher-file>' with IP"
cmdOutput=$(hzn sdo voucher inspect /tmp/sdo_voucher-ip.json 2>&1)
rc=$?
if [[ $rc -ne 0 ]]; then
	echo -e "${PREFIX} Failed: exit code $rc from 'hzn sdo voucher inspect' with IP: $cmdOutput."
	exit 1
elif [[ "$cmdOutput" != "$inspectOutputIp" ]]; then
	echo -e "${PREFIX} Failed: Wrong output for 'hzn sdo voucher inspect <voucher-file>' with IP: $cmdOutput."
	exit 1
fi

# Test inspect error cases
echo -e "${PREFIX} Testing 'hzn sdo voucher inspect' with missing arg"
cmdOutput=$(hzn sdo voucher inspect 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'required argument'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher inspect': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo voucher inspect file-not-there'"
cmdOutput=$(hzn sdo voucher inspect file-not-there 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'no such file'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher inspect file-not-there': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo voucher inspect <voucher-file> 2nd-arg'"
cmdOutput=$(hzn sdo voucher inspect /tmp/sdo_voucher.json 2nd-arg 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'unexpected 2nd-arg'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher inspect <voucher-file> 2nd-arg': exit code: $rc, output: $cmdOutput."
	exit 1
fi

# Test hzn sdo voucher import

# USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"
USERDEV_ADMIN_AUTH=$HZN_EXCHANGE_USER_AUTH
# export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"
# NODE_AUTH="userdev/an12345"
# export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"
HZN_ORG_ID_SAVE=$HZN_ORG_ID

# todo: currently only testing cmd syntax error cases. Full tests will be added in https://github.com/open-horizon/anax/issues/1677
# HZN_SDO_SVC_URL_SAVE=foobar
# export HZN_SDO_SVC_URL="https://127.0.0.1:9008/api"
HZN_SDO_SVC_URL_SAVE=$HZN_SDO_SVC_URL

echo -e "${PREFIX} Testing 'hzn sdo voucher import' with missing arg"
cmdOutput=$(hzn sdo voucher import 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'required argument'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher import': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo voucher import file-not-there'"
cmdOutput=$(hzn sdo voucher import file-not-there 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'no such file'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher import file-not-there': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo voucher import <voucher-file> 2nd-arg'"
cmdOutput=$(hzn sdo voucher import /tmp/sdo_voucher.json 2nd-arg 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'unexpected 2nd-arg'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher import <voucher-file> 2nd-arg': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo voucher import /tmp/voucher.badextension'"
touch /tmp/voucher.badextension
cmdOutput=$(hzn sdo voucher import /tmp/voucher.badextension 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'unsupported voucher file type extension'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher import /tmp/voucher.badextension': exit code: $rc, output: $cmdOutput."
	exit 1
fi
rm -f /tmp/voucher.badextension

echo -e "${PREFIX} Testing 'hzn sdo voucher import <voucher-file> --policy policy-not-there.json'"
cmdOutput=$(hzn sdo voucher import /tmp/sdo_voucher.json --policy policy-not-there.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'rror:'*'accessing policy-not-there.json'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher import <voucher-file> --policy policy-not-there.json': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo voucher import <voucher-file> with mutually exclusive -e and --policy'"
touch /tmp/node-policy.json
cmdOutput=$(hzn sdo voucher import /tmp/sdo_voucher.json -e foo --policy /tmp/node-policy.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'rror:'*'mutually exclusive'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher import <voucher-file> -e foo --policy /tmp/node-policy.json': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo voucher import <voucher-file> with mutually exclusive -e and --pattern'"
cmdOutput=$(hzn sdo voucher import /tmp/sdo_voucher.json -e foo --pattern bar 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'rror:'*'mutually exclusive'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher import <voucher-file> -e foo --pattern bar': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo voucher import <voucher-file> with mutually exclusive --pattern and --policy'"
cmdOutput=$(hzn sdo voucher import /tmp/sdo_voucher.json --pattern bar --policy /tmp/node-policy.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'rror:'*'mutually exclusive'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher import <voucher-file> --pattern bar --policy /tmp/node-policy.json': exit code: $rc, output: $cmdOutput."
	exit 1
fi
rm -f /tmp/node-policy.json

echo -e "${PREFIX} Testing 'hzn sdo voucher import <voucher-file>' without HZN_SDO_SVC_URL set"
unset HZN_SDO_SVC_URL
cmdOutput=$(hzn sdo voucher import /tmp/sdo_voucher.json 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Could not get'*'HZN_SDO_SVC_URL'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher import <voucher-file>' without HZN_SDO_SVC_URL set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL_SAVE

echo -e "${PREFIX} Testing 'hzn sdo voucher import <voucher-file>' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$(hzn sdo voucher import /tmp/sdo_voucher.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'organization ID must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher import <voucher-file>' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo voucher import <voucher-file>' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$(hzn sdo voucher import /tmp/sdo_voucher.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'exchange user authentication must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher import <voucher-file>' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"

# Test hzn sdo voucher list
echo -e "${PREFIX} Testing 'hzn sdo voucher list' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$(hzn sdo voucher list 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'exchange user authentication must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher list' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"

echo -e "${PREFIX} Testing 'hzn sdo voucher list' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$(hzn sdo voucher list 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'organization ID must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher list' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo voucher list' without HZN_SDO_SVC_URL set"
unset HZN_SDO_SVC_URL
cmdOutput=$(hzn sdo voucher list 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Could not get'*'HZN_SDO_SVC_URL'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher list' without HZN_SDO_SVC_URL set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL_SAVE

echo -e "${PREFIX} Testing 'hzn sdo voucher list <voucher> 2nd-arg'"
cmdOutput=$(hzn sdo voucher list file-not-there 2nd-arg 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'unexpected 2nd-arg'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher list <voucher> 2nd-arg': exit code: $rc, output: $cmdOutput."
	exit 1
fi

# Test hzn sdo voucher download

echo -e "${PREFIX} adding test key to SDO owner services..."
hzn voucher import /tmp/sdo_voucher.json &> /dev/null
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing 'hzn sdo voucher download' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$(hzn sdo voucher download $VOUCHER_DEVICE_ID 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'exchange user authentication must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher download' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"

echo -e "${PREFIX} Testing 'hzn sdo voucher download' with incorrect HZN_EXCHANGE_USER_AUTH set"
export HZN_EXCHANGE_USER_AUTH=fakeuser:fakepw
cmdOutput=$(hzn sdo voucher download $VOUCHER_DEVICE_ID 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid credentials.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher download' with incorrect HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH=$USERDEV_ADMIN_AUTH

echo -e "${PREFIX} Testing 'hzn sdo voucher download' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$(hzn sdo voucher download $VOUCHER_DEVICE_ID 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'organization ID must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher download' with incorrect HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo voucher download' with incorrect HZN_ORG_ID set"
export HZN_ORG_ID=fakeorg
cmdOutput=$(hzn sdo voucher download $VOUCHER_DEVICE_ID 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid credentials.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher download' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo voucher download' without HZN_SDO_SVC_URL set"
unset HZN_SDO_SVC_URL
cmdOutput=$(hzn sdo voucher download $VOUCHER_DEVICE_ID 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Could not get'*'HZN_SDO_SVC_URL'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher download' without HZN_SDO_SVC_URL set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL_SAVE

echo -e "${PREFIX} Testing 'hzn sdo voucher download <keyName>"
cmdOutput=$(hzn sdo voucher download $VOUCHER_DEVICE_ID 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"$(cat /tmp/sdo_voucher.json)"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher download': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo voucher download <device-id> -f /tmp/test_voucher.json'"
cmdOutput=$(hzn sdo voucher download $VOUCHER_DEVICE_ID -f /tmp/test_voucher.json 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	if [[ -f /tmp/test_voucher.json && "$(cat /tmp/test_voucher.json)" == "$(cat /tmp/sdo_voucher.json)" ]]; then
		echo -e "${PREFIX} completed."
	elif [[ ! -f /tmp/test_voucher.json ]]; then
		echo -e "${PREFIX} Failed: 'hzn sdo voucher download <device-id> -f /tmp/test_voucher.json' not downloaded to /tmp/test_voucher.json"
		exit 1
	else
		echo -e "${PREFIX} Failed: 'hzn sdo voucher download <device-id> -f /tmp/test_voucher.json': downloaded file does not match expected output."
		exit 1
	fi
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher download': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo voucher download <device-id> -f /tmp/test_voucher.json' when /tmp/test_voucher.json already exists"
cmdOutput=$(hzn sdo voucher download $VOUCHER_DEVICE_ID -f /tmp/test_voucher.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'File'*'already exists. Please specify a different file path or file name.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher download' when /tmp/test_voucher.json already exists: exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo voucher download <device-id> -f /tmp/test_voucher.json -O' when /tmp/test_voucher.json already exists"
cmdOutput=$(hzn sdo voucher download $VOUCHER_DEVICE_ID -f /tmp/test_voucher.json -O 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	if [[ -f /tmp/test_voucher.json && "$(cat /tmp/test_voucher.json)" == "$(cat /tmp/sdo_voucher.json)" ]]; then
		echo -e "${PREFIX} completed."
	elif [[ ! -f /tmp/test_voucher.json ]]; then
		echo -e "${PREFIX} Failed: 'hzn sdo voucher download <device-id> -f /tmp/test_voucher.json' not downloaded to /tmp/test_voucher.json"
		exit 1
	else
		echo -e "${PREFIX} Failed: 'hzn sdo voucher download <device-id> -f /tmp/test_voucher.json': downloaded file does not match expected output."
		exit 1
	fi
elif [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'File'*'already exists. Please specify a different file path or file name.'* ]]; then
	echo -e "${PREFIX} Failed: 'hzn sdo voucher download' did not overwrite output file with -O flag set."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher download': exit code: $rc, output: $cmdOutput."
	exit 1
fi
rm /tmp/test_voucher.json

echo -e "${PREFIX} Testing 'hzn sdo voucher download <device-id> -f /tmp'"
cmdOutput=$(hzn sdo voucher download $VOUCHER_DEVICE_ID -f /tmp 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	if [[ -f /tmp/$VOUCHER_DEVICE_ID.json && "$(cat /tmp/$VOUCHER_DEVICE_ID.json)" == "$(cat /tmp/sdo_voucher.json)" ]]; then
		echo -e "${PREFIX} completed."
	elif [[ ! -f /tmp/$VOUCHER_DEVICE_ID.json ]]; then
		echo -e "${PREFIX} Failed: 'hzn sdo voucher download <device-id> -f /tmp' not downloaded to /tmp/$VOUCHER_DEVICE_ID.json"
		exit 1
	else
		echo -e "${PREFIX} Failed: 'hzn sdo voucher download <device-id> -f /tmp': downloaded file does not match expected output."
		exit 1
	fi
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher download': exit code: $rc, output: $cmdOutput."
	exit 1
fi
rm /tmp/$VOUCHER_DEVICE_ID.json

echo -e "${PREFIX} Testing 'hzn sdo voucher download' with incorrect voucher name argument"
cmdOutput=$(hzn sdo voucher download wrong_voucher_name -f /tmp/test_voucher.json 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid voucher name.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo voucher download' with incorrect voucher name argument: exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} removing test key from SDO owner services, if exists..."
hzn sdo key rm test-sdo-key &> /dev/null
echo -e "${PREFIX} done."
echo -e "${PREFIX} removing second test key from SDO owner services, if exists..."
hzn sdo key rm test-sdo-key2 &> /dev/null
echo -e "${PREFIX} done."

# Test hzn sdo key new

echo -e "${PREFIX} Testing 'hzn sdo key new' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$(hzn sdo key new -f /tmp/sample_key.json 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key new' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"
rm /tmp/sample_key.json

echo -e "${PREFIX} Testing 'hzn sdo key new' with incorrect HZN_EXCHANGE_USER_AUTH set"
export HZN_EXCHANGE_USER_AUTH=fakeuser:fakepw
cmdOutput=$(hzn sdo key new -f /tmp/sample_key.json 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key new' with incorrect HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH=$USERDEV_ADMIN_AUTH
rm /tmp/sample_key.json

echo -e "${PREFIX} Testing 'hzn sdo key new' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$(hzn sdo key new -f /tmp/sample_key.json 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key new' with incorrect HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE
rm /tmp/sample_key.json

echo -e "${PREFIX} Testing 'hzn sdo key new' with incorrect HZN_ORG_ID set"
export HZN_ORG_ID=fakeorg
cmdOutput=$(hzn sdo key new -f /tmp/sample_key.json 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key new' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE
rm /tmp/sample_key.json

echo -e "${PREFIX} Testing 'hzn sdo key new' without HZN_SDO_SVC_URL set"
unset HZN_SDO_SVC_URL
cmdOutput=$(hzn sdo key new -f /tmp/sample_key.json 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key new' without HZN_SDO_SVC_URL set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL_SAVE
rm /tmp/sample_key.json

echo -e "${PREFIX} Testing 'hzn sdo key new'"
cmdOutput=$(hzn sdo key new 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"$inspectSampleKey"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key new' without HZN_SDO_SVC_URL set: exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo key new -f /tmp/sample_key.json'"
cmdOutput=$(hzn sdo key new -f /tmp/sample_key.json 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	if [[ -f /tmp/sample_key.json && "$(cat /tmp/sample_key.json)" == $inspectSampleKey ]]; then
		echo -e "${PREFIX} completed."
	elif [[ ! -f /tmp/sample_key.json ]]; then
		echo -e "${PREFIX} Failed: 'hzn sdo key new <file-path> -f /tmp/sample_key.json' not downloaded to /tmp/sample_key.json"
		exit 1
	else
		echo -e "${PREFIX} Failed: 'hzn sdo key new <file-path> -f /tmp/sample_key.json': downloaded file does not match expected output."
		exit 1
	fi
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key new': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo key new -f /tmp/sample_key.json' when /tmp/sample_key.json already exists"
cmdOutput=$(hzn sdo key new -f /tmp/sample_key.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'File'*'already exists. Please specify a different file path or file name.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key new' when /tmp/sample_key.json already exists: exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo key new -f /tmp/sample_key.json -O' when /tmp/sample_key.json already exists"
cmdOutput=$(hzn sdo key new -f /tmp/sample_key.json -O 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	if [[ -f /tmp/sample_key.json && "$(cat /tmp/sample_key.json)" == $inspectSampleKey ]]; then
		echo -e "${PREFIX} completed."
	elif [[ ! -f /tmp/sample_key.json ]]; then
		echo -e "${PREFIX} Failed: 'hzn sdo key new <file-path> -f /tmp/sample_key.json' not downloaded to /tmp/sample_key.json"
		exit 1
	else
		echo -e "${PREFIX} Failed: 'hzn sdo key new <file-path> -f /tmp/sample_key.json': downloaded file does not match expected output."
		exit 1
	fi
elif [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'File'*'already exists. Please specify a different file path or file name.'* ]]; then
	echo -e "${PREFIX} Failed: 'hzn sdo key new' did not overwrite output file with -O flag set."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key new': exit code: $rc, output: $cmdOutput."
	exit 1
fi
rm /tmp/sample_key.json

echo -e "${PREFIX} Testing 'hzn sdo key new -f /tmp/sample_key.json'"
cmdOutput=$(hzn sdo key new 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	if [[ -f /tmp/sample_key.json && "$cmdOutput" == *"$inspectSampleKey"* ]]; then
		echo -e "${PREFIX} completed."
	fi
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key new': exit code: $rc, output: $cmdOutput."
	exit 1
fi

# Test hzn sdo key create

echo -e "${PREFIX} Testing 'hzn sdo key create' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$(hzn sdo key create /tmp/sdo_key.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'exchange user authentication must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key create' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"

echo -e "${PREFIX} Testing 'hzn sdo key create' with incorrect HZN_EXCHANGE_USER_AUTH set"
export HZN_EXCHANGE_USER_AUTH=fakeuser:fakepw
cmdOutput=$(hzn sdo key create /tmp/sdo_key.json 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid credentials.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key create' with incorrect HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH=$USERDEV_ADMIN_AUTH

echo -e "${PREFIX} Testing 'hzn sdo key create' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$(hzn sdo key create /tmp/sdo_key.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'organization ID must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key create' with incorrect HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key create' with incorrect HZN_ORG_ID set"
export HZN_ORG_ID=fakeorg
cmdOutput=$(hzn sdo key create /tmp/sdo_key.json 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid credentials.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key create' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key create' without HZN_SDO_SVC_URL set"
unset HZN_SDO_SVC_URL
cmdOutput=$(hzn sdo key create /tmp/sdo_key.json 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Could not get'*'HZN_SDO_SVC_URL'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key create' without HZN_SDO_SVC_URL set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key create <key-file>'"
cmdOutput=$(hzn sdo key create /tmp/sdo_key.json 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *'-----BEGIN PUBLIC KEY-----'*'-----END PUBLIC KEY-----'* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key create': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} creating template key..."
hzn sdo key new -f /tmp/sample_key.json
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing 'hzn sdo key create <key-file>' with empty fields"
cmdOutput=$(hzn sdo key create /tmp/sample_key.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'given key'*'has missing fields:'*'field'*'is missing'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key create': exit code: $rc, output: $cmdOutput."
	exit 1
fi
rm /tmp/sample_key.json

echo -e "${PREFIX} Testing 'hzn sdo key create <key-file>' when <key-file> already exists in SDO owner services"
cmdOutput=$(hzn sdo key create /tmp/sdo_key.json 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid key file.'*'Key'*'already exists in SDO owner services'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key create': exit code: $rc, output: $cmdOutput."
	exit 1
fi

# Test hzn sdo key remove

echo -e "${PREFIX} Testing 'hzn sdo key remove' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$(hzn sdo key remove test-sdo-key 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'exchange user authentication must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key remove' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"

echo -e "${PREFIX} Testing 'hzn sdo key remove' with incorrect HZN_EXCHANGE_USER_AUTH set"
export HZN_EXCHANGE_USER_AUTH=fakeuser:fakepw
cmdOutput=$(hzn sdo key remove test-sdo-key 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid credentials.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key remove' with incorrect HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH=$USERDEV_ADMIN_AUTH

echo -e "${PREFIX} Testing 'hzn sdo key remove' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$(hzn sdo key remove test-sdo-key 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'organization ID must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key remove' with incorrect HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key remove' with incorrect HZN_ORG_ID set"
export HZN_ORG_ID=fakeorg
cmdOutput=$(hzn sdo key remove test-sdo-key 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid credentials.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key remove' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key remove ' without HZN_SDO_SVC_URL set"
unset HZN_SDO_SVC_URL
cmdOutput=$(hzn sdo key remove test-sdo-key 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Could not get'*'HZN_SDO_SVC_URL'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key remove' without HZN_SDO_SVC_URL set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key remove <keyName>'"
cmdOutput=$(hzn sdo key remove test-sdo-key 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *'Key'*'successfully deleted from the SDO owner services.'* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key remove': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo key remove <keyName>' when key does not exist"
cmdOutput=$(hzn sdo key remove test-sdo-key 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid key name.'* ]]; then
	echo -e "${PREFIX} received expected error response.."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key remove' when key does not exist: exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} adding test key to SDO owner services..."
hzn sdo key create /tmp/sdo_key.json &> /dev/null
echo -e "${PREFIX} done."

# Test hzn sdo key list

echo -e "${PREFIX} Testing 'hzn sdo key list' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$(hzn sdo key list 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'exchange user authentication must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key list' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"

echo -e "${PREFIX} Testing 'hzn sdo key list' with incorrect HZN_EXCHANGE_USER_AUTH set"
export HZN_EXCHANGE_USER_AUTH=fakeuser:fakepw
cmdOutput=$(hzn sdo key list 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid credentials.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key list' with incorrect HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH=$USERDEV_ADMIN_AUTH

echo -e "${PREFIX} Testing 'hzn sdo key list' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$(hzn sdo key list 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'organization ID must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key list' with incorrect HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key list' with incorrect HZN_ORG_ID set"
export HZN_ORG_ID=fakeorg
cmdOutput=$(hzn sdo key list 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid credentials.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key list' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key list' without HZN_SDO_SVC_URL set"
unset HZN_SDO_SVC_URL
cmdOutput=$(hzn sdo key list 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Could not get'*'HZN_SDO_SVC_URL'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key list' without HZN_SDO_SVC_URL set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key list' with 1 keys in SDO owner services"
cmdOutput=$(hzn sdo key list 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == "$inspectSingleKeyList" ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key list': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} adding test key to SDO owner services..."
hzn sdo key create /tmp/sdo_key2.json &> /dev/null
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing 'hzn sdo key list' with 2 keys in SDO owner services"
cmdOutput=$(hzn sdo key list 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == "$inspectDoubleKeyList" ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key list': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo key list <keyName>' with incorrect key name argument"
cmdOutput=$(hzn sdo key list fake-sdo-key 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'SDO key name'*'not found'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key list': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} removing test key from SDO owner services..."
hzn sdo key rm test-sdo-key &> /dev/null
echo -e "${PREFIX} done."
echo -e "${PREFIX} removing second test key from SDO owner services..."
hzn sdo key rm test-sdo-key2 &> /dev/null
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing 'hzn sdo key list' with 0 keys in SDO owner services"
cmdOutput=$(hzn sdo key list 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *'[]'* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key list': exit code: $rc, output: $cmdOutput."
	exit 1
fi
echo -e "${PREFIX} adding test key to SDO owner services..."
hzn sdo key create /tmp/sdo_key.json &> /dev/null
echo -e "${PREFIX} done."
# Test hzn sdo key download

echo -e "${PREFIX} Testing 'hzn sdo key download' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$(hzn sdo key download $SDO_KEY_NAME 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'exchange user authentication must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key download' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"

echo -e "${PREFIX} Testing 'hzn sdo key download' with incorrect HZN_EXCHANGE_USER_AUTH set"
export HZN_EXCHANGE_USER_AUTH=fakeuser:fakepw
cmdOutput=$(hzn sdo key download $SDO_KEY_NAME 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid credentials.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key download' with incorrect HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH=$USERDEV_ADMIN_AUTH

echo -e "${PREFIX} Testing 'hzn sdo key download' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$(hzn sdo key download $SDO_KEY_NAME 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'organization ID must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key download' with incorrect HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key download' with incorrect HZN_ORG_ID set"
export HZN_ORG_ID=fakeorg
cmdOutput=$(hzn sdo key download $SDO_KEY_NAME 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid credentials.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key download' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key download' without HZN_SDO_SVC_URL set"
unset HZN_SDO_SVC_URL
cmdOutput=$(hzn sdo key download $SDO_KEY_NAME 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Could not get'*'HZN_SDO_SVC_URL'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key download' without HZN_SDO_SVC_URL set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL_SAVE

echo -e "${PREFIX} Testing 'hzn sdo key download <keyName>"
cmdOutput=$(hzn sdo key download $SDO_KEY_NAME 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *'-----BEGIN PUBLIC KEY-----'*'-----END PUBLIC KEY-----'* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key download': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo key download <keyName> -f /tmp/test_sdo_key'"
cmdOutput=$(hzn sdo key download $SDO_KEY_NAME -f /tmp/test_sdo_key 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	if [[ -f /tmp/test_sdo_key && "$(cat /tmp/test_sdo_key)" == *'-----BEGIN PUBLIC KEY-----'*'-----END PUBLIC KEY-----'* ]]; then
		echo -e "${PREFIX} completed."
	elif [[ ! -f /tmp/test_sdo_key ]]; then
		echo -e "${PREFIX} Failed: 'hzn sdo key download <keyName> -f /tmp/test_sdo_key' not downloaded to /tmp/test_sdo_key"
		exit 1
	else
		echo -e "${PREFIX} Failed: 'hzn sdo key download <keyName> -f /tmp/test_sdo_key': downloaded file does not match expected output."
		exit 1
	fi
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key download': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo key download <keyName> -f /tmp/test_sdo_key' when /tmp/test_sdo_key already exists"
cmdOutput=$(hzn sdo key download $SDO_KEY_NAME -f /tmp/test_sdo_key 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'File'*'already exists. Please specify a different file path or file name.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key download' when /tmp/test_sdo_key already exists: exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo key download <keyName> -f /tmp/test_sdo_key -O' when /tmp/test_sdo_key already exists"
cmdOutput=$(hzn sdo key download $SDO_KEY_NAME -f /tmp/test_sdo_key -O 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	if [[ -f /tmp/test_sdo_key && "$(cat /tmp/test_sdo_key)" == *'-----BEGIN PUBLIC KEY-----'*'-----END PUBLIC KEY-----'* ]]; then
		echo -e "${PREFIX} completed."
	elif [[ ! -f /tmp/test_sdo_key ]]; then
		echo -e "${PREFIX} Failed: 'hzn sdo key download <keyName> -f /tmp/test_sdo_key' not downloaded to /tmp/test_sdo_key"
		exit 1
	else
		echo -e "${PREFIX} Failed: 'hzn sdo key download <keyName> -f /tmp/test_sdo_key': downloaded file does not match expected output."
		exit 1
	fi
elif [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'File'*'already exists. Please specify a different file path or file name.'* ]]; then
	echo -e "${PREFIX} Failed: 'hzn sdo key download' did not overwrite output file with -O flag set."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key download': exit code: $rc, output: $cmdOutput."
	exit 1
fi
rm /tmp/test_sdo_key

echo -e "${PREFIX} Testing 'hzn sdo key download <keyName> -f /tmp'"
cmdOutput=$(hzn sdo key download $SDO_KEY_NAME -f /tmp 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	if [[ -f /tmp/$SDO_KEY_NAME && "$(cat /tmp/$SDO_KEY_NAME)" == *'-----BEGIN PUBLIC KEY-----'*'-----END PUBLIC KEY-----'* ]]; then
		echo -e "${PREFIX} completed."
	elif [[ ! -f /tmp/$SDO_KEY_NAME ]]; then
		echo -e "${PREFIX} Failed: 'hzn sdo key download <keyName> -f /tmp' not downloaded to /tmp/$SDO_KEY_NAME"
		exit 1
	else
		echo -e "${PREFIX} Failed: 'hzn sdo key download <keyName> -f /tmp': downloaded file does not match expected output."
		exit 1
	fi
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key download': exit code: $rc, output: $cmdOutput."
	exit 1
fi
rm /tmp/$SDO_KEY_NAME

echo -e "${PREFIX} Testing 'hzn sdo key download' with incorrect key name argument"
cmdOutput=$(hzn sdo key download wrong_key_name -f /tmp/test_sdo_key 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Invalid key name.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key download' with incorrect key name argument: exit code: $rc, output: $cmdOutput."
	exit 1
fi

# More sdo keys create tests

echo -e "${PREFIX} removing test key from SDO owner services..."
hzn sdo key rm test-sdo-key &> /dev/null
echo -e "${PREFIX} done."
echo -e "${PREFIX} removing second test key from SDO owner services..."
hzn sdo key rm test-sdo-key2 &> /dev/null
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing 'hzn sdo key create /tmp/sdo_key.json -f /tmp/test_sdo_pub_key'"
cmdOutput=$(hzn sdo key create /tmp/sdo_key.json -f /tmp/test_sdo_pub_key 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	if [[ -f /tmp/test_sdo_pub_key && "$(cat /tmp/test_sdo_pub_key)" == *'-----BEGIN PUBLIC KEY-----'*'-----END PUBLIC KEY-----'* ]]; then
		echo -e "${PREFIX} completed."
	elif [[ ! -f /tmp/test_sdo_pub_key ]]; then
		echo -e "${PREFIX} Failed: 'hzn sdo key create /tmp/sdo_key.json -f /tmp/test_sdo_pub_key' not downloaded to /tmp/test_sdo_pub_key"
		exit 1
	else
		echo -e "${PREFIX} Failed: 'hzn sdo key create /tmp/sdo_key.json -f /tmp/test_sdo_pub_key': downloaded file does not match expected output."
		exit 1
	fi
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key create': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} removing test key from SDO owner services..."
hzn sdo key rm test-sdo-key &> /dev/null
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing 'hzn sdo key create /tmp/sdo_key.json -f /tmp/test_sdo_pub_key' when /tmp/test_sdo_pub_key already exists"
cmdOutput=$(hzn sdo key create /tmp/sdo_key.json -f /tmp/test_sdo_pub_key 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'File'*'already exists. Please specify a different file path or file name.'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key create' when /tmp/test_sdo_pub_key already exists: exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn sdo key create /tmp/sdo_key.json -f /tmp/test_sdo_pub_key -O' when /tmp/test_sdo_pub_key already exists"
cmdOutput=$(hzn sdo key create /tmp/sdo_key.json -f /tmp/test_sdo_pub_key -O 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	if [[ -f /tmp/test_sdo_pub_key && "$(cat /tmp/test_sdo_pub_key)" == *'-----BEGIN PUBLIC KEY-----'*'-----END PUBLIC KEY-----'* ]]; then
		echo -e "${PREFIX} completed."
	elif [[ ! -f /tmp/test_sdo_pub_key ]]; then
		echo -e "${PREFIX} Failed: 'hzn sdo key create /tmp/sdo_key.json -f /tmp/test_sdo_pub_key' not downloaded to /tmp/test_sdo_pub_key"
		exit 1
	else
		echo -e "${PREFIX} Failed: 'hzn sdo key create /tmp/sdo_key.json -f /tmp/test_sdo_pub_key': downloaded file does not match expected output."
		exit 1
	fi
elif [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'File'*'already exists. Please specify a different file path or file name.'* ]]; then
	echo -e "${PREFIX} Failed: 'hzn sdo key new' did not overwrite output file with -O flag set."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn sdo key create': exit code: $rc, output: $cmdOutput."
	exit 1
fi
rm /tmp/test_sdo_pub_key

# Cleanup
echo -e "${PREFIX} removing test key from SDO owner services..."
hzn sdo key rm test-sdo-key &> /dev/null
echo -e "${PREFIX} done."
rm /tmp/sdo_key.json
rm /tmp/sdo_key2.json

echo -e "${PREFIX} Done"