package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type HorizonConfig struct {
	Edge         Config
	AgreementBot AGConfig
}

// This is the configuration options for Edge component flavor of Anax
type Config struct {
	WorkloadROStorage    string
	TorrentDir           string
	APIListen            string
	DBPath               string
	GethURL              string
	DockerEndpoint       string
	DefaultCPUSet        string
	StaticWebContent     string
	PublicKeyPath        string
	CACertsPath          string
	ExchangeURL          string
	PolicyPath           string
	ExchangeHeartbeat    int            // Seconds between heartbeats

	// these Ids could be provided in config or discovered after startup by the system
	BlockchainAccountId        string
	BlockchainDirectoryAddress string
}

// This is the configuration options for Agreement bot flavor of Anax
type AGConfig struct {
	TxLostDelayTolerationSeconds int
	AgreementWorkers             int
	DBPath                       string
	GethURL                      string
	PayloadPath                  string
	ActiveAccountExpirationS     int64     // If set to zero, disables active account checking
	AgreementLockDurationS       int64
	FullpaymentIntervalS         uint64    // default should be 28 days == 28*24*60*60 == 2419200
	MicropaymentIntervalS        uint64    // default should be 5 mins == 5*60 == 300. Set to zero to turn off micropayment whisper messages.
	                                       // Even with micropayments turned off, data verification can still be enabled in the policy. In this
	                                       // case, data will be checked for every 5 mins.
	NoDataIntervalS              uint64    // default should be 15 mins == 15*60 == 900. Ignored if the policy has data verification disabled.
	ActiveContractsURL           string    // This field is the default and cannot be removed until all workloads are using their own URL
	LoanIncrement                int       // default is 1000000
	EtcdUrl                      string    // default is http://localhost:2379/v2/keys. If not specified then ectd interactions are skipped.
	PolicyPath                   string    // The directory where policy files are kept, default /etc/provider-tremor/policy/
	NewContractIntervalS         uint64    // default should be 1
	ProcessMicropaymentIntervalS uint64    // How long the gov sleeps before checking if a micropayment needs to be sent. It is
	                                       // ignored if MicropaymentIntervalS == 0 because micro payments are turned off.
	ProcessGovernanceIntervalS   uint64    // How long the gov sleeps before general gov checks (new payloads, interval payments, etc).
	                                       // The default should be 5.
	IgnoreContractWithAttribs    string    // A comma seperated list of contract attributes. If set, the contracts that contain one or more of the attributes will be ignored. The default is "ethereum_account".
	ExchangeURL                  string    // The URL of the Horizon exchange. If not configured, the exchange will not be used.
	ExchangeHeartbeat            int       // Seconds between heartbeats to the exchange
	// Following are config fields set during initialization
	ExchangeId                   string    // The id of the agbot, not the userid of the exchange user
	ExchangeToken                string    // The agbot's authentication token


}

func Read(file string) (*HorizonConfig, error) {

	if _, err := os.Stat(file); err != nil {
		return nil, fmt.Errorf("Config file not found: %s. Error: %v", file, err)
	}

	// attempt to parse config file
	path, err := os.Open(filepath.Clean(file))
	if err != nil {
		return nil, fmt.Errorf("Unable to read config file: %s. Error: %v", file, err)
	} else {
		// instantiate empty which will be filled
		config := HorizonConfig{}

		err := json.NewDecoder(path).Decode(&config)
		if err != nil {
			return nil, fmt.Errorf("Unable to decode content of config file: %v", err)
		}

		// success at last!
		return &config, nil
	}
}
