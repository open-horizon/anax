package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/secrets"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
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

// Available to all users within the org
func (vs *AgbotVaultSecrets) ListOrgUserSecret(user, password, org, name string) (map[string]string, error) {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("list secret %v for org %v user %v", name, org, user)))

	_, userId := cutil.SplitOrgSpecUrl(user)
	url := fmt.Sprintf("%s/v1/openhorizon/%s/user/%s/%s", vs.cfg.GetAgbotVaultURL(), org, userId, name)
	return vs.listSecret(user, password, org, name, url)
}

// Available to only org admin users
func (vs *AgbotVaultSecrets) ListOrgSecret(user, password, org, name string) (map[string]string, error) {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("list secret %v for org %v", name, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/%s/%s", vs.cfg.GetAgbotVaultURL(), org, name)
	return vs.listSecret(user, password, org, name, url)
}

// Get the secret at a specified path within the vault
func (vs *AgbotVaultSecrets) listSecret(user, password, org, name, url string) (map[string]string, error) {

	// Login the user to ensure that the vault ACLs can take effect
	userVaultToken, err := vs.loginUser(user, password, org)
	if err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to login user %s, error: %v", user, err), Details: "", RespCode: http.StatusUnauthorized}
	}

	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, http.MethodGet, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to list %s secret %s, error: %v", org, name, err), Details: "", RespCode: http.StatusServiceUnavailable}
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to read secret %s from %s, error: %v", name, org, err), Details: "", RespCode: http.StatusInternalServerError}
	} else if httpCode := resp.StatusCode; httpCode == http.StatusNotFound {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Secret does not exist. Vault response %s", string(respBytes)), Details: "", RespCode: http.StatusNotFound}
	} else if httpCode == http.StatusForbidden {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Secret not available with the specified credentials. Vault response %s", string(respBytes)), Details: "", RespCode: http.StatusForbidden}
	} else if httpCode != http.StatusOK {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to find secret %s for org %s, HTTP status code: %v", name, org, httpCode), Details: "", RespCode: httpCode}
	}

	respMsg := ListSecretResponse{}
	if err := json.Unmarshal(respBytes, &respMsg); err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to parse response body %v", err), Details: "", RespCode: http.StatusInternalServerError}
	}

	glog.V(3).Infof(vaultPluginLogString("Done reading secret value."))

	return respMsg.Data, nil
}

// List all secrets at a specified path in vault. Available only to org admin users.
func (vs *AgbotVaultSecrets) ListOrgSecrets(user, password, org string) ([]string, error) {

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("listing secrets in %v", org)))

	// Login the user to ensure that the vault ACLs can take effect
	userVaultToken, err := vs.loginUser(user, password, org)
	if err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to login user %s, error: %v", user, err), Details: "", RespCode: http.StatusUnauthorized}
	}

	// Query the vault using the user's credentials
	url := fmt.Sprintf("%s/v1/openhorizon/%s?list=true", vs.cfg.GetAgbotVaultURL(), org)

	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, http.MethodGet, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to list %s secrets, error: %v", org, err), Details: "", RespCode: http.StatusServiceUnavailable}
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to read list %s secrets response, error: %v", org, err), Details: "", RespCode: http.StatusInternalServerError}
	} else if httpCode := resp.StatusCode; httpCode == http.StatusNotFound {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to list secrets for %s. Vault response %s ", org, string(respBytes)), Details: string(respBytes), RespCode: http.StatusNotFound}
	} else if httpCode == http.StatusForbidden {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to list secrets for %s. Vault response %s ", org, string(respBytes)), Details: string(respBytes), RespCode: http.StatusForbidden}
	} else if httpCode != http.StatusOK {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to list secrets for %s, HTTP status code: %v", org, httpCode), Details: "", RespCode: httpCode}
	}

	glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("list %s secrets response: %v", org, string(respBytes))))

	respMsg := ListSecretsResponse{}
	if err := json.Unmarshal(respBytes, &respMsg); err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to parse response %v", string(respBytes)), Details: "", RespCode: http.StatusInternalServerError}
	}

	glog.V(3).Infof(vaultPluginLogString("Done listing secrets."))

	return respMsg.Data.Keys, nil
}

// Available to all users within the org
func (vs *AgbotVaultSecrets) CreateOrgUserSecret(user, password, org, vaultSecretName string, data secrets.CreateSecretRequest) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("create secret %s for org %s for user %s", vaultSecretName, org, user)))

	_, userId := cutil.SplitOrgSpecUrl(user)
	url := fmt.Sprintf("%s/v1/openhorizon/%s/user/%s/%s", vs.cfg.GetAgbotVaultURL(), org, userId, vaultSecretName)
	return vs.createSecret(user, password, org, vaultSecretName, url, data)
}

// Available to only org admin users
func (vs *AgbotVaultSecrets) CreateOrgSecret(user, password, org, vaultSecretName string, data secrets.CreateSecretRequest) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("create secret %s for org %s", vaultSecretName, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/%s/%s", vs.cfg.GetAgbotVaultURL(), org, vaultSecretName)
	return vs.createSecret(user, password, org, vaultSecretName, url, data)
}

// This utility will be used to create secrets.
func (vs *AgbotVaultSecrets) createSecret(user, password, org, vaultSecretName, url string, data secrets.CreateSecretRequest) error {

	userVaultToken, err := vs.loginUser(user, password, org)
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to login user %s, error %v", user, err), Details: "", RespCode: http.StatusUnauthorized}
	}

	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, http.MethodPost, map[string]string{data.Key: data.Value})
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to create secret, error %v", err), Details: "", RespCode: http.StatusServiceUnavailable}
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to read response, error: %v", err), Details: "", RespCode: http.StatusInternalServerError}
	} else if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to create secret for %s, error %v", org, string(respBytes)), Details: "", RespCode: resp.StatusCode}
	}
	glog.V(3).Infof(vaultPluginLogString("Done creating secret in vault."))

	return nil
}

// Available to all users within the org
func (vs *AgbotVaultSecrets) DeleteOrgUserSecret(user, password, org, name string) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("delete secret %s for org %s for user %s", name, org, user)))

	_, userId := cutil.SplitOrgSpecUrl(user)
	url := fmt.Sprintf("%s/v1/openhorizon/%s/user/%s/%s", vs.cfg.GetAgbotVaultURL(), org, userId, name)
	return vs.deleteSecret(user, password, org, name, url)
}

// Available to only org admin users
func (vs *AgbotVaultSecrets) DeleteOrgSecret(user, password, org, name string) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("delete secret %s for org %s", name, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/%s/%s", vs.cfg.GetAgbotVaultURL(), org, name)
	return vs.deleteSecret(user, password, org, name, url)
}

// This utility will be used to delete secrets.
func (vs *AgbotVaultSecrets) deleteSecret(user, password, org, name, url string) error {

	userVaultToken, err := vs.loginUser(user, password, org)
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to login user %s, error %v", user, err), Details: "", RespCode: http.StatusUnauthorized}
	}

	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, http.MethodDelete, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to delete secret, error %v", err), Details: "", RespCode: http.StatusServiceUnavailable}
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to read response, error: %v", err), Details: "", RespCode: http.StatusInternalServerError}
	} else if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to delete secret for %s, error %v", org, string(respBytes)), Details: "", RespCode: resp.StatusCode}
	}
	glog.V(3).Infof(vaultPluginLogString("Done deleting secret in vault."))

	return nil
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
