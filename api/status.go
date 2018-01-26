package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/version"
)

type Configuration struct {
	ExchangeAPI     string `json:"exchange_api"`
	ExchangeVersion string `json:"exchange_version"`
	ReqExchVersion  string `json:"required_exchange_version"`
	Arch            string `json:"architecture"`
	HorizonVersion  string `json:"horizon_version"`
}

type Info struct {
	Geths         []Geth          `json:"geth"`
	Configuration *Configuration  `json:"configuration"`
	Connectivity  map[string]bool `json:"connectivity"`
}

func NewInfo(config *config.HorizonConfig) *Info {

	exch_version, err := exchange.GetExchangeVersion(config.Collaborators.HTTPClientFactory, config.Edge.ExchangeURL)
	if err != nil {
		glog.Errorf("Failed to get exchange version: %v", err)
	}

	return &Info{
		Geths: []Geth{},
		Configuration: &Configuration{
			ExchangeAPI:     config.Edge.ExchangeURL,
			ExchangeVersion: exch_version,
			ReqExchVersion:  version.REQUIRED_EXCHANGE_VERSION,
			Arch:            runtime.GOARCH,
			HorizonVersion:  version.HORIZON_VERSION,
		},
		Connectivity: map[string]bool{},
	}
}

func (info *Info) AddGeth(geth *Geth) *Info {
	info.Geths = append(info.Geths, *geth)

	return info
}

// Geth is an external type exposing the health of the go-ethereum process used by this anax instance
type Geth struct {
	NetPeerCount   int64    `json:"net_peer_count"`
	EthSyncing     bool     `json:"eth_syncing"`
	EthBlockNumber int64    `json:"eth_block_number"`
	EthAccounts    []string `json:"eth_accounts"`
	EthBalance     string   `json:"eth_balance"` // a string b/c this is a huge number
}

func NewGeth() *Geth {
	return &Geth{
		NetPeerCount:   -1,
		EthSyncing:     false,
		EthBlockNumber: -1,
		EthAccounts:    []string{},
		EthBalance:     "",
	}
}

func WriteGethStatus(gethURL string, geth *Geth) error {

	singleResult := func(meth string, params []string) interface{} {
		serial, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "method": meth, "params": params, "id": 1})
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
	switch singleResult("eth_syncing", []string{}).(type) {
	case bool:
		geth.EthSyncing = false
	default:
		geth.EthSyncing = true
	}

	// get current the number of the current block
	blockStr := singleResult("eth_blockNumber", []string{}).(string)
	if blockStr != "" {
		blockNum, err := strconv.ParseInt(strings.TrimPrefix(blockStr, "0x"), 16, 64)
		if err != nil {
			return err
		}
		geth.EthBlockNumber = blockNum
	}

	// get number of peers
	peerStr := singleResult("net_peerCount", []string{}).(string)
	if peerStr != "" {
		peers, err := strconv.ParseInt(strings.TrimPrefix(peerStr, "0x"), 16, 64)
		if err != nil {
			return err
		}

		geth.NetPeerCount = peers
	}

	// get the account
	if account := singleResult("eth_accounts", []string{}); account != nil {
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

	// get account balance
	if len(geth.EthAccounts) == 0 {
		geth.EthBalance = "0x0"
	} else {
		eth_balance_params := make([]string, 2)
		eth_balance_params[0] = geth.EthAccounts[0]
		eth_balance_params[1] = "latest"
		geth.EthBalance = singleResult("eth_getBalance", eth_balance_params).(string)
	}

	return nil
}
