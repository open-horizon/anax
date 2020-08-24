#!/bin/bash

PREFIX="hzn voucher test:"

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

# Test hzn voucher inspect
echo -e "${PREFIX} Testing 'hzn voucher inspect <voucher-file>'"
cmdOutput=$(hzn voucher inspect /tmp/sdo_voucher.json 2>&1)
rc=$?
if [[ $rc -ne 0 ]]; then
	echo -e "${PREFIX} Failed: exit code $rc from 'hzn voucher inspect': $cmdOutput."
	exit 1
elif [[ "$cmdOutput" != "$inspectOutput" ]]; then
	echo -e "${PREFIX} Failed: Wrong output for 'hzn voucher inspect <voucher-file>': $cmdOutput."
	exit 1
fi

# Test hzn voucher inspect with a voucher that has an IP address for the rendezvous svr
echo -e "${PREFIX} Testing 'hzn voucher inspect <voucher-file>' with IP"
cmdOutput=$(hzn voucher inspect /tmp/sdo_voucher-ip.json 2>&1)
rc=$?
if [[ $rc -ne 0 ]]; then
	echo -e "${PREFIX} Failed: exit code $rc from 'hzn voucher inspect' with IP: $cmdOutput."
	exit 1
elif [[ "$cmdOutput" != "$inspectOutputIp" ]]; then
	echo -e "${PREFIX} Failed: Wrong output for 'hzn voucher inspect <voucher-file>' with IP: $cmdOutput."
	exit 1
fi

# Test inspect error cases
echo -e "${PREFIX} Testing 'hzn voucher inspect' with missing arg"
cmdOutput=$(hzn voucher inspect 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'required argument'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher inspect': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn voucher inspect file-not-there'"
cmdOutput=$(hzn voucher inspect file-not-there 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'no such file'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher inspect file-not-there': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn voucher inspect <voucher-file> 2nd-arg'"
cmdOutput=$(hzn voucher inspect /tmp/sdo_voucher.json 2nd-arg 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'unexpected 2nd-arg'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher inspect <voucher-file> 2nd-arg': exit code: $rc, output: $cmdOutput."
	exit 1
fi

# Test hzn voucher import

USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"
export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"
#NODE_AUTH="userdev/an12345"
#export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"
HZN_ORG_ID_SAVE=$HZN_ORG_ID

#todo: currently only testing cmd syntax error cases. Full tests will be added in https://github.com/open-horizon/anax/issues/1677
HZN_SDO_SVC_URL_SAVE=foobar
export HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL_SAVE

echo -e "${PREFIX} Testing 'hzn voucher import' with missing arg"
cmdOutput=$(hzn voucher import 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'required argument'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher import': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn voucher import file-not-there'"
cmdOutput=$(hzn voucher import file-not-there 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'no such file'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher import file-not-there': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn voucher import <voucher-file> 2nd-arg'"
cmdOutput=$(hzn voucher import /tmp/sdo_voucher.json 2nd-arg 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'unexpected 2nd-arg'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher import <voucher-file> 2nd-arg': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn voucher import /tmp/voucher.badextension'"
touch /tmp/voucher.badextension
cmdOutput=$(hzn voucher import /tmp/voucher.badextension 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'unsupported voucher file type extension'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher import /tmp/voucher.badextension': exit code: $rc, output: $cmdOutput."
	exit 1
fi
rm -f /tmp/voucher.badextension

echo -e "${PREFIX} Testing 'hzn voucher import <voucher-file> --policy policy-not-there.json'"
cmdOutput=$(hzn voucher import /tmp/sdo_voucher.json --policy policy-not-there.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'rror:'*'accessing policy-not-there.json'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher import <voucher-file> --policy policy-not-there.json': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn voucher import <voucher-file> with mutually exclusive -e and --policy'"
touch /tmp/node-policy.json
cmdOutput=$(hzn voucher import /tmp/sdo_voucher.json -e foo --policy /tmp/node-policy.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'rror:'*'mutually exclusive'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher import <voucher-file> -e foo --policy /tmp/node-policy.json': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn voucher import <voucher-file> with mutually exclusive -e and --pattern'"
cmdOutput=$(hzn voucher import /tmp/sdo_voucher.json -e foo --pattern bar 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'rror:'*'mutually exclusive'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher import <voucher-file> -e foo --pattern bar': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Testing 'hzn voucher import <voucher-file> with mutually exclusive --pattern and --policy'"
cmdOutput=$(hzn voucher import /tmp/sdo_voucher.json --pattern bar --policy /tmp/node-policy.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'rror:'*'mutually exclusive'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher import <voucher-file> --pattern bar --policy /tmp/node-policy.json': exit code: $rc, output: $cmdOutput."
	exit 1
fi
rm -f /tmp/node-policy.json

echo -e "${PREFIX} Testing 'hzn voucher import <voucher-file>' without HZN_SDO_SVC_URL set"
unset HZN_SDO_SVC_URL
cmdOutput=$(hzn voucher import /tmp/sdo_voucher.json 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Could not get'*'HZN_SDO_SVC_URL'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher import <voucher-file>' without HZN_SDO_SVC_URL set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL_SAVE

echo -e "${PREFIX} Testing 'hzn voucher import <voucher-file>' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$(hzn voucher import /tmp/sdo_voucher.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'organization ID must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher import <voucher-file>' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn voucher import <voucher-file>' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$(hzn voucher import /tmp/sdo_voucher.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'exchange user authentication must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher import <voucher-file>' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"

# Test hzn voucher list
echo -e "${PREFIX} Testing 'hzn voucher list' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$(hzn voucher list 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'exchange user authentication must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher list' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$USERDEV_ADMIN_AUTH"

echo -e "${PREFIX} Testing 'hzn voucher list' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$(hzn voucher list 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'Error:'*'organization ID must be specified'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher list' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing 'hzn voucher list' without HZN_SDO_SVC_URL set"
unset HZN_SDO_SVC_URL
cmdOutput=$(hzn voucher list 2>&1)
rc=$?
if [[ $rc -eq 7 && "$cmdOutput" == *'Error:'*'Could not get'*'HZN_SDO_SVC_URL'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher list' without HZN_SDO_SVC_URL set: exit code: $rc, output: $cmdOutput."
	exit 1
fi
export HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL_SAVE

echo -e "${PREFIX} Testing 'hzn voucher list <voucher> 2nd-arg'"
cmdOutput=$(hzn voucher list file-not-there 2nd-arg 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *'error:'*'unexpected 2nd-arg'* ]]; then
	echo -e "${PREFIX} received expected error response."
else
	echo -e "${PREFIX} Failed: Wrong error response from 'hzn voucher list <voucher> 2nd-arg': exit code: $rc, output: $cmdOutput."
	exit 1
fi

echo -e "${PREFIX} Done"
