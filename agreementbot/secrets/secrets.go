package secrets

import (
	"github.com/open-horizon/anax/config"
)

// An agbot can be configured to run with several different secrets implementations, but only one can be used at runtime.
// This file contains the abstract interface representing the handle used by the runtime to access the secrets implementation.

type AgbotSecrets interface {

	// Database related functions
	Initialize(cfg *config.HorizonConfig) error
	Login() error
	Renew() error
	Close()
	IsReady() bool

	ListOrgSecret(user, token, org, name string) (map[string]string, error)
	ListOrgSecrets(user, token, org string) ([]string, error)
	CreateOrgSecret(user, token, org, vaultSecretName string, data CreateSecretRequest) error
	DeleteOrgSecret(user, token, org, name string) error

	ListOrgUserSecret(user, token, org, name string) (map[string]string, error)
	CreateOrgUserSecret(user, token, org, vaultSecretName string, data CreateSecretRequest) error
	DeleteOrgUserSecret(user, token, org, name string) error
}

type CreateSecretRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ErrorResponse struct {
	Msg      string // the error message which shall be logged and added to response body
	Details  string // optional log message
	RespCode int    // response type from the agbot API
}

func (e ErrorResponse) Error() string {
	return e.Msg
}
