package apicommon

import (
	"runtime"
	"sync"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/version"
)

type Configuration struct {
	ExchangeAPI     string `json:"exchange_api"`
	ExchangeVersion string `json:"exchange_version,omitempty"`
	MinExchVersion  string `json:"required_minimum_exchange_version"`
	PrefExchVersion string `json:"preferred_exchange_version"`
	MMSAPI          string `json:"mms_api"`
	Arch            string `json:"architecture"`
	HorizonVersion  string `json:"horizon_version"`
}

// These fields are filled in by the API specific code, not the common code.
type HealthTimestamps struct {
	LastDBHeartbeatTime uint64 `json:"lastDBHeartbeat"`
}

type Info struct {
	Configuration *Configuration    `json:"configuration"`
	Connectivity  map[string]bool   `json:"connectivity,omitempty"`
	LiveHealth    *HealthTimestamps `json:"liveHealth"`
}

func NewInfo(httpClientFactory *config.HTTPClientFactory, exchangeUrl string, mmsUrl string, id string, token string) *Info {

	customHTTPClientFactory := &config.HTTPClientFactory{
		NewHTTPClient: httpClientFactory.NewHTTPClient,
		RetryCount:    5,
		RetryInterval: 2,
	}

	exch_version, err := exchange.GetExchangeVersion(customHTTPClientFactory, exchangeUrl, id, token)
	if err != nil {
		glog.Errorf("Failed to get exchange version: %v", err)
	}

	return &Info{
		Configuration: &Configuration{
			ExchangeAPI:     exchangeUrl,
			ExchangeVersion: exch_version,
			MinExchVersion:  version.MINIMUM_EXCHANGE_VERSION,
			PrefExchVersion: version.PREFERRED_EXCHANGE_VERSION,
			MMSAPI:          mmsUrl,
			Arch:            runtime.GOARCH,
			HorizonVersion:  version.HORIZON_VERSION,
		},
	}
}

// NewLocalInfo is like NewInfo, except this does not attempt to get the exchange version
func NewLocalInfo(exchangeUrl string, mmsUrl string, id string, token string) *Info {
	return &Info{
		Configuration: &Configuration{
			ExchangeAPI:     exchangeUrl,
			MinExchVersion:  version.MINIMUM_EXCHANGE_VERSION,
			PrefExchVersion: version.PREFERRED_EXCHANGE_VERSION,
			MMSAPI:          mmsUrl,
			Arch:            runtime.GOARCH,
			HorizonVersion:  version.HORIZON_VERSION,
		},
	}
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
