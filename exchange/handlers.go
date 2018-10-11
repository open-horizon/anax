package exchange

import (
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/policy"
)

// The handlers module defines replaceable functions that represent the exchange API's external dependencies. These
// handlers can be replaced by unit or integration tests to mock the external dependency.

type ExchangeContext interface {
	GetExchangeId() string
	GetExchangeToken() string
	GetExchangeURL() string
	GetHTTPFactory() *config.HTTPClientFactory
}

// A handler for querying the exchange for an organization.
type OrgHandler func(org string) (*Organization, error)

func GetHTTPExchangeOrgHandler(ec ExchangeContext) OrgHandler {
	return func(org string) (*Organization, error) {
		return GetOrganization(ec.GetHTTPFactory(), org, ec.GetExchangeURL(), ec.GetExchangeId(), ec.GetExchangeToken())
	}
}

// A handler for querying the exchange for an org when the caller doesnt have exchange identity at the time of creating the handler, but
// can supply the exchange context when it's time to make the call. Only used by the API package when trying to register an edge device.
type OrgHandlerWithContext func(org string, id string, token string) (*Organization, error)

func GetHTTPExchangeOrgHandlerWithContext(cfg *config.HorizonConfig) OrgHandlerWithContext {
	return func(org string, id string, token string) (*Organization, error) {
		return GetOrganization(cfg.Collaborators.HTTPClientFactory, org, cfg.Edge.ExchangeURL, id, token)
	}
}

// A handler for querying the exchange for patterns.
type PatternHandler func(org string, pattern string) (map[string]Pattern, error)

func GetHTTPExchangePatternHandler(ec ExchangeContext) PatternHandler {
	return func(org string, pattern string) (map[string]Pattern, error) {
		return GetPatterns(ec.GetHTTPFactory(), org, pattern, ec.GetExchangeURL(), ec.GetExchangeId(), ec.GetExchangeToken())
	}
}

// A handler for querying the exchange for patterns when the caller doesnt have exchange identity at the time of creating the handler, but
// can supply the exchange context when it's time to make the call. Only used by the API package when trying to register an edge device.
type PatternHandlerWithContext func(org string, pattern string, id string, token string) (map[string]Pattern, error)

func GetHTTPExchangePatternHandlerWithContext(cfg *config.HorizonConfig) PatternHandlerWithContext {
	return func(org string, pattern string, id string, token string) (map[string]Pattern, error) {
		return GetPatterns(cfg.Collaborators.HTTPClientFactory, org, pattern, cfg.Edge.ExchangeURL, id, token)
	}
}

// A handler for getting the device information from the exchange
type DeviceHandler func(id string, token string) (*Device, error)

func GetHTTPDeviceHandler(ec ExchangeContext) DeviceHandler {
	return func(id string, token string) (*Device, error) {
		return GetExchangeDevice(ec.GetHTTPFactory(), ec.GetExchangeId(), ec.GetExchangeToken(), ec.GetExchangeURL())
	}
}

// A handler for modifying the device information on the exchange
type PutDeviceHandler func(deviceId string, deviceToken string, pdr *PutDeviceRequest) (*PutDeviceResponse, error)

func GetHTTPPutDeviceHandler(ec ExchangeContext) PutDeviceHandler {
	return func(id string, token string, pdr *PutDeviceRequest) (*PutDeviceResponse, error) {
		return PutExchangeDevice(ec.GetHTTPFactory(), ec.GetExchangeId(), ec.GetExchangeToken(), ec.GetExchangeURL(), pdr)
	}
}

// A handler for resolving service references in the exchange.
type ServiceResolverHandler func(wUrl string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, *ServiceDefinition, error)

func GetHTTPServiceResolverHandler(ec ExchangeContext) ServiceResolverHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, *ServiceDefinition, error) {
		return ServiceResolver(wUrl, wOrg, wVersion, wArch, GetHTTPServiceHandler(ec))
	}
}

// A handler for getting service metadata from the exchange.
type ServiceHandler func(wUrl string, wOrg string, wVersion string, wArch string) (*ServiceDefinition, string, error)

func GetHTTPServiceHandler(ec ExchangeContext) ServiceHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (*ServiceDefinition, string, error) {
		return GetService(ec, wUrl, wOrg, wVersion, wArch)
	}
}

// a handler for getting microservice keys from the exchange
type ObjectSigningKeysHandler func(oType, oUrl string, oOrg string, oVersion string, oArch string) (map[string]string, error)

func GetHTTPObjectSigningKeysHandler(ec ExchangeContext) ObjectSigningKeysHandler {
	return func(oType string, oUrl string, oOrg string, oVersion string, oArch string) (map[string]string, error) {
		return GetObjectSigningKeys(ec, oType, oUrl, oOrg, oVersion, oArch)
	}
}

// A handler for getting the image docker auths for a service in the exchange.
type ServiceDockerAuthsHandler func(sUrl string, sOrg string, sVersion string, sArch string) ([]ImageDockerAuth, error)

func GetHTTPServiceDockerAuthsHandler(ec ExchangeContext) ServiceDockerAuthsHandler {
	return func(sUrl string, sOrg string, sVersion string, sArch string) ([]ImageDockerAuth, error) {
		return GetServiceDockerAuths(ec, sUrl, sOrg, sVersion, sArch)
	}
}

// A handler for getting the image docker auths for a service by the id in the exchange.
type ServiceDockerAuthsWithIdHandler func(sId string) ([]ImageDockerAuth, error)

func GetHTTPServiceDockerAuthsWithIdHandler(ec ExchangeContext) ServiceDockerAuthsWithIdHandler {
	return func(sId string) ([]ImageDockerAuth, error) {
		return GetServiceDockerAuthsWithId(ec, sId)
	}
}
