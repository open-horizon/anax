package vault

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/secrets"
)

// This function registers an uninitialized agbot secrets implementation with the secrets plugin registry. The plugin's Initialize
// method is used to configure the object.
func init() {
	secrets.Register("vault", new(AgbotVaultSecrets))
}

// The fields in this object are initialized in the Initialize method in this package.
type AgbotVaultSecrets struct {
	token string   // The identity of this agbot in the vault
}

func (vs *AgbotVaultSecrets) String() string {
	return fmt.Sprintf("Token: %v", vs.token)
}

func (vs *AgbotVaultSecrets) Close() {
	glog.V(2).Infof("Closed Vault secrets implementation")
}

func (vs *AgbotVaultSecrets) ListSecrets(org string) ([]string, error) {
	return []string{}, nil
}

func (vs *AgbotVaultSecrets) ListUserSecrets(org string, user string) ([]string, error) {
	return []string{}, nil
}
