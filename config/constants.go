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
