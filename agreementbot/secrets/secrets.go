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

	ListOrgSecret(user, token, org, name string) ([]string, error)
	ListOrgSecrets(user, token, org string) ([]string, error)
	
}
