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
const HZN_FSS_DOMAIN_SOCKET = "fssapi.sock"

// The default relative path of files downloaded by the sync service. This path should be combined with the HZN_VAR_BASE_DEFAULT.
const HZN_FSS_STORAGE_PATH = "fss-store"

// The relative path of authentication credentials used by services to access the sync service. This path should be combined with the HZN_VAR_BASE_DEFAULT.
const HZN_FSS_AUTH_PATH = "fss-auth"

// The name of the file mount that a service uses to find its FSS credential file.
const HZN_FSS_AUTH_MOUNT = HZN_FSS_AUTH_PATH