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

// This is the time format used by the vault.
const VaultTimeFormat = "2006-01-02T15:04:05.999999999Z"

// The fields in this object are initialized in the Initialize method in this package.
type AgbotVaultSecrets struct {
	token                string       // The identity of this agbot in the vault
	httpClient           *http.Client // A cached http client to use for invoking the vault
	cfg                  *config.HorizonConfig
	lastVaultInteraction uint64
}

func (vs *AgbotVaultSecrets) String() string {
	return fmt.Sprintf("Token: %v", vs.token)
}

// Available to all users within the org
func (vs *AgbotVaultSecrets) ListOrgUserSecret(user, password, org, name string) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("list secret %v in org %v as user %v", name, org, user)))

	_, userId := cutil.SplitOrgSpecUrl(user)
	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s/user/%s/%s", vs.cfg.GetAgbotVaultURL(), org, userId, name)
	return vs.listSecret(user, password, org, name, url)
}

// Available to only org admin users
func (vs *AgbotVaultSecrets) ListOrgSecret(user, password, org, name string) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("list secret %v in org %v", name, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s/%s", vs.cfg.GetAgbotVaultURL(), org, name)
	return vs.listSecret(user, password, org, name, url)
}

// Get the secret at a specified path within the vault
func (vs *AgbotVaultSecrets) listSecret(user, password, org, name, url string) error {

	// Login the user to ensure that the vault ACLs can take effect
	userVaultToken, err := vs.loginUser(user, password, org)
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to login user %s, error: %v", user, err), Details: "", RespCode: http.StatusUnauthorized}
	}

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("listing secret %s in org %s as user %s", name, org, user)))
	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, http.MethodGet, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to list %s secret %s, error: %v", org, name, err), Details: "", RespCode: http.StatusServiceUnavailable}
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to read secret %s from %s, error: %v", name, org, err), Details: "", RespCode: http.StatusInternalServerError}
	} else if httpCode := resp.StatusCode; httpCode == http.StatusNotFound {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Secret does not exist."), Details: "", RespCode: http.StatusNotFound}
	} else if httpCode == http.StatusForbidden {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Secret not available with the specified credentials. Vault response %s", string(respBytes)), Details: "", RespCode: http.StatusForbidden}
	} else if httpCode != http.StatusOK {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to find secret %s for org %s, HTTP status code: %v", name, org, httpCode), Details: "", RespCode: httpCode}
	}

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("done listing %s.", name)))

	return nil
}

// List all secrets at a specified path in vault. Available only to org admin users.
func (vs *AgbotVaultSecrets) ListOrgSecrets(user, password, org string) ([]string, error) {

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("list secrets in %v", org)))

	// Login the user to ensure that the vault ACLs can take effect
	userVaultToken, err := vs.loginUser(user, password, org)
	if err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to login user %s, error: %v", user, err), Details: "", RespCode: http.StatusUnauthorized}
	}

	// Query the vault using the user's credentials
	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s", vs.cfg.GetAgbotVaultURL(), org)

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("listing secrets in org %s as %s", org, user)))
	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, "LIST", nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to list %s secrets, error: %v", org, err), Details: "", RespCode: http.StatusServiceUnavailable}
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if respBytes != nil {
		glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("HTTP: %v, listing %s secrets response: %v", resp.StatusCode, org, string(respBytes))))
	}

	if err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to read list %s secrets response, error: %v", org, err), Details: "", RespCode: http.StatusInternalServerError}
	} else if httpCode := resp.StatusCode; httpCode == http.StatusNotFound {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("No secrets for %s.", org), Details: "", RespCode: http.StatusNotFound}
	} else if httpCode == http.StatusForbidden {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to list secrets for %s. Vault response %s ", org, string(respBytes)), Details: string(respBytes), RespCode: http.StatusForbidden}
	} else if httpCode != http.StatusOK {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to list secrets for %s, HTTP status code: %v", org, httpCode), Details: "", RespCode: httpCode}
	}

	respMsg := ListSecretsResponse{}
	if err := json.Unmarshal(respBytes, &respMsg); err != nil {
		return nil, secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to parse response %v", string(respBytes)), Details: "", RespCode: http.StatusInternalServerError}
	}

	// filter out user/ and empty secret directories
	secrets := []string{}
	for _, secret := range respMsg.Data.Keys {
		if secret == "user/" {
			// filter out the user/ directory for user level secrets
			continue
		} else if secret[len(secret)-1] == '/' {
			// filter out empty directories
			res, listErr := vs.ListOrgSecrets(user, password, org+"/"+(secret[:len(secret)-1]))
			if listErr != nil && len(res) == 0 {
				continue
			} else {
				// non-empty secret directory
				secrets = append(secrets, secret)
			}
		} else {
			// secret
			secrets = append(secrets, secret)
		}
	}

	glog.V(3).Infof(vaultPluginLogString("done listing secrets."))

	return secrets, nil
}

// Available to all users within the org
func (vs *AgbotVaultSecrets) CreateOrgUserSecret(user, password, org, vaultSecretName string, data secrets.SecretDetails) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("creating secret %s in org %s as user %s", vaultSecretName, org, user)))

	_, userId := cutil.SplitOrgSpecUrl(user)
	url := fmt.Sprintf("%s/v1/openhorizon/data/%s/user/%s/%s", vs.cfg.GetAgbotVaultURL(), org, userId, vaultSecretName)
	return vs.createSecret(user, password, org, vaultSecretName, url, data)
}

// Available to only org admin users
func (vs *AgbotVaultSecrets) CreateOrgSecret(user, password, org, vaultSecretName string, data secrets.SecretDetails) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("creating secret %s in org %s", vaultSecretName, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/data/%s/%s", vs.cfg.GetAgbotVaultURL(), org, vaultSecretName)
	return vs.createSecret(user, password, org, vaultSecretName, url, data)
}

// This utility will be used to create secrets.
func (vs *AgbotVaultSecrets) createSecret(user, password, org, vaultSecretName, url string, data secrets.SecretDetails) error {

	userVaultToken, err := vs.loginUser(user, password, org)
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to login user %s, error %v", user, err), Details: "", RespCode: http.StatusUnauthorized}
	}

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("create secret %s in org %s as user %s", vaultSecretName, org, user)))

	body := SecretCreateRequest{
		Data: data,
	}

	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, http.MethodPost, body)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to create secret, error %v", err), Details: "", RespCode: http.StatusServiceUnavailable}
	}

	respBytes, err := ioutil.ReadAll(resp.Body)

	if respBytes != nil {
		glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("HTTP: %v, creating secret response: %v", resp.StatusCode, string(respBytes))))
	}

	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to read response, error: %v", err), Details: "", RespCode: http.StatusInternalServerError}
	} else if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to create secret for %s, error %v", org, string(respBytes)), Details: "", RespCode: resp.StatusCode}
	}
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("done creating %s in vault.", vaultSecretName)))

	return nil
}

// Available to all users within the org
func (vs *AgbotVaultSecrets) DeleteOrgUserSecret(user, password, org, name string) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("delete secret %s in org %s as user %s", name, org, user)))

	_, userId := cutil.SplitOrgSpecUrl(user)
	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s/user/%s/%s", vs.cfg.GetAgbotVaultURL(), org, userId, name)
	return vs.deleteSecret(user, password, org, name, url)
}

// Available to only org admin users
func (vs *AgbotVaultSecrets) DeleteOrgSecret(user, password, org, name string) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("delete secret %s in org %s", name, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s/%s", vs.cfg.GetAgbotVaultURL(), org, name)
	return vs.deleteSecret(user, password, org, name, url)
}

// This utility will be used to delete secrets.
func (vs *AgbotVaultSecrets) deleteSecret(user, password, org, name, url string) error {

	userVaultToken, err := vs.loginUser(user, password, org)
	if err != nil {
		return secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to login user %s, error %v", user, err), Details: "", RespCode: http.StatusUnauthorized}
	}

	// check if the secret exists before deleting (if the secret doesn't exist, return 404)
	listErr := vs.listSecret(user, password, org, name, url)
	if listErr != nil {
		return listErr
	}

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("deleting secret %s in org %s as user %s", name, org, user)))
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
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("done deleting %s in vault.", name)))

	return nil
}

func (vs *AgbotVaultSecrets) GetSecretDetails(org, secretUser, secretName string) (res secrets.SecretDetails, err error) {

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("extract secret details for %s in org %s as user %s", secretName, org, secretUser)))

	res = secrets.SecretDetails{}
	err = nil

	if org == "" {
		err = secrets.ErrorResponse{Msg: "Organization name must not be an empty string", Details: "", RespCode: http.StatusBadRequest}
		return
	}

	if secretName == "" {
		err = secrets.ErrorResponse{Msg: "Secret name must not be an empty string", Details: "", RespCode: http.StatusBadRequest}
		return
	}

	url := fmt.Sprintf("%s/v1/openhorizon/data/%s/", vs.cfg.GetAgbotVaultURL(), org)
	if secretUser != "" {
		url += fmt.Sprintf("user/%s/%s", secretUser, secretName)
	} else {
		url += secretName
	}

	resp, err := vs.invokeVaultWithRetry(vs.token, url, http.MethodGet, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to read %s secret %s, error: %v", org, secretName, err), Details: "", RespCode: http.StatusServiceUnavailable}
		return
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	// if respBytes != nil {
	// 	glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("HTTP: %v, details response: %v", resp.StatusCode, string(respBytes))))
	// }

	if err != nil {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to read secret %s from %s, error: %v", secretName, org, err), Details: "", RespCode: http.StatusInternalServerError}
		return
	} else if httpCode := resp.StatusCode; httpCode == http.StatusNotFound {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Secret does not exist. Vault response %s", string(respBytes)), Details: "", RespCode: http.StatusNotFound}
		return
	} else if httpCode == http.StatusForbidden {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Secret not available with the specified credentials. Vault response %s", string(respBytes)), Details: "", RespCode: http.StatusForbidden}
		return
	} else if httpCode != http.StatusOK {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to find secret %s for org %s, HTTP status code: %v", secretName, org, httpCode), Details: "", RespCode: httpCode}
		return
	}

	r := GetSecretResponse{}
	if uerr := json.Unmarshal(respBytes, &r); uerr != nil {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to parse response body %v", uerr), Details: "", RespCode: http.StatusInternalServerError}
		return
	}

	res = r.Data.Data
	glog.V(3).Infof(vaultPluginLogString("done extracting secret details"))

	return

}

// Retrieve the metadata for a secret.
func (vs *AgbotVaultSecrets) GetSecretMetadata(secretOrg, secretUser, secretName string) (res secrets.SecretMetadata, err error) {

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("extract secret metadata for %s in org %s as user %s", secretName, secretOrg, secretUser)))

	if secretOrg == "" {
		err = secrets.ErrorResponse{Msg: "Organization name must not be an empty string", Details: "", RespCode: http.StatusBadRequest}
		return
	}

	if secretName == "" {
		err = secrets.ErrorResponse{Msg: "Secret name must not be an empty string", Details: "", RespCode: http.StatusBadRequest}
		return
	}

	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s/", vs.cfg.GetAgbotVaultURL(), secretOrg)
	if secretUser != "" {
		url += fmt.Sprintf("user/%s/%s", secretUser, secretName)
	} else {
		url += secretName
	}

	resp, err := vs.invokeVaultWithRetry(vs.token, url, http.MethodGet, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to read %s secret %s, error: %v", secretOrg, secretName, err), Details: "", RespCode: http.StatusServiceUnavailable}
		return
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if respBytes != nil {
		glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("HTTP: %v, metadata response: %v", resp.StatusCode, string(respBytes))))
	}

	if err != nil {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to read secret %s from %s, error: %v", secretName, secretOrg, err), Details: "", RespCode: http.StatusInternalServerError}
		return
	} else if httpCode := resp.StatusCode; httpCode == http.StatusNotFound {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Secret does not exist. Vault response %s", string(respBytes)), Details: "", RespCode: http.StatusNotFound}
		return
	} else if httpCode == http.StatusForbidden {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Secret not available with the specified credentials. Vault response %s", string(respBytes)), Details: "", RespCode: http.StatusForbidden}
		return
	} else if httpCode != http.StatusOK {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to find secret %s for org %s, HTTP status code: %v", secretName, secretOrg, httpCode), Details: "", RespCode: httpCode}
		return
	}

	r := ListSecretResponse{}
	if uerr := json.Unmarshal(respBytes, &r); uerr != nil {
		err = secrets.ErrorResponse{Msg: fmt.Sprintf("Unable to parse response body %v", uerr), Details: "", RespCode: http.StatusInternalServerError}
		return
	}

	res.CreationTime = cutil.TimeInSeconds(r.Data.CreationTime, VaultTimeFormat)
	res.UpdateTime = cutil.TimeInSeconds(r.Data.UpdateTime, VaultTimeFormat)

	glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("Metadata: %v", res)))
	glog.V(3).Infof(vaultPluginLogString("done extracting secret metadata"))

	return
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
