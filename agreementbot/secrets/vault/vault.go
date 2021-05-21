package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/secrets"
	"github.com/open-horizon/anax/config"
	"io/ioutil"
	"net/http"
)

// This function registers an uninitialized agbot secrets implementation with the secrets plugin registry. The plugin's Initialize
// method is used to configure the object.
func init() {
	secrets.Register("vault", new(AgbotVaultSecrets))
}

// The fields in this object are initialized in the Initialize method in this package.
type AgbotVaultSecrets struct {
	token      string       // The identity of this agbot in the vault
	httpClient *http.Client // A cached http client to use for invoking the vault
	cfg        *config.HorizonConfig
}

func (vs *AgbotVaultSecrets) String() string {
	return fmt.Sprintf("Token: %v", vs.token)
}

func (vs *AgbotVaultSecrets) ListOrgSecret(user, password, org, name string) ([]string, error) {
	return []string{}, nil
}

func (vs *AgbotVaultSecrets) ListOrgSecrets(user, password, org string) ([]string, error) {

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("listing secrets in %v", org)))

	// Login the user to ensure that the vault ACLs can take effect
	userVaultToken, err := vs.loginUser(user, password, org)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to login user %s, error: %v", user, err))
	}

	// Query the vault using the user's credentials
	url := fmt.Sprintf("%s/v1/openhorizon/%s?list=true", vs.cfg.GetAgbotVaultURL(), org)

	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, http.MethodGet, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to list %s secrets, error: %v", org, err))
	}

	httpCode := resp.StatusCode
	if httpCode == http.StatusNotFound {
		return []string{}, nil
	}

	if httpCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("unable to list %s secrets, HTTP status code: %v", org, httpCode))
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read list %s secrets response, error: %v", org, err))
	}

	glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("list %s secrets response: %v", org, string(respBytes))))

	respMsg := ListSecretsResponse{}
	err = json.Unmarshal(respBytes, &respMsg)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to parse response %v", string(respBytes)))
	}

	glog.V(3).Infof(vaultPluginLogString("done listing secrets."))

	return respMsg.Data.Keys, nil

}

func (vs *AgbotVaultSecrets) loginUser(user, password, org string) (string, error) {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("logging in to vault as %s", user)))

	// Login to the vault using user creds
	url := fmt.Sprintf("%s/v1/auth/openhorizon/login", vs.cfg.GetAgbotVaultURL())

	body := LoginBody{
		Id:    user,
		Token: password,
	}

	resp, err := vs.invokeVaultWithRetry("", url, http.MethodPost, body)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return "", errors.New(fmt.Sprintf("agbot unable to login user %s, error: %v", user, err))
	}

	httpCode := resp.StatusCode
	if httpCode != http.StatusOK && httpCode != http.StatusCreated {
		return "", errors.New(fmt.Sprintf("agbot unable to login user %s, HTTP status code: %v", user, httpCode))
	}

	// Save the login token
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New(fmt.Sprintf("unable to read login response, error: %v", err))
	}

	glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("login response: %v", string(respBytes))))

	respMsg := LoginResponse{}
	err = json.Unmarshal(respBytes, &respMsg)
	if err != nil {
		return "", errors.New(fmt.Sprintf("unable to parse response %v", string(respBytes)))
	}

	return respMsg.Auth.ClientToken, nil
}

// Log string prefix api
var vaultPluginLogString = func(v interface{}) string {
	return fmt.Sprintf("Vault Plugin: %v", v)
}
