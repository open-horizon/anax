package config

// ENVVAR_PREFIX is used when Anax sets envvars in orchestrated containers
const ENVVAR_PREFIX = "HZN_"

const USERKEYDIR = "/userkeys"

// container start execution timeout for a microservice upgrade
const MICROSERVICE_EXEC_TIMEOUT = 180

// MaxHTTPIdleConnections see https://golang.org/pkg/net/http/
const MaxHTTPIdleConnections = 20

// HTTPRequestTimeoutS see https://golang.org/pkg/net/http/
const HTTPRequestTimeoutS = 30

// HTTPRequestTimeoutOverride environment variable
const HTTPRequestTimeoutOverride = "HZN_HTTP_TIMEOUT"

// HTTPIdleConnectionTimeoutS see https://golang.org/pkg/net/http/
const HTTPIdleConnectionTimeoutS = 120

const HZN_VAR_BASE_DEFAULT = "/var/horizon"

// The default location for ess authentication and secret manager files
const HZN_VAR_RUN_BASE_DEFAULT = "/var/run/horizon"

// The path to the agent's unix domain socket for the file sync service
const HZN_FSS_DOMAIN_SOCKET_PATH = "/var/run/horizon"
const HZN_FSS_DOMAIN_SOCKET = "essapi.sock"

// The default listen address for the FSS over https
const HZN_FSS_API_LISTEN_DEFAULT = "localhost"
const HZN_FSS_API_LISTEN_PORT_DEFAULT = 8443

// The default relative path of files downloaded by the sync service. This path should be combined with the HZN_VAR_BASE_DEFAULT.
const HZN_FSS_STORAGE_PATH = "ess-store"

// The relative path of authentication credentials used by services to access the sync service. This path should be combined with the HZN_VAR_BASE_DEFAULT.
const HZN_FSS_AUTH_PATH = "ess-auth"

// The name of the file mount that a service uses to find its FSS credential file.
const HZN_FSS_AUTH_MOUNT = "/" + HZN_FSS_AUTH_PATH

// The name of the authentication file that a service can use to authenticate to the FSS (ESS) API.
const HZN_FSS_AUTH_FILE = "auth.json"

// The relative path of SSL client certificate used by services to access the sync service.
const HZN_FSS_CERT_PATH = "ess-cert"

// The name of the file mount that a service uses to find its FSS SSl client certificate.
const HZN_FSS_CERT_MOUNT = "/" + HZN_FSS_CERT_PATH

// The name of the SSL certificate file that a service can use to make an SSL connection to the FSS (ESS) API.
const HZN_FSS_CERT_FILE = "cert.pem"

// The name of the SSL certificate key file that the ESS uses to establish an SSL listener.
const HZN_FSS_CERT_KEY_FILE = "key.pem"

// The number of seconds between polls to the CSS for updates.
const HZN_FSS_POLLING_RATE = 60

// The buffer size of object queue to send notifications
const HZN_FSS_OBJECT_QUEUE_BUFFER_SIZE = 2

// The HTTP client timeout in seconds for ESS
const HZN_FSS_HTTP_ESS_CLIENT_TIMEOUT = 120

// The http client timeout for downloading models (or objects) in seconds for ESS
const HZN_FSS_HTTP_ESS_OBJ_CLIENT_TIMEOUT = 600

// The chunk size of data transferring between CSS and agent
const HZN_FSS_MAX_CHUNK_SIZE = 5242880 

// The name of the folder where secrets from the agreement protocol will be stored within a workload container
const HZN_SECRETS_MOUNT = "/open-horizon-secrets"

// The Default starting exchange message polling interval.
const ExchangeMessagePollInterval_DEFAULT = 20

// The Default message poll interval maximum.
const ExchangeMessagePollMaxInterval_DEFAULT = 120

// The Default message poll increment size.
const ExchangeMessagePollIncrement_DEFAULT = 20

// The maximum numbers of minutes to wait for workload to start in an agreement
const EdgeMaxAgreementPrelaunchTimeM_DEFAULT = 10

// The Default interval at which the agbot verifies that its message key is present in the exchange.
const AgbotMessageKeyCheck_DEFAULT = 60

// The Default anax API port number
const AnaxAPIPortDefault = "8510"

// The default agreement batch size. This is essentially the maximum number of results that will be returned in a search call.
const AgbotAgreementBatchSize_DEFAULT = 300

// The default max agreement bot work queue size. This is essentially the maximum queue depth for a given agbot protocol worker pool.
const AgbotAgreementQueueSize_DEFAULT = 300

// The default scaling factor applied to Agreement Queue size inorder to keep the message queue full.
const AgbotMessageQueueScale_DEFAULT = 33.0

// The default number of prioritized queue history records to keep before aging out the old ones.
const AgbotQueueHistorySize_DEFAULT = 30

// The default full rescan interval
const AgbotFullRescan_DEFAULT = 600

// The maximum number of changes to retrieve at once from the exchange
const AgbotMaxChanges_DEFAULT = 1000

// Retry lookback window
const AgbotRetryLookBackWindow_DEFAULT = 3600

// Policy search order
const AgbotPolicySearchOrder_DEFAULT = true

// Scale factor of node max hb interval to wait before declaring an a agreement for that node did not finalize
const AgreementTimeoutScaleFactor_DEFAULT = 2

// Scale factor of node max hb interval to wait before declaring a proposal response is lost for that node
const AgbotProtocolTimeoutScaleFactor_DEFAULT = 2

// Time to allow a kube agent to attempt to install a custom resource before timing out
const K8sCRInstallTimeoutS_DEFAULT = 180

// Time between secret update checks
const SecretsUpdateCheck_DEFAULT = 60
