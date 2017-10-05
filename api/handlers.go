package api

import (
	"github.com/open-horizon/anax/exchange"
)

// The handlers module defines replaceable functions that represent the API's external dependencies. These
// handlers can be replaced by unit or integration tests to mock the external dependency.

// A handler for querying the exchange for an organization.
type OrgHandler func(org string, id string, token string) (*exchange.Organization, error)

func GetHTTPExchangeOrgHandler(a *API) OrgHandler {
	return func(org string, id string, token string) (*exchange.Organization, error) {
		return exchange.GetOrganization(a.Config.Collaborators.HTTPClientFactory, org, a.Config.Edge.ExchangeURL, id, token)
	}
}

// A handler for querying the exchange for patterns.
type PatternHandler func(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error)

func GetHTTPExchangePatternHandler(a *API) PatternHandler {
	return func(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
		return exchange.GetPatterns(a.Config.Collaborators.HTTPClientFactory, org, pattern, a.Config.Edge.ExchangeURL, id, token)
	}
}
