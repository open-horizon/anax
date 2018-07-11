#!/bin/bash

#exec 3>&1 4>&2
#trap 'exec 2>&4 1>&3' 0 1 2 3

mkdir -p $HOME/.colonus

# init and create ethereum account
cd /root
rm -rf .ethereum .ethash
mkdir .ethereum # to avoid geth y/N question

# get a prebuilt DAG that ethereum needs for mining to avoid 7 mins of dynamic generation time.
echo "Move the DAG into place if there is one."
mkdir .ethash
cd .ethash
mv /tmp/full-R23-0000000000000000 . 2>/dev/null || :
touch full-R23-0000000000000000
cd ..

# Create an ethereum account
echo "Creating Ethereum account."
echo $PASSWD >passwd
geth --password passwd account new | perl -p -e 's/[{}]//g' | awk '{print $NF}' > $HOME/.colonus/accounts

# create genesis block
echo "Setting up genesis block."
cd /root
cat >genesis.json <<EOF
{
    "nonce": "0x0000000000000042",
    "difficulty": "0x000000100",
    "alloc": {},
    "mixhash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "coinbase": "0x0000000000000000000000000000000000000000",
    "timestamp": "0x00",
    "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "gasLimit": "0x5dc6c0"
}
EOF

# set network ID
echo "Publishing networkid."
NETWORKID=$((RANDOM * RANDOM))
echo $NETWORKID > /root/networkid

# Publish NETWORKID
PUB_NID_OUT=$(curl -sL -X PUT http://$MYHOST:2379/v2/keys/networkid -d value=$NETWORKID)

ETHERBASE=$(cat $HOME/.colonus/accounts)

# Start go ethereum and give it a chance to initialize
echo "Starting Ethereum."
geth init /root/genesis.json
geth --lightkdf --fast --shh --verbosity 4 --nodiscover --nat extip:$MYHOST --networkid $NETWORKID --mine --minerthreads 1 --rpc --rpcapi "admin,db,eth,debug,miner,net,shh,txpool,personal,web3" >/tmp/geth.log 2>&1 &

sleep 5

# Publish our ethereum node URL soo that the RPI simultor can join our blockchain
echo "Publishing our node URL."
NODEURL=$(geth --exec 'admin.nodeInfo.enode' attach)
# Update the URL with our machine's IP address so that cross container communication through TCPIP will work
# FULL_NODEURL=$( echo "${NODEURL//\@\[\:\:\]/@$MYHOST}" )
MY_IP=$(ifconfig eth0 | grep inet | cut -d: -f2 | cut -d' ' -f1)
# echo $MY_IP
FULL_NODEURL=$( echo "${NODEURL//\@\[\:\:\]/@$MY_IP}" )
# Publish the URL
PUB_NURL_OUT=$(curl -sL -X PUT http://$MYHOST:2379/v2/keys/peer -d value=$FULL_NODEURL)
echo $FULL_NODEURL > /root/nodeurl

# We need ether to do anything, mining will generate ether. Mining was started when geth was started.
echo "Wait for mining to begin."
BALANCE=0
while ! perl -e "exit($BALANCE == 0)"
do
    sleep 15
    BALANCE=$(geth --exec 'eth.getBalance(eth.accounts[0])' attach)
done

# Mining is running. The on-demand miner will shut it down and then look for pending transactions.
#echo "Starting on-demand miner."
#MS=$(geth --exec "miner.stop()" attach)
#./odminer.sh >/tmp/odminer.log 2>&1 &

# Unlock the ethereum account so that ether can be spent.
echo "Unlocking account for bootstrap."
UNLOCKED="false"
while test "$UNLOCKED" = "false"
do
    UNLOCKED=$(geth --exec "personal.unlockAccount('$ETHERBASE','$PASSWD',0)" attach)
    sleep 1
done

# Enable tracing for go-solidity
export mtn_soliditycontract=1

export CMTN_DIRECTORY_VERSION=0
echo "Bootstrapping smart contracts, version 0."
mtn-bootstrap $ETHERBASE >/tmp/bootstrap.log 2>&1
BRC=$?
if [ "$BRC" -ne 0 ]; then
    echo "Bootstrap failed."
    echo "$BRC"
    #exit -1
fi

# The contract bootstrap function writes the directory contract address into a file.
DIRADDR=$(cat directory)
echo $DIRADDR >$HOME/.colonus/directory.address

# Install our smart contracts onto the blockchain at version 999.
# export CMTN_DIRECTORY_VERSION=999
# echo "Bootstrapping smart contracts, version 999."
# mtn-bootstrap $ETHERBASE $DIRADDR >/tmp/bootstrap2.log 2>&1
# BRC=$?
# if [ "$BRC" -ne 0 ]; then
#     echo "Bootstrap failed."
#     echo "$BRC"
#     #exit -1
# fi

export CMTN_DIRECTORY_VERSION=0

# The RPI simulator needs to know the directory address so we will publish it for him.
echo "Publishing directory address."
PUB_DIRADDR_OUT=$(curl -sL -X PUT http://$MYHOST:2379/v2/keys/directory -d value=$DIRADDR)

# Disable tracing for go-solidity
unset mtn_soliditycontract

# Wait for gorest to initialize before proceeding
#sleep 5

# Start the smart contract monitor
echo "Starting Smart Contract Monitor"
smartcontract-monitor -v=5 -alsologtostderr=true -dirAddr=$DIRADDR >/tmp/monitor.log 2>&1 &

# Start the governor
echo "Starting the Governor."
WHISPERP=$(curl -sL http://localhost:8545 -X POST --data '{"jsonrpc":"2.0","method":"shh_newIdentity","params":[],"id":1}' | jq -r '.result')
echo "$WHISPERP" >/root/.colonus/shhid
/usr/local/bin/start >/tmp/agbot.log 2>&1 &

echo "all done"

# The governor is the only miner. The RPI simulator has it's own ethereum account that needs ether too, so periodically send 1 ether to the RPI simulator's ethereum account.
LIMIT=1000000000000000000
while :
do
    FUND_RPISIM=$(curl -sL http://$MYHOST:2379/v2/keys/ether | jq -r '.node.value' 2>/dev/null)
    #echo $FUND_RPISIM
    if test "$FUND_RPISIM" != "null"; then
        BALANCE=$(geth --exec "eth.getBalance('$FUND_RPISIM')" attach)
        CONVBAL=`expr $BALANCE`
        #echo $BALANCE
        #echo $CONVBAL
        if [ "$CONVBAL" -lt "$LIMIT" ]; then
            FUND_OUT=$(geth --exec "web3.eth.sendTransaction({from: '$ETHERBASE', to: '$FUND_RPISIM', value: '$LIMIT'})" attach)
            #echo $FUND_OUT
            echo "Sending funds to RPI simulator."
        fi
    fi
    
    sleep 300
done
