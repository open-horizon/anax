package compcheck

import (
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/policy"
)

// An implementation of common.AbstractPatternFile
type Pattern struct {
	Org string `json:"org"`
	exchange.Pattern
}

func (p *Pattern) GetOrg() string {
	return p.Org
}

func (p *Pattern) IsPublic() bool {
	return p.Public
}

func (p *Pattern) GetServices() []exchange.ServiceReference {
	return p.Services
}

func (p *Pattern) GetUserInputs() []policy.UserInput {
	return p.UserInput
}

func (p *Pattern) GetSecretBinding() []exchangecommon.SecretBinding {
	return p.SecretBinding
}

func (p *Pattern) GetClusterNamespace() string {
	return p.ClusterNamespace
}
