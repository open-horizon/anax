package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
)

type Configuration struct {
	ExchangeAPI string `json:"exchange_api"`
}

type Info struct {
	Geth          *Geth           `json:"geth"`
	Configuration *Configuration  `json:"configuration"`
	Connectivity  map[string]bool `json:"connectivity"`
}

func NewInfo(config *config.HorizonConfig) *Info {
	return &Info{
		Geth: &Geth{
			NetPeerCount:   -1,
			EthSyncing:     false,
			EthBlockNumber: -1,
			EthAccounts:    []string{},
			EthBalance:     "",
		},
		Configuration: &Configuration{
			ExchangeAPI: config.Edge.ExchangeURL,
		},
		Connectivity: map[string]bool{},
	}
}

// Geth is an external type exposing the health of the go-ethereum process used by this anax instance
type Geth struct {
	NetPeerCount   int64    `json:"net_peer_count"`
	EthSyncing     bool     `json:"eth_syncing"`
	EthBlockNumber int64    `json:"eth_block_number"`
	EthAccounts    []string `json:"eth_accounts"`
	EthBalance     string   `json:"eth_balance"` // a string b/c this is a huge number
}

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

	geth.EthBalance = singleResult("eth_getBalance").(string)

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
			geth.EthAccounts = []string{}
		}
	}
	return nil
}
