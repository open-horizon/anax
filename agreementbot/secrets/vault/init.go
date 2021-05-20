package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"io/ioutil"
	"net/http"
)

// This function is called by the anax main to allow the plugin a chance to initialize itself.
// This function is called every time the agbot starts. This function should not do anything that
// requires it to block for long periods of time. The Login function will be called following this
// function and is able to block for long periods of time if necessary.
func (vs *AgbotVaultSecrets) Initialize(cfg *config.HorizonConfig) (err error) {

	glog.V(1).Infof(vaultPluginLogString("Initializing vault as the secrets plugin."))

	vs.httpClient, err = vs.newHTTPClient(cfg)
	vs.cfg = cfg

	glog.V(1).Infof(vaultPluginLogString("Initialized vault as the secrets plugin"))

	return nil

}

// This function is called by the agbot worker to allow the plugin a chance to connect with the vault.
// This function is called every time the agbot starts, so it has to handle the following cases:
// - Vault has never been used before by openhorizon
// - Vault has policies and secrets from a previous start of the agbot
func (vs *AgbotVaultSecrets) Login() (err error) {

	glog.V(3).Infof(vaultPluginLogString(fmt.Sprintf("logging in to vault: %v", vs.cfg.GetAgbotVaultURL())))

	// Login to the vault using agbot creds
	url := fmt.Sprintf("%s/v1/auth/openhorizon/login", vs.cfg.GetAgbotVaultURL())

	body := LoginBody{
		Id:    vs.cfg.AgreementBot.ExchangeId,
		Token: vs.cfg.AgreementBot.ExchangeToken,
	}

	resp, err := vs.invokeVaultWithRetry("", url, http.MethodPost, body)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return errors.New(fmt.Sprintf("agbot unable to login, error: %v", err))
	}

	httpCode := resp.StatusCode
	if httpCode != http.StatusOK && httpCode != http.StatusCreated {
		return errors.New(fmt.Sprintf("agbot unable to login, HTTP status code: %v", httpCode))
	}

	// Save the login token
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to read login response, error: %v", err))
	}

	glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("login response: %v", string(respBytes))))

	respMsg := LoginResponse{}
	err = json.Unmarshal(respBytes, &respMsg)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to parse response %v", string(respBytes)))
	}

	vs.token = respMsg.Auth.ClientToken

	glog.V(3).Infof(vaultPluginLogString("logged in to the vault."))

	return nil

}

func (vs *AgbotVaultSecrets) Renew() (err error) {

	glog.V(3).Infof(vaultPluginLogString("renewing token"))

	// Login to the vault using agbot creds
	url := fmt.Sprintf("%s/v1/auth/token/renew-self", vs.cfg.GetAgbotVaultURL())

	body := RenewBody{
		Token: vs.token,
	}

	resp, err := vs.invokeVaultWithRetry(vs.token, url, http.MethodPost, body)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return errors.New(fmt.Sprintf("agbot unable to renew token, error: %v", err))
	}

	httpCode := resp.StatusCode
	if httpCode != http.StatusOK && httpCode != http.StatusCreated {
		return errors.New(fmt.Sprintf("agbot unable to renew token, HTTP status code: %v", httpCode))
	}

	// Save the token locally

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to read renew response, error: %v", err))
	}

	glog.V(5).Infof(vaultPluginLogString(fmt.Sprintf("renew response: %v", string(respBytes))))
	glog.V(3).Infof(vaultPluginLogString("done renewing token"))

	return nil
}

func (vs *AgbotVaultSecrets) IsReady() bool {
	return vs.token != ""
}

func (vs *AgbotVaultSecrets) Close() {
	glog.V(2).Infof("Closed Vault secrets implementation")
}
