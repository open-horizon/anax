package compcheck

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/semanticversion"
	"golang.org/x/text/message"
	"strings"
)

// The input format for the userinput check
type SecretBindingCheck struct {
	NodeId         string                                `json:"node_id,omitempty"`
	NodeArch       string                                `json:"node_arch,omitempty"`
	NodeType       string                                `json:"node_type,omitempty"` // can be omitted if node_id is specified
	NodeOrg        string                                `json:"node_org,omitempty"`  // can be omitted if node_id is specified
	BusinessPolId  string                                `json:"business_policy_id,omitempty"`
	BusinessPolicy *businesspolicy.BusinessPolicy        `json:"business_policy,omitempty"`
	PatternId      string                                `json:"pattern_id,omitempty"`
	Pattern        common.AbstractPatternFile            `json:"pattern,omitempty"`
	Service        []common.AbstractServiceFile          `json:"service,omitempty"`
	ServiceToCheck []string                              `json:"service_to_check,omitempty"`   // for internal use for performance. only check the service with the ids. If empty, check all.
	DepServices    map[string]exchange.ServiceDefinition `json:"dependent_services,omitempty"` // for internal use for performance. A map of service definition keyed by id.
	// It is either empty or provides ALL the dependent services needed. It is expected the top level service definitions are provided
	// in the 'Service' attribute when this attribute is not empty.
}

// unmashal handler for SecretBindingCheck object to handle AbstractPatternFile and AbstractServiceFile
func (p *SecretBindingCheck) UnmarshalJSON(b []byte) error {

	var cc CompCheck_NoAbstract
	if err := json.Unmarshal(b, &cc); err != nil {
		return err
	}

	p.NodeId = cc.NodeId
	p.NodeArch = cc.NodeArch
	p.NodeType = cc.NodeType
	p.NodeOrg = cc.NodeOrg
	p.BusinessPolId = cc.BusinessPolId
	p.BusinessPolicy = cc.BusinessPolicy
	p.PatternId = cc.PatternId

	if cc.Pattern != nil {
		p.Pattern = cc.Pattern
	}

	if cc.Service != nil && len(cc.Service) != 0 {
		p.Service = []common.AbstractServiceFile{}
		for index, _ := range cc.Service {
			p.Service = append(p.Service, &cc.Service[index])
		}
	}

	return nil
}

// This is the function that HZN and the agbot secure API calls.
// Given the SecretBindingCheck input, check if the service bindings
// defined in the deployment policy or pattern are compatible.
// The required fields in SecretBindingCheck are:
//  (NodeId or (NodeArch and NodeType and NodeOrg)) and
//  (BusinessPolId or BusinessPolicy or PatternId or Pattern)
//
// When checking whether the secret binding compatible or not, the
// node org will be used as the secret org.
// When agbotUrl is an empty string, it will not do the secret name varification
// in the secret manager. Instead, the function returns an array of secret bindings
// that are required by the services and an array containing the ones that are extraneous.
// This will allow the caller to do own secret name verification in the secret manager.
func SecretBindingCompatible(ec exchange.ExchangeContext,
	agbotUrl string,
	sbcInput *SecretBindingCheck,
	checkAllSvcs bool,
	msgPrinter *message.Printer) (*CompCheckOutput, error) {

	getDeviceHandler := exchange.GetHTTPDeviceHandler(ec)
	getBusinessPolicies := exchange.GetHTTPBusinessPoliciesHandler(ec)
	getPatterns := exchange.GetHTTPExchangePatternHandler(ec)
	serviceDefResolverHandler := exchange.GetHTTPServiceDefResolverHandler(ec)
	getSelectedServices := exchange.GetHTTPSelectedServicesHandler(ec)

	if agbotUrl != "" {
		vaultSecretExists := exchange.GetHTTPVaultSecretExistsHandler(ec)
		return secretBindingCompatible(getDeviceHandler, getBusinessPolicies, getPatterns, serviceDefResolverHandler, getSelectedServices, vaultSecretExists, agbotUrl, sbcInput, checkAllSvcs, msgPrinter)
	} else {
		// when agbotUrl is an empty string, it will not do the secret name varification in the secret manager.
		return secretBindingCompatible(getDeviceHandler, getBusinessPolicies, getPatterns, serviceDefResolverHandler, getSelectedServices, nil, "", sbcInput, checkAllSvcs, msgPrinter)
	}
}

// Internal function for SecretBindingCompatible
func secretBindingCompatible(getDeviceHandler exchange.DeviceHandler,
	getBusinessPolicies exchange.BusinessPoliciesHandler,
	getPatterns exchange.PatternHandler,
	serviceDefResolverHandler exchange.ServiceDefResolverHandler,
	getSelectedServices exchange.SelectedServicesHandler,
	vaultSecretExists exchange.VaultSecretExistsHandler,
	agbotUrl string,
	sbcInput *SecretBindingCheck,
	checkAllSvcs bool,
	msgPrinter *message.Printer) (*CompCheckOutput, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if sbcInput == nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The SecretBindingCheck input cannot be null")), COMPCHECK_INPUT_ERROR)
	}

	// make a copy of the input because the process will change it. The pointer to policies will stay the same.
	input_temp := SecretBindingCheck(*sbcInput)
	input := &input_temp

	// copy the input to a internal structure
	resources := NewCompCheckRsrcFromSecretBindingCheck(sbcInput)

	nodeId := input.NodeId
	if nodeId != "" {
		// get node org from the node id
		if resources.NodeOrg == "" {
			resources.NodeOrg = exchange.GetOrg(nodeId)
		}

		// get node type and node arch only when they are not
		node, err := GetExchangeNode(getDeviceHandler, nodeId, msgPrinter)
		if err != nil {
			return nil, err
		} else if input.NodeArch != "" {
			if node.Arch != "" && node.Arch != input.NodeArch {
				return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The input node architecture %v does not match the Exchange node architecture %v for node %v.", input.NodeArch, node.Arch, nodeId)), COMPCHECK_INPUT_ERROR)
			}
		} else {
			resources.NodeArch = node.Arch
		}

		resources.NodeType = node.NodeType
	}

	// verify the input node type value and get the node type for the node from
	// node id or from the input.
	if nodeType, err := VerifyNodeType(input.NodeType, resources.NodeType, nodeId, msgPrinter); err != nil {
		return nil, err
	} else if nodeType == persistence.DEVICE_TYPE_CLUSTER {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Node type '%v' does not support secret binding check.", nodeType)), COMPCHECK_INPUT_ERROR)
	} else {
		resources.NodeType = nodeType
	}

	// make sure only specify one: business policy or pattern
	useBPol := false
	if input.BusinessPolId != "" || input.BusinessPolicy != nil {
		useBPol = true
		if input.PatternId != "" || input.Pattern != nil {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Deployment policy and pattern are mutually exclusive.")), COMPCHECK_INPUT_ERROR)
		}
	} else {
		if input.PatternId == "" && input.Pattern == nil {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Neither deployment policy nor pattern is specified.")), COMPCHECK_INPUT_ERROR)
		}
	}

	// validate the business policy/pattern, get the user input and workloads from them.
	var secretBinding []exchangecommon.SecretBinding
	var serviceRefs []exchange.ServiceReference
	if useBPol {
		bPolicy, _, err := processBusinessPolicy(getBusinessPolicies, input.BusinessPolId, input.BusinessPolicy, false, msgPrinter)
		if err != nil {
			return nil, err
		} else if input.BusinessPolicy == nil {
			resources.BusinessPolicy = bPolicy
		}
		secretBinding = bPolicy.SecretBinding
		serviceRefs = getWorkloadsFromBPol(bPolicy, resources.NodeArch)
	} else {
		pattern, err := processPattern(getPatterns, input.PatternId, input.Pattern, msgPrinter)
		if err != nil {
			return nil, err
		} else if input.Pattern == nil {
			resources.Pattern = pattern
		}
		secretBinding = pattern.GetSecretBinding()
		serviceRefs = getWorkloadsFromPattern(pattern, resources.NodeArch)
	}
	if serviceRefs == nil || len(serviceRefs) == 0 {
		if resources.NodeArch != "" {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No service versions with architecture %v specified in the deployment policy or pattern.", resources.NodeArch)), COMPCHECK_VALIDATION_ERROR)
		} else {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No service versions specified in the deployment policy or pattern.")), COMPCHECK_VALIDATION_ERROR)
		}
	}

	// check if the given services match the services defined in the business policy or pattern
	if err := validateServices(resources.Service, resources.BusinessPolicy, resources.Pattern, input.ServiceToCheck, msgPrinter); err != nil {
		return nil, err
	}
	inServices := input.Service

	messages := map[string]string{}
	msg_incompatible := msgPrinter.Sprintf("Secret Binding Incompatible")
	msg_compatible := msgPrinter.Sprintf("Compatible")
	type_incompatible := msgPrinter.Sprintf("Type Incompatible")

	// go through all the workloads and check if user input is compatible or not
	service_comp := map[string]common.AbstractServiceFile{}
	service_incomp := map[string]common.AbstractServiceFile{}
	svc_type_mismatch := map[string]bool{}
	overall_compatible := true

	// save all the services that are retrieved from the exchange so that
	// they can be used later
	dep_services := map[string]exchange.ServiceDefinition{}
	top_services := []common.AbstractServiceFile{}

	// keep track of which indexes in the secretBinding array were used
	index_map := map[int]map[string]bool{}

	for _, serviceRef := range serviceRefs {
		service_compatible := false
		for _, workload := range serviceRef.ServiceVersions {

			// get service + dependen services and then compare the user inputs
			if inServices == nil || len(inServices) == 0 {
				if serviceRef.ServiceArch != "*" && serviceRef.ServiceArch != "" {
					sId := cutil.FormExchangeIdForService(serviceRef.ServiceURL, workload.Version, serviceRef.ServiceArch)
					sId = fmt.Sprintf("%v/%v", serviceRef.ServiceOrg, sId)
					if !needHandleService(sId, input.ServiceToCheck) {
						continue
					}
					sSpec := NewServiceSpec(serviceRef.ServiceURL, serviceRef.ServiceOrg, workload.Version, serviceRef.ServiceArch)
					if compatible, reason, imap, topSvcDef, _, depSvcDefs, err := VerifySecretBindingForService(sSpec, serviceDefResolverHandler, vaultSecretExists, agbotUrl, secretBinding, resources.NodeOrg, msgPrinter); err != nil {
						return nil, err
					} else {
						// for performance, save the services that gotten from the exchange for use later
						for depId, depDef := range depSvcDefs {
							dep_services[depId] = depDef
						}
						top_services = append(top_services, topSvcDef)

						CombineIndexMap(index_map, imap)

						// check service type and node type compatibility
						compatible_t, reason_t := CheckTypeCompatibility(resources.NodeType, topSvcDef, msgPrinter)
						if !compatible_t {
							reason = reason_t
							svc_type_mismatch[sId] = true
						}

						if compatible && compatible_t {
							service_compatible = true
							service_comp[sId] = topSvcDef
							messages[sId] = msg_compatible
							if !checkAllSvcs {
								break
							}
						} else {
							service_incomp[sId] = topSvcDef
							messages[sId] = FormatReasonMessage(reason, !compatible_t, msg_incompatible, type_incompatible)
						}
					}
				} else {
					// since workload arch is empty, need to go through all the arches
					if svcMeta, err := getSelectedServices(serviceRef.ServiceURL, serviceRef.ServiceOrg, workload.Version, ""); err != nil {
						return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error getting services for all archetctures for %v/%v version %v. %v", serviceRef.ServiceOrg, serviceRef.ServiceURL, workload.Version, err)), COMPCHECK_EXCHANGE_ERROR)
					} else {
						for sId, s := range svcMeta {
							org := exchange.GetOrg(sId)
							svc := ServiceDefinition{org, s}
							if !needHandleService(sId, input.ServiceToCheck) {
								continue
							}
							if compatible, reason, imap, depSvcDefs, err := VerifySecretBindingForServiceDef(&svc, resources.DepServices, serviceDefResolverHandler, vaultSecretExists, agbotUrl, secretBinding, resources.NodeOrg, msgPrinter); err != nil {
								return nil, err
							} else {
								// for performance, save the services that gotten from the exchange for use later
								for depId, depDef := range depSvcDefs {
									dep_services[depId] = depDef
								}
								top_services = append(top_services, &svc)

								CombineIndexMap(index_map, imap)

								// check service type and node type compatibility
								compatible_t, reason_t := CheckTypeCompatibility(resources.NodeType, &svc, msgPrinter)
								if !compatible_t {
									reason = reason_t
									svc_type_mismatch[sId] = true
								}

								if compatible && compatible_t {
									service_compatible = true
									service_comp[sId] = &svc
									messages[sId] = msg_compatible
									if !checkAllSvcs {
										break
									}
								} else {
									service_incomp[sId] = &svc
									messages[sId] = FormatReasonMessage(reason, !compatible_t, msg_incompatible, type_incompatible)
								}
							}
						}
						if service_compatible && !checkAllSvcs {
							break
						}
					}
				}
			} else {
				useSDef := GetServiceFromInput(serviceRef.ServiceURL, serviceRef.ServiceOrg, workload.Version, serviceRef.ServiceArch, inServices)

				sId := cutil.FormExchangeIdForService(serviceRef.ServiceURL, workload.Version, serviceRef.ServiceArch)
				sId = fmt.Sprintf("%v/%v", serviceRef.ServiceOrg, sId)
				if !needHandleService(sId, input.ServiceToCheck) {
					continue
				}
				if useSDef == nil {
					messages[sId] = fmt.Sprintf("%v: %v", msg_incompatible, msgPrinter.Sprintf("Service definition not found in the input."))
					// add a fake service for easy logic later
					service_incomp[sId] = &ServiceDefinition{}
				} else {
					if useSDef.GetOrg() == "" {
						useSDef.(*common.ServiceFile).Org = serviceRef.ServiceOrg
					}
					if compatible, reason, imap, depSvcDefs, err := VerifySecretBindingForServiceDef(useSDef, resources.DepServices, serviceDefResolverHandler, vaultSecretExists, agbotUrl, secretBinding, resources.NodeOrg, msgPrinter); err != nil {
						return nil, err
					} else {
						// for performance, save the services that gotten from the exchange for use later
						for depId, depDef := range depSvcDefs {
							dep_services[depId] = depDef
						}
						top_services = append(top_services, useSDef)

						CombineIndexMap(index_map, imap)

						// check service type and node type compatibility
						compatible_t, reason_t := CheckTypeCompatibility(resources.NodeType, useSDef, msgPrinter)
						if !compatible_t {
							reason = reason_t
							svc_type_mismatch[sId] = true
						}

						if compatible && compatible_t {
							service_compatible = true
							service_comp[sId] = useSDef
							messages[sId] = msg_compatible
							if !checkAllSvcs {
								break
							}
						} else {
							service_incomp[sId] = useSDef
							messages[sId] = FormatReasonMessage(reason, !compatible_t, msg_incompatible, type_incompatible)
						}
					}
				}
			}
		}

		// for policy case, overall_compatible turn to false if any service is not compatible.
		// for pattern case, cannot turn overall_compatible to false because we need to ignore
		// the type mismatch case.
		if overall_compatible && !service_compatible {
			if useBPol {
				overall_compatible = false
			} else if len(service_incomp) != len(svc_type_mismatch) {
				// some compatibility errors are from non type mismatch
				overall_compatible = false
			}
		}
	}

	// save the services retrieved from the exchange
	resources.DepServices = dep_services
	resources.Service = top_services

	// for the pattern case, if all the services are type mismatch then the overall_compatible should
	// turn to false
	if overall_compatible && !useBPol {
		if len(service_comp) == 0 && len(svc_type_mismatch) > 0 {
			overall_compatible = false
		}
	}

	// not that all the secrets have bindings, check the extraneous secret bindings
	neededSB, extraneousSB := GroupSecretBindings(secretBinding, index_map)
	resources.NeededSB = neededSB
	resources.ExtraneousSB = extraneousSB

	if messages != nil && len(messages) != 0 {
		if overall_compatible {
			resources.Service = ServicesFromServiceMap(service_comp)

			if len(extraneousSB) != 0 {
				messages["general"] = msgPrinter.Sprintf("Warning: The following secret bindings are not required by any services: %v", extraneousSB)
			}
		} else {
			resources.Service = ServicesFromServiceMap(service_incomp)
		}

		return NewCompCheckOutput(overall_compatible, messages, resources), nil
	} else {
		// If we get here, it means that no workload is found in the bp/pattern that matches the required node arch.
		if resources.NodeArch != "" {
			messages["general"] = fmt.Sprintf("%v: %v", msg_incompatible, msgPrinter.Sprintf("Service with 'arch' %v cannot be found in the deployment policy or pattern.", resources.NodeArch))
		} else {
			messages["general"] = fmt.Sprintf("%v: %v", msg_incompatible, msgPrinter.Sprintf("No services found in the deployment policy or pattern."))
		}

		return NewCompCheckOutput(false, messages, resources), nil
	}
}

// Given the top level and dependent services, assuming the dependentServices contain
// all and only the dependent services for this top level service,
// this function does the following:
// 1. check if the given secret bindings satisfy the service secret requirements.
// 2. if vaultSecretExists is not nil, check if the required secret binding names exist in the secret manager.
// It returns (compatible, reason for not compatible, index map, error).
//    where the index map is keyed by the index of the secretBinding array,
//    the value is a map of service secret names in the binding
//    that are needed. Using map here instead of array to make it easy to remove the
//    duplicates.
// The caller must make sure that the dependent services are accurate and no extranous ones.
// For performance reason, this function does not check the valadity of the dependent services.
func VerifySecretBindingForServiceCache(sTopDef common.AbstractServiceFile,
	dependentServices map[string]exchange.ServiceDefinition,
	secretBinding []exchangecommon.SecretBinding,
	vaultSecretExists exchange.VaultSecretExistsHandler,
	agbotUrl string, nodeOrg string,
	msgPrinter *message.Printer) (bool, string, map[int]map[string]bool, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// nothing to check
	if sTopDef == nil {
		return false, "", nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The input service definition object cannot be null.")), COMPCHECK_INPUT_ERROR)
	}

	// map: keyed by index of the secretBinding array, the value is a map of
	// service secret names in the binding that are needed, i.e. not redundant.
	//    map[index]map[service_secret_names]bool
	index_map := map[int]map[string]bool{}

	// verify top level services
	sId := cutil.FormExchangeIdForService(sTopDef.GetURL(), sTopDef.GetVersion(), sTopDef.GetArch())
	sId = fmt.Sprintf("%v/%v", sTopDef.GetOrg(), sId)
	if index, neededSb, err := ValidateSecretBindingForSingleService(secretBinding, sTopDef, msgPrinter); err != nil {
		return false, err.Error(), index_map, nil
	} else {
		UpdateIndexMap(index_map, index, neededSb)
	}

	// verify dependent services
	for id, s := range dependentServices {
		if index, neededSb, err := ValidateSecretBindingForSingleService(secretBinding, &ServiceDefinition{exchange.GetOrg(id), s}, msgPrinter); err != nil {
			return false, err.Error(), index_map, nil
		} else {
			UpdateIndexMap(index_map, index, neededSb)
		}
	}

	// verify secrets exist in the secret manager
	if agbotUrl != "" && vaultSecretExists != nil {
		neededSB, _ := GroupSecretBindings(secretBinding, index_map)
		if verified, reason, err := VerifyVaultSecrets_strict(neededSB, nodeOrg, agbotUrl, vaultSecretExists, msgPrinter); err != nil {
			return false, "", index_map, fmt.Errorf(msgPrinter.Sprintf("Error verifying secret in the secret manager. %v", err))
		} else {
			return verified, reason, index_map, nil
		}
	}

	return true, "", index_map, nil
}

// This function does the following:
// 1. go to the exchange and get the top level service and its dependent services.
// 2. check if the given secret bindings satisfy the service secret requirements.
// 3. if vaultSecretExists is not nil, check if the required secret binding names exist in the secret manager.
// It returns (compatible, reason for not compatible, index map, top level service def, top level service id, a map dependen service defs keyed by service id, error).
//    where the index map is keyed by the index of the secretBinding array,
//    the value is a map of service secret names in the binding
//    that are needed. Using map here instead of array to make it easy to remove the
//    duplicates.
func VerifySecretBindingForService(svcSpec *ServiceSpec,
	serviceDefResolverHandler exchange.ServiceDefResolverHandler,
	vaultSecretExists exchange.VaultSecretExistsHandler, agbotUrl string,
	secretBinding []exchangecommon.SecretBinding, nodeOrg string,
	msgPrinter *message.Printer) (bool, string, map[int]map[string]bool, common.AbstractServiceFile, string, map[string]exchange.ServiceDefinition, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// nothing to check
	if svcSpec == nil {
		return false, "", nil, nil, "", nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The input service spec object cannot be null.")), COMPCHECK_INPUT_ERROR)
	}

	_, svc_map, sDef, sId, err := serviceDefResolverHandler(svcSpec.ServiceUrl, svcSpec.ServiceOrgid, svcSpec.ServiceVersionRange, svcSpec.ServiceArch)
	if err != nil {
		return false, "", nil, nil, "", nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error retrieving service from the Exchange for %v. %v", svcSpec, err)), COMPCHECK_EXCHANGE_ERROR)
	}

	compSDef := ServiceDefinition{svcSpec.ServiceOrgid, *sDef}

	compatible, reason, inxex_map, err := VerifySecretBindingForServiceCache(&compSDef, svc_map, secretBinding, vaultSecretExists, agbotUrl, nodeOrg, msgPrinter)
	return compatible, reason, inxex_map, &compSDef, sId, svc_map, err
}

// This function does the following:
// 1. go to the given dependentServices or exchange to get all the dependent services for the given service.
// 2. check if the given secret bindings satisfy the service secret requirements.
// 3. if vaultSecretExists is not nil, check if the required secret binding names exist in the secret manager.
// It returns (compatible, reason for not compatible, index map, a map dependen service defs keyed by service id, error).
//    where the index map is keyed by the index of the secretBinding array,
//    the value is a map of service secret names in the binding
//    that are needed. Using map here instead of array to make it easy to remove the
//    duplicates.
func VerifySecretBindingForServiceDef(sDef common.AbstractServiceFile,
	dependentServices map[string]exchange.ServiceDefinition, // can be nil
	serviceDefResolverHandler exchange.ServiceDefResolverHandler,
	vaultSecretExists exchange.VaultSecretExistsHandler, agbotUrl string,
	secretBinding []exchangecommon.SecretBinding, nodeOrg string,
	msgPrinter *message.Printer) (bool, string, map[int]map[string]bool, map[string]exchange.ServiceDefinition, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// nothing to check
	if sDef == nil {
		return false, "", nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The input service definition object cannot be null.")), COMPCHECK_INPUT_ERROR)
	}

	// get all the service defs for the dependent services for device type node
	service_map, err := GetServiceDependentDefs(sDef, dependentServices, serviceDefResolverHandler, msgPrinter)
	if err != nil {
		return false, "", nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to find the dependent services for %v/%v %v %v. %v", sDef.GetOrg(), sDef.GetURL(), sDef.GetArch(), sDef.GetVersion(), err)), COMPCHECK_GENERAL_ERROR)
	}

	compatible, reason, inxex_map, err := VerifySecretBindingForServiceCache(sDef, service_map, secretBinding, vaultSecretExists, agbotUrl, nodeOrg, msgPrinter)

	return compatible, reason, inxex_map, service_map, err
}

// Validate that the given secretBinding covers all the secrets defined in the given service.
// It also gives error if the secretBinding has bindings defined for the service but
// the service has no secrets.
// It returns the index of the SecretBinding object in the given that it is used for validation
// and an array of service secret names in the service binding that are needed by this service.
func ValidateSecretBindingForSingleService(secretBinding []exchangecommon.SecretBinding,
	sdef common.AbstractServiceFile, msgPrinter *message.Printer) (int, []string, error) {
	if sdef == nil {
		return -1, nil, nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// get the secret bindings for this service
	index, err := GetSecretBindingForService(secretBinding, sdef.GetOrg(), sdef.GetURL(), sdef.GetVersion(), sdef.GetArch(), msgPrinter)
	if err != nil {
		return index, nil, err
	}

	// cluster type does not have secrets
	if sdef.GetServiceType() == exchangecommon.SERVICE_TYPE_CLUSTER {
		if index == -1 {
			return index, nil, nil
		} else {
			return index, nil, fmt.Errorf(msgPrinter.Sprintf("Secret binding for a cluster service is not supported."))
		}
	}

	// convert the deployment string into object
	dConfig, err := common.ConvertToDeploymentConfig(sdef.GetDeployment(), msgPrinter)
	if err != nil {
		return index, nil, err
	}

	// create a map of all the secrets in the SecretBinding
	// for this service, it will be used to check if all the
	// bindings are used or not
	sbNeeded := map[string]bool{}

	// make sure each service secret has a binding
	noBinding := map[string]bool{}
	if dConfig != nil {
		for _, svcConf := range dConfig.Services {
			for sn, _ := range svcConf.Secrets {
				found := false
				if index != -1 {
					for _, vbind := range secretBinding[index].Secrets {
						key, vs := vbind.GetBinding()
						if sn == key {
							found = true
							if _, _, err := ParseVaultSecretName(vs, msgPrinter); err != nil {
								return index, nil, err
							}
							sbNeeded[sn] = true
							break
						}
					}
				}
				if !found {
					noBinding[sn] = true
				}
			}
		}
	}

	if len(noBinding) > 0 {
		// convert to array to display
		nbArray := []string{}
		for sn, _ := range noBinding {
			nbArray = append(nbArray, sn)
		}
		return index, nil, fmt.Errorf(msgPrinter.Sprintf("No secret binding found for the following service secrets: %v.", nbArray))
	}

	// return an array of service secret names in the biniding that are needed.
	used_sb := []string{}
	for k, v := range sbNeeded {
		if v == true {
			used_sb = append(used_sb, k)
		}
	}

	return index, used_sb, nil
}

// Given a list of SecretBinding's for multiples services, return index for
// the secret binding object in the given array that will be used by the given service.
// -1 means no secret binding defined for the given service
func GetSecretBindingForService(secretBinding []exchangecommon.SecretBinding, svcOrg, svcName, svcVersion, svcArch string,
	msgPrinter *message.Printer) (int, error) {

	if secretBinding == nil {
		return -1, nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	for index, sb := range secretBinding {
		if sb.ServiceUrl != svcName || sb.ServiceOrgid != svcOrg {
			continue
		}
		if sb.ServiceArch != "" && sb.ServiceArch != "*" && sb.ServiceArch != svcArch {
			continue
		}
		if sb.ServiceVersionRange != "" && sb.ServiceVersionRange != svcVersion {
			if vExp, err := semanticversion.Version_Expression_Factory(sb.ServiceVersionRange); err != nil {
				return -1, fmt.Errorf(msgPrinter.Sprintf("Wrong version string %v specified in secret binding for service %v/%v %v %v, error %v", sb.ServiceVersionRange, svcOrg, svcName, svcVersion, svcArch, err))
			} else if inRange, err := vExp.Is_within_range(svcVersion); err != nil {
				return -1, fmt.Errorf(msgPrinter.Sprintf("Error checking version %v in range %v. %v", svcVersion, vExp, err))
			} else if !inRange {
				continue
			}
		}

		return index, nil
	}

	return -1, nil
}

// Call the agbot API to verify the vault secrets exists.
// It does not return when the vault secret does not exist or there is an error accessing
// the vault api. Instead it will return a messages for each vault secret name that could
// not be verified.
func VerifyVaultSecrets(secretBinding []exchangecommon.SecretBinding, nodeOrg string, agbotURL string,
	vaultSecretExists exchange.VaultSecretExistsHandler, msgPrinter *message.Printer) (map[string]string, error) {

	if secretBinding == nil || len(secretBinding) == 0 {
		return nil, nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if agbotURL == "" {
		return nil, fmt.Errorf(msgPrinter.Sprintf("agbot URL cannot be an empty string when checking secret binding. Please make sure HZN_AGBOT_URL is set."))
	}

	if nodeOrg == "" {
		return nil, fmt.Errorf(msgPrinter.Sprintf("The node organization must be provided."))
	}

	// go through each secret binding making sure the vault secret exist in vault
	ret := map[string]string{}
	vs_checked := map[string]bool{}
	for _, sn := range secretBinding {
		for _, vbind := range sn.Secrets {

			// make sure each vault get checked only once
			_, vaultSecretName := vbind.GetBinding()
			if _, ok := vs_checked[vaultSecretName]; ok {
				continue
			} else {
				vs_checked[vaultSecretName] = true
			}

			if exists, err := VerifySingleVaultSecret(vaultSecretName, nodeOrg, agbotURL, vaultSecretExists, msgPrinter); err != nil {
				ret[vaultSecretName] = err.Error()
			} else if !exists {
				ret[vaultSecretName] = msgPrinter.Sprintf("Secret %v does not exist in the secret manager.", vaultSecretName)
			}
		}
	}

	return ret, nil
}

// Call the agbot API to verify the vault secrets exists.
// It returns immediately when a vault secret does not exist or there is an error accessing
// the vault api.
func VerifyVaultSecrets_strict(secretBinding []exchangecommon.SecretBinding, nodeOrg string, agbotURL string,
	vaultSecretExists exchange.VaultSecretExistsHandler, msgPrinter *message.Printer) (bool, string, error) {
	if secretBinding == nil || len(secretBinding) == 0 {
		return true, "", nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if agbotURL == "" {
		return false, "", fmt.Errorf(msgPrinter.Sprintf("agbot URL cannot be an empty string when checking secret binding. Please make sure HZN_AGBOT_URL is set."))
	}

	if nodeOrg == "" {
		return false, "", fmt.Errorf(msgPrinter.Sprintf("The node organization must be provided."))
	}

	// go through each secret binding making sure the vault secret exist in vault
	vs_checked := map[string]bool{}
	for _, sn := range secretBinding {
		for _, vbind := range sn.Secrets {
			_, vaultSecretName := vbind.GetBinding()

			// make sure each vault get checked only once
			if _, ok := vs_checked[vaultSecretName]; ok {
				continue
			} else {
				vs_checked[vaultSecretName] = true
			}

			if exists, err := VerifySingleVaultSecret(vaultSecretName, nodeOrg, agbotURL, vaultSecretExists, msgPrinter); err != nil {
				return false, "", err
			} else if !exists {
				return false, msgPrinter.Sprintf("Secret %v does not exist in the secret manager.", vaultSecretName), nil
			}
		}
	}

	return true, "", nil
}

// It calls the agbot API to verify whether the given secret name exist in vault or not.
func VerifySingleVaultSecret(vaultSecretName string, nodeOrg string, agbotURL string,
	vaultSecretExists exchange.VaultSecretExistsHandler, msgPrinter *message.Printer) (bool, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// parse the name
	userName, sName, err_parse := ParseVaultSecretName(vaultSecretName, msgPrinter)
	if err_parse != nil {
		return false, fmt.Errorf(msgPrinter.Sprintf("Error parsing secret name in the secret binding. %v", err_parse))
	}

	// check the existance
	if exists, err := vaultSecretExists(agbotURL, nodeOrg, userName, sName); err != nil {
		return false, fmt.Errorf(msgPrinter.Sprintf("Error checking secret %v in the secret manager. %v", vaultSecretName, err))
	} else {
		return exists, nil
	}
}

// Parse the given vault secret name and return (user_name, secret_name, fully_qualified_name)
// The vault secret name has the following formats:
//     mysecret
//     user/myusername/mysecrte
// The fully qualified name in vault is the name above preceded by "openhorizon/<orgname>".
// However, it is not valid to specify the fully qualified name in the deployment policy or the pattern.
// The <org_name> will always be the node's org name at the deployment time.
// For deployment policy and private pattern, it is actually the org name of the policy or pattrn.
// For public pattern, it is the org name of the node.
func ParseVaultSecretName(secretName string, msgPrinter *message.Printer) (string, string, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// cannot be empty string
	if secretName == "" {
		return "", "", fmt.Errorf(msgPrinter.Sprintf("The binding secret name cannot be an empty string. The valid formats are: '<secretname>' for the organization level secret and 'user/<username>/<secretname>' for the user level secret."))
	}

	parts := strings.Split(secretName, "/")
	length := len(parts)
	if parts[0] != "openhorizon" {
		if parts[0] != "user" && parts[0] != "" {
			// case: mysecret
			return "", secretName, nil
		} else if parts[0] == "" && parts[1] != "user" {
			// case: /mysecret
			return "", strings.Join(parts[1:], "/"), nil
		} else if parts[0] == "user" && length >= 3 {
			// case: user/myusername/mysecrte
			return parts[1], strings.Join(parts[2:], "/"), nil
		} else if parts[0] == "" && parts[1] == "user" && length >= 4 {
			// case: /usr/myusername/mysecrte
			return parts[2], strings.Join(parts[3:], "/"), nil
		}
	}

	return "", "", fmt.Errorf(msgPrinter.Sprintf("Invalid format for the binding secret name: %v. The valid formats are: '<secretname>' for the organization level secret and 'user/<username>/<secretname>' for the user level secret.", secretName))
}

// update the index map.
func UpdateIndexMap(indexMap map[int]map[string]bool, index int, neededSb []string) {
	if index == -1 {
		return
	}

	if neededSb == nil || len(neededSb) == 0 {
		return
	}

	if _, ok := indexMap[index]; !ok {
		indexMap[index] = map[string]bool{}
	}

	for _, sn := range neededSb {
		indexMap[index][sn] = true
	}
}

// update the index map with the new one.
func CombineIndexMap(indexMap map[int]map[string]bool, newIndexMap map[int]map[string]bool) {

	if newIndexMap == nil || len(newIndexMap) == 0 {
		return
	}

	for index, sn_map := range newIndexMap {
		if _, ok := indexMap[index]; !ok {
			indexMap[index] = newIndexMap[index]
		} else {
			for sn, _ := range sn_map {
				indexMap[index][sn] = true
			}
		}
	}
}

// given an array of secret bindings and an index map, group the
// secret bindings into 2 groups: needed and extraneous.
func GroupSecretBindings(secretBinding []exchangecommon.SecretBinding, indexMap map[int]map[string]bool) ([]exchangecommon.SecretBinding, []exchangecommon.SecretBinding) {
	// group needed and extraneous secret bindings
	neededSB := []exchangecommon.SecretBinding{}
	extraneousSB := []exchangecommon.SecretBinding{}

	if secretBinding == nil || len(secretBinding) == 0 {
		return neededSB, extraneousSB
	}

	if indexMap == nil || len(indexMap) == 0 {
		extraneousSB = append(extraneousSB, secretBinding...)
		return neededSB, extraneousSB
	}

	for index, sb := range secretBinding {
		if _, ok := indexMap[index]; !ok {
			// the whole SecretBinding object is extraneous.
			extraneousSB = append(extraneousSB, secretBinding[index])
		} else {
			if len(sb.Secrets) == len(indexMap[index]) {
				// the whole SecretBinding object is needed.
				neededSB = append(neededSB, secretBinding[index])
			} else {
				// partially extraneous. break it into 2
				// copy the structure, reset the Secrets to empty
				sb_needed := secretBinding[index]
				sb_extraneous := secretBinding[index]
				sb_needed.Secrets = []exchangecommon.BoundSecret{}
				sb_extraneous.Secrets = []exchangecommon.BoundSecret{}
				for _, s := range sb.Secrets {
					k, _ := s.GetBinding()
					if _, ok1 := indexMap[index][k]; !ok1 {
						sb_extraneous.Secrets = append(sb_extraneous.Secrets, s)
					} else {
						sb_needed.Secrets = append(sb_needed.Secrets, s)
					}
				}

				neededSB = append(neededSB, sb_needed)
				extraneousSB = append(extraneousSB, sb_extraneous)
			}
		}
	}

	return neededSB, extraneousSB
}
