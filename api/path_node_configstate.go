package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
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
	getMicroservice exchange.MicroserviceHandler,
	getPatterns exchange.PatternHandler,
	resolveWorkload exchange.WorkloadResolverHandler,
	resolveService exchange.ServiceResolverHandler,
	getService exchange.ServiceHandler,
	db *bolt.DB,
	config *config.HorizonConfig) (bool, *Configstate, []*events.PolicyCreatedMessage) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		eventlog.LogDatabaseEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Unable to read node object from database, error %v", err), persistence.EC_DATABASE_ERROR)
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Error in node configuration. The node is not found from the database."), persistence.EC_ERROR_NODE_CONFIG_REG, nil)
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
			fmt.Sprintf("Error in node configuration. The node must be in 'configured' or 'configuring' state in order to change the state to %v.", cfg.State),
			persistence.EC_ERROR_NODE_CONFIG_REG, pDevice)
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("Supported state values are '%v' and '%v'.", persistence.CONFIGSTATE_CONFIGURING, persistence.CONFIGSTATE_CONFIGURED), "configstate.state")), nil, nil
	} else if NoOpStateChange(pDevice.Config.State, *cfg.State) {
		exDev := ConvertFromPersistentHorizonDevice(pDevice)
		return false, exDev.Config, nil
	} else if !ValidStateChange(pDevice.Config.State, *cfg.State) {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Node state transition from '%v' to '%v' is not supported.", pDevice.Config.State, *cfg.State), persistence.EC_ERROR_NODE_CONFIG_REG, pDevice)
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("Transition from '%v' to '%v' is not supported.", pDevice.Config.State, *cfg.State), "configstate.state")), nil, nil
	}

	// From the node's pattern, resolve all the workloads/top-level services to dependent microservices/services and then register each microservice/service that is not already registered.
	usingServices := false
	if pDevice.Pattern != "" {

		glog.V(3).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig of services starting")))

		pattern_org, pattern_name, pat := persistence.GetFormatedPatternString(pDevice.Pattern, pDevice.Org)
		pDevice.Pattern = pat

		common_apispec_list, pattern, err := getSpecRefsForPattern(pattern_name, pattern_org, getPatterns, resolveWorkload, resolveService, db, config, true)
		if err != nil {
			LogDeviceEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("%v", err), persistence.EC_ERROR_NODE_CONFIG_REG, pDevice)
			return errorhandler(err), nil, nil
		}

		// Reject the config attempt if the dependencies are inconsistent. There are always dependencies for the workload model, but not for the service model.
		if !pattern.UsingServiceModel() && len(*common_apispec_list) == 0 {
			LogDeviceEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Services in pattern %v don't have a common version range.", pDevice.Pattern), persistence.EC_ERROR_NODE_CONFIG_REG, pDevice)
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("services in pattern %v don't have a common version range.", pDevice.Pattern), "configstate.state")), nil, nil
		}

		// Check for inconsistencies between what might have been configured up to this point and what is in the pattern. Both the service based and
		// workload based flags could be off at this point, that's ok.
		if pattern.UsingServiceModel() && pDevice.IsWorkloadBased() {
			return errorhandler(NewAPIUserInputError("The node is configured to use workloads and microservices, cannot use a pattern that is service based.", "configstate.state")), nil, nil
		} else if !pattern.UsingServiceModel() && pDevice.IsServiceBased() {
			return errorhandler(NewAPIUserInputError("The node is configured to use services, cannot use a pattern that is workload based.", "configstate.state")), nil, nil
		}

		// Using the list of APISpec objects, we can create a microservice/service on this node automatically, for each microservice/service
		// that already has configuration or which doesn't need it.
		var createServiceError error
		passthruHandler := GetPassThroughErrorHandler(&createServiceError)
		for _, apiSpec := range *common_apispec_list {

			if pattern.UsingServiceModel() {
				s := NewService(apiSpec.SpecRef, apiSpec.Org, makeServiceName(apiSpec.SpecRef, apiSpec.Org, apiSpec.Version), apiSpec.Arch, apiSpec.Version)
				if errHandled := configureService(s, getPatterns, resolveService, getService, errorhandler, &msgs, db, config); errHandled {
					return errHandled, nil, nil
				}

			} else {
				service := NewMicroService(apiSpec.SpecRef, apiSpec.Org, makeServiceName(apiSpec.SpecRef, apiSpec.Org, apiSpec.Version), apiSpec.Arch, apiSpec.Version)
				errHandled, newService, msg := CreateMicroService(service, passthruHandler, getPatterns, resolveWorkload, getMicroservice, db, config, false)
				if errHandled {
					switch createServiceError.(type) {
					case *MSMissingVariableConfigError:
						glog.Errorf(apiLogString(fmt.Sprintf("Configstate autoconfig received error (%T) %v", createServiceError, createServiceError)))
						msErr := createServiceError.(*MSMissingVariableConfigError)
						// Cannot autoconfig this microservice because it has variables that need to be configured.
						return errorhandler(NewAPIUserInputError(fmt.Sprintf("Configstate autoconfig, service %v %v %v, %v", apiSpec.SpecRef, apiSpec.Org, apiSpec.Version, msErr.Err), "configstate.state")), nil, nil

					case *DuplicateServiceError:
						// If the microservice is already registered, that's ok because the node user is allowed to configure any of the
						// required microservices before calling the configstate API.
						glog.V(3).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig found duplicate service %v %v, overwriting the version range to %v.", apiSpec.SpecRef, apiSpec.Org, apiSpec.Version)))

					default:
						return errorhandler(NewSystemError(fmt.Sprintf("unexpected error returned from service create (%T) %v", createServiceError, createServiceError))), nil, nil
					}
				} else {
					glog.V(5).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig created service %v", newService)))
					msgs = append(msgs, msg)
				}
			}
		}

		// The top-level services in a pattern also need to be registered just like the dependent services.
		if pattern.UsingServiceModel() {
			for _, service := range pattern.Services {

				// Ignore top-level services that don't match this node's hardware architecture.
				thisArch := cutil.ArchString()
				if service.ServiceArch != thisArch && config.ArchSynonyms.GetCanonicalArch(service.ServiceArch) != thisArch {
					glog.Infof(apiLogString(fmt.Sprintf("skipping service because it is for a different hardware architecture, this node is %v. Skipped service is: %v", thisArch, service.ServiceArch)))
					continue
				}

				s := NewService(service.ServiceURL, service.ServiceOrg, makeServiceName(service.ServiceURL, service.ServiceOrg, "[0.0.0,INFINITY)"), service.ServiceArch, "[0.0.0,INFINITY)")
				if errHandled := configureService(s, getPatterns, resolveService, getService, errorhandler, &msgs, db, config); errHandled {
					return errHandled, nil, nil
				}

			}

			// Remember that this node is using the service model.
			usingServices = true

		}

		glog.V(3).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig of services complete")))

	}

	// Update the state in the local database
	updatedDev, err := pDevice.SetConfigstate(db, pDevice.Id, *cfg.State, usingServices)
	if err != nil {
		eventlog.LogDatabaseEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Error persisting new config state: %v", err), persistence.EC_DATABASE_ERROR)
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting new config state: %v", err))), nil, nil
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Update configstate: updated device: %v", updatedDev)))

	exDev := ConvertFromPersistentHorizonDevice(updatedDev)

	LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("Complete node configuration/registration for node %v.", updatedDev.Id), persistence.EC_NODE_CONFIG_REG_COMPLETE, updatedDev)

	return false, exDev.Config, msgs

}

// Common function used to create/configure a service on an edge node. The boolean response indicates that an error occurred
// and was handled (or no error occurred).
func configureService(service *Service,
	getPatterns exchange.PatternHandler,
	resolveService exchange.ServiceResolverHandler,
	getService exchange.ServiceHandler,
	errorhandler ErrorHandler,
	msgs *[]*events.PolicyCreatedMessage,
	db *bolt.DB,
	config *config.HorizonConfig) bool {

	var createServiceError error
	passthruHandler := GetPassThroughErrorHandler(&createServiceError)

	create_service_error_handler := func(err error) bool {
		if !strings.Contains(err.Error(), "Duplicate registration") {
			LogServiceEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Error in service configuration for %v. %v", *service.Url, err), persistence.EC_ERROR_SERVICE_CONFIG, service)
		}
		return passthruHandler(err)
	}

	if errHandled, newService, msg := CreateService(service, create_service_error_handler, getPatterns, resolveService, getService, db, config, false); errHandled {

		switch createServiceError.(type) {

		// This is a real error, the service is not configurable with supplying values for non-defaulted user inputs.
		case *MSMissingVariableConfigError:
			glog.Errorf(apiLogString(fmt.Sprintf("Configstate autoconfig received error (%T) %v", createServiceError, createServiceError)))
			msErr := createServiceError.(*MSMissingVariableConfigError)
			// Cannot autoconfig this microservice because it has variables that need to be configured.
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("Configstate autoconfig, service %v %v %v, %v", *service.Url, *service.Org, "[0.0.0,INFINITY)", msErr.Err), "configstate.state"))

		// This is not an error because the service has already been registered by a call to /service/config. The node user is allowed
		// to configure any of the required services before calling the configstate API.
		case *DuplicateServiceError:
			glog.V(3).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig found duplicate service %v %v, overwriting the version range to %v.", *service.Url, *service.Org, "[0.0.0,INFINITY)")))

		default:
			return errorhandler(NewSystemError(fmt.Sprintf("unexpected error returned from service create (%T) %v", createServiceError, createServiceError)))
		}

	} else {
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig created service %v", newService)))
		(*msgs) = append((*msgs), msg)
	}

	return false
}

// This function verifies that if the given workload needs variable configuration, that there is a workloadconfig
// object holding that config.
func workloadConfigPresent(ed exchange.ExchangeDefinition, wUrl string, wVersion string, db *bolt.DB) (bool, error) {

	// If the definition needs no config, exit early.
	if !ed.NeedsUserInput() {
		return true, nil
	}

	// Filter to return workload configs with versions less than or equal to the input workload version range
	OlderWorkloadWCFilter := func(workload_url string, version string) persistence.WCFilter {
		return func(e persistence.WorkloadConfigOnly) bool {
			if vExp, err := policy.Version_Expression_Factory(e.VersionExpression); err != nil {
				return false
			} else if inRange, err := vExp.Is_within_range(version); err != nil {
				return false
			} else if e.WorkloadURL == workload_url && inRange {
				return true
			} else {
				return false
			}
		}
	}

	// Find the eligible workload config objects. We know that the /workload/config API validates that all required
	// variables are set BEFORE saving the config, so if we find any matching config objects, we can assume the
	// workload is configured.
	cfgs, err := persistence.FindWorkloadConfigs(db, []persistence.WCFilter{OlderWorkloadWCFilter(wUrl, wVersion)})
	if err != nil {
		return false, errors.New(fmt.Sprintf("unable to read workload config objects %v %v, error: %v", wUrl, wVersion, err))
	} else if len(cfgs) != 0 {
		return true, nil
	}

	// The workload being configured might actually be a top level service. In that case, if there are any
	// user input variables configured, they will be found in the attributes database. We know that the /service/config
	// API validates that all required variables are set BEFORE saving the config, so if we find any matching userinput
	// attribute objects, we can assume the service is configured.
	attrs, err := persistence.FindApplicableAttributes(db, wUrl)
	if err != nil {
		return false, fmt.Errorf("Unable to fetch service %v attributes, error: %v", wUrl, err)
	} else {
		for _, attr := range attrs {
			switch attr.(type) {
			case persistence.UserInputAttributes:
				return true, nil
			}
		}
	}

	return false, nil

}

// This function returns the referenced microservices from a given pattern.
// If the checkWorkloadConfig is true, it will check if the user has given the correct input for the workload/top-level service already.
func getSpecRefsForPattern(patName string,
	patOrg string,
	getPatterns exchange.PatternHandler,
	resolveWorkload exchange.WorkloadResolverHandler,
	resolveService exchange.ServiceResolverHandler,
	db *bolt.DB,
	config *config.HorizonConfig,
	checkWorkloadConfig bool) (*policy.APISpecList, *exchange.Pattern, error) {

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

	if patternDef.UsingServiceModel() {

		// This parameter is nil if the caller is configuring a workload based pattern.
		if resolveService == nil {
			return nil, nil, NewAPIUserInputError(fmt.Sprintf("cannot configure a microservice on a node that is using a service based pattern: %v", patId), "microservice")
		}

		for _, service := range patternDef.Services {

			// Ignore top-level services that don't match this node's hardware architecture.
			if service.ServiceArch != thisArch && config.ArchSynonyms.GetCanonicalArch(service.ServiceArch) != thisArch {
				glog.Infof(apiLogString(fmt.Sprintf("skipping service because it is for a different hardware architecture, this node is %v. Skipped service is: %v", thisArch, service.ServiceArch)))
				continue
			}

			// Each top-level service in the pattern can specify rollback versions, so to get a fully qualified top-level service URL,
			// we need to iterate each "workloadChoice" to grab the version.
			for _, serviceChoice := range service.ServiceVersions {

				apiSpecList, serviceDef, err := resolveService(service.ServiceURL, service.ServiceOrg, serviceChoice.Version, service.ServiceArch)
				if err != nil {
					return nil, nil, NewSystemError(fmt.Sprintf("Error resolving service %v %v %v %v, error %v", service.ServiceURL, service.ServiceOrg, serviceChoice.Version, thisArch, err))
				}

				if checkWorkloadConfig {
					// The top-level service might have variables that need to be configured. If so, find all relevant service attribute objects to make sure
					// there is userinput config available.
					if present, err := workloadConfigPresent(serviceDef, service.ServiceURL, serviceChoice.Version, db); err != nil {
						return nil, nil, NewSystemError(fmt.Sprintf("Error checking service config, error %v", err))
					} else if !present {
						return nil, nil, NewMSMissingVariableConfigError(fmt.Sprintf("service config for %v %v is missing", service.ServiceURL, serviceChoice.Version), "configstate.state")
					}
				}

				// Look for inconsistencies in the hardware architecture of the list of dependencies.
				for _, apiSpec := range *apiSpecList {
					if apiSpec.Arch != thisArch && config.ArchSynonyms.GetCanonicalArch(apiSpec.Arch) != thisArch {
						return nil, nil, NewSystemError(fmt.Sprintf("The referenced service %v by service %v has a hardware architecture that is not supported by this node: %v.", apiSpec, service.ServiceURL, thisArch))
					}
				}

				// MergeWith will omit exact duplicates when merging the 2 lists.
				(*completeAPISpecList) = completeAPISpecList.MergeWith(apiSpecList)
			}

		}

	} else {

		// This parameter is nil if the caller is configuring a service based pattern.
		if resolveWorkload == nil {
			return nil, nil, NewAPIUserInputError(fmt.Sprintf("cannot configure a service on a node that is using a workload based pattern: %v", patId), "service")

		}

		for _, workload := range patternDef.Workloads {

			// Ignore workloads that don't match this node's hardware architecture.
			if workload.WorkloadArch != thisArch && config.ArchSynonyms.GetCanonicalArch(workload.WorkloadArch) != thisArch {
				glog.Infof(apiLogString(fmt.Sprintf("skipping workload because it is for a different hardware architecture, this node is %v. Skipped workload is: %v", thisArch, workload.WorkloadArch)))
				continue
			}

			// Each workload in the pattern can specify rollback versions, so to get a fully qualified workload URL,
			// we need to iterate each "workloadChoice" to grab the version.
			for _, workloadChoice := range workload.WorkloadVersions {

				apiSpecList, workloadDef, err := resolveWorkload(workload.WorkloadURL, workload.WorkloadOrg, workloadChoice.Version, workload.WorkloadArch)
				if err != nil {
					return nil, nil, NewSystemError(fmt.Sprintf("Error resolving workload %v %v %v %v, error %v", workload.WorkloadURL, workload.WorkloadOrg, workloadChoice.Version, thisArch, err))
				}

				if checkWorkloadConfig {
					// The workload might have variables that need to be configured. If so, find all relevant workloadconfig objects to make sure
					// there is a workload config available.
					if present, err := workloadConfigPresent(workloadDef, workload.WorkloadURL, workloadChoice.Version, db); err != nil {
						return nil, nil, NewSystemError(fmt.Sprintf("Error checking workload config, error %v", err))
					} else if !present {
						return nil, nil, NewMSMissingVariableConfigError(fmt.Sprintf("Workload config for %v %v is missing", workload.WorkloadURL, workloadChoice.Version), "configstate.state")
					}
				}

				// Look for inconsistencies in the hardware architecture of the list of dependencies.
				for _, apiSpec := range *apiSpecList {
					if apiSpec.Arch != thisArch && config.ArchSynonyms.GetCanonicalArch(apiSpec.Arch) != thisArch {
						return nil, nil, NewSystemError(fmt.Sprintf("The referenced microservice %v by workload %v has a hardware architecture that is not supported by this node: %v.", apiSpec, workload.WorkloadURL, thisArch))
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

	// for now, anax only allow one microservice version, so we need to get the common version range for each microservice.
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
	vExp, err := policy.Version_Expression_Factory(msVersion)
	if err == nil {
		version = fmt.Sprintf("%v-%v", vExp.Get_start_version(), vExp.Get_end_version())
	}

	return fmt.Sprintf("%v_%v_%v", url, msOrg, version)

}
