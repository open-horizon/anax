package secrets

import (
	"fmt"

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
	GetLastVaultStatus() uint64

	ListAllSecrets(user, token, org, path string) ([]string, error)

	ListOrgSecret(user, token, org, path string) error
	ListOrgSecrets(user, token, org, path string) ([]string, error)
	CreateOrgSecret(user, token, org, path string, data SecretDetails) error
	DeleteOrgSecret(user, token, org, path string) error

	ListOrgUserSecret(user, token, org, path string) error
	ListOrgUserSecrets(user, token, org, path string) ([]string, error)
	CreateOrgUserSecret(user, token, org, path string, data SecretDetails) error
	DeleteOrgUserSecret(user, token, org, path string) error

	ListOrgNodeSecret(user, token, org, path string) error
	ListOrgNodeSecrets(user, token, org, node, path string) ([]string, error)
	CreateOrgNodeSecret(user, token, org, path string, data SecretDetails) error
	DeleteOrgNodeSecret(user, token, org, path string) error

	ListUserNodeSecret(user, token, org, path string) error
	ListUserNodeSecrets(user, token, org, node, path string) ([]string, error)
	CreateUserNodeSecret(user, token, org, path string, data SecretDetails) error
	DeleteUserNodeSecret(user, token, org, path string) error

	// This function assumes that the plugin maintains an authentication to the secret manager that it can use
	// when it doesnt need to call APIs with user creds. The creds used instead have the ability to READ secrets.
	// "user" argument is the user who is accessing the secret, "secretUser" is the owner of the secret being accessed,
	// if an org-level secret then this will be empty
	GetSecretDetails(user, token, org, secretUser, secretNode, secretName string) (SecretDetails, error)

	// This function returns the secret manager's metadata about a given secret.
	// "user" argument is the user who is accessing the secret, "secretUser" is the owner of the secret being accessed,
	// if an org-level secret then this will be empty
	GetSecretMetadata(secretOrg, secretUser, secretNode, secretName string) (SecretMetadata, error)
}

// SecretDetails The key value pair of one secret
// swagger:model
type SecretDetails struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (m SecretDetails) String() string {
	return fmt.Sprintf("Key: %v, Value: ********", m.Key)
}

type SecretMetadata struct {
	CreationTime int64 `json:"created_time"`
	UpdateTime   int64 `json:"updated_time"`
}

func (m SecretMetadata) String() string {
	return fmt.Sprintf("Created: %v, Updated: %v", m.CreationTime, m.UpdateTime)
}

type ErrorResponse struct {
	Msg      string // the error message which shall be logged and added to response body
	Details  string // optional log message
	RespCode int    // response type from the agbot API
}

func (e ErrorResponse) Error() string {
	return e.Msg
}
