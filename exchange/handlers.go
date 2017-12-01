package exchange

import (
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/policy"
)

// The handlers module defines replaceable functions that represent the API's external dependencies. These
// handlers can be replaced by unit or integration tests to mock the external dependency.

type ExchangeApiHandlers struct {
	Config *config.HorizonConfig
}

func NewExchangeApiHandlers(config *config.HorizonConfig) *ExchangeApiHandlers {
	return &ExchangeApiHandlers{
		Config: config,
	}
}

// A handler for querying the exchange for an organization.
type OrgHandler func(org string, id string, token string) (*Organization, error)

func (e *ExchangeApiHandlers) GetHTTPExchangeOrgHandler() OrgHandler {
	return func(org string, id string, token string) (*Organization, error) {
		return GetOrganization(e.Config.Collaborators.HTTPClientFactory, org, e.Config.Edge.ExchangeURL, id, token)
	}
}

// A handler for querying the exchange for patterns.
type PatternHandler func(org string, pattern string, id string, token string) (map[string]Pattern, error)

func (e *ExchangeApiHandlers) GetHTTPExchangePatternHandler() PatternHandler {
	return func(org string, pattern string, id string, token string) (map[string]Pattern, error) {
		return GetPatterns(e.Config.Collaborators.HTTPClientFactory, org, pattern, e.Config.Edge.ExchangeURL, id, token)
	}
}

// A handler for querying the exchange for microservices.
type MicroserviceHandler func(mUrl string, mOrg string, mVersion string, mArch string, id string, token string) (*MicroserviceDefinition, error)

func (e *ExchangeApiHandlers) GetHTTPMicroserviceHandler() MicroserviceHandler {
	return func(mUrl string, mOrg string, mVersion string, mArch string, id string, token string) (*MicroserviceDefinition, error) {
		return GetMicroservice(e.Config.Collaborators.HTTPClientFactory, mUrl, mOrg, mVersion, mArch, e.Config.Edge.ExchangeURL, id, token)
	}
}

// A handler for resolving workload references in the exchange.
type WorkloadResolverHandler func(wUrl string, wOrg string, wVersion string, wArch string, id string, token string) (*policy.APISpecList, *WorkloadDefinition, error)

func (e *ExchangeApiHandlers) GetHTTPWorkloadResolverHandler() WorkloadResolverHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string, id string, token string) (*policy.APISpecList, *WorkloadDefinition, error) {
		return WorkloadResolver(e.Config.Collaborators.HTTPClientFactory, wUrl, wOrg, wVersion, wArch, e.Config.Edge.ExchangeURL, id, token)
	}
}

// A handler for getting workload metadata from the exchange.
type WorkloadHandler func(wUrl string, wOrg string, wVersion string, wArch string, id string, token string) (*WorkloadDefinition, error)

func (e *ExchangeApiHandlers) GetHTTPWorkloadHandler() WorkloadHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string, id string, token string) (*WorkloadDefinition, error) {
		return GetWorkload(e.Config.Collaborators.HTTPClientFactory, wUrl, wOrg, wVersion, wArch, e.Config.Edge.ExchangeURL, id, token)
	}
}
