package config

// ENVVAR_PREFIX is used when Anax sets envvars in orchestrated containers
const ENVVAR_PREFIX = "HZN_"

const USERKEYDIR = "/userkeys"

// container start execution timeout for a microservice upgrade
const MICROSERVICE_EXEC_TIMEOUT = 180

// MaxHTTPIdleConnections see https://golang.org/pkg/net/http/
const MaxHTTPIdleConnections = 20

// HTTPIdleConnectionTimeoutS see https://golang.org/pkg/net/http/
const HTTPIdleConnectionTimeoutS = 120

const HZN_VAR_BASE_DEFAULT = "/var/horizon"

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

// The number of seconds between polls to the CSS for updates.
const HZN_FSS_POLLING_RATE = 60
