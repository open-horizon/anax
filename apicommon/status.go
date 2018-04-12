package apicommon

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/version"
)

type Configuration struct {
	ExchangeAPI     string `json:"exchange_api"`
	ExchangeVersion string `json:"exchange_version"`
	MinExchVersion  string `json:"required_minimum_exchange_version"`
	PrefExchVersion string `json:"preferred_exchange_version"`
	Arch            string `json:"architecture"`
	HorizonVersion  string `json:"horizon_version"`
}

type Info struct {
	Geths         []Geth          `json:"geth"`
	Configuration *Configuration  `json:"configuration"`
	Connectivity  map[string]bool `json:"connectivity"`
}

func NewInfo(httpClientFactory *config.HTTPClientFactory, exchangeUrl string, id string, token string) *Info {

	exch_version, err := exchange.GetExchangeVersion(httpClientFactory, exchangeUrl, id, token)
	if err != nil {
		glog.Errorf("Failed to get exchange version: %v", err)
	}

	return &Info{
		Geths: []Geth{},
		Configuration: &Configuration{
			ExchangeAPI:     exchangeUrl,
			ExchangeVersion: exch_version,
			MinExchVersion:  version.MINIMUM_EXCHANGE_VERSION,
			PrefExchVersion: version.PREFERRED_EXCHANGE_VERSION,
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

type BlockchainState struct {
	ready       bool   // the blockchain is ready
	writable    bool   // the blockchain is writable
	service     string // the network endpoint name of the container
	servicePort string // the network port of the container
}

func (b *BlockchainState) GetService() string {
	return b.service
}

func (b *BlockchainState) GetServicePort() string {
	return b.servicePort
}

// Functions to manage the blockchain state events so that the status API has accurate info to display.

func HandleNewBCInit(ev *events.BlockchainClientInitializedMessage, bcState map[string]map[string]BlockchainState, bcStateLock *sync.Mutex) {

	bcStateLock.Lock()
	defer bcStateLock.Unlock()

	nameMap := GetBCNameMap(ev.BlockchainType(), bcState)
	namedBC, ok := nameMap[ev.BlockchainInstance()]
	if !ok {
		nameMap[ev.BlockchainInstance()] = BlockchainState{
			ready:       true,
			writable:    false,
			service:     ev.ServiceName(),
			servicePort: ev.ServicePort(),
		}
	} else {
		namedBC.ready = true
		namedBC.service = ev.ServiceName()
		namedBC.servicePort = ev.ServicePort()
	}

}

func HandleStoppingBC(ev *events.BlockchainClientStoppingMessage, bcState map[string]map[string]BlockchainState, bcStateLock *sync.Mutex) {

	bcStateLock.Lock()
	defer bcStateLock.Unlock()

	nameMap := GetBCNameMap(ev.BlockchainType(), bcState)
	delete(nameMap, ev.BlockchainInstance())
}

func GetBCNameMap(typeName string, bcState map[string]map[string]BlockchainState) map[string]BlockchainState {
	nameMap, ok := bcState[typeName]
	if !ok {
		bcState[typeName] = make(map[string]BlockchainState)
		nameMap = bcState[typeName]
	}
	return nameMap
}
