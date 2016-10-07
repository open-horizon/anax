package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

func WriteGethStatus(gethURL string, geth *Geth) error {

	singleResult := func(meth string) interface{} {
		serial, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "method": meth, "params": []string{}, "id": 1})
		if err != nil {
			glog.Error(err)
			return ""
		}

		glog.V(5).Infof("encoded: %v", string(serial))

		resp, err := http.Post(gethURL, "application/json", bytes.NewBuffer(serial))
		if err != nil {
			glog.Error(err)
			return ""
		}

		defer resp.Body.Close()

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Error(err)
			return ""
		}

		var m map[string]interface{}
		err = json.Unmarshal(b, &m)
		if err != nil {
			glog.Error(err)
			return ""
		}

		glog.V(2).Infof("returned: %v", m)

		return m["result"]
	}

	// the return val is either a boolean if false, or an object
	switch singleResult("eth_syncing").(type) {
	case bool:
		geth.EthSyncing = false
	default:
		geth.EthSyncing = true
	}

	blockStr := singleResult("eth_blockNumber").(string)
	if blockStr != "" {
		blockNum, err := strconv.ParseInt(strings.TrimPrefix(blockStr, "0x"), 16, 64)
		if err != nil {
			return err
		}
		geth.EthBlockNumber = blockNum
	}

	peerStr := singleResult("net_peerCount").(string)
	if peerStr != "" {
		peers, err := strconv.ParseInt(strings.TrimPrefix(peerStr, "0x"), 16, 64)
		if err != nil {
			return err
		}

		geth.NetPeerCount = peers
	}

	// get the account
	if account := singleResult("eth_accounts"); account != nil {
		switch account.(type) {
		case []interface{}:
			a1 := account.([]interface{})
			geth.EthAccounts = make([]string, len(a1))
			for i := range a1 {
				geth.EthAccounts[i] = a1[i].(string)
			}
		default:
			geth.EthAccounts = nil
		}
	}
	return nil
}
