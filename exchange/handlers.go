package exchange

import (
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/edge-sync-service/common"
)

// The handlers module defines replaceable functions that represent the exchange and CSS API's external dependencies. These
// handlers can be replaced by unit or integration tests to mock the external dependency.

type ExchangeContext interface {
	GetExchangeId() string
	GetExchangeToken() string
	GetExchangeURL() string
	GetCSSURL() string
	GetHTTPFactory() *config.HTTPClientFactory
}

// This is a custom exchange context that con be configured for special usage scenarios. Normally, an anax worker
// should be used as the ExchangeContext because it is automatically configured. However, there are times when
// special behavior is needed.

type CustomExchangeContext struct {
	userId      string
	password    string
	exchangeURL string
	cssURL      string
	httpFactory *config.HTTPClientFactory
}

func (c *CustomExchangeContext) GetExchangeId() string {
	return c.userId
}

func (c *CustomExchangeContext) GetExchangeToken() string {
	return c.password
}

func (c *CustomExchangeContext) GetExchangeURL() string {
	return c.exchangeURL
}

func (c *CustomExchangeContext) GetCSSURL() string {
	return c.cssURL
}

func (c *CustomExchangeContext) GetHTTPFactory() *config.HTTPClientFactory {
	return c.httpFactory
}

func NewCustomExchangeContext(userId string, passwd string, exchangeURL string, cssURL string,
	httpFactory *config.HTTPClientFactory) *CustomExchangeContext {
	return &CustomExchangeContext{
		userId:      userId,
		password:    passwd,
		exchangeURL: exchangeURL,
		cssURL:      cssURL,
		httpFactory: httpFactory,
	}
}

// A handler for querying the exchange for an organization.
type OrgHandler func(org string) (*Organization, error)

func GetHTTPExchangeOrgHandler(ec ExchangeContext) OrgHandler {
	return func(org string) (*Organization, error) {
		return GetOrganization(ec.GetHTTPFactory(), org, ec.GetExchangeURL(), ec.GetExchangeId(), ec.GetExchangeToken())
	}
}

// A handler for querying the exchange version. The id and token is used for auth.
type ExchangeVersionHandler func(id string, token string) (string, error)

func GetHTTPExchangeVersionHandler(cfg *config.HorizonConfig) ExchangeVersionHandler {
	return func(id string, token string) (string, error) {
		return GetExchangeVersion(cfg.Collaborators.HTTPClientFactory, cfg.Edge.ExchangeURL, id, token)
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
		if token != "" {
			return GetExchangeDevice(ec.GetHTTPFactory(), id, id, token, ec.GetExchangeURL())
		} else {
			return GetExchangeDevice(ec.GetHTTPFactory(), id, ec.GetExchangeId(), ec.GetExchangeToken(), ec.GetExchangeURL())
		}
	}
}

// this is used when ExchangeContext is not set up yet.
func GetHTTPDeviceHandler2(cfg *config.HorizonConfig) DeviceHandler {
	return func(id string, token string) (*Device, error) {
		return GetExchangeDevice(cfg.Collaborators.HTTPClientFactory, id, id, token, cfg.Edge.ExchangeURL)
	}
}

// A handler for modifying the device information on the exchange
type PutDeviceHandler func(deviceId string, deviceToken string, pdr *PutDeviceRequest) (*PutDeviceResponse, error)

func GetHTTPPutDeviceHandler(ec ExchangeContext) PutDeviceHandler {
	return func(id string, token string, pdr *PutDeviceRequest) (*PutDeviceResponse, error) {
		return PutExchangeDevice(ec.GetHTTPFactory(), ec.GetExchangeId(), ec.GetExchangeToken(), ec.GetExchangeURL(), pdr)
	}
}

// A handler for patching the device information on the exchange
type PatchDeviceHandler func(deviceId string, deviceToken string, pdr *PatchDeviceRequest) error

func GetHTTPPatchDeviceHandler(ec ExchangeContext) PatchDeviceHandler {
	return func(id string, token string, pdr *PatchDeviceRequest) error {
		return PatchExchangeDevice(ec.GetHTTPFactory(), ec.GetExchangeId(), ec.GetExchangeToken(), ec.GetExchangeURL(), pdr)
	}
}

// this is used when ExchangeContext is not set up yet.
func GetHTTPPatchDeviceHandler2(cfg *config.HorizonConfig) PatchDeviceHandler {
	return func(id string, token string, pdr *PatchDeviceRequest) error {
		return PatchExchangeDevice(cfg.Collaborators.HTTPClientFactory, id, token, cfg.Edge.ExchangeURL, pdr)
	}
}

// A handler for modifying the device information on the exchange
type PostDeviceServicesConfigStateHandler func(deviceId string, deviceToken string, svcsConfigState *ServiceConfigState) error

func GetHTTPPostDeviceServicesConfigStateHandler(ec ExchangeContext) PostDeviceServicesConfigStateHandler {
	return func(id string, token string, svcsConfigState *ServiceConfigState) error {
		return PostDeviceServicesConfigState(ec.GetHTTPFactory(), ec.GetExchangeId(), ec.GetExchangeToken(), ec.GetExchangeURL(), svcsConfigState)
	}
}

// A handler for service config state information from the exchange
type ServicesConfigStateHandler func(id string, token string) ([]ServiceConfigState, error)

func GetHTTPServicesConfigStateHandler(ec ExchangeContext) ServicesConfigStateHandler {
	return func(id string, token string) ([]ServiceConfigState, error) {
		return GetServicesConfigState(ec.GetHTTPFactory(), ec.GetExchangeId(), ec.GetExchangeToken(), ec.GetExchangeURL())
	}
}

// A handler for resolving service references in the exchange.
type ServiceResolverHandler func(wUrl string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, *ServiceDefinition, []string, error)

func GetHTTPServiceResolverHandler(ec ExchangeContext) ServiceResolverHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, *ServiceDefinition, []string, error) {
		return ServiceResolver(wUrl, wOrg, wVersion, wArch, GetHTTPServiceHandler(ec))
	}
}

// A handler for resolving service refrences in the exchange. It returns the service definitions in stead of APISpecList.
type ServiceDefResolverHandler func(wUrl string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, map[string]ServiceDefinition, *ServiceDefinition, string, error)

func GetHTTPServiceDefResolverHandler(ec ExchangeContext) ServiceDefResolverHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, map[string]ServiceDefinition, *ServiceDefinition, string, error) {
		return ServiceDefResolver(wUrl, wOrg, wVersion, wArch, GetHTTPServiceHandler(ec))
	}
}

// A handler for getting service metadata from the exchange.
type ServiceHandler func(wUrl string, wOrg string, wVersion string, wArch string) (*ServiceDefinition, string, error)

func GetHTTPServiceHandler(ec ExchangeContext) ServiceHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (*ServiceDefinition, string, error) {
		return GetService(ec, wUrl, wOrg, wVersion, wArch)
	}
}

// A handler for getting service metadata from the exchange. version can be a selection string, arch can be empty to mean all arches
type SelectedServicesHandler func(wUrl string, wOrg string, wVersion string, wArch string) (map[string]ServiceDefinition, error)

func GetHTTPSelectedServicesHandler(ec ExchangeContext) SelectedServicesHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (map[string]ServiceDefinition, error) {
		return GetSelectedServices(ec, wUrl, wOrg, wVersion, wArch)
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

// A handler for getting the node policy from the exchange.
type NodePolicyHandler func(deviceId string) (*ExchangePolicy, error)

func GetHTTPNodePolicyHandler(ec ExchangeContext) NodePolicyHandler {
	return func(deviceId string) (*ExchangePolicy, error) {
		return GetNodePolicy(ec, deviceId)
	}
}

// A handler for updating the node policy to the exchange.
type PutNodePolicyHandler func(deviceId string, ep *ExchangePolicy) (*PutDeviceResponse, error)

func GetHTTPPutNodePolicyHandler(ec ExchangeContext) PutNodePolicyHandler {
	return func(deviceId string, ep *ExchangePolicy) (*PutDeviceResponse, error) {
		return PutNodePolicy(ec, deviceId, ep)
	}
}

// A handler for deleting the node policy from the exchange.
type DeleteNodePolicyHandler func(deviceId string) error

func GetHTTPDeleteNodePolicyHandler(ec ExchangeContext) DeleteNodePolicyHandler {
	return func(deviceId string) error {
		return DeleteNodePolicy(ec, deviceId)
	}
}

// A handler for getting the list of node surface errors from the exchange
type SurfaceErrorsHandler func(deviceId string) (*ExchangeSurfaceError, error)

func GetHTTPSurfaceErrorsHandler(ec ExchangeContext) SurfaceErrorsHandler {
	return func(deviceId string) (*ExchangeSurfaceError, error) {
		return GetSurfaceErrors(ec, deviceId)
	}
}

// A handler for putting a list of node surface errors in the exchange
type PutSurfaceErrorsHandler func(deviceId string, errorList *ExchangeSurfaceError) (*PutDeviceResponse, error)

func GetHTTPPutSurfaceErrorsHandler(ec ExchangeContext) PutSurfaceErrorsHandler {
	return func(deviceId string, errorList *ExchangeSurfaceError) (*PutDeviceResponse, error) {
		return PutSurfaceErrors(ec, deviceId, errorList)
	}
}

// A handler for deleting the node surface errors from the exchange.
type DeleteSurfaceErrorsHandler func(deviceId string) error

func GetHTTPDeleteSurfaceErrorsHandler(ec ExchangeContext) DeleteSurfaceErrorsHandler {
	return func(deviceId string) error {
		return DeleteSurfaceErrors(ec, deviceId)
	}
}

// A handler for getting the node status.
type NodeStatusHandler func(deviceId string) (*NodeStatus, error)

func GetHTTPNodeStatusHandler(ec ExchangeContext) NodeStatusHandler {
	return func(deviceId string) (*NodeStatus, error) {
		return GetNodeStatus(ec, deviceId)
	}
}

// Two handlers for getting the service policy from the exchange.
type ServicePolicyWithIdHandler func(service_id string) (*ExchangePolicy, error)

func GetHTTPServicePolicyWithIdHandler(ec ExchangeContext) ServicePolicyWithIdHandler {
	return func(service_id string) (*ExchangePolicy, error) {
		return GetServicePolicyWithId(ec, service_id)
	}
}

type ServicePolicyHandler func(sUrl string, sOrg string, sVersion string, sArch string) (*ExchangePolicy, string, error)

func GetHTTPServicePolicyHandler(ec ExchangeContext) ServicePolicyHandler {
	return func(sUrl string, sOrg string, sVersion string, sArch string) (*ExchangePolicy, string, error) {
		return GetServicePolicy(ec, sUrl, sOrg, sVersion, sArch)
	}
}

// Two handlers for updating the service policy to the exchange.
type PutServicePolicyWithIdHandler func(service_id string, ep *ExchangePolicy) (*PutDeviceResponse, error)

func GetHTTPPutServicePolicyWithIdHandler(ec ExchangeContext) PutServicePolicyWithIdHandler {
	return func(service_id string, ep *ExchangePolicy) (*PutDeviceResponse, error) {
		return PutServicePolicyWithId(ec, service_id, ep)
	}
}

type PutServicePolicyHandler func(sUrl string, sOrg string, sVersion string, sArch string, ep *ExchangePolicy) (*PutDeviceResponse, error)

func GetHTTPPutServicePolicyHandler(ec ExchangeContext) PutServicePolicyHandler {
	return func(sUrl string, sOrg string, sVersion string, sArch string, ep *ExchangePolicy) (*PutDeviceResponse, error) {
		return PutServicePolicy(ec, sUrl, sOrg, sVersion, sArch, ep)
	}
}

// Two handlers for deleting the service policy from the exchange.
type DeleteServicePolicyWithIdHandler func(service_id string) error

func GetHTTPDeleteServicePolicyWithIdHandler(ec ExchangeContext) DeleteServicePolicyWithIdHandler {
	return func(service_id string) error {
		return DeleteServicePolicyWithId(ec, service_id)
	}
}

type DeleteServicePolicyHandler func(sUrl string, sOrg string, sVersion string, sArch string) error

func GetHTTPDeleteServicePolicyHandler(ec ExchangeContext) DeleteServicePolicyHandler {
	return func(sUrl string, sOrg string, sVersion string, sArch string) error {
		return DeleteServicePolicy(ec, sUrl, sOrg, sVersion, sArch)
	}
}

// A handler for getting the business policies from the exchange.
type BusinessPoliciesHandler func(org string, policy_id string) (map[string]ExchangeBusinessPolicy, error)

func GetHTTPBusinessPoliciesHandler(ec ExchangeContext) BusinessPoliciesHandler {
	return func(org string, policy_id string) (map[string]ExchangeBusinessPolicy, error) {
		return GetBusinessPolicies(ec, org, policy_id)
	}
}

// A handler for getting the policy of objects in the Model Management System.
type ObjectPolicyQueryHandler func(org string, serviceId string) (*ObjectDestinationPolicies, error)

func GetHTTPObjectPolicyQueryHandler(ec ExchangeContext) ObjectPolicyQueryHandler {
	return func(org string, serviceId string) (*ObjectDestinationPolicies, error) {
		return GetObjectsByService(ec, org, serviceId)
	}
}

// A handler for getting the policy of objects in the Model Management System.
type ObjectQueryHandler func(org string, objID string, objType string) (*common.MetaData, error)

func GetHTTPObjectQueryHandler(ec ExchangeContext) ObjectQueryHandler {
	return func(org string, objID string, objType string) (*common.MetaData, error) {
		return GetObject(ec, org, objID, objType)
	}
}

// A handler for getting the destinations of objects in the Model Management System.
type ObjectDestinationQueryHandler func(org string, objID string, objType string) (*ObjectDestinationStatuses, error)

func GetHTTPObjectDestinationQueryHandler(ec ExchangeContext) ObjectDestinationQueryHandler {
	return func(org string, objID string, objType string) (*ObjectDestinationStatuses, error) {
		return GetObjectDestinations(ec, org, objID, objType)
	}
}

// A handler for add or delete the object destinations in the Model Management System.
type AddOrRemoveObjectDestinationHandler func(org string, objType string, objID string, destsRequest *PostDestsRequest) error

func GetHTTPAddOrRemoveObjectDestinationHandler(ec ExchangeContext) AddOrRemoveObjectDestinationHandler {
	return func(org string, objType string, objID string, destsRequest *PostDestsRequest) error {
		return AddOrRemoveDestinations(ec, org, objType, objID, destsRequest)
	}
}

// A handler for getting new policy for objects in the Model Management System.
type ObjectPolicyUpdatesQueryHandler func(org string, since int64) (*ObjectDestinationPolicies, error)

func GetHTTPObjectPolicyUpdatesQueryHandler(ec ExchangeContext) ObjectPolicyUpdatesQueryHandler {
	return func(org string, since int64) (*ObjectDestinationPolicies, error) {
		return GetUpdatedObjects(ec, org, since)
	}
}

// A handler for telling the Model Management System that a policy update has been received.
type ObjectPolicyUpdateReceivedHandler func(objPol *ObjectDestinationPolicy) error

func GetHTTPObjectPolicyUpdateReceivedHandler(ec ExchangeContext) ObjectPolicyUpdateReceivedHandler {
	return func(objPol *ObjectDestinationPolicy) error {
		return SetPolicyReceived(ec, objPol)
	}
}

// A handler for retrieving changes from the exchange.
type ExchangeChangeHandler func(changeId uint64, maxRecords int, orgList []string) (*ExchangeChanges, error)

func GetHTTPExchangeChangeHandler(ec ExchangeContext) ExchangeChangeHandler {
	return func(changeId uint64, maxRecords int, orgList []string) (*ExchangeChanges, error) {
		return GetExchangeChanges(ec, changeId, maxRecords, orgList)
	}
}

// A handler for retrieving current max change ID from the exchange.
type ExchangeMaxChangeIDHandler func() (*ExchangeChangeIDResponse, error)

func GetHTTPExchangeMaxChangeIDHandler(ec ExchangeContext) ExchangeMaxChangeIDHandler {
	return func() (*ExchangeChangeIDResponse, error) {
		return GetExchangeChangeID(ec)
	}
}

// A handler for retrieving which deployment policies (in which orgs) an agbot is serving.
type AgbotServedDeploymentPolicyHandler func() (map[string]ServedBusinessPolicy, error)

func GetHTTPAgbotServedDeploymentPolicy(ec ExchangeContext) AgbotServedDeploymentPolicyHandler {
	return func() (map[string]ServedBusinessPolicy, error) {
		return GetAgbotDeploymentPols(ec)
	}
}

// A handler for retrieving which patterns (in which orgs) an agbot is serving.
type AgbotServedPatternHandler func() (map[string]ServedPattern, error)

func GetHTTPAgbotServedPattern(ec ExchangeContext) AgbotServedPatternHandler {
	return func() (map[string]ServedPattern, error) {
		return GetAgbotPatterns(ec)
	}
}

// A handler for searching for nodes by deployment policy.
type AgbotPolicyNodeSearchHandler func(req *SearchExchBusinessPolRequest, policyOrg string, policyName string) (*SearchExchBusinessPolResponse, error)

func GetHTTPAgbotPolicyNodeSearchHandler(ec ExchangeContext) AgbotPolicyNodeSearchHandler {
	return func(req *SearchExchBusinessPolRequest, policyOrg string, policyName string) (*SearchExchBusinessPolResponse, error) {
		return GetPolicyNodes(ec, policyOrg, policyName, req)
	}
}

// A handler for searching for nodes by pattern.
type AgbotPatternNodeSearchHandler func(req *SearchExchangePatternRequest, policyOrg string, patternId string) (*[]SearchResultDevice, error)

func GetHTTPAgbotPatternNodeSearchHandler(ec ExchangeContext) AgbotPatternNodeSearchHandler {
	return func(req *SearchExchangePatternRequest, policyOrg string, patternId string) (*[]SearchResultDevice, error) {
		return GetPatternNodes(ec, policyOrg, patternId, req)
	}
}

// A handler for checking if a vault secret exists.
type VaultSecretExistsHandler func(agbotURL string, org string, userName string, secretName string) (bool, error)

func GetHTTPVaultSecretExistsHandler(ec ExchangeContext) VaultSecretExistsHandler {
	return func(agbotURL string, org string, userName string, secretName string) (bool, error) {
		return VaultSecretExists(ec, agbotURL, org, userName, secretName)
	}
}
