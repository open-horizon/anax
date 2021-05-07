package secrets

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/config"
)

// The registry is a mechanism that enables different secret vault implementations to be plugged into the
// runtime. The implementation registers itself with this registry when the implementation's package init()
// method is driven. This mechanism prevents the need for the secrets package to import each of the
// secrets implementation packages. The only tricky part of this is that name of each secrets implementation is hard
// coded here and in the implementation's call to the Register() method. Sharing constants would re-introduce
// the package dependency that we want to avoid.
type SecretsProviderRegistry map[string]AgbotSecrets

var SecretsProviders = SecretsProviderRegistry{}

func Register(name string, as AgbotSecrets) {
	SecretsProviders[name] = as
}

// Initialize the underlying Agbot Secrets implementation depending on what is configured. If vault is configured, it is used.
// If nothing is configured, an error is returned.
func InitSecrets(cfg *config.HorizonConfig) (AgbotSecrets, error) {

	if cfg.IsVaultConfigured() {
		secretsObj := SecretsProviders["vault"]
		return secretsObj, secretsObj.Initialize(cfg)

	}
	return nil, errors.New(fmt.Sprintf("Vault is not configured correctly."))

}
