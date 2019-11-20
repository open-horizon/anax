package compcheck

import (
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/policy"
)

// **** Note: All the errors returned by the functions in this file are of type *CompCheckError,
// You can cast it by err.(*compcheck.CompCheckError) if you want to get the error code.

// error code for compatibility check error: CompCheckError
const (
	COMPCHECK_INPUT_ERROR      = 10
	COMPCHECK_VALIDATION_ERROR = 11
	COMPCHECK_CONVERTION_ERROR = 12
	COMPCHECK_MERGING_ERROR    = 13
	COMPCHECK_EXCHANGE_ERROR   = 14
)

// Error for policy compatibility check
type CompCheckError struct {
	Err     string `json:"error"`
	ErrCode int    `json:"error_code,omitempty"`
}

func (e *CompCheckError) Error() string {
	if e == nil {
		return ""
	} else {
		return fmt.Sprintf("%v", e.Err)
	}
}

func (e *CompCheckError) String() string {
	if e == nil {
		return ""
	} else {
		return fmt.Sprintf("ErrCode: %v, Error: %v", e.ErrCode, e.Err)
	}
}

func NewCompCheckError(err error, errCode int) *CompCheckError {
	return &CompCheckError{
		Err:     err.Error(),
		ErrCode: errCode,
	}
}

// The output format for the policy check
type PolicyCompOutput struct {
	Compatible     bool           `json:"compatible"`
	Reason         string         `json:"reason"` // set when not compatible
	ProducerPolicy *policy.Policy `json:"producer_policy,omitempty"`
	ConsumerPolicy *policy.Policy `json:"consumer_policy,omitempty"`
}

func (p *PolicyCompOutput) String() string {
	return fmt.Sprintf("Compatible: %v, Reason: %v, ProducerPolicy: %v, ConsumerPolicy: %v",
		p.Compatible, p.Reason, p.ProducerPolicy, p.ConsumerPolicy)

}

func NewPolicyCompOutput(compatible bool, reason string, producerPolicy *policy.Policy, consumerPolicy *policy.Policy) *PolicyCompOutput {
	return &PolicyCompOutput{
		Compatible:     compatible,
		Reason:         reason,
		ProducerPolicy: producerPolicy,
		ConsumerPolicy: consumerPolicy,
	}
}

// The input format for the policy check
type PolicyCompInput struct {
	NodeId         string                         `json:"node_id,omitempty"`
	NodePolicy     *externalpolicy.ExternalPolicy `json:"node_policy,omitempty"`
	BusinessPolId  string                         `json:"business_policy_id,omitempty"`
	BusinessPolicy *businesspolicy.BusinessPolicy `json:"business_policy,omitempty"`
	ServicePolicy  *externalpolicy.ExternalPolicy `json:"service_policy,omitempty"`
}

func (p PolicyCompInput) String() string {
	return fmt.Sprintf("NodeId: %v, NodePolicy: %v, BusinessPolId: %v, BusinessPolicy: %v, ServicePolicy: %v",
		p.NodeId, p.NodePolicy, p.BusinessPolId, p.BusinessPolicy, p.ServicePolicy)

}

func (p *PolicyCompInput) SetNodeId(nId string) {
	p.NodeId = nId
}

func (p *PolicyCompInput) SetNodePolicy(nodePolicy *externalpolicy.ExternalPolicy) {
	p.NodePolicy = nodePolicy
}

func (p *PolicyCompInput) SetBusinessPolId(bId string) {
	p.BusinessPolId = bId
}

func (p *PolicyCompInput) SetBusinessPolicy(businessPolicy *businesspolicy.BusinessPolicy) {
	p.BusinessPolicy = businessPolicy
}

func (p *PolicyCompInput) SetServicePolicy(servicePolicy *externalpolicy.ExternalPolicy) {
	p.ServicePolicy = servicePolicy
}

// exchange context using user credential
type UserExchangeContext struct {
	UserId      string
	Password    string
	URL         string
	CSSURL      string
	HTTPFactory *config.HTTPClientFactory
}

func (u *UserExchangeContext) GetExchangeId() string {
	return u.UserId
}

func (u *UserExchangeContext) GetExchangeToken() string {
	return u.Password
}

func (u *UserExchangeContext) GetExchangeURL() string {
	return u.URL
}

func (u *UserExchangeContext) GetCSSURL() string {
	return u.CSSURL
}

func (u *UserExchangeContext) GetHTTPFactory() *config.HTTPClientFactory {
	return u.HTTPFactory
}

// Given the PolicyCompInput, check if the policies are compatible.
// The required fields in PolicyCompInput are:
//  (NodeId or NodePolicy) and (BusinessPolId or BusinessPolicy)
//
// When checking whether the policies are compatible or not, we devide policies into two side:
//    Edge side: node policy (including the node built-in policy)
//    Agbot side: business policy + service policy + service built-in properties
func PolicyCompatible(ec exchange.ExchangeContext, input *PolicyCompInput) (*PolicyCompOutput, error) {
	if input == nil {
		return nil, NewCompCheckError(fmt.Errorf("The input cannot be null"), COMPCHECK_INPUT_ERROR)
	}

	// validate node policy and convert it to internal policy
	var nPolicy *policy.Policy
	nodeId := input.NodeId
	if input.NodePolicy != nil {
		if err := input.NodePolicy.Validate(); err != nil {
			return nil, NewCompCheckError(fmt.Errorf("Failed to validate the node policy. %v", err), COMPCHECK_VALIDATION_ERROR)
		} else {
			// give nodeId a default if it is empty
			if nodeId == "" {
				nodeId = "TempNodePolicyId"
			}

			var err1 error
			nPolicy, err1 = policy.GenPolicyFromExternalPolicy(input.NodePolicy, policy.MakeExternalPolicyHeaderName(nodeId))
			if err1 != nil {
				return nil, NewCompCheckError(fmt.Errorf("Failed to convert node policy to internal policy format for node %v: %v", nodeId, err1), COMPCHECK_CONVERTION_ERROR)
			}
		}
	} else if nodeId != "" {
		var err1 error
		nPolicy, err1 = GetNodePolicy(ec, nodeId)
		if err1 != nil {
			return nil, err1
		} else if nPolicy == nil {
			return nil, NewCompCheckError(fmt.Errorf("No node policy found for this node %v.", nodeId), COMPCHECK_INPUT_ERROR)
		}
	} else {
		return nil, NewCompCheckError(fmt.Errorf("Neither node policy nor node id is specified."), COMPCHECK_INPUT_ERROR)
	}

	// validate and convert the business policy to internal policy
	var bPolicy *policy.Policy
	bpId := input.BusinessPolId
	if input.BusinessPolicy != nil {
		// give bpId a default value if it is empty
		if bpId == "" {
			bpId = "TempBusinessPolicyId"
		}
		//convert busineess policy to internal policy, the validate is done in this function
		var err1 error
		bPolicy, err1 = input.BusinessPolicy.GenPolicyFromBusinessPolicy(bpId)
		if err1 != nil {
			return nil, NewCompCheckError(fmt.Errorf("Failed to convert business policy %v to internal policy format: %v", bpId, err1), COMPCHECK_CONVERTION_ERROR)
		}
	} else if bpId != "" {
		var err1 error
		bPolicy, err1 = GetBusinessPolicy(ec, bpId)
		if err1 != nil {
			return nil, err1
		} else if bPolicy == nil {
			return nil, NewCompCheckError(fmt.Errorf("No business policy found for this id %v.", bpId), COMPCHECK_INPUT_ERROR)
		}
	} else {
		return nil, NewCompCheckError(fmt.Errorf("Neither business policy nor business policy is are specified."), COMPCHECK_INPUT_ERROR)
	}

	return PolicyCompatible_Pols(ec, nPolicy, bPolicy, input.ServicePolicy)

}

// Given the node id and the business id, check if the policies are compatible.
// When checking whether the policies are compatible or not, we devide policies into two side:
//    Edge side: node policy (including the node built-in policy)
//    Agbot side: business policy + service policy + service built-in properties
func PolicyCompatible_IDs(ec exchange.ExchangeContext, nodeId string, bpId string) (*PolicyCompOutput, error) {
	// get node policy
	nodePolicy, err := GetNodePolicy(ec, nodeId)
	if err != nil {
		return nil, err
	} else if nodePolicy == nil {
		return nil, NewCompCheckError(fmt.Errorf("No node policy found for this node %v.", nodeId), COMPCHECK_INPUT_ERROR)
	}

	// get business policy
	businessPolicy, err := GetBusinessPolicy(ec, bpId)
	if err != nil {
		return nil, err
	} else if businessPolicy == nil {
		return nil, NewCompCheckError(fmt.Errorf("No business policy found for %v.", bpId), COMPCHECK_INPUT_ERROR)
	}

	return PolicyCompatible_Pols(ec, nodePolicy, businessPolicy, nil)
}

// Given the node policy, business policy,  check if the policies are compatible.
// servicePolicy is optional, the function will get the service policy from the exchange if it is nil.
// If the servicePolicy is not nil, ec can be nil because no exchange calls will be made.
// bpId and nodeId are optional.
//
// When checking whether the policies are compatible or not, we devide policies into two side:
//    Edge side: node policy (including the node built-in policy)
//    Agbot side: business policy + service policy + service built-in properties
func PolicyCompatible_ExtPols(ec exchange.ExchangeContext,
	nodePolicy *externalpolicy.ExternalPolicy,
	businessPolicy *businesspolicy.BusinessPolicy,
	servicePolicy *externalpolicy.ExternalPolicy,
	nodeId string, bpId string) (*PolicyCompOutput, error) {

	// validate node policy and convert it to internal policy
	var nPolicy *policy.Policy
	if nodePolicy == nil {
		return nil, NewCompCheckError(fmt.Errorf("Node policy cannot be nil"), COMPCHECK_INPUT_ERROR)
	} else if err := nodePolicy.Validate(); err != nil {
		return nil, NewCompCheckError(fmt.Errorf("Failed to validate the node policy. %v", err), COMPCHECK_VALIDATION_ERROR)
	} else {
		// give nodeId a default if it is empty
		if nodeId == "" {
			nodeId = "TempNodePolicyId"
		}

		var err1 error
		nPolicy, err1 = policy.GenPolicyFromExternalPolicy(nodePolicy, policy.MakeExternalPolicyHeaderName(nodeId))
		if err1 != nil {
			return nil, NewCompCheckError(fmt.Errorf("Failed to convert node policy to internal policy format for node %v: %v", nodeId, err1), COMPCHECK_CONVERTION_ERROR)
		}
	}

	// validate and convert the business policy to internal policy
	var bPolicy *policy.Policy
	if businessPolicy == nil {
		return nil, NewCompCheckError(fmt.Errorf("Business policy cannot be nil."), COMPCHECK_INPUT_ERROR)
	} else {
		// give bpId a default value if it is empty
		if bpId == "" {
			bpId = "TempBusinessPolicyId"
		}
		//convert busineess policy to internal policy, the validate is done in this function
		var err1 error
		bPolicy, err1 = businessPolicy.GenPolicyFromBusinessPolicy(bpId)
		if err1 != nil {
			return nil, NewCompCheckError(fmt.Errorf("Failed to convert business policy %v to internal policy format: %v", bpId, err1), COMPCHECK_CONVERTION_ERROR)
		}
	}

	return PolicyCompatible_Pols(ec, nPolicy, bPolicy, servicePolicy)
}

// Given the node internal policy and the business internal policy, check if the policies are compatible.
// servicePolicy is optional, the function will get the service policy from the exchange if it is nil.
// If the servicePolicy is not nil, ec can be nil because no exchange calls will be made.
// When checking whether the policies are compatible or not, we devide policies into two side:
//    Edge side: node policy (including the node built-in policy)
//    Agbot side: business policy + service policy + service built-in properties
func PolicyCompatible_Pols(ec exchange.ExchangeContext,
	nodePolicy, businessPolicy *policy.Policy,
	servicePolicy *externalpolicy.ExternalPolicy) (*PolicyCompOutput, error) {
	// check node policy
	if nodePolicy == nil {
		return nil, NewCompCheckError(fmt.Errorf("No node policy cannot be nil"), COMPCHECK_INPUT_ERROR)
	}

	// check bussiness policy
	if businessPolicy == nil {
		return nil, NewCompCheckError(fmt.Errorf("Business policy cannot be nil."), COMPCHECK_INPUT_ERROR)
	}
	if businessPolicy.Workloads == nil || len(businessPolicy.Workloads) == 0 {
		return nil, NewCompCheckError(fmt.Errorf("No services specified in the business policy %v.", businessPolicy.Header.Name), COMPCHECK_VALIDATION_ERROR)
	}
	service := businessPolicy.Workloads[0]

	// get service policy + default properties
	var servicePolicyToUse *externalpolicy.ExternalPolicy
	if servicePolicy == nil {
		// get service policy + service default properties
		var err1 error
		servicePolicyToUse, _, err1 = GetServicePolicyWithDefaultProperties(ec, service.WorkloadURL, service.Org, service.Version, service.Arch)
		if err1 != nil {
			return nil, err1
		}
	} else {
		// get default service properties
		builtInSvcPol := externalpolicy.CreateServiceBuiltInPolicy(service.WorkloadURL, service.Org, service.Version, service.Arch)
		// add built-in service properties to the service policy
		servicePolicyToUse = AddDefaultPropertiesToServicePolicy(servicePolicy, builtInSvcPol)
	}

	// validate service policy
	if err := servicePolicyToUse.Validate(); err != nil {
		return nil, NewCompCheckError(fmt.Errorf("Failed to validate the service policy. %v", err), COMPCHECK_VALIDATION_ERROR)
	}

	// merge the service policy, the default service properties
	mergedConsumerPol, err := MergeFullServicePolicyToBusinessPolicy(businessPolicy, servicePolicyToUse)
	if err != nil {
		return nil, err
	}

	// check if the node policy and merged bp policy are compatible
	if err := policy.Are_Compatible(nodePolicy, mergedConsumerPol); err != nil {
		return NewPolicyCompOutput(false, err.Error(), nodePolicy, mergedConsumerPol), nil
	} else {
		// policy match
		return NewPolicyCompOutput(true, "", nodePolicy, mergedConsumerPol), nil
	}
}

// Get node policy from the exchange and convert it to internal policy.
// It returns (nil, nil) If there is no node policy found.
func GetNodePolicy(ec exchange.ExchangeContext, nodeId string) (*policy.Policy, error) {

	// check input
	if nodeId == "" {
		return nil, NewCompCheckError(fmt.Errorf("Node id is empty."), COMPCHECK_INPUT_ERROR)
	} else {
		nodeOrg := exchange.GetOrg(nodeId)
		if nodeOrg == "" {
			return nil, NewCompCheckError(fmt.Errorf("Organization is not specified in the node id: %v.", nodeId), COMPCHECK_INPUT_ERROR)
		}
	}

	// get node policy
	nodePolicyHandler := exchange.GetHTTPNodePolicyHandler(ec)
	nodePolicy, err := nodePolicyHandler(nodeId)
	if err != nil {
		return nil, NewCompCheckError(fmt.Errorf("Error trying to query node policy for %v: %v", nodeId, err), COMPCHECK_EXCHANGE_ERROR)
	}

	if nodePolicy == nil {
		return nil, nil
	}

	// validate the policy
	extPolicy := nodePolicy.GetExternalPolicy()
	if err := extPolicy.Validate(); err != nil {
		return nil, NewCompCheckError(fmt.Errorf("Failed to validate the node policy for node %v. %v", nodeId, err), COMPCHECK_VALIDATION_ERROR)
	}

	// convert the policy to internal policy format
	pPolicy, err := policy.GenPolicyFromExternalPolicy(&extPolicy, policy.MakeExternalPolicyHeaderName(nodeId))
	if err != nil {
		return nil, NewCompCheckError(fmt.Errorf("Failed to convert node policy to internal policy format for node %v: %v", nodeId, err), COMPCHECK_CONVERTION_ERROR)
	}
	return pPolicy, nil
}

// Get business policy from the exchange the convert it to internal policy
func GetBusinessPolicy(ec exchange.ExchangeContext, bpId string) (*policy.Policy, error) {

	// check input
	if bpId == "" {
		return nil, NewCompCheckError(fmt.Errorf("Business policy id is empty."), COMPCHECK_INPUT_ERROR)
	} else {
		bpOrg := exchange.GetOrg(bpId)
		if bpOrg == "" {
			return nil, NewCompCheckError(fmt.Errorf("Organization is not specified in the business policy id: %v.", bpId), COMPCHECK_INPUT_ERROR)
		}
	}

	// get poicy from the exchange
	getBusinessPolicies := exchange.GetHTTPBusinessPoliciesHandler(ec)
	exchPols, err := getBusinessPolicies(exchange.GetOrg(bpId), exchange.GetId(bpId))
	if err != nil {
		return nil, NewCompCheckError(fmt.Errorf("Unable to get business polices for %v, error %v", bpId, err), COMPCHECK_EXCHANGE_ERROR)
	}
	if exchPols == nil || len(exchPols) == 0 {
		return nil, nil
	}

	// convert the business policy to internal policy format
	for polId, exchPol := range exchPols {
		bPol := exchPol.GetBusinessPolicy()

		pPolicy, err := bPol.GenPolicyFromBusinessPolicy(polId)
		if err != nil {
			return nil, NewCompCheckError(fmt.Errorf("Failed to convert business policy %v to internal policy format: %v", bpId, err), COMPCHECK_CONVERTION_ERROR)
		}
		return pPolicy, nil
	}

	return nil, nil
}

// Get service policy from the exchange
func GetServicePolicyWithId(ec exchange.ExchangeContext, svcId string) (*externalpolicy.ExternalPolicy, error) {
	// check input
	if svcId == "" {
		return nil, NewCompCheckError(fmt.Errorf("Service policy id is empty."), COMPCHECK_INPUT_ERROR)
	} else {
		svcOrg := exchange.GetOrg(svcId)
		if svcOrg == "" {
			return nil, NewCompCheckError(fmt.Errorf("Organization is not specified in the service policy id: %v.", svcId), COMPCHECK_INPUT_ERROR)
		}
	}

	// get service policy from the exchange
	servicePolicyHandler := exchange.GetHTTPServicePolicyWithIdHandler(ec)
	servicePolicy, err := servicePolicyHandler(svcId)
	if err != nil {
		return nil, NewCompCheckError(fmt.Errorf("Error trying to query service policy for %v: %v", svcId, err), COMPCHECK_EXCHANGE_ERROR)
	} else if servicePolicy == nil {
		return nil, nil
	} else {
		// validate the policy
		extPolicy := servicePolicy.GetExternalPolicy()
		if err := extPolicy.Validate(); err != nil {
			return nil, NewCompCheckError(fmt.Errorf("Failed to validate the service policy %v. %v", svcId, err), COMPCHECK_VALIDATION_ERROR)
		}
		return &extPolicy, nil
	}
}

// Get service policy from the exchange
func GetServicePolicy(ec exchange.ExchangeContext, svcUrl string, svcOrg string, svcVersion string, svcArch string) (*externalpolicy.ExternalPolicy, string, error) {
	// check input
	if svcUrl == "" {
		return nil, "", NewCompCheckError(fmt.Errorf("Service name is empty."), COMPCHECK_INPUT_ERROR)
	} else if svcOrg == "" {
		return nil, "", NewCompCheckError(fmt.Errorf("Service organization is empty."), COMPCHECK_INPUT_ERROR)
	}

	// get service policy fromt the exchange
	servicePolicyHandler := exchange.GetHTTPServicePolicyHandler(ec)
	servicePolicy, sId, err := servicePolicyHandler(svcUrl, svcOrg, svcVersion, svcArch)
	if err != nil {
		return nil, "", NewCompCheckError(fmt.Errorf("Error trying to query service policy for %v/%v %v %v. %v", svcOrg, svcUrl, svcVersion, svcArch, err), COMPCHECK_EXCHANGE_ERROR)
	} else if servicePolicy == nil {
		return nil, "", nil
	} else {
		extPolicy := servicePolicy.GetExternalPolicy()
		if err := extPolicy.Validate(); err != nil {
			return nil, "", NewCompCheckError(fmt.Errorf("Failed to validate the service policy for %v/%v %v %v. %v", svcOrg, svcUrl, svcVersion, svcArch, err), COMPCHECK_VALIDATION_ERROR)
		}
		return &extPolicy, sId, nil
	}
}

// Get service policy from the exchange and then add the service defalt properties
func GetServicePolicyWithDefaultProperties(ec exchange.ExchangeContext, svcUrl string, svcOrg string, svcVersion string, svcArch string) (*externalpolicy.ExternalPolicy, string, error) {

	servicePol, sId, err := GetServicePolicy(ec, svcUrl, svcOrg, svcVersion, svcArch)
	if err != nil {
		return nil, "", err
	}

	// get default service properties
	builtInSvcPol := externalpolicy.CreateServiceBuiltInPolicy(svcUrl, svcOrg, svcVersion, svcArch)

	// add built-in service properties to the service policy
	merged_pol := AddDefaultPropertiesToServicePolicy(servicePol, builtInSvcPol)

	return merged_pol, sId, nil
}

// Add service default properties to the given service policy
func AddDefaultPropertiesToServicePolicy(servicePol, defaultSvcProps *externalpolicy.ExternalPolicy) *externalpolicy.ExternalPolicy {
	var merged_pol externalpolicy.ExternalPolicy
	if servicePol != nil {
		merged_pol = externalpolicy.ExternalPolicy(*servicePol)
		if defaultSvcProps != nil {
			(&merged_pol).MergeWith(defaultSvcProps, false)
		}
	} else {
		if defaultSvcProps != nil {
			merged_pol = externalpolicy.ExternalPolicy(*defaultSvcProps)
		}
	}

	return &merged_pol
}

// Merge a service policy into a business policy. The service policy
func MergeFullServicePolicyToBusinessPolicy(businessPolicy *policy.Policy, servicePolicy *externalpolicy.ExternalPolicy) (*policy.Policy, error) {
	if businessPolicy == nil {
		return nil, NewCompCheckError(fmt.Errorf("The given business policy should not be nil for MergeFullServicePolicyToBusinessPolicy"), COMPCHECK_INPUT_ERROR)
	}
	if servicePolicy == nil {
		return businessPolicy, nil
	}

	if pPolicy, err := policy.MergePolicyWithExternalPolicy(businessPolicy, servicePolicy); err != nil {
		return nil, NewCompCheckError(fmt.Errorf("error merging business policy with service policy. %v", err), COMPCHECK_MERGING_ERROR)
	} else {
		return pPolicy, nil
	}
}

// This function merges the given business policy with the given built-in properties of the service and the given service policy
// from the top level service, if any.
func MergeServicePolicyToBusinessPolicy(businessPol *policy.Policy, builtInSvcPol *externalpolicy.ExternalPolicy, servicePol *externalpolicy.ExternalPolicy) (*policy.Policy, error) {
	if businessPol == nil {
		return nil, nil
	}

	// add built-in service properties to the service policy
	merged_pol1 := AddDefaultPropertiesToServicePolicy(servicePol, builtInSvcPol)

	//merge service policy
	if merged_pol2, err := MergeFullServicePolicyToBusinessPolicy(businessPol, merged_pol1); err != nil {
		return nil, err
	} else {
		return merged_pol2, nil
	}
}
