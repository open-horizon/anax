package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
	"strings"
)

func NoOpStateChange(from string, to string) bool {
	if from == to {
		return true
	}
	return false
}

func ValidStateChange(from string, to string) bool {
	if from == persistence.CONFIGSTATE_CONFIGURING && to == persistence.CONFIGSTATE_CONFIGURED {
		return true
	}
	return false
}

func FindConfigstateForOutput(db *bolt.DB) (*Configstate, error) {

	var device *HorizonDevice

	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read node object, error %v", err))
	} else if pDevice == nil {
		state := persistence.CONFIGSTATE_UNCONFIGURED
		if Unconfiguring {
			state = persistence.CONFIGSTATE_UNCONFIGURING
		}
		cfg := &Configstate{
			State: &state,
		}
		return cfg, nil

	} else {
		device = ConvertFromPersistentHorizonDevice(pDevice)
		return device.Config, nil
	}

}

// Given a demarshalled Configstate object, validate it and save, returning any errors.
func UpdateConfigstate(cfg *Configstate,
	errorhandler ErrorHandler,
	getPatterns exchange.PatternHandler,
	resolveService exchange.ServiceDefResolverHandler,
	getService exchange.ServiceHandler,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler,
	db *bolt.DB,
	config *config.HorizonConfig) (bool, *Configstate, []*events.PolicyCreatedMessage) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		eventlog.LogDatabaseEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_READ_NODE_FROM_DB, err.Error()), persistence.EC_DATABASE_ERROR)
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_NODE_CONF_NOT_FOUND), persistence.EC_ERROR_NODE_CONFIG_REG, nil)
		return errorhandler(NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil, nil
	}

	glog.V(3).Infof(apiLogString(fmt.Sprintf("Update configstate: device in local database: %v", pDevice)))
	msgs := make([]*events.PolicyCreatedMessage, 0, 10)

	// Device registration is in the database, so verify that the requested state change is suported.
	// The only (valid) state transition that is currently unsupported is configuring to configured. The state
	// transition of unconfigured to configuring occurs when POST /node is called.
	// If the caller is requesting a state change that is a noop, just return the current state.
	if *cfg.State != persistence.CONFIGSTATE_CONFIGURING && *cfg.State != persistence.CONFIGSTATE_CONFIGURED {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_API_ERR_NODE_CONF_WRONG_STATE, *cfg.State),
			persistence.EC_ERROR_NODE_CONFIG_REG, pDevice)
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("Supported state values are '%v' and '%v'.", persistence.CONFIGSTATE_CONFIGURING, persistence.CONFIGSTATE_CONFIGURED), "configstate.state")), nil, nil
	} else if NoOpStateChange(pDevice.Config.State, *cfg.State) {
		exDev := ConvertFromPersistentHorizonDevice(pDevice)
		return false, exDev.Config, nil
	} else if !ValidStateChange(pDevice.Config.State, *cfg.State) {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_UNSUP_NODE_STATE_TRANS, pDevice.Config.State, *cfg.State), persistence.EC_ERROR_NODE_CONFIG_REG, pDevice)
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("Transition from '%v' to '%v' is not supported.", pDevice.Config.State, *cfg.State), "configstate.state")), nil, nil
	}

	// From the node's pattern, resolve all the top-level services to dependent services and then register each service that is not already registered.
	if pDevice.Pattern != "" {

		glog.V(3).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig of services starting")))

		pattern_org, pattern_name, pat := persistence.GetFormatedPatternString(pDevice.Pattern, pDevice.Org)
		pDevice.Pattern = pat

		common_apispec_list, pattern, err := getSpecRefsForPattern(pDevice.GetNodeType(), pattern_name, pattern_org, getPatterns, resolveService, db, config, true, true)
		if err != nil {
			LogDeviceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_GET_SREFS_FOR_PATTERN, pattern_name, err.Error()), persistence.EC_ERROR_NODE_CONFIG_REG, pDevice)
			return errorhandler(err), nil, nil
		}

		// get node and pattern user input
		nodeUserInput, err := persistence.FindNodeUserInput(db)
		if err != nil {
			LogDeviceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_FAIL_GET_UI_FROM_DB, err.Error()), persistence.EC_ERROR_NODE_CONFIG_REG, pDevice)
			return errorhandler(fmt.Errorf("Failed get user input from local db. %v", err)), nil, nil
		}

		// merge node user input it with pattern user input
		mergedUserInput := policy.MergeUserInputArrays(pattern.UserInput, nodeUserInput, true)
		if mergedUserInput == nil {
			mergedUserInput = []policy.UserInput{}
		}

		// Using the list of APISpec objects, we can create a service on this node automatically, for each service
		// that already has configuration or which doesn't need it.
		if pDevice.GetNodeType() == persistence.DEVICE_TYPE_DEVICE {
			for _, apiSpec := range *common_apispec_list {

				// get the user input for this service
				ui_merged, _, err := policy.FindUserInput(apiSpec.SpecRef, apiSpec.Org, "", apiSpec.Arch, mergedUserInput)
				if err != nil {
					LogDeviceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_FAIL_FIND_SVC_PREF_FROM_UI, apiSpec.Org, apiSpec.SpecRef, err.Error()), persistence.EC_ERROR_NODE_CONFIG_REG, pDevice)
					return errorhandler(fmt.Errorf("Failed to find preferences for service %v/%v from the merged user input, error: %v", apiSpec.Org, apiSpec.SpecRef, err)), nil, nil
				}

				s := NewService(apiSpec.SpecRef, apiSpec.Org, makeServiceName(apiSpec.SpecRef, apiSpec.Org, apiSpec.Version), apiSpec.Arch, apiSpec.Version)
				if errHandled := configureService(s, getPatterns, resolveService, getService, getDevice, patchDevice, ui_merged, errorhandler, &msgs, db, config); errHandled {
					return errHandled, nil, nil
				}
			}
		}

		// The top-level services in a pattern also need to be registered just like the dependent services.
		for _, service := range pattern.Services {

			// Ignore top-level services that don't match this node's hardware architecture.
			thisArch := cutil.ArchString()
			if service.ServiceArch != thisArch && config.ArchSynonyms.GetCanonicalArch(service.ServiceArch) != thisArch {
				glog.Infof(apiLogString(fmt.Sprintf("skipping service because it is for a different hardware architecture, this node is %v. Skipped service is: %v", thisArch, service.ServiceArch)))
				continue
			}

			// get the user input for this service
			ui_merged, _, err := policy.FindUserInput(service.ServiceURL, service.ServiceOrg, "", service.ServiceArch, mergedUserInput)
			if err != nil {
				LogDeviceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_FAIL_FIND_SVC_PREF_FROM_UI, service.ServiceOrg, service.ServiceURL, err.Error()), persistence.EC_ERROR_NODE_CONFIG_REG, pDevice)
				return errorhandler(fmt.Errorf("Failed to find preferences for service %v/%v from the merged user input, error: %v", service.ServiceOrg, service.ServiceURL, err)), nil, nil
			}

			s := NewService(service.ServiceURL, service.ServiceOrg, makeServiceName(service.ServiceURL, service.ServiceOrg, "[0.0.0,INFINITY)"), service.ServiceArch, "[0.0.0,INFINITY)")
			if errHandled := configureService(s, getPatterns, resolveService, getService, getDevice, patchDevice, ui_merged, errorhandler, &msgs, db, config); errHandled {
				return errHandled, nil, nil
			}
		}

		glog.V(3).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig of services complete")))

	}

	// Update the state in the local database
	updatedDev, err := pDevice.SetConfigstate(db, pDevice.Id, *cfg.State)
	if err != nil {
		eventlog.LogDatabaseEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_SAVE_NODE_CONFSTATE, err.Error()), persistence.EC_DATABASE_ERROR)
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting new config state: %v", err))), nil, nil
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Update configstate: updated device: %v", updatedDev)))

	exDev := ConvertFromPersistentHorizonDevice(updatedDev)

	LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_COMPLETE_NODE_REG, updatedDev.Id), persistence.EC_NODE_CONFIG_REG_COMPLETE, updatedDev)

	return false, exDev.Config, msgs

}

// check if the node has the 'openhorizon.allowPrivileged' set to true
func nodeAllowPrivilegedService(db *bolt.DB) (bool, error) {
	nodePol, err := FindNodePolicyForOutput(db)
	if err != nil {
		return false, err
	}

	nodePriv := false
	if nodePol != nil && nodePol.Properties != nil && nodePol.Properties.HasProperty(externalpolicy.PROP_NODE_PRIVILEGED) {
		privProv, err := nodePol.Properties.GetProperty(externalpolicy.PROP_NODE_PRIVILEGED)
		if err != nil {
			return false, err
		}
		nodePriv = privProv.Value.(bool)
	}
	return nodePriv, nil
}

// Common function used to create/configure a service on an edge node. The boolean response indicates that an error occurred
// and was handled (or no error occurred).
func configureService(service *Service,
	getPatterns exchange.PatternHandler,
	resolveService exchange.ServiceDefResolverHandler,
	getService exchange.ServiceHandler,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler,
	mergedUserInput *policy.UserInput,
	errorhandler ErrorHandler,
	msgs *[]*events.PolicyCreatedMessage,
	db *bolt.DB,
	config *config.HorizonConfig) bool {

	var createServiceError error
	passthruHandler := GetPassThroughErrorHandler(&createServiceError)

	create_service_error_handler := func(err error) bool {
		if strings.Contains(err.Error(), "Type mismatch") {
			LogServiceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_IGNORE_TYPE_MISMATCH, err.Error()), persistence.EC_SERVICE_CONFIG_IGNORE_TYPE_MISMATCH, service)
		} else if !strings.Contains(err.Error(), "Duplicate registration") {
			LogServiceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_SVC_CONF, *service.Url, err.Error()), persistence.EC_ERROR_SERVICE_CONFIG, service)
		}
		return passthruHandler(err)
	}

	// Make sure it is not nil
	if mergedUserInput == nil {
		mergedUserInput = &policy.UserInput{
			ServiceOrgid:        *service.Org,
			ServiceUrl:          *service.Url,
			ServiceArch:         *service.Arch,
			ServiceVersionRange: "",
			Inputs:              []policy.Input{},
		}
	}
	if errHandled, newService, msg := CreateService(service, create_service_error_handler, getPatterns, resolveService, getService, getDevice, patchDevice, mergedUserInput, db, config, false); errHandled {

		switch createServiceError.(type) {

		// This is a real error, the service is not configurable without supplying values for non-defaulted user inputs.
		case *MSMissingVariableConfigError:
			glog.Errorf(apiLogString(fmt.Sprintf("Configstate autoconfig received error (%T) %v", createServiceError, createServiceError)))
			msErr := createServiceError.(*MSMissingVariableConfigError)
			// Cannot autoconfig this microservice because it has variables that need to be configured.
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("Configstate autoconfig, service %v %v %v, %v", *service.Url, *service.Org, "[0.0.0,INFINITY)", msErr.Err), "configstate.state"))

		// This is not an error because the service has already been registered by a call to /service/config. The node user is allowed
		// to configure any of the required services before calling the configstate API.
		case *DuplicateServiceError:
			glog.V(3).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig found duplicate service %v %v, overwriting the version range to %v.", *service.Url, *service.Org, "[0.0.0,INFINITY)")))

		// This occurs when a patterns contains a service that does not match the node type. Ignore it.
		case *TypeMismatchError:
			glog.V(3).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig found service type not match the node type for service %v %v, ignoring it.", *service.Url, *service.Org)))

		default:
			return errorhandler(NewSystemError(fmt.Sprintf("unexpected error returned from service create (%T) %v", createServiceError, createServiceError)))
		}

	} else {
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig created service %v", newService)))
		if msg != nil {
			(*msgs) = append((*msgs), msg)
		}
	}

	return false
}

// This function verifies that if the given workload needs variable configuration, that there is a workloadconfig
// object holding that config.
func workloadConfigPresent(sd *exchange.ServiceDefinition, wUrl string, wOrg, wVersion string, patternUserInput []policy.UserInput, db *bolt.DB) (bool, error) {
	if sd == nil {
		return true, nil
	}

	// If the definition needs no config, exit early.
	if !sd.NeedsUserInput() {
		return true, nil
	}

	// The workload being configured is a top level service, and if there are any
	// user input variables configured, they will be found in the attributes database. We know that the /service/config
	// API validates that all required variables are set BEFORE saving the config, so if we find any matching userinput
	// attribute objects, we can assume the service is configured.
	attrs, err := persistence.FindApplicableAttributes(db, wUrl, wOrg)
	if err != nil {
		return false, fmt.Errorf("Unable to fetch service %v/%v attributes, error: %v", wOrg, wUrl, err)
	} else {
		for _, attr := range attrs {
			switch attr.(type) {
			case persistence.UserInputAttributes:
				return true, nil
			}
		}
	}

	// if not found in attributes, then check the node userinput
	nodeUserInput, err := persistence.FindNodeUserInput(db)
	if err != nil {
		return false, fmt.Errorf(apiLogString(fmt.Sprintf("Failed get user input from local db. %v", err)))
	}
	if ui, _, err := policy.FindUserInput(wUrl, wOrg, wVersion, sd.Arch, nodeUserInput); err != nil {
		return false, fmt.Errorf("Failed to find preferences for service %v/%v  from the local user input, error: %v", wOrg, wUrl, err)
	} else if ui != nil {
		return true, nil
	}

	// try to get the default from pattern
	if ui, _, err := policy.FindUserInput(wUrl, wOrg, wVersion, sd.Arch, patternUserInput); err != nil {
		return false, fmt.Errorf("Failed to find preferences for service %v/%v  from the pattern userInput section, error: %v", wOrg, wUrl, err)
	} else if ui != nil {
		return true, nil
	}

	return false, nil

}

// This function returns the referenced dependent services from a given pattern.
// If the checkWorkloadConfig is true, it will check if the user has given the correct input for the workload/top-level service already.
func getSpecRefsForPattern(nodeType string, patName string,
	patOrg string,
	getPatterns exchange.PatternHandler,
	resolveService exchange.ServiceDefResolverHandler,
	db *bolt.DB,
	config *config.HorizonConfig,
	checkWorkloadConfig bool,
	checkNodePrivilege bool) (*policy.APISpecList, *exchange.Pattern, error) {

	glog.V(5).Infof(apiLogString(fmt.Sprintf("getSpecRefsForPattern %v org %v. Check service config: %v", patName, patOrg, checkWorkloadConfig)))

	// Get the pattern definition from the exchange. There should only be one pattern returned in the map.
	pattern, err := getPatterns(patOrg, patName)
	if err != nil {
		return nil, nil, NewSystemError(fmt.Sprintf("Unable to read pattern object %v from exchange, error %v", patName, err))
	} else if len(pattern) != 1 {
		return nil, nil, NewSystemError(fmt.Sprintf("Expected only 1 pattern from exchange, received %v", len(pattern)))
	}

	// Get the pattern definition that we need to analyze.
	patId := fmt.Sprintf("%v/%v", patOrg, patName)
	patternDef, ok := pattern[patId]
	if !ok {
		return nil, nil, NewSystemError(fmt.Sprintf("Expected pattern id not found in GET pattern response: %v", pattern))
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("working with pattern definition %v", patternDef)))

	// For each workload/top-level service in the pattern, resolve it to a list of required services.
	// A pattern can have references to workloads or to services, but not a mixture of both.
	completeAPISpecList := new(policy.APISpecList)
	thisArch := cutil.ArchString()

	// This parameter is nil if the caller is configuring a workload based pattern.
	if resolveService == nil {
		return nil, nil, NewAPIUserInputError(fmt.Sprintf("cannot configure a dependent service on a node that is using a service based pattern: %v", patId), "microservice")
	}

	// get node policy and then check if it has PROP_NODE_PRIVILEGED to true
	nodePriv := false
	var err1 error
	if checkNodePrivilege {
		nodePriv, err1 = nodeAllowPrivilegedService(db)
		if err1 != nil {
			return nil, nil, NewSystemError(fmt.Sprintf("Error getting node openhorizon.allowPrivileged setting. %v", err))
		}
	}

	for _, service := range patternDef.Services {

		// Ignore top-level services that don't match this node's hardware architecture.
		if service.ServiceArch != thisArch && config.ArchSynonyms.GetCanonicalArch(service.ServiceArch) != thisArch {
			glog.Infof(apiLogString(fmt.Sprintf("skipping service %v/%v because it is for a different hardware architecture, this node is %v. Skipped service is: %v", service.ServiceOrg, service.ServiceURL, thisArch, service.ServiceArch)))
			continue
		}

		// Each top-level service in the pattern can specify rollback versions, so to get a fully qualified top-level service URL,
		// we need to iterate each "workloadChoice" to grab the version.
		for _, serviceChoice := range service.ServiceVersions {

			dependentDefs, serviceDef, topSvcID, err := resolveService(service.ServiceURL, service.ServiceOrg, serviceChoice.Version, service.ServiceArch)
			if err != nil {
				return nil, nil, NewSystemError(fmt.Sprintf("Error resolving service %v/%v %v %v, error %v", service.ServiceOrg, service.ServiceURL, serviceChoice.Version, thisArch, err))
			}

			// skip the service because the type mis-match.
			serviceType := serviceDef.GetServiceType()
			if serviceType != exchange.SERVICE_TYPE_BOTH && nodeType != serviceType {
				glog.Infof(apiLogString(fmt.Sprintf("skipping service %v/%v because it's type %v does not match the node type %v. ", service.ServiceOrg, service.ServiceURL, serviceType, nodeType)))
				break
			}

			if checkWorkloadConfig {
				// The top-level service might have variables that need to be configured. If so, find all relevant service attribute objects to make sure
				// there is userinput config available.
				if present, err := workloadConfigPresent(serviceDef, service.ServiceURL, service.ServiceOrg, serviceChoice.Version, patternDef.UserInput, db); err != nil {
					return nil, nil, NewSystemError(fmt.Sprintf("Error checking service config, error %v", err))
				} else if !present {
					return nil, nil, NewMSMissingVariableConfigError(fmt.Sprintf(cutil.ANAX_SVC_MISSING_CONFIG, serviceChoice.Version, cutil.FormOrgSpecUrl(service.ServiceURL, service.ServiceOrg)), "configstate.state")
				}
			}

			if checkNodePrivilege {
				if svcPriv, err := compcheck.DeploymentRequiresPrivilege(serviceDef.GetDeploymentString(), nil); err != nil {
					return nil, nil, NewSystemError(fmt.Sprintf("Error checking if service %v requires privileged mode. %v", topSvcID, err))
				} else if svcPriv && !nodePriv {
					return nil, nil, NewSystemError(fmt.Sprintf("Service %v requires privileged mode, but the node does not have openhorizon.allowPrivileged property set to true.", topSvcID))
				}
			}

			if dependentDefs != nil {
				apiSpecList := new(policy.APISpecList)

				for sId, dDef := range dependentDefs {
					// Look for inconsistencies in the hardware architecture of the list of dependencies.
					if dDef.Arch != thisArch && config.ArchSynonyms.GetCanonicalArch(dDef.Arch) != thisArch {
						return nil, nil, NewSystemError(fmt.Sprintf("The referenced service %v by service %v/%v has a hardware architecture that is not supported by this node: %v.", sId, service.ServiceOrg, service.ServiceURL, thisArch))
					}

					// generate apiSpecList from dependent def
					newAPISpec := policy.APISpecification_Factory(dDef.URL, exchange.GetOrg(sId), dDef.Version, dDef.Arch)
					if dDef.Sharable == exchange.MS_SHARING_MODE_SINGLETON || dDef.Sharable == exchange.MS_SHARING_MODE_SINGLE {
						newAPISpec.ExclusiveAccess = false
					}
					apiSpecList.Add_API_Spec(newAPISpec)
				}

				if checkNodePrivilege {
					if svcPriv, err, privSvcs := compcheck.ServicesRequirePrivilege(&dependentDefs, nil); err != nil {
						return nil, nil, NewSystemError(fmt.Sprintf("Error checking if dependent services for %v require privileged mode. %v", topSvcID, err))
					} else if svcPriv && !nodePriv {
						return nil, nil, NewSystemError(fmt.Sprintf("Dependent services %v for %v require privileged mode, but the node does not have openhorizon.allowPrivileged property set to true.", privSvcs, topSvcID))
					}
				}

				// MergeWith will omit exact duplicates when merging the 2 lists.
				(*completeAPISpecList) = completeAPISpecList.MergeWith(apiSpecList)
			}
		}
	}

	// If the pattern search doesnt find any microservices/services then there might be a problem.
	if len(*completeAPISpecList) == 0 {
		return completeAPISpecList, &patternDef, nil
	}

	// for now, anax only allow one service version, so we need to get the common version range for each service.
	common_apispec_list, err := completeAPISpecList.GetCommonVersionRanges()
	if err != nil {
		return nil, nil, NewAPIUserInputError(fmt.Sprintf("Error resolving the common version ranges for the referenced services for %v %v. %v", patId, thisArch, err), "configstate.state")
	}
	glog.V(5).Infof(apiLogString(fmt.Sprintf("getSpecRefsForPattern resolved service version ranges to %v", *common_apispec_list)))

	return common_apispec_list, &patternDef, nil
}

// Generate a name for the autoconfigured services.
func makeServiceName(msURL string, msOrg string, msVersion string) string {

	url := ""
	pieces := strings.SplitN(msURL, "/", 3)
	if len(pieces) >= 3 {
		url = strings.TrimSuffix(pieces[2], "/")
		url = strings.Replace(url, "/", "-", -1)
	}

	version := ""
	vExp, err := semanticversion.Version_Expression_Factory(msVersion)
	if err == nil {
		version = fmt.Sprintf("%v-%v", vExp.Get_start_version(), vExp.Get_end_version())
	}

	return fmt.Sprintf("%v_%v_%v", url, msOrg, version)

}
