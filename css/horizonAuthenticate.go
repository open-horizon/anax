package css

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/edge-sync-service/core/security"
	"github.com/open-horizon/edge-utilities/logger"
	"github.com/open-horizon/edge-utilities/logger/log"
	"github.com/open-horizon/edge-utilities/logger/trace"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// Set this env var to a value that will be used to identify the http header that contains the user identity, when this
// plugin is running behind something that is doing the authentication.
const CSS_PRE_AUTHENTICATED_IDENTITY = "CSS_PRE_AUTHENTICATED_HEADER"

// The env var that holds the exchange API endpoint when this authenticator should use the exchange to do authentication.
const HZN_EXCHANGE_URL = "HZN_EXCHANGE_URL"

// The env var that holds the path to an SSL CA certificate that should be used when accessing the exchange API.
const HZN_EXCHANGE_CA_CERT = "HZN_EXCHANGE_CA_CERT"

const EX_ROOT_USER = "root"
const EX_ORG_ADMIN = "exchange_org_admin"
const EX_HUB_ADMIN = "exchange_hub_admin"

const EX_MAX_RETRY = 5
const EX_RETRY_INTERVAL = 2

// HorizonAuthenticate is the Horizon plugin for authentication used by the Cloud sync service (CSS). This plugin
// can be used in environments where authentication is handled by something else in the network before
// the CSS or where the CSS itself is deployed with a public facing API and so this plugin utilizes the exchange
// to authenticate users.
type HorizonAuthenticate struct {
	httpClient *http.Client
}

// Start initializes the HorizonAuthenticate plugin.
func (auth *HorizonAuthenticate) Start() {

	// Make sure one of the authentication behaviors has been clearly chosen.
	if AlreadyAuthenticatedIdentityHeader() != "" && ExchangeURL() != "" {
		panic(fmt.Sprintf("Must not specify both %v=%v and %v=%v.", CSS_PRE_AUTHENTICATED_IDENTITY, AlreadyAuthenticatedIdentityHeader(), HZN_EXCHANGE_URL, ExchangeURL()))
	} else if AlreadyAuthenticatedIdentityHeader() == "" && ExchangeURL() == "" {
		panic(fmt.Sprintf("Must specify an environment variable to indicate authentication behavior, either %v or %v.", CSS_PRE_AUTHENTICATED_IDENTITY, HZN_EXCHANGE_URL))
	}

	// Setup for the authentication method that was chosen.
	if id := AlreadyAuthenticatedIdentityHeader(); id == "" {
		var err error
		auth.httpClient, err = newHTTPClient(ExchangeCACert())
		if err != nil {
			panic(fmt.Sprintf("Unable to create HTTP client, error %v", err))
		}
		if log.IsLogging(logger.INFO) {
			log.Info(cssALS("starting with exchange authenticated identity"))
		}
	} else {
		if log.IsLogging(logger.INFO) {
			log.Info(cssALS(fmt.Sprintf("starting with pre-authenticated identities in header: %v", id)))
		}
	}
	return
}

func AlreadyAuthenticatedIdentityHeader() string {
	return os.Getenv(CSS_PRE_AUTHENTICATED_IDENTITY)
}

func ExchangeURL() string {
	return os.Getenv(HZN_EXCHANGE_URL)
}

func ExchangeCACert() string {
	return os.Getenv(HZN_EXCHANGE_CA_CERT)
}

// Authenticate authenticates a particular appKey/appSecret pair and indicates
// whether it is an edge node, an agbot, an org admin, plain user, or node user. Also returned is the
// user's org and identity.
//
// When this authenticator is using the exchange to authenticate, the expected form for an appKey is:
// <org>/<destination type>/<destination id> - for a node identity, where destination type is mapped to a pattern in horizon and destination id is the node id.
// <org>/<agbot id> - for an agbot identity, where agbot id is the agbot's exchange Id.
// <org>/<user> - for a real person user
// <org>/<node id> for a node user
//
// When this authenticator is allowing something infront of it in the network to do the authentication, the expected form for an appKey is irrelevant.
// What's important is what's in the HTTP request header:
// the CSS_PRE_AUTHENTICATED_IDENTITY header will contain the identity
// the "type" header will contain "dev" for a node or "person" for a user
// the "orgid" header will contain the orgid
//

// Returns authentication result code, the user's org and id.
func (auth *HorizonAuthenticate) Authenticate(request *http.Request) (int, string, string) {

	if request == nil {
		if log.IsLogging(logger.ERROR) {
			log.Error(cssALS(fmt.Sprintf("called with a nil HTTP request")))
		}
		return security.AuthFailed, "", ""
	}

	appKey, appSecret, ok := request.BasicAuth()
	if !ok {
		if log.IsLogging(logger.ERROR) {
			log.Error(cssALS(fmt.Sprintf("unable to extract basic auth information")))
		}
		return security.AuthFailed, "", ""
	}

	// If the exchange is being used for authentication, then use the env var to access the exchange endpoint.
	if exURL := ExchangeURL(); exURL != "" {
		return auth.authenticateWithExchange(request.URL.Path, appKey, appSecret, exURL)

	} else {
		// Otherwise use the env var to know which header to access for the authenticated identity.
		return auth.authenticationAlreadyDone(request, AlreadyAuthenticatedIdentityHeader())

	}
}

// KeyandSecretForURL returns an app key and an app secret pair to be
// used by the ESS when communicating with the specified URL. This method is not needed in the CSS.
func (auth *HorizonAuthenticate) KeyandSecretForURL(url string) (string, string) {
	return "", ""
}

// Internal function used to separate the code for authenticating with the exchange away from the main
// Authenticate function.
func (auth *HorizonAuthenticate) authenticateWithExchange(otherOrg string, appKey string, appSecret string, exURL string) (int, string, string) {
	if log.IsLogging(logger.DEBUG) {
		log.Debug(cssALS(fmt.Sprintf("received exchange authentication request for URL Path %v user %v", otherOrg, appKey)))
	}
	if trace.IsLogging(logger.TRACE) {
		trace.Debug(cssALS(fmt.Sprintf("received exchange authentication request for URL Path %v for user %v with secret %v", otherOrg, appKey, appSecret)))
	}

	// Assume the request will be rejected.
	authCode := security.AuthFailed
	authOrg := ""
	authId := ""

	// If the appKey is shaped like a node identity, then let's make sure it is a node identity.
	if parts := strings.Split(appKey, "/"); len(parts) == 3 {

		// A 3 part '/' delimited identity has to be a node identity.
		if trace.IsLogging(logger.TRACE) {
			trace.Debug(cssALS(fmt.Sprintf("authentication request for user %v appears to be a node identity", appKey)))
		}

		if err := auth.verifyNodeIdentity(parts[2], parts[0], appSecret, ExchangeURL()); err != nil {
			if log.IsLogging(logger.ERROR) {
				log.Error(cssALS(fmt.Sprintf("unable to verify identity %v, error %v", appKey, err)))
			}
		} else {
			authCode = security.AuthEdgeNode
			authOrg = parts[0]                 //orgId
			authId = parts[1] + "/" + parts[2] //destinationType/destinationId (pattern/nodeId)
		}

	} else if parts := strings.Split(appKey, "/"); len(parts) == 2 {
		// If the appKey is shaped like a user identity, a node user identity or an agbot identity, then let's make sure it is one of these.
		// The identity is checked for agbot first because it is expected to be an agbot most of the time.

		// A 2 part '/' delimited identity could be an agbot identity.
		if trace.IsLogging(logger.TRACE) {
			trace.Debug(cssALS(fmt.Sprintf("attempting authentication request as an agbot %v", appKey)))
		}

		// Agbots are admins by default. If an error is returned, check if the identity is a user.
		if err := auth.verifyAgbotIdentity(parts[1], parts[0], appSecret, ExchangeURL()); err == nil {
			// We have a valid agbot identity. Agbots only call a few of the APIs. Verify that it's one of these:
			// org - this is the API used to query for object policies
			if !strings.Contains(otherOrg, "/") {
				authCode = security.AuthSyncAdmin // This makes the agbot a super user in the CSS so that it can query multiple orgs.
				authOrg = otherOrg
				authId = parts[1]
			}

		} else {
			// Check if the identity is a user, since we know its not an agbot. If an error is returned, check if the identity is a node user.
			if trace.IsLogging(logger.WARNING) {
				log.Warning(cssALS(fmt.Sprintf("unable to verify identity %v as agbot, error %v", appKey, err)))
			}
			if trace.IsLogging(logger.TRACE) {
				trace.Debug(cssALS(fmt.Sprintf("attempting authentication request as a user %v", appKey)))
			}
			// appkey: {org}/{username} or {org}/iamapikey.
			// parts[1] is {username} or iamapikey, parts[0] is {orgId}
			if exchangeRole, username, err := auth.verifyUserIdentity(parts[1], parts[0], appSecret, ExchangeURL()); err != nil {
				if log.IsLogging(logger.WARNING) {
					log.Warning(cssALS(fmt.Sprintf("unable to verify identity %v as user, error %v", appKey, err)))
				}
				if trace.IsLogging(logger.TRACE) {
					trace.Debug(cssALS(fmt.Sprintf("attempting authentication request as an exchange node %v", appKey)))
				}
				// Check if the identity is an exchange node
				// appkey: {org}/{nodeId}. appSecret is {nodeToken}.
				// parts[0] is {orgId}, parts[1] is {nodeId}
				if err := auth.verifyNodeIdentity(parts[1], parts[0], appSecret, ExchangeURL()); err != nil {
					if log.IsLogging(logger.ERROR) {
						log.Error(cssALS(fmt.Sprintf("unable to verify identity %v as exchange node, error %v", appKey, err)))
					}
				} else {
					// exchange node is AuthNodeUser. Without configuring ACLs, AuthNodeUser only has read access to public objects of any org
					authCode = security.AuthNodeUser
					authOrg = parts[0]
					authId = parts[1]
				}
			} else if exchangeRole == EX_ROOT_USER || exchangeRole == EX_HUB_ADMIN {
				// root/root:{pwd} and hub admin are super users across all orgs
				authCode = security.AuthSyncAdmin
				authOrg = parts[0]
				authId = username
			} else {
				// exchange regular users are always mapped to authAdmin so that ACLs do not need to be configured in order for a regular user to get read/write access to objects in their own org.
				// It also gives them read access to public objects in other orgs without needing an ACL
				authCode = security.AuthAdmin
				authOrg = parts[0]
				authId = username
			}
		}

	} else {
		if log.IsLogging(logger.ERROR) {
			log.Error(cssALS(fmt.Sprintf("request identity %v is not in a supported format, must be either <org>/<destination type>/<destination id> for a node, <org>/<agbot id> for an agbot, <org>/<user id> for a user, or <org>/<node id> for a node user.", appKey)))
		}
	}

	// Log the results of the authentication.
	if log.IsLogging(logger.DEBUG) {
		log.Debug(cssALS(fmt.Sprintf("returned exchange authentication result code %v org %v id %v", authCode, authOrg, authId)))
	}
	return authCode, authOrg, authId
}

type UserDefinition struct {
	Password    string `json:"password"`
	Admin       bool   `json:"admin"`
	HubAdmin    bool   `json:"hubAdmin"`
	Email       string `json:"email"`
	LastUpdated string `json:"lastUpdated,omitempty"`
	UpdatedBy   string `json:"updatedBy,omitempty"`
}

type GetUsersResponse struct {
	Users     map[string]UserDefinition `json:"users"`
	LastIndex int                       `json:"lastIndex"`
}

// Returns ${exchange_auth}, ${username}, nil for users that are valid user. Returns "", "", error for invalid user
// ${exchange_auth} values: EX_ROOT_USER, EX_HUB_ADMIN, EX_ORG_ADMIN, ""
func (auth *HorizonAuthenticate) verifyUserIdentity(id string, orgId string, appSecret string, exURL string) (string, string, error) {

	// Log which API we're about to use.
	url := fmt.Sprintf("%v/orgs/%v/users/%v", exURL, orgId, id)
	apiMsg := fmt.Sprintf("%v %v", http.MethodGet, url)
	if trace.IsLogging(logger.TRACE) {
		trace.Debug(cssALS(fmt.Sprintf("checking exchange %v", apiMsg)))
	}

	// Invoke the exchange API to verify the user.
	user := fmt.Sprintf("%v/%v", orgId, id)
	resp, err := auth.invokeExchangeWithRetry(url, user, appSecret)

	// If there was an error invoking the HTTP API, return it.
        if err != nil {
                return "", "", err
        }

	// Make sure the response reader is closed if we exit quickly.
	defer resp.Body.Close()

	// Log the HTTP response code.
	if trace.IsLogging(logger.TRACE) {
		trace.Debug(cssALS(fmt.Sprintf("received HTTP code: %d", resp.StatusCode)))
	}

	// If the response code was not expected, then return the error.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 {
			return "", "", errors.New(fmt.Sprintf("unable to verify user %v in the exchange, HTTP code %v, either the user is undefined or the user's password is incorrect.", user, resp.StatusCode))
		} else {
			return "", "", errors.New(fmt.Sprintf("unable to verify user %v in the exchange, HTTP code %v", user, resp.StatusCode))
		}
	} else {
		users := new(GetUsersResponse)
		if bodyBytes, err := ioutil.ReadAll(resp.Body); err != nil {
			return "", "", errors.New(fmt.Sprintf("unable to read HTTP response to %v, error %v", apiMsg, err))
		} else if err = json.Unmarshal(bodyBytes, users); err != nil {
			return "", "", errors.New(fmt.Sprintf("failed to unmarshal exchange body response from %s to user: %v", apiMsg, err))
		} else {
			for key, userInfo := range users.Users {
				// key is {orgid}/{username}
				orgAndUsername := strings.Split(key, "/")
				if len(orgAndUsername) != 2 {
					return "", "", errors.New(fmt.Sprintf("exchange user %s in invalid format, should be {orgId}/{username}", key))
				}
				exOrgId := orgAndUsername[0]
				exUsername := orgAndUsername[1]

				if exOrgId == EX_ROOT_USER && exUsername == EX_ROOT_USER {
					// exchange root user (root/root:{pwd}), should be authSyncAdmin in CSS
					return EX_ROOT_USER, EX_ROOT_USER, nil
				} else if userInfo.HubAdmin {
					// hubAdmin should be authSyncAdmin in CSS
					return EX_HUB_ADMIN, exUsername, nil
				} else if userInfo.Admin {
					// exchange org admin should be authAdmin in CSS
					return EX_ORG_ADMIN, exUsername, nil
				} else if exOrgId == orgId {
					// authUser for regular user {org}/iamapikey:{apikey} ({org}/{username}:{pwd})
					return "", exUsername, nil
				} else {
					return "", "", errors.New(fmt.Sprintf("no exchange user found %v", apiMsg))
				}

			}

		}

		return "", "", errors.New(fmt.Sprintf("no exchange user found %v", apiMsg))

	}
}

type GetAgbotsResponse struct {
	Agbots    map[string]UserDefinition `json:"agbots"`
	LastIndex int                       `json:"lastIndex"`
}

// Returns nil for agbots, and error otherwise.
func (auth *HorizonAuthenticate) verifyAgbotIdentity(id string, orgId string, appSecret string, exURL string) error {

	// Log which API we're about to use.
	url := fmt.Sprintf("%v/orgs/%v/agbots/%v", exURL, orgId, id)
	apiMsg := fmt.Sprintf("%v %v", http.MethodGet, url)
	if trace.IsLogging(logger.TRACE) {
		trace.Debug(cssALS(fmt.Sprintf("checking exchange %v", apiMsg)))
	}

	// Invoke the exchange API to verify the user.
	agbot := fmt.Sprintf("%v/%v", orgId, id)
	resp, err := auth.invokeExchangeWithRetry(url, agbot, appSecret)

	// If there was an error invoking the HTTP API, return it.
        if err != nil {
                return err
        }

	// Make sure the response reader is closed if we exit quickly.
	defer resp.Body.Close()

	// Log the HTTP response code.
	if trace.IsLogging(logger.TRACE) {
		trace.Debug(cssALS(fmt.Sprintf("received HTTP code: %d", resp.StatusCode)))
	}

	// If the response code was not expected, then return the error.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 {
			return errors.New(fmt.Sprintf("unable to verify agbot %v in the exchange, HTTP code %v, either the agbot is undefined or the agbot's token is incorrect.", agbot, resp.StatusCode))
		} else {
			return errors.New(fmt.Sprintf("unable to verify agbot %v in the exchange, HTTP code %v", agbot, resp.StatusCode))
		}
	} else {
		agbots := new(GetAgbotsResponse)

		// Read in the response object and check if this is an agbot or not.
		if outBytes, err := ioutil.ReadAll(resp.Body); err != nil {
			return errors.New(fmt.Sprintf("unable to read HTTP response to %v, error %v", apiMsg, err))
		} else if err := json.Unmarshal(outBytes, agbots); err != nil {
			return errors.New(fmt.Sprintf("unable to demarshal response %v from %v, error %v", string(outBytes), apiMsg, err))
		} else if _, ok := agbots.Agbots[agbot]; !ok {
			return errors.New(fmt.Sprintf("agbot %v was not returned in response to %v", agbot, apiMsg))
		} else {
			return nil
		}

	}
}

type GetNodesResponse struct {
	Nodes     map[string]interface{} `json:"nodes"`
	LastIndex int                    `json:"lastIndex"`
}

// Returns nil for valid nodes, otherwise error.
func (auth *HorizonAuthenticate) verifyNodeIdentity(id string, orgId string, appSecret string, exURL string) error {

	// Log which API we're about to use.
	url := fmt.Sprintf("%v/orgs/%v/nodes/%v", exURL, orgId, id)
	apiMsg := fmt.Sprintf("%v %v", http.MethodGet, url)
	if trace.IsLogging(logger.TRACE) {
		trace.Debug(cssALS(fmt.Sprintf("checking exchange %v", apiMsg)))
	}

	// Invoke the exchange API to verify the node.
	node := fmt.Sprintf("%v/%v", orgId, id)
	resp, err := auth.invokeExchangeWithRetry(url, node, appSecret)

	// If there was an error invoking the HTTP API, return it.
        if err != nil {
                return err
        }

	// Make sure the response reader is closed if we exit quickly.
	defer resp.Body.Close()

	// Log the HTTP response code.
	if trace.IsLogging(logger.TRACE) {
		trace.Debug(cssALS(fmt.Sprintf("received HTTP code: %d", resp.StatusCode)))
	}

	// If the response code was not expected, then return the error.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 {
			return errors.New(fmt.Sprintf("unable to verify node %v in the exchange, HTTP code %v, either the node is undefined or the node's token is probably incorrect.", node, resp.StatusCode))
		} else {
			return errors.New(fmt.Sprintf("unable to verify node %v in the exchange, HTTP code %v", node, resp.StatusCode))
		}
	} else {
		nodes := new(GetNodesResponse)

		// Read in the response object and check if this node is in it.
		if outBytes, err := ioutil.ReadAll(resp.Body); err != nil {
			return errors.New(fmt.Sprintf("unable to read HTTP response to %v, error %v", apiMsg, err))
		} else if err := json.Unmarshal(outBytes, nodes); err != nil {
			return errors.New(fmt.Sprintf("unable to demarshal response %v from %v, error %v", string(outBytes), apiMsg, err))
		} else if _, ok := nodes.Nodes[node]; !ok {
			return errors.New(fmt.Sprintf("node %v was not returned in response to %v", node, apiMsg))
		} else {
			return nil
		}

	}
}

// Common function to invoke the Exchange API when checking for valid users and nodes.
func (auth *HorizonAuthenticate) invokeExchange(url string, user string, pw string) (*http.Response, error) {

	apiMsg := fmt.Sprintf("%v %v", http.MethodGet, url)

	// Create an outgoing HTTP request for the exchange.
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to create HTTP request for %v, error %v", apiMsg, err))
	}

	// Add the basic auth header so that the exchange will authenticate.
	req.SetBasicAuth(user, pw)
	req.Header.Add("Accept", "application/json")
	req.Close = true

	// Send the request to verify the user.
	resp, err := auth.httpClient.Do(req)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to send HTTP request for %v, error %v", apiMsg, err))
	} else {
		return resp, nil
	}
}

// Common function to invoke the Exchange API with retry
func (auth *HorizonAuthenticate) invokeExchangeWithRetry(url string, user string, pw string) (*http.Response, error) {
	fmt.Println("in invokeExchangeWithRetry")
	var currRetry int
	var resp *http.Response
	var err error
	for currRetry = EX_MAX_RETRY; currRetry > 0; {
		resp, err = auth.invokeExchange(url, user, pw)

		// Log the HTTP response code.
		if trace.IsLogging(logger.TRACE) {
			if resp == nil {
				trace.Debug(cssALS(fmt.Sprintf("received nil response")))
			} else {
				trace.Debug(cssALS(fmt.Sprintf("received HTTP code: %d", resp.StatusCode)))
			}
		}

		if err == nil {
			break
		}
		if exchange.IsTransportError(resp, err) {
			// Log the transport error and retry
			if trace.IsLogging(logger.TRACE) {
				trace.Debug(cssALS(fmt.Sprintf("received transport error, retry...")))
			}

			currRetry--
			time.Sleep(time.Duration(EX_RETRY_INTERVAL) * time.Second)
		} else {
			return resp, err
		}
	}

	if currRetry == 0 {
		return resp, errors.New(fmt.Sprintf("unable to verify %v in the exchange, exceeded %v retries", user, EX_MAX_RETRY))
	}

	return resp, err
}

// Create an https connection, using a supplied SSL CA certificate.
func newHTTPClient(certPath string) (*http.Client, error) {
	var caBytes []byte

	if certPath != "" {
		var err error
		caBytes, err = ioutil.ReadFile(certPath)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("unable to read %v, error %v", certPath, err))
		}
		if log.IsLogging(logger.INFO) {
			log.Info(cssALS(fmt.Sprintf("read CA cert from provided file %v", certPath)))
		}
	}

	var tlsConf tls.Config
	tlsConf.InsecureSkipVerify = false
	// do not allow negotiation to previous versions of TLS
	tlsConf.MinVersion = tls.VersionTLS12

	var certPool *x509.CertPool

	var err error
	certPool, err = x509.SystemCertPool()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to get system cert pool, error %v", err))
	}

	certPool.AppendCertsFromPEM(caBytes)
	tlsConf.RootCAs = certPool

	if trace.IsLogging(logger.TRACE) {
		trace.Debug(cssALS(fmt.Sprintf("added CA Cert %v to trust", certPath)))
	}

	tlsConf.BuildNameToCertificate()

	return &http.Client{
		// remember that this timouet is for the whole request, including
		// body reading. This means that you must set the timeout according
		// to the total payload size you expect
		Timeout: time.Second * time.Duration(20),
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   60 * time.Second,
				KeepAlive: 120 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   20 * time.Second,
			ResponseHeaderTimeout: 20 * time.Second,
			ExpectContinueTimeout: 8 * time.Second,
			MaxIdleConns:          20,
			IdleConnTimeout:       120 * time.Second,
			TLSClientConfig:       &tlsConf,
		},
	}, nil

}

// Internal function used to separate the code for authenticating with the exchange away from the main
// Authenticate function. This implementation assumes that the authentication info is in headers as parsed
// out by the netowrk device in front of the CSS. The Basic Auth header is not interesting anymore.
func (auth *HorizonAuthenticate) authenticationAlreadyDone(request *http.Request, idHeaderName string) (int, string, string) {

	if log.IsLogging(logger.DEBUG) {
		log.Debug(cssALS(fmt.Sprintf("request header type %v", request.Header.Get("type"))))
		log.Debug(cssALS(fmt.Sprintf("request header orgId %v", request.Header.Get("orgId"))))
		log.Debug(cssALS(fmt.Sprintf("request header %v %v", idHeaderName, request.Header.Get(idHeaderName))))

		user, pw, _ := request.BasicAuth()
		log.Debug(cssALS(fmt.Sprintf("request basic auth header id %v", user)))
		log.Debug(cssALS(fmt.Sprintf("request basic auth header pw %v", pw)))
	}

	if request.Header.Get("type") == "person" {
		return security.AuthAdmin, request.Header.Get("orgId"), request.Header.Get(idHeaderName)
	} else if request.Header.Get("type") == "dev" {
		return security.AuthEdgeNode, request.Header.Get("orgId"), request.Header.Get(idHeaderName)
	}

	return security.AuthFailed, "", ""
}

// Logging function
var cssALS = func(v interface{}) string {
	return fmt.Sprintf("Horizon Authenticator %v", v)
}
