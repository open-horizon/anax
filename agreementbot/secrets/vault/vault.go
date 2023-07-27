package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/secrets"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"io/ioutil"
	"net/http"
	"strings"
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
func (vs *AgbotVaultSecrets) ListOrgUserSecret(user, token, org, path string) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("list secret %v in org %v as user %v", path, org, user)))

	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
	return vs.listSecret(user, token, org, path, url)
}

// Available to all users within the org
func (vs *AgbotVaultSecrets) ListOrgSecret(user, token, org, path string) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("list secret %v in org %v", path, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
	return vs.listSecret(user, token, org, path, url)
}

// Available to all users in the org
func (vs *AgbotVaultSecrets) ListOrgNodeSecret(user, token, org, path string) error {
        glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("list secret %v in org %v", path, org)))
        url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
        return vs.listSecret(user, token, org, path, url)
}

// Available to admins and the user that owns the secret
func (vs *AgbotVaultSecrets) ListUserNodeSecret(user, token, org, path string) error {
        glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("list secret %v in org %v as user %v", path, org, user)))
        url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
        return vs.listSecret(user, token, org, path, url)
}

// Get the secret at a specified path within the vault
func (vs *AgbotVaultSecrets) listSecret(user, token, org, name, url string) error {

	// Login the user to ensure that the vault ACLs can take effect
	userVaultToken, exUser, err := vs.loginUser(user, token, org)
	if err != nil {
		return &secrets.Unauthenticated{LoginError: err, ExchangeUser: user}
	}

	// invoke the vault to get the secret details
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("listing secret %s in org %s as user %s", name, org, exUser)))
	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, http.MethodGet, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return &secrets.SecretsProviderUnavailable{ProviderError: err}
	}

	// parse the vault response
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &secrets.InvalidResponse{ReadError: err, HttpMethod: http.MethodGet, SecretPath: url}
	} else {
		glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("HTTP: %v, listing %s secret response: %v", resp.StatusCode, org, string(respBytes))))
	}

	// check for error
	httpCode := resp.StatusCode
	if httpCode != http.StatusOK {
		// parse the vault response
		var vaultResponse map[string][]string
		perr := json.Unmarshal(respBytes, &vaultResponse)
		if perr != nil {
			return &secrets.InvalidResponse{ParseError: perr, Response: respBytes, HttpMethod: http.MethodGet, SecretPath: url}
		}

		// check the response code
		if httpCode := resp.StatusCode; httpCode == http.StatusNotFound {
			return &secrets.NoSecretFound{Response: vaultResponse, SecretPath: url}
		} else if httpCode == http.StatusForbidden {
			return &secrets.PermissionDenied{Response: vaultResponse, HttpMethod: http.MethodGet, SecretPath: url, ExchangeUser: exUser}
		} else if httpCode == http.StatusBadRequest {
			return &secrets.BadRequest{Response: vaultResponse, HttpMethod: http.MethodGet, SecretPath: url}
		} else {
			return &secrets.Unknown{Response: vaultResponse, ResponseCode: httpCode, HttpMethod: http.MethodGet, SecretPath: url}
		}
	}

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("done listing %s.", name)))

	return nil
}

// List all org-level secrets at a specified path in vault.
func (vs *AgbotVaultSecrets) ListOrgSecrets(user, token, org, path string) ([]string, error) {

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("list secrets in %v", org)))

	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("listing secrets in org %s", org)))
	return vs.listSecrets(user, token, org, url, path)
}

// List all user-level secrets at a specified path in vault.
func (vs *AgbotVaultSecrets) ListOrgUserSecrets(user, token, org, path string) ([]string, error) {

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("listing secrets for user %v in %v", user, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
	secrets, err := vs.listSecrets(user, token, org, url, path)
	if err != nil {
		return nil, err
	}

	// trim the user/<user> prefix from the names
	secretList := make([]string, 0)
	for _, secret := range secrets {
		secretList = append(secretList, strings.TrimPrefix(secret, path+"/"))
	}
	return secretList, nil
}

// List all org-level node secrets at a specified path in vault.
func (vs *AgbotVaultSecrets) ListOrgNodeSecrets(user, token, org, node, path string) ([]string, error) {

        glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("listing secrets for node %v in %v", node, org)))

        url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
        secrets, err := vs.listSecrets(user, token, org, url, path)
        if err != nil {
                return nil, err
        }

        // trim the node/<node> prefix from the names
        secretList := make([]string, 0)
        for _, secret := range secrets {
                secretList = append(secretList, strings.TrimPrefix(secret, path+"/"))
        }
        return secretList, nil
}

// List all user-level node secrets at a specified path in vault.
func (vs *AgbotVaultSecrets) ListUserNodeSecrets(user, token, org, node, path string) ([]string, error) {

        glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("listing secrets for node %v in %v as user %v", node, org, user)))

        url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
        secrets, err := vs.listSecrets(user, token, org, url, path)
        if err != nil {
                return nil, err
        }

        // trim the user/<user>/node/<node> prefix from the names
        secretList := make([]string, 0)
        for _, secret := range secrets {
                secretList = append(secretList, strings.TrimPrefix(secret, path+"/"))
        }
        return secretList, nil
}

// the input queue is a list of secret names and directories. this function gathers the secret names provided
// and recurses on the directories to output a list of secret names with the input path as the directory
// of the names in the input queue ("" if top-level vault directory)
func (vs *AgbotVaultSecrets) gatherSecretNames(user, token, org, path string, queue []string) []string {

	secretNames := make([]string, 0)

	// go through the queue and check for directories and names
	for _, secret := range queue {
		newPath := path + secret
		if secret[len(secret)-1] == '/' {
			// secret directory
			secretList, err := vs.ListOrgSecrets(user, token, org, newPath[:len(newPath)-1])
			if err == nil {
				secretNames = append(secretNames, secretList...)
			}
		} else {
			// secret name
			secretNames = append(secretNames, newPath)
		}
	}

	return secretNames
}

// List the secrets at a specified path within the vault
func (vs *AgbotVaultSecrets) listSecrets(user, token, org, url, path string) ([]string, error) {

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("url: %s", url)))

	// Login the user to ensure that the vault ACLs can take effect
	userVaultToken, exUser, err := vs.loginUser(user, token, org)
	if err != nil {
		return nil, &secrets.Unauthenticated{LoginError: err, ExchangeUser: user}
	}

	// invoke the vault to get the secret list
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("listing secrets as user %s", exUser)))
	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, "LIST", nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, &secrets.SecretsProviderUnavailable{ProviderError: err}
	}

	// parse the vault response
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, &secrets.InvalidResponse{ReadError: err, HttpMethod: "LIST", SecretPath: url}
	} else {
		glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("HTTP: %v, listing %s secrets response: %v", resp.StatusCode, org, string(respBytes))))
	}

	// check for error
	httpCode := resp.StatusCode
	if httpCode != http.StatusOK {
		// parse the vault response
		var vaultResponse map[string][]string
		perr := json.Unmarshal(respBytes, &vaultResponse)
		if perr != nil {
			return nil, &secrets.InvalidResponse{ParseError: perr, Response: respBytes, HttpMethod: "LIST", SecretPath: url}
		}

		// check the response code
		if httpCode := resp.StatusCode; httpCode == http.StatusNotFound {
			return nil, &secrets.NoSecretFound{Response: vaultResponse, SecretPath: url}
		} else if httpCode == http.StatusForbidden {
			return nil, &secrets.PermissionDenied{Response: vaultResponse, HttpMethod: "LIST", SecretPath: url, ExchangeUser: exUser}
		} else if httpCode == http.StatusBadRequest {
			return nil, &secrets.BadRequest{Response: vaultResponse, HttpMethod: "LIST", SecretPath: url}
		} else {
			return nil, &secrets.Unknown{Response: vaultResponse, ResponseCode: httpCode, HttpMethod: "LIST", SecretPath: url}
		}
	}

	// parse the vault response into the secret list
	respMsg := ListSecretsResponse{}
	if err := json.Unmarshal(respBytes, &respMsg); err != nil {
		return nil, &secrets.InvalidResponse{ParseError: err, Response: respBytes, HttpMethod: "LIST", SecretPath: url}
	}

	// filter out user/ directory if top-level
	secrets := make([]string, 0)
	if path == "" {
		for _, secret := range respMsg.Data.Keys {
			if secret != "user/" && secret != "node/" {
				secrets = append(secrets, secret)
			}
		}
	} else {
		secrets = respMsg.Data.Keys
	}

	// gather all the multi-part secret names
	runningPath := ""
	if path != "" {
		runningPath = path + "/"
	}
	secretList := vs.gatherSecretNames(user, token, org, runningPath, secrets)

	return secretList, nil
}

// Available to all users within the org
func (vs *AgbotVaultSecrets) CreateOrgUserSecret(user, token, org, path string, data secrets.SecretDetails) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("creating secret %s in org %s", path, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/data/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
	return vs.createSecret(user, token, org, path, url, data)
}

// Available to only org admin users
func (vs *AgbotVaultSecrets) CreateOrgSecret(user, token, org, path string, data secrets.SecretDetails) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("creating secret %s in org %s", path, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/data/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
	return vs.createSecret(user, token, org, path, url, data)
}

// Available only to org admins
func (vs *AgbotVaultSecrets) CreateOrgNodeSecret(user, token, org, path string, data secrets.SecretDetails) error {
        glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("creating secret %s in org %s", path, org)))

        url := fmt.Sprintf("%s/v1/openhorizon/data/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
        return vs.createSecret(user, token, org, path, url, data)
}

// Available only to all users in an org
func (vs *AgbotVaultSecrets) CreateUserNodeSecret(user, token, org, path string, data secrets.SecretDetails) error {
        glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("creating secret %s in org %s", path, org)))

        url := fmt.Sprintf("%s/v1/openhorizon/data/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
        return vs.createSecret(user, token, org, path, url, data)
}

// This utility will be used to create secrets.
func (vs *AgbotVaultSecrets) createSecret(user, token, org, vaultSecretName, url string, data secrets.SecretDetails) error {

	// Login the user to ensure that the vault ACLs can take effect
	userVaultToken, exUser, err := vs.loginUser(user, token, org)
	if err != nil {
		return &secrets.Unauthenticated{LoginError: err, ExchangeUser: user}
	}

	body := SecretCreateRequest{
		Data: data,
	}

	// invoke the vault to get the secret details
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("create secret %s in org %s as user %s", vaultSecretName, org, exUser)))
	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, http.MethodPost, body)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return &secrets.SecretsProviderUnavailable{ProviderError: err}
	}

	// parse the vault response
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &secrets.InvalidResponse{ReadError: err, HttpMethod: http.MethodPost, SecretPath: url}
	} else {
		glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("HTTP: %v, creating secret response: %v", resp.StatusCode, string(respBytes))))
	}

	// check for error
	httpCode := resp.StatusCode
	if httpCode != http.StatusCreated && httpCode != http.StatusOK && httpCode != http.StatusNoContent {
		// parse the vault response
		var vaultResponse map[string][]string
		perr := json.Unmarshal(respBytes, &vaultResponse)
		if perr != nil {
			return &secrets.InvalidResponse{ParseError: perr, Response: respBytes, HttpMethod: http.MethodPost, SecretPath: url}
		}

		// check the response code
		if httpCode := resp.StatusCode; httpCode == http.StatusForbidden {
			return &secrets.PermissionDenied{Response: vaultResponse, HttpMethod: http.MethodPost, SecretPath: url, ExchangeUser: exUser}
		} else if httpCode == http.StatusBadRequest {
			return &secrets.BadRequest{Response: vaultResponse, HttpMethod: http.MethodPost, SecretPath: url, RequestBody: &data}
		} else {
			return &secrets.Unknown{Response: vaultResponse, ResponseCode: httpCode, HttpMethod: http.MethodPost, SecretPath: url}
		}
	}

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("done creating %s in vault.", vaultSecretName)))

	return nil
}

// Available to all users within the org
func (vs *AgbotVaultSecrets) DeleteOrgUserSecret(user, token, org, path string) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("delete secret %s in org %s", path, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
	return vs.deleteSecret(user, token, org, path, url)
}

// Available to only org admin users
func (vs *AgbotVaultSecrets) DeleteOrgSecret(user, token, org, path string) error {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("delete secret %s in org %s", path, org)))

	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
	return vs.deleteSecret(user, token, org, path, url)
}

// Available to only org admin users
func (vs *AgbotVaultSecrets) DeleteOrgNodeSecret(user, token, org, path string) error {
        glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("delete secret %s in org %s", path, org)))

        url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
        return vs.deleteSecret(user, token, org, path, url)
}

// Available to all users in the org
func (vs *AgbotVaultSecrets) DeleteUserNodeSecret(user, token, org, path string) error {
        glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("delete secret %s in org %s", path, org)))

        url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s"+cliutils.AddSlash(path), vs.cfg.GetAgbotVaultURL(), org)
        return vs.deleteSecret(user, token, org, path, url)
}

// This utility will be used to delete secrets.
func (vs *AgbotVaultSecrets) deleteSecret(user, token, org, name, url string) error {

	// Login the user to ensure that the vault ACLs can take effect
	userVaultToken, exUser, err := vs.loginUser(user, token, org)
	if err != nil {
		return &secrets.Unauthenticated{LoginError: err, ExchangeUser: user}
	}

	// check if the secret exists before deleting (if the secret doesn't exist, return 404)
	listErr := vs.listSecret(user, token, org, name, url)
	if listErr != nil {
		return listErr
	}

	// invoke the vault to remove the secret
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("deleting secret %s in org %s as user %s", name, org, exUser)))
	resp, err := vs.invokeVaultWithRetry(userVaultToken, url, http.MethodDelete, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return &secrets.SecretsProviderUnavailable{ProviderError: err}
	}

	// parse the vault response
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &secrets.InvalidResponse{ReadError: err, HttpMethod: http.MethodDelete, SecretPath: url}
	} else {
		glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("HTTP: %v, deleting secret response: %v", resp.StatusCode, string(respBytes))))
	}

	// check for error
	httpCode := resp.StatusCode
	if httpCode != http.StatusOK && httpCode != http.StatusNoContent {
		// parse the vault response
		var vaultResponse map[string][]string
		perr := json.Unmarshal(respBytes, &vaultResponse)
		if perr != nil {
			return &secrets.InvalidResponse{ParseError: perr, Response: respBytes, HttpMethod: http.MethodDelete, SecretPath: url}
		}

		// check the response code
		if httpCode := resp.StatusCode; httpCode == http.StatusForbidden {
			return &secrets.PermissionDenied{Response: vaultResponse, HttpMethod: http.MethodDelete, SecretPath: url, ExchangeUser: exUser}
		} else if httpCode == http.StatusBadRequest {
			return &secrets.BadRequest{Response: vaultResponse, HttpMethod: http.MethodDelete, SecretPath: url}
		} else {
			return &secrets.Unknown{Response: vaultResponse, ResponseCode: httpCode, HttpMethod: http.MethodDelete, SecretPath: url}
		}
	}

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("done deleting %s in vault.", name)))

	return nil
}

func (vs *AgbotVaultSecrets) GetSecretDetails(user, token, org, secretUser, secretNode, secretName string) (res secrets.SecretDetails, err error) {

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("extract secret details for %s in org %s as user %s", secretName, org, secretUser)))

	res = secrets.SecretDetails{}
	err = nil

	// check the input
	if org == "" {
		err = &secrets.BadRequest{Response: map[string][]string{"errors": {"Organization name must not be an empty string"}},
			HttpMethod: "",
			SecretPath: ""}
		return
	}
	if secretName == "" {
		err = &secrets.BadRequest{Response: map[string][]string{"errors": {"Secret name must not be an empty string"}},
			HttpMethod: "",
			SecretPath: ""}
		return
	}

	// build the vault URL
	url := fmt.Sprintf("%s/v1/openhorizon/data/%s/", vs.cfg.GetAgbotVaultURL(), org)
	var fullSecretName string
	if secretUser != "" && secretNode != "" {
		fullSecretName = fmt.Sprintf("user/%s/node/%s/%s", secretUser, secretNode, secretName)
	} else if secretUser != "" {
		fullSecretName = fmt.Sprintf("user/%s/%s", secretUser, secretName)
	} else if secretNode != "" {
		fullSecretName = fmt.Sprintf("node/%s/%s", secretNode, secretName)
	} else {
		fullSecretName = secretName
	}
	url += fullSecretName

	// check the credentials against the agbot's
	var userVaultToken string
	var exUser string
	var lerr error
	if user == vs.cfg.AgreementBot.ExchangeId && token == vs.cfg.AgreementBot.ExchangeToken {
		userVaultToken = vs.token
	} else {
		userVaultToken, exUser, lerr = vs.loginUser(user, token, org)
		if lerr != nil {
			err = &secrets.Unauthenticated{LoginError: lerr, ExchangeUser: user}
			return
		}
	}

	// invoke the vault to get the secret details
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("getting secret details for %s as user %s", fullSecretName, exUser)))
	resp, verr := vs.invokeVaultWithRetry(userVaultToken, url, http.MethodGet, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if verr != nil {
		err = &secrets.SecretsProviderUnavailable{ProviderError: verr}
		return
	}

	// parse the vault response
	respBytes, rerr := ioutil.ReadAll(resp.Body)
	if rerr != nil {
		err = &secrets.InvalidResponse{ReadError: rerr, HttpMethod: http.MethodGet, SecretPath: url}
		return
	}
	if safeLogEntry, oerr := obscureVaultResponse(respBytes, url); oerr != nil {
		err = errors.New(fmt.Sprintf("unable to obscure secret details: %v", oerr))
		return
	} else {
		glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("HTTP: %v, reading secret details response: %v", resp.StatusCode, safeLogEntry)))
	}

	// check for error
	httpCode := resp.StatusCode
	if httpCode != http.StatusOK {
		// parse the vault response
		var vaultResponse map[string][]string
		perr := json.Unmarshal(respBytes, &vaultResponse)
		if perr != nil {
			err = &secrets.InvalidResponse{ParseError: perr, Response: respBytes, HttpMethod: http.MethodGet, SecretPath: url}
			return
		}

		// check the response code
		if httpCode := resp.StatusCode; httpCode == http.StatusNotFound {
			err = &secrets.NoSecretFound{Response: vaultResponse, SecretPath: url}
			return
		} else if httpCode == http.StatusForbidden {
			err = &secrets.PermissionDenied{Response: vaultResponse, HttpMethod: http.MethodGet, SecretPath: url, ExchangeUser: exUser}
			return
		} else if httpCode == http.StatusBadRequest {
			err = &secrets.BadRequest{Response: vaultResponse, HttpMethod: http.MethodGet, SecretPath: url}
			return
		} else {
			err = &secrets.Unknown{Response: vaultResponse, ResponseCode: httpCode, HttpMethod: http.MethodGet, SecretPath: url}
			return
		}
	}

	// parse the vault response into the secret details
	r := GetSecretResponse{}
	if uerr := json.Unmarshal(respBytes, &r); uerr != nil {
		err = &secrets.InvalidResponse{ParseError: uerr, Response: respBytes, HttpMethod: http.MethodGet, SecretPath: url}
		return
	}

	res = r.Data.Data
	glog.V(3).Infof(vaultPluginLogString("done extracting secret details"))

	return

}

// Retrieve the metadata for a secret.
func (vs *AgbotVaultSecrets) GetSecretMetadata(secretOrg, secretUser, secretNode, secretName string) (res secrets.SecretMetadata, err error) {

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("extract secret metadata for %s in org %s as user %s", secretName, secretOrg, secretUser)))

	// check the input
	if secretOrg == "" {
		err = &secrets.BadRequest{Response: map[string][]string{"errors": {"Organization name must not be an empty string"}},
			HttpMethod: http.MethodGet,
			SecretPath: ""}
		return
	}
	if secretName == "" {
		err = &secrets.BadRequest{Response: map[string][]string{"errors": {"Secret name must not be an empty string"}},
			HttpMethod: http.MethodGet,
			SecretPath: ""}
		return
	}

	// build the vault URL
	url := fmt.Sprintf("%s/v1/openhorizon/metadata/%s/", vs.cfg.GetAgbotVaultURL(), secretOrg)
	if secretUser != "" && secretNode != "" {
		url += fmt.Sprintf("user/%s/node/%s/%s", secretUser, secretNode, secretName)
	} else if secretUser != "" {
		url += fmt.Sprintf("user/%s/%s", secretUser, secretName)
	} else if secretNode != "" {
		url += fmt.Sprintf("node/%s/%s", secretNode, secretName)
	} else {
		url += secretName
	}

	// invoke the vault to get the secret metadata
	resp, verr := vs.invokeVaultWithRetry(vs.token, url, http.MethodGet, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if verr != nil {
		err = &secrets.SecretsProviderUnavailable{ProviderError: verr}
		return
	}

	// parse the vault response
	respBytes, rerr := ioutil.ReadAll(resp.Body)
	if rerr != nil {
		err = &secrets.InvalidResponse{ReadError: rerr, HttpMethod: http.MethodGet, SecretPath: url}
		return
	} else {
		glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("HTTP: %v, reading secret metadata response: %v", resp.StatusCode, string(respBytes))))
	}

	// check for error
	httpCode := resp.StatusCode
	if httpCode != http.StatusOK {
		// parse the vault response
		var vaultResponse map[string][]string
		perr := json.Unmarshal(respBytes, &vaultResponse)
		if perr != nil {
			err = &secrets.InvalidResponse{ParseError: perr, Response: respBytes, HttpMethod: http.MethodGet, SecretPath: url}
			return
		}

		// check the response code
		if httpCode := resp.StatusCode; httpCode == http.StatusNotFound {
			err = &secrets.NoSecretFound{Response: vaultResponse, SecretPath: url}
			return
		} else if httpCode == http.StatusForbidden {
			err = &secrets.PermissionDenied{Response: vaultResponse, HttpMethod: http.MethodGet, SecretPath: url, ExchangeUser: secretUser}
			return
		} else if httpCode == http.StatusBadRequest {
			err = &secrets.BadRequest{Response: vaultResponse, HttpMethod: http.MethodGet, SecretPath: url}
		} else {
			err = &secrets.Unknown{Response: vaultResponse, ResponseCode: httpCode, HttpMethod: http.MethodGet, SecretPath: url}
			return
		}
	}

	// parse the vault response into the secret metadata
	r := ListSecretResponse{}
	if uerr := json.Unmarshal(respBytes, &r); uerr != nil {
		err = &secrets.InvalidResponse{ParseError: uerr, Response: respBytes, HttpMethod: http.MethodGet, SecretPath: url}
		return
	}

	res.CreationTime = cutil.TimeInSeconds(r.Data.CreationTime, VaultTimeFormat)
	res.UpdateTime = cutil.TimeInSeconds(r.Data.UpdateTime, VaultTimeFormat)

	glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("Metadata: %v", res)))
	glog.V(3).Infof(vaultPluginLogString("done extracting secret metadata"))

	return
}

// on a successful login, returns the user token and exchange username
func (vs *AgbotVaultSecrets) loginUser(user, token, org string) (string, string, error) {
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("logging in to vault as %s", user)))

	// Login to the vault using user creds
	url := fmt.Sprintf("%s/v1/auth/openhorizon/login", vs.cfg.GetAgbotVaultURL())

	body := LoginBody{
		Id:    user,
		Token: token,
	}

	resp, err := vs.invokeVaultWithRetry("", url, http.MethodPost, body)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return "", "", errors.New(fmt.Sprintf("agbot unable to login user %s: %v", user, err))
	}

	httpCode := resp.StatusCode
	if httpCode != http.StatusOK && httpCode != http.StatusCreated {
		return "", "", errors.New(fmt.Sprintf("agbot unable to login user %s, HTTP status code: %v", user, httpCode))
	}

	// Save the login token
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", errors.New(fmt.Sprintf("unable to read login response: %v", err))
	}

	respMsg := LoginResponse{}
	err = json.Unmarshal(respBytes, &respMsg)
	if err != nil {
		return "", "", errors.New(fmt.Sprintf("unable to parse response %v", string(respBytes)))
	}

	// log the exchange username
	exUser := respMsg.Auth.Metadata["exchangeUser"]
	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("logged into the vault as user %s", exUser)))

	return respMsg.Auth.ClientToken, exUser, nil
}

// Log string prefix api
var vaultPluginLogString = func(v interface{}) string {
	return fmt.Sprintf("Vault Plugin: %v", v)
}

func obscureVaultResponse(response []byte, url string) (string, error) {
	r := GetSecretResponse{}
	if uerr := json.Unmarshal(response, &r); uerr != nil {
		err := &secrets.InvalidResponse{ParseError: uerr, Response: response, HttpMethod: http.MethodGet, SecretPath: url}
		return "", err
	}
	r.Data.Data.Value = "********"
	if newResponse, uerr := json.Marshal(r); uerr != nil {
		err := &secrets.InvalidResponse{ParseError: uerr, Response: response, HttpMethod: http.MethodGet, SecretPath: url}
		return "", err
	} else {
		return string(newResponse), nil
	}
}
