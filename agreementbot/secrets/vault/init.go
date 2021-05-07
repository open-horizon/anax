package vault

import (
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
)

// This function is called by the anax main to allow the plugin a chance to initialize itself.
// This function is called every time the agbot starts, so it has to handle the following cases:
// - Vault has never been used before by openhorizon
// - Vault has policies and secrets from a previous start of the agbot
func (as *AgbotVaultSecrets) Initialize(cfg *config.HorizonConfig) error {

	glog.V(1).Infof("Initializing vault as secrets plugin: %v", cfg.GetAgbotVaultURL())
	return nil

}
