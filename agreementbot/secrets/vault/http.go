package vault

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/secrets"
	"github.com/open-horizon/anax/config"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// Retry intervals when connecting to the vault
const EX_MAX_RETRY = 10
const EX_RETRY_INTERVAL = 2

type ErrorResponse struct {
	Errors []string `json:"errors"`
}

type LoginBody struct {
	Id    string `json:"id"`
	Token string `json:"token"`
}

type LoginAuthResponse struct {
	ClientToken   string            `json:"client_token"`
	Accessor      string            `json:"accessor"`
	Policies      []string          `json:"policies"`
	TokenPolicies []string          `json:"token_policies"`
	Metadata      map[string]string `json:"metadata"`
	LeaseDuration int               `json:"lease_duration"`
	Renewable     bool              `json:"renewable"`
	EntityId      string            `json:"entity_id"`
	TokenType     string            `json:"token_type"`
	Orphan        bool              `json:"orphan"`
}

type LoginResponse struct {
	ReqId     string            `json:"request_id"`
	LeaseId   string            `json:"lease_id"`
	Renewable bool              `json:"renewable"`
	Auth      LoginAuthResponse `json:"auth"`
}

type RenewBody struct {
	Token string `json:"token"`
}

type SecretCreateRequest struct {
	Options map[string]string     `json:"options,omitempty"`
	Data    secrets.SecretDetails `json:"data"`
}

type ListSecretResponse struct {
	Data SecretMetadata `json:"data"`
}

type KeyData struct {
	Keys []string `json:"keys"`
}

type ListSecretsResponse struct {
	Data KeyData `json:"data"`
}

type SecretData struct {
	Data secrets.SecretDetails `json:"data"`
}

type GetSecretResponse struct {
	Data SecretData `json:"data"`
}

type SecretMetadata struct {
	CreationTime string `json:"created_time"` // Has format 2018-03-22T02:24:06.945319214Z
	UpdateTime   string `json:"updated_time"`
}

// Create an https connection, using a supplied SSL CA certificate.
func (vs *AgbotVaultSecrets) newHTTPClient(cfg *config.HorizonConfig) (*http.Client, error) {

	// Consume the openhorizon hub certificate
	var err error
	var caBytes []byte
	var tlsConf tls.Config

	if _, err = os.Stat(cfg.GetVaultCertPath()); err == nil {

		caBytes, err = ioutil.ReadFile(cfg.GetVaultCertPath())
		if err != nil {
			return nil, errors.New(fmt.Sprintf("unable to read %v, error %v", cfg.GetVaultCertPath(), err))
		}

		// Setup the TLS config if there is a cert.
		tlsConf.InsecureSkipVerify = false

		// Do not allow negotiation to previous versions of TLS.
		tlsConf.MinVersion = tls.VersionTLS12

		certPool := x509.NewCertPool()

		certPool.AppendCertsFromPEM(caBytes)
		tlsConf.RootCAs = certPool

		tlsConf.BuildNameToCertificate()
	}

	return &http.Client{
		// remember that this timeout is for the whole request, including
		// body reading. This means that you must set the timeout according
		// to the total payload size you expect
		Timeout: time.Second * time.Duration(20),
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   60 * time.Second,
				KeepAlive: 120 * time.Second,
			}).Dial,
			ResponseHeaderTimeout: 20 * time.Second,
			ExpectContinueTimeout: 8 * time.Second,
			// Guidance from https://www.loginradius.com/blog/engineering/tune-the-go-http-client-for-high-performance/
			MaxIdleConns:          20,
			MaxConnsPerHost:       20,
			MaxIdleConnsPerHost:   20,
			IdleConnTimeout:       120 * time.Second,
			TLSClientConfig:       &tlsConf,
		},
	}, nil

}

// Common function to invoke the Vault API with builtin retry logic.
func (vs *AgbotVaultSecrets) invokeVaultWithRetry(token string, url string, method string, body interface{}) (*http.Response, error) {
	var currRetry int
	var resp *http.Response
	var err error
	for currRetry = EX_MAX_RETRY; currRetry > 0; {
		resp, err = vs.invokeVault(token, url, method, body)

		// Log the HTTP response code.
		if resp == nil {
			glog.Warningf(vaultPluginLogString("received nil response from vault"))
		}

		if resp != nil {
			glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("received HTTP code: %d", resp.StatusCode)))
		}

		if err == nil {
			break
		}

		// If the invocation resulted in a retryable network error, log it and retry the exchange invocation.
		if isTransportError(resp, err) {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			// Log the transport error and retry
			glog.Warningf(vaultPluginLogString("received transport error, retry..."))

			currRetry--
			time.Sleep(time.Duration(EX_RETRY_INTERVAL) * time.Second)
		} else if token == "" && resp != nil && resp.StatusCode == http.StatusForbidden {
			// The agbot failed to authenticate, something must have happened to the agbot's token, so login again to get a new token.
			glog.Warningf(vaultPluginLogString("unexpected agbot token expiration, logging in again"))
			err := vs.Login()
			if err != nil {
				glog.Warningf(vaultPluginLogString(fmt.Sprintf("error logging in trying to recover expired token: %v", err)))
			}

			// Decrement the retry count so that we dont accidentally loop forever.
			currRetry--

			// No need to delay the retry because this error was not a transport problem.

		} else {
			return resp, err
		}
	}

	if currRetry == 0 {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return resp, errors.New(fmt.Sprintf("unable to invoke %v %v in the vault, exceeded %v retries", url, method, EX_MAX_RETRY))
	}

	return resp, err
}


// Common function to invoke the Vault API.
func (vs *AgbotVaultSecrets) invokeVault(token string, url string, method string, body interface{}) (*http.Response, error) {

	apiMsg := fmt.Sprintf("%v %v", method, url)

	var requestBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to marshal body %s for %s, error: %v", body, apiMsg, err))
		}
		requestBody = bytes.NewBuffer(jsonBytes)
	}

	// Create an outgoing HTTP request for the vault.
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to create HTTP request for %v, error %v", apiMsg, err))
	}


	if token != "" {
		req.Header.Add("X-Vault-Token", token)
	}

	// Close code commented out so we can reuse connection and avoid SSL handshake
	//req.Close = true

	// Send the request to the vault.
	resp, err := vs.httpClient.Do(req)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to send HTTP request for %v, error %v", apiMsg, err))
	} else {
		// NOTE: 500, 502, 503 are the only server error status codes returned by the vault API
		// https://www.vaultproject.io/api#http-status-codes
		if resp.StatusCode < 500 {
			vs.lastVaultInteraction = uint64(time.Now().Unix())
		}
		return resp, nil
	}
}

// Return true if an exchange invocation resulted in an error that is retryable. In general, errors which
// result from network level problems can be retried due the transient nature of these errors, especially
// if the exchange is under heavy load.
func isTransportError(pResp *http.Response, err error) bool {
	if err != nil {
		if strings.Contains(err.Error(), ": EOF") {
			return true
		}

		l_error_string := strings.ToLower(err.Error())
		if strings.Contains(l_error_string, "time") && strings.Contains(l_error_string, "out") {
			return true
		} else if strings.Contains(l_error_string, "connection") && (strings.Contains(l_error_string, "refused") || strings.Contains(l_error_string, "reset")) {
			return true
		}
	}

	if pResp != nil {
		if pResp.StatusCode == http.StatusBadGateway {
			// 502: bad gateway error
			return true
		} else if pResp.StatusCode == http.StatusGatewayTimeout {
			// 504: gateway timeout
			return true
		} else if pResp.StatusCode == http.StatusServiceUnavailable {
			//503: service unavailable
			return true
		}
	}
	return false
}
