package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const ExchangeURLEnvvarName = "HZN_EXCHANGE_URL"
const FileSyncServiceCSSURLEnvvarName = "HZN_FSS_CSSURL"
const ExchangeMessageNoDynamicPollEnvvarName = "HZN_NO_DYNAMIC_POLL"
const OldMgmtHubCertPath = "HZN_ICP_CA_CERT_PATH"
const ManagementHubCertPath = "HZN_MGMT_HUB_CERT_PATH"
const AnaxAPIPort = "HZN_AGENT_PORT"

type HorizonConfig struct {
	Edge          Config
	AgreementBot  AGConfig
	Collaborators Collaborators
	ArchSynonyms  ArchSynonyms
}

// This is the configuration options for Edge component flavor of Anax
type Config struct {
	ServiceStorage                   string // The base storage directory where the service can write or get the data.
	APIListen                        string
	DBPath                           string
	DockerEndpoint                   string
	DockerCredFilePath               string
	DefaultCPUSet                    string
	DefaultServiceRegistrationRAM    int64
	StaticWebContent                 string
	PublicKeyPath                    string
	TrustSystemCACerts               bool   // If equal to true, the HTTP client factory will set up clients that trust CA certs provided by a Linux distribution (see https://golang.org/pkg/crypto/x509/#SystemCertPool and https://golang.org/src/crypto/x509/root_linux.go)
	CACertsPath                      string // Path to a file containing PEM-encoded x509 certs HTTP clients in Anax will trust (additive to the configuration option "TrustSystemCACerts")
	ExchangeURL                      string
	DefaultHTTPClientTimeoutS        uint
	PolicyPath                       string
	ExchangeHeartbeat                int       // Seconds between heartbeats
	ExchangeVersionCheckIntervalM    int64     // Exchange version check interval in minutes. The default is 720. This is now deprecated with the usage of /changes API which returns exchange version on every call.
	AgreementTimeoutS                uint64    // Number of seconds to wait before declaring agreement not finalized in blockchain
	DVPrefix                         string    // When passing agreement ids into a workload container, add this prefix to the agreement id
	RegistrationDelayS               uint64    // The number of seconds to wait after blockchain init before registering with the exchange. This is for testing initialization ONLY.
	ExchangeMessageTTL               int       // The number of seconds the exchange will keep this message before automatically deleting it
	ExchangeMessageDynamicPoll       bool      // Will the runtime dynamically increase the message poll interval? Default is true. Set to false to turn off dynamic message poll interval adjustments.
	ExchangeMessagePollInterval      int       // The number of seconds the node will wait between polls to the exchange. This is the starting value, but at runtime this interval will increase if there is no message activity to reduce load on the exchange. If ExchangeMessageDynamicPoll is false, then the value of this field will never be changed by the runtime.
	ExchangeMessagePollMaxInterval   int       // As the runtime increases the ExchangeMessagePollInterval, this value is the maximum that value can attain.
	ExchangeMessagePollIncrement     int       // The number of seconds to increment the ExchangeMessagePollInterval when its time to increase the poll interval.
	UserPublicKeyPath                string    // The location to store user keys uploaded through the REST API
	ReportDeviceStatus               bool      // whether to report the device status to the exchange or not.
	TrustCertUpdatesFromOrg          bool      // whether to trust the certs provided by the organization on the exchange or not.
	TrustDockerAuthFromOrg           bool      // whether to turst the docker auths provided by the organization on the exchange or not.
	ServiceUpgradeCheckIntervalS     int64     // service upgrade check interval in seconds. The default is 300 seconds.
	MultipleAnaxInstances            bool      // multiple anax instances running on the same machine
	DefaultServiceRetryCount         int       // the default service retry count if retries are not specified by the policy file. The default value is 2.
	DefaultServiceRetryDuration      uint64    // the default retry duration in seconds. The next retry cycle occurs after the duration. The default value is 600
	DefaultNodePolicyFile            string    // the default node policy file name.
	NodeCheckIntervalS               int       // the node check interval. The default is 15 seconds.
	NodePolicyCheckIntervalS         int       // the node policy check interval. The default is 15 seconds.
	FileSyncService                  FSSConfig // The config for the embedded ESS sync service.
	SurfaceErrorTimeoutS             int       // How long surfaced errors will remain active after they're created. Default is no timeout
	SurfaceErrorCheckIntervalS       int       // Deprecated. Used to be how often the node will check for errors that are no longer active and update the exchange. Default is 15 seconds
	SurfaceErrorAgreementPersistentS int       // How long an agreement needs to persist before it is considered persistent and the related errors are dismisse. Default is 90 seconds
	InitialPollingBuffer             int       // the number of seconds to wait before increasing the polling interval while there is no agreement on the node.
	MaxAgreementPrelaunchTimeM       int64     // The maximum numbers of minutes to wait for workload to start in an agreement

	// these Ids could be provided in config or discovered after startup by the system
	BlockchainAccountId        string
	BlockchainDirectoryAddress string
}

// This is the configuration options for Agreement bot flavor of Anax
type AGConfig struct {
	TxLostDelayTolerationSeconds int
	AgreementWorkers             int
	DBPath                       string
	Postgresql                   PostgresqlConfig // The Postgresql config if it is being used
	PartitionStale               uint64           // Number of seconds to wait before declaring a partition to be stale (i.e. the previous owner has unexpectedly terminated).
	ProtocolTimeoutS             uint64           // Number of seconds to wait before declaring proposal response is lost
	AgreementTimeoutS            uint64           // Number of seconds to wait before declaring agreement not finalized in blockchain
	NoDataIntervalS              uint64           // default should be 15 mins == 15*60 == 900. Ignored if the policy has data verification disabled.
	ActiveAgreementsURL          string           // This field is used when policy files indicate they want data verification but they dont specify a URL
	ActiveAgreementsUser         string           // This is the userid the agbot uses to authenticate to the data verifivcation API
	ActiveAgreementsPW           string           // This is the password for the ActiveAgreementsUser
	PolicyPath                   string           // The directory where policy files are kept, default /etc/provider-tremor/policy/
	NewContractIntervalS         uint64           // default should be 1
	ProcessGovernanceIntervalS   uint64           // How long the gov sleeps before general gov checks (new payloads, interval payments, etc).
	IgnoreContractWithAttribs    string           // A comma seperated list of contract attributes. If set, the contracts that contain one or more of the attributes will be ignored. The default is "ethereum_account".
	ExchangeURL                  string           // The URL of the Horizon exchange. If not configured, the exchange will not be used.
	ExchangeHeartbeat            int              // Seconds between heartbeats to the exchange
	ExchangeId                   string           // The id of the agbot, not the userid of the exchange user. Must be org qualified.
	ExchangeToken                string           // The agbot's authentication token
	DVPrefix                     string           // When looking for agreement ids in the data verification API response, look for agreement ids with this prefix.
	ActiveDeviceTimeoutS         int              // The amount of time a device can go without heartbeating and still be considered active for the purposes of search
	ExchangeMessageTTL           int              // The number of seconds the exchange will keep this message before automatically deleting it
	MessageKeyPath               string           // The path to the location of messaging keys
	MessageKeyCheck              int              // The interval (in seconds) indicating how often the agbot checks its own object in the exchange to ensure that the message key is still available.
	DefaultWorkloadPW            string           // The default workload password if none is specified in the policy file
	APIListen                    string           // Host and port for the API to listen on
	SecureAPIListenHost          string           // The host for the secure API to listen on
	SecureAPIListenPort          string           // The port for the secure API to listen on
	SecureAPIServerCert          string           // The path to the certificate file for the secure api
	SecureAPIServerKey           string           // The path to the server key file for the secure api
	PurgeArchivedAgreementHours  int              // Number of hours to leave an archived agreement in the database before automatically deleting it
	CheckUpdatedPolicyS          int              // The number of seconds to wait between checks for an updated policy file. Zero means auto checking is turned off.
	CSSURL                       string           // The URL used to access the CSS.
	CSSSSLCert                   string           // The path to the client side SSL certificate for the CSS.
	MMSGarbageCollectionInterval int64            // The amount of time to wait between MMS object cache garbage collection scans.
	AgreementBatchSize           uint64           // The number of nodes that the agbot will process in a batch.
	AgreementQueueSize           uint64           // The agreement bot work queue max size.
	FullRescanS                  uint64           // The number of seconds between policy scans when there have been no changes reported by the exchange.
	MaxExchangeChanges           int              // The maximum number of exchange changes to request on a given call the exchange /changes API.
	RetryLookBackWindow          uint64           // The time window (in seconds) used by the agbot to look backward in time for node changes when node agreements are retried.
	PolicySearchOrder            bool             // When true, search policies from most recently changed to least recently changed.
}

func (c *HorizonConfig) UserPublicKeyPath() string {
	if c.Edge.UserPublicKeyPath == "" {
		if commonPath := os.Getenv("HZN_VAR_BASE"); commonPath != "" {
			thePath := path.Join(os.Getenv("HZN_VAR_BASE"), USERKEYDIR)
			c.Edge.UserPublicKeyPath = thePath
		} else {
			return HZN_VAR_BASE_DEFAULT
		}
	}
	return c.Edge.UserPublicKeyPath
}

func (c *HorizonConfig) IsBoltDBConfigured() bool {
	return len(c.AgreementBot.DBPath) != 0
}

func (c *HorizonConfig) IsPostgresqlConfigured() bool {
	return (c.AgreementBot.Postgresql != (PostgresqlConfig{})) && (c.GetPartitionStale() != 0)
}

func (c *HorizonConfig) GetPartitionStale() uint64 {
	if c.AgreementBot.PartitionStale == 0 {
		return 60
	} else {
		return c.AgreementBot.PartitionStale
	}
}

func (c *HorizonConfig) GetAgbotCSSURL() string {
	return strings.TrimRight(c.AgreementBot.CSSURL, "/")
}

func (c *HorizonConfig) GetAgbotCSSCert() string {
	return c.AgreementBot.CSSSSLCert
}

func (c *HorizonConfig) GetAgbotAgreementBatchSize() uint64 {
	return c.AgreementBot.AgreementBatchSize
}

func (c *HorizonConfig) GetAgbotAgreementQueueSize() uint64 {
	return c.AgreementBot.AgreementQueueSize
}

func (c *HorizonConfig) GetAgbotFullRescan() uint64 {
	return c.AgreementBot.FullRescanS
}

func (c *HorizonConfig) GetAgbotRetryLookBackWindow() uint64 {
	return c.AgreementBot.RetryLookBackWindow
}

func (c *HorizonConfig) GetAgbotPolicyOrder() bool {
	return c.AgreementBot.PolicySearchOrder
}

func getDefaultBase() string {
	basePath := os.Getenv("HZN_VAR_BASE")
	if basePath == "" {
		basePath = HZN_VAR_BASE_DEFAULT
	}
	return basePath
}

// some configuration is provided by envvars; in this case we populate this config object from expected envvars
func enrichFromEnvvars(config *HorizonConfig) error {

	if exchangeURL := os.Getenv(ExchangeURLEnvvarName); exchangeURL != "" {
		config.Edge.ExchangeURL = exchangeURL
		config.AgreementBot.ExchangeURL = exchangeURL
	}

	if fssCSSURL := os.Getenv(FileSyncServiceCSSURLEnvvarName); fssCSSURL != "" {
		config.Edge.FileSyncService.CSSURL = fssCSSURL
	}

	if noDynamicPoll := os.Getenv(ExchangeMessageNoDynamicPollEnvvarName); noDynamicPoll != "" {
		config.Edge.ExchangeMessageDynamicPoll = false
	}

	if apiPort := os.Getenv(AnaxAPIPort); apiPort != "" {
		if config.Edge.APIListen != "" {
			listen := strings.Split(config.Edge.APIListen, ":")
			if len(listen) == 2 {
				config.Edge.APIListen = fmt.Sprintf("%v:%v", listen[0], apiPort)
			} else {
				config.Edge.APIListen = fmt.Sprintf("127.0.0.1:%v", apiPort)
			}
		} else {
			config.Edge.APIListen = fmt.Sprintf("127.0.0.1:%v", apiPort)
		}
	} else {
		if config.Edge.APIListen == "" {
			config.Edge.APIListen = fmt.Sprintf("127.0.0.1:%v", AnaxAPIPortDefault)
		}
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
				DefaultHTTPClientTimeoutS:      HTTPRequestTimeoutS,
				ExchangeMessageDynamicPoll:     true,
				ExchangeMessagePollInterval:    ExchangeMessagePollInterval_DEFAULT,
				ExchangeMessagePollMaxInterval: ExchangeMessagePollMaxInterval_DEFAULT,
				ExchangeMessagePollIncrement:   ExchangeMessagePollIncrement_DEFAULT,
				MaxAgreementPrelaunchTimeM:     EdgeMaxAgreementPrelaunchTimeM_DEFAULT,
			},
			AgreementBot: AGConfig{
				MessageKeyCheck:     AgbotMessageKeyCheck_DEFAULT,
				AgreementBatchSize:  AgbotAgreementBatchSize_DEFAULT,
				AgreementQueueSize:  AgbotAgreementQueueSize_DEFAULT,
				FullRescanS:         AgbotFullRescan_DEFAULT,
				MaxExchangeChanges:  AgbotMaxChanges_DEFAULT,
				RetryLookBackWindow: AgbotRetryLookBackWindow_DEFAULT,
				PolicySearchOrder:   AgbotPolicySearchOrder_DEFAULT,
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
		if config.Edge.ServiceUpgradeCheckIntervalS == 0 {
			config.Edge.ServiceUpgradeCheckIntervalS = 300
		}

		if config.Edge.NodeCheckIntervalS == 0 {
			config.Edge.NodeCheckIntervalS = 15
		}

		if config.Edge.NodePolicyCheckIntervalS == 0 {
			config.Edge.NodePolicyCheckIntervalS = 15
		}

		if config.Edge.SurfaceErrorCheckIntervalS == 0 {
			config.Edge.SurfaceErrorCheckIntervalS = 15
		}

		if config.Edge.SurfaceErrorAgreementPersistentS == 0 {
			config.Edge.SurfaceErrorAgreementPersistentS = 90
		}

		// set default retry parameters
		// the default DefaultServiceRetryCount is 2. It means 2 tries including the original one.
		// so it is actually 1 retry.
		if config.Edge.DefaultServiceRetryCount == 0 {
			config.Edge.DefaultServiceRetryCount = 2
		}
		if config.Edge.DefaultServiceRetryDuration == 0 {
			config.Edge.DefaultServiceRetryDuration = 600
		}

		// default InitialPollingBuffer
		if config.Edge.InitialPollingBuffer == 0 {
			config.Edge.InitialPollingBuffer = 120
		}

		// add a slash at the back of the ExchangeUrl
		if config.Edge.ExchangeURL != "" {
			config.Edge.ExchangeURL = strings.TrimRight(config.Edge.ExchangeURL, "/") + "/"
		}
		if config.AgreementBot.ExchangeURL != "" {
			config.AgreementBot.ExchangeURL = strings.TrimRight(config.AgreementBot.ExchangeURL, "/") + "/"
		}

		// add a slash at the back of the PolicyPath
		if config.Edge.PolicyPath != "" {
			config.Edge.PolicyPath = strings.TrimRight(config.Edge.PolicyPath, "/") + "/"
		}

		if config.AgreementBot.PolicyPath != "" {
			config.AgreementBot.PolicyPath = strings.TrimRight(config.AgreementBot.PolicyPath, "/") + "/"
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

		if config.AgreementBot.MMSGarbageCollectionInterval == 0 {
			config.AgreementBot.MMSGarbageCollectionInterval = 300
		}

		// success at last!
		return &config, nil
	}
}

func (c *HorizonConfig) String() string {
	return fmt.Sprintf("Edge: {%v}, AgreementBot: {%v}, Collaborators: {%v}, ArchSynonyms: {%v}", c.Edge.String(), c.AgreementBot.String(), c.Collaborators.String(), c.ArchSynonyms)
}

func (con *Config) String() string {
	return fmt.Sprintf("ServiceStorage %v"+
		", APIListen %v"+
		", DBPath %v"+
		", DockerEndpoint %v"+
		", DockerCredFilePath %v"+
		", DefaultCPUSet %v"+
		", DefaultServiceRegistrationRAM: %v"+
		", StaticWebContent: %v"+
		", PublicKeyPath: %v"+
		", TrustSystemCACerts: %v"+
		", CACertsPath: %v"+
		", ExchangeURL: %v"+
		", DefaultHTTPClientTimeoutS: %v"+
		", PolicyPath: %v"+
		", ExchangeHeartbeat: %v"+
		", AgreementTimeoutS: %v"+
		", DVPrefix: %v"+
		", RegistrationDelayS: %v"+
		", ExchangeMessageTTL: %v"+
		", ExchangeMessageDynamicPoll: %v"+
		", ExchangeMessagePollInterval: %v"+
		", ExchangeMessagePollMaxInterval: %v"+
		", ExchangeMessagePollIncrement: %v"+
		", UserPublicKeyPath: %v"+
		", ReportDeviceStatus: %v"+
		", TrustCertUpdatesFromOrg: %v"+
		", TrustDockerAuthFromOrg: %v"+
		", ServiceUpgradeCheckIntervalS: %v"+
		", MultipleAnaxInstances: %v"+
		", DefaultServiceRetryCount: %v"+
		", DefaultServiceRetryDuration: %v"+
		", NodeCheckIntervalS: %v"+
		", FileSyncService: {%v}"+
		", InitialPollingBuffer: {%v}"+
		", BlockchainAccountId: %v"+
		", BlockchainDirectoryAddress %v",
		con.ServiceStorage, con.APIListen, con.DBPath, con.DockerEndpoint, con.DockerCredFilePath, con.DefaultCPUSet,
		con.DefaultServiceRegistrationRAM, con.StaticWebContent, con.PublicKeyPath, con.TrustSystemCACerts, con.CACertsPath, con.ExchangeURL,
		con.DefaultHTTPClientTimeoutS, con.PolicyPath, con.ExchangeHeartbeat, con.AgreementTimeoutS,
		con.DVPrefix, con.RegistrationDelayS, con.ExchangeMessageTTL, con.ExchangeMessageDynamicPoll, con.ExchangeMessagePollInterval,
		con.ExchangeMessagePollMaxInterval, con.ExchangeMessagePollIncrement, con.UserPublicKeyPath, con.ReportDeviceStatus,
		con.TrustCertUpdatesFromOrg, con.TrustDockerAuthFromOrg, con.ServiceUpgradeCheckIntervalS, con.MultipleAnaxInstances,
		con.DefaultServiceRetryCount, con.DefaultServiceRetryDuration, con.NodeCheckIntervalS, con.FileSyncService.String(),
		con.InitialPollingBuffer, con.BlockchainAccountId, con.BlockchainDirectoryAddress)
}

func (agc *AGConfig) String() string {
	mask := "******"
	return fmt.Sprintf("TxLostDelayTolerationSeconds: %v"+
		", AgreementWorkers: %v"+
		", DBPath: %v"+
		", Postgresql: {%v}"+
		", PartitionStale: %v"+
		", ProtocolTimeoutS: %v"+
		", AgreementTimeoutS: %v"+
		", NoDataIntervalS: %v"+
		", ActiveAgreementsURL: %v"+
		", ActiveAgreementsUser: %v"+
		", ActiveAgreementsPW: %v"+
		", PolicyPath: %v"+
		", NewContractIntervalS: %v"+
		", ProcessGovernanceIntervalS: %v"+
		", IgnoreContractWithAttribs: %v"+
		", ExchangeURL: %v"+
		", ExchangeHeartbeat: %v"+
		", ExchangeId: %v"+
		", ExchangeToken: %v"+
		", DVPrefix: %v"+
		", ActiveDeviceTimeoutS: %v"+
		", ExchangeMessageTTL: %v"+
		", MessageKeyPath: %v"+
		", DefaultWorkloadPW: %v"+
		", APIListen: %v"+
		", SecureAPIListenHost: %v"+
		", SecureAPIListenPort: %v"+
		", SecureAPIServerCert: %v"+
		", SecureAPIServerkey: %v"+
		", PurgeArchivedAgreementHours: %v"+
		", CheckUpdatedPolicyS: %v"+
		", CSSURL: %v"+
		", CSSSSLCert: %v"+
		", AgreementBatchSize: %v",
		agc.TxLostDelayTolerationSeconds, agc.AgreementWorkers, agc.DBPath, agc.Postgresql.String(),
		agc.PartitionStale, agc.ProtocolTimeoutS, agc.AgreementTimeoutS, agc.NoDataIntervalS, agc.ActiveAgreementsURL,
		agc.ActiveAgreementsUser, mask, agc.PolicyPath, agc.NewContractIntervalS, agc.ProcessGovernanceIntervalS,
		agc.IgnoreContractWithAttribs, agc.ExchangeURL, agc.ExchangeHeartbeat, agc.ExchangeId,
		mask, agc.DVPrefix, agc.ActiveDeviceTimeoutS, agc.ExchangeMessageTTL, agc.MessageKeyPath, mask, agc.APIListen,
		agc.SecureAPIListenHost, agc.SecureAPIListenPort, agc.SecureAPIServerCert, agc.SecureAPIServerKey,
		agc.PurgeArchivedAgreementHours, agc.CheckUpdatedPolicyS, agc.CSSURL, agc.CSSSSLCert, agc.AgreementBatchSize)
}
