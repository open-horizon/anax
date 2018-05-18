package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

const ExchangeURLEnvvarName = "HZN_EXCHANGE_URL"

type HorizonConfig struct {
	Edge          Config
	AgreementBot  AGConfig
	Collaborators Collaborators
	ArchSynonyms  ArchSynonyms
}

// This is the configuration options for Edge component flavor of Anax
type Config struct {
	ServiceStorage                string // The base storage directory where the service can write or get the data.
	TorrentDir                    string
	APIListen                     string
	DBPath                        string
	DockerEndpoint                string
	DockerCredFilePath            string
	DefaultCPUSet                 string
	DefaultServiceRegistrationRAM int64
	StaticWebContent              string
	PublicKeyPath                 string
	TrustSystemCACerts            bool   // If equal to true, the HTTP client factory will set up clients that trust CA certs provided by a Linux distribution (see https://golang.org/pkg/crypto/x509/#SystemCertPool and https://golang.org/src/crypto/x509/root_linux.go)
	CACertsPath                   string // Path to a file containing PEM-encoded x509 certs HTTP clients in Anax will trust (additive to the configuration option "TrustSystemCACerts")
	ExchangeURL                   string
	DefaultHTTPClientTimeoutS     uint
	PolicyPath                    string
	ExchangeHeartbeat             int    // Seconds between heartbeats
	ExchangeVersionCheckIntervalM int64  // Exchange version check interval in minutes. The default is 720.
	AgreementTimeoutS             uint64 // Number of seconds to wait before declaring agreement not finalized in blockchain
	DVPrefix                      string // When passing agreement ids into a workload container, add this prefix to the agreement id
	RegistrationDelayS            uint64 // The number of seconds to wait after blockchain init before registering with the exchange. This is for testing initialization ONLY.
	ExchangeMessageTTL            int    // The number of seconds the exchange will keep this message before automatically deleting it
	TorrentListenAddr             string // Override the torrent listen address just in case there are conflicts, syntax is "host:port"
	UserPublicKeyPath             string // The location to store user keys uploaded through the REST API
	ReportDeviceStatus            bool   // whether to report the device status to the exchange or not.
	TrustCertUpdatesFromOrg       bool   // whether to trust the certs provided by the organization on the exchange or not.
	TrustDockerAuthFromOrg        bool   // whether to turst the docker auths provided by the organization on the exchange or not.
	ServiceUpgradeCheckIntervalS  int64  // service upgrade check interval in seconds. The default is 300 seconds.
	MultipleAnaxInstances         bool   // multiple anax instances running on the same machine

	// these Ids could be provided in config or discovered after startup by the system
	BlockchainAccountId        string
	BlockchainDirectoryAddress string
}

// This is the configuration options for Agreement bot flavor of Anax
type AGConfig struct {
	TxLostDelayTolerationSeconds  int
	AgreementWorkers              int
	DBPath                        string
	ProtocolTimeoutS              uint64 // Number of seconds to wait before declaring proposal response is lost
	AgreementTimeoutS             uint64 // Number of seconds to wait before declaring agreement not finalized in blockchain
	NoDataIntervalS               uint64 // default should be 15 mins == 15*60 == 900. Ignored if the policy has data verification disabled.
	ActiveAgreementsURL           string // This field is used when policy files indicate they want data verification but they dont specify a URL
	ActiveAgreementsUser          string // This is the userid the agbot uses to authenticate to the data verifivcation API
	ActiveAgreementsPW            string // This is the password for the ActiveAgreementsUser
	PolicyPath                    string // The directory where policy files are kept, default /etc/provider-tremor/policy/
	NewContractIntervalS          uint64 // default should be 1
	ProcessGovernanceIntervalS    uint64 // How long the gov sleeps before general gov checks (new payloads, interval payments, etc).
	IgnoreContractWithAttribs     string // A comma seperated list of contract attributes. If set, the contracts that contain one or more of the attributes will be ignored. The default is "ethereum_account".
	ExchangeURL                   string // The URL of the Horizon exchange. If not configured, the exchange will not be used.
	ExchangeHeartbeat             int    // Seconds between heartbeats to the exchange
	ExchangeVersionCheckIntervalM int64  // Exchange version check interval in minutes. The default is 5. 0 means no periodic checking.
	ExchangeId                    string // The id of the agbot, not the userid of the exchange user. Must be org qualified.
	ExchangeToken                 string // The agbot's authentication token
	DVPrefix                      string // When looking for agreement ids in the data verification API response, look for agreement ids with this prefix.
	ActiveDeviceTimeoutS          int    // The amount of time a device can go without heartbeating and still be considered active for the purposes of search
	ExchangeMessageTTL            int    // The number of seconds the exchange will keep this message before automatically deleting it
	MessageKeyPath                string // The path to the location of messaging keys
	DefaultWorkloadPW             string // The default workload password if none is specified in the policy file
	APIListen                     string // Host and port for the API to listen on
	PurgeArchivedAgreementHours   int    // Number of hours to leave an archived agreement in the database before automatically deleting it
	CheckUpdatedPolicyS           int    // The number of seconds to wait between checks for an updated policy file. Zero means auto checking is turned off.
}

func (c *HorizonConfig) UserPublicKeyPath() string {
	if c.Edge.UserPublicKeyPath == "" {
		if commonPath := os.Getenv("SNAP_COMMON"); commonPath != "" {
			thePath := path.Join(os.Getenv("SNAP_COMMON"), USERKEYDIR)
			c.Edge.UserPublicKeyPath = thePath
		} else {
			return ""
		}
	}
	return c.Edge.UserPublicKeyPath
}

// some configuration is provided by envvars; in this case we populate this config object from expected envvars
func enrichFromEnvvars(config *HorizonConfig) error {

	if exchangeURL := os.Getenv(ExchangeURLEnvvarName); exchangeURL != "" {
		config.Edge.ExchangeURL = exchangeURL
		config.AgreementBot.ExchangeURL = exchangeURL
	} else {
		// TODO: Enable this once we require the envvar to be set. For now, we don't return the error
		// return fmt.Errorf("Unspecified but required envvar: %s", ExchangeURLEnvvarName)
	}

	return nil
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
		// instantiate mostly empty which will be filled. Values here are defaults that can be overridden by the user
		config := HorizonConfig{
			Edge: Config{
				DefaultHTTPClientTimeoutS: 20,
			},
		}

		err := json.NewDecoder(path).Decode(&config)
		if err != nil {
			return nil, fmt.Errorf("Unable to decode content of config file: %v", err)
		}

		err = enrichFromEnvvars(&config)

		if err != nil {
			return nil, fmt.Errorf("Unable to enrich content of config file with envvars: %v", err)
		}

		// set the defaults here in case the attributes are not setup by the user.
		if config.Edge.ExchangeVersionCheckIntervalM == 0 {
			config.Edge.ExchangeVersionCheckIntervalM = 720
		}
		if config.AgreementBot.ExchangeVersionCheckIntervalM == 0 {
			config.AgreementBot.ExchangeVersionCheckIntervalM = 5
		}
		if config.Edge.ServiceUpgradeCheckIntervalS == 0 {
			config.Edge.ServiceUpgradeCheckIntervalS = 300
		}

		// now make collaborators instance and assign it to member in this config
		collaborators, err := NewCollaborators(config)
		if err != nil {
			return nil, err
		}

		config.Collaborators = *collaborators

		if config.ArchSynonyms == nil {
			config.ArchSynonyms = NewArchSynonyms()
		}

		// success at last!
		return &config, nil
	}
}
