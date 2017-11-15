package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"strings"
)

const CONFIGSTATE_UNCONFIGURING = "unconfiguring"
const CONFIGSTATE_UNCONFIGURED = "unconfigured"
const CONFIGSTATE_CONFIGURING = "configuring"
const CONFIGSTATE_CONFIGURED = "configured"

func NoOpStateChange(from string, to string) bool {
	if from == to {
		return true
	}
	return false
}

func ValidStateChange(from string, to string) bool {
	if from == CONFIGSTATE_CONFIGURING && to == CONFIGSTATE_CONFIGURED {
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
		state := CONFIGSTATE_UNCONFIGURED
		if Unconfiguring {
			state = CONFIGSTATE_UNCONFIGURING
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
	getOrg OrgHandler,
	getMicroservice MicroserviceHandler,
	getPatterns PatternHandler,
	resolveWorkload WorkloadResolverHandler,
	db *bolt.DB,
	config *config.HorizonConfig) (bool, *Configstate, []*events.PolicyCreatedMessage) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil, nil
	}

	glog.V(3).Infof(apiLogString(fmt.Sprintf("Update configstate: device in local database: %v", pDevice)))
	msgs := make([]*events.PolicyCreatedMessage, 0, 10)

	// Device registration is in the database, so verify that the requested state change is suported.
	// The only (valid) state transition that is currently unsupported is configuring to configured. The state
	// transition of unconfigured to configuring occurs when POST /node is called.
	// If the caller is requesting a state change that is a noop, just return the current state.
	if *cfg.State != CONFIGSTATE_CONFIGURING && *cfg.State != CONFIGSTATE_CONFIGURED {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("Supported state values are '%v' and '%v'.", CONFIGSTATE_CONFIGURING, CONFIGSTATE_CONFIGURED), "configstate.state")), nil, nil
	} else if NoOpStateChange(pDevice.Config.State, *cfg.State) {
		exDev := ConvertFromPersistentHorizonDevice(pDevice)
		return false, exDev.Config, nil
	} else if !ValidStateChange(pDevice.Config.State, *cfg.State) {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("Transition from '%v' to '%v' is not supported.", pDevice.Config.State, *cfg.State), "configstate.state")), nil, nil
	}

	// From the node's pattern, resolve all the workloads to microservices and then register each microservice that is not already registered.
	if pDevice.Pattern != "" {

		glog.V(3).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig of microservices starting")))

		// Get the pattern definition from the exchange. There should only be one pattern returned in the map.
		pattern, err := getPatterns(pDevice.Org, pDevice.Pattern, pDevice.GetId(), pDevice.Token)
		if err != nil {
			return errorhandler(NewSystemError(fmt.Sprintf("Unable to read pattern object %v from exchange, error %v", pDevice.Pattern, err))), nil, nil
		} else if len(pattern) != 1 {
			return errorhandler(NewSystemError(fmt.Sprintf("Expected only 1 pattern from exchange, received %v", len(pattern)))), nil, nil
		}

		// Get the pattern definition that we need to analyze.
		patId := fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Pattern)
		patternDef, ok := pattern[patId]
		if !ok {
			return errorhandler(NewSystemError(fmt.Sprintf("Expected pattern id not found in GET pattern response: %v", pattern))), nil, nil
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Configstate working with pattern definition %v", patternDef)))

		// For each workload in the pattern, resolve the workload to a list of required microservices.
		completeAPISpecList := new(policy.APISpecList)
		thisArch := cutil.ArchString()
		for _, workload := range patternDef.Workloads {

			// Ignore workloads that don't match this node's hardware architecture.
			if workload.WorkloadArch != thisArch {
				glog.Infof(apiLogString(fmt.Sprintf("Configstate skipping workload because it is for a different hardware architecture, this node is %v. Skipped workload is: %v", thisArch, workload)))
				continue
			}

			// Each workload in the pattern can specify rollback workload versions, so to get a fully qualified workload URL,
			// we need to iterate each workload choice to grab the version.
			for _, workloadChoice := range workload.WorkloadVersions {
				_, workloadDef, err := resolveWorkload(workload.WorkloadURL, workload.WorkloadOrg, workloadChoice.Version, thisArch, pDevice.GetId(), pDevice.Token)
				if err != nil {
					return errorhandler(NewSystemError(fmt.Sprintf("Error resolving workload %v %v %v %v, error %v", workload.WorkloadURL, workload.WorkloadOrg, workloadChoice.Version, thisArch, err))), nil, nil
				}

				// The workload might have variables that need to be configured. If so, find all relevant workloadconfig objects to make sure
				// there is a workload config available.
				if present, err := workloadConfigPresent(workloadDef, workload.WorkloadURL, workloadChoice.Version, db); err != nil {
					return errorhandler(NewSystemError(fmt.Sprintf("Error checking workload config, error %v", err))), nil, nil
				} else if !present {
					return errorhandler(NewMSMissingVariableConfigError(fmt.Sprintf("Workload config for %v %v is missing", workload.WorkloadURL, workloadChoice.Version), "configstate.state")), nil, nil
				}

				// get the ms references from the workload, the version here is a version range.
				apiSpecList := new(policy.APISpecList)
				for _, apiSpec := range workloadDef.APISpecs {
					newAPISpec := policy.APISpecification_Factory(apiSpec.SpecRef, apiSpec.Org, apiSpec.Version, apiSpec.Arch)
					(*apiSpecList) = append((*apiSpecList), (*newAPISpec))
				}

				// Microservices that are defined as being shared singletons can only appear once in the complete API spec list. If there
				// are 2 versions of the same shared singleton microservice, the higher version of the 2 will be auto configured.
				//completeAPISpecList.ReplaceHigherSharedSingleton(apiSpecList)

				// MergeWith will omit exact duplicates when merging the 2 lists.
				(*completeAPISpecList) = completeAPISpecList.MergeWith(apiSpecList)
			}

		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Configstate resolved pattern to APISpecs %v", *completeAPISpecList)))

		// If the pattern search doesnt find any microservices then there is a problem.
		if len(*completeAPISpecList) == 0 {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("No microservices found for %v %v.", patId, thisArch), "configstate.state")), nil, nil
		} 
		
		// for now, anax only allow one microservice version, so we need to get the common version range for each microservice.	
		common_apispec_list, err := completeAPISpecList.GetCommonVersionRanges()
		if err != nil {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("Error resolving microservice version ranges for %v %v.", patId, thisArch), "configstate.state")), nil, nil
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Configstate resolved microservice version ranges to %v", *common_apispec_list)))

		if len(*common_apispec_list) == 0 {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("No microservices have the common version ranges for %v %v.", patId, thisArch), "configstate.state")), nil, nil
		}


		// Using the list of APISpec objects, we can create a service (microservice) on this node automatically, for each microservice
		// that already has configuration or which doesnt need it.
		var createServiceError error
		passthruHandler := GetPassThroughErrorHandler(&createServiceError)
		for _, apiSpec := range *common_apispec_list {

			service := NewService(apiSpec.SpecRef, apiSpec.Org, makeServiceName(apiSpec.SpecRef, apiSpec.Org, apiSpec.Version), apiSpec.Version)
			errHandled, newService, msg := CreateService(service, passthruHandler, getMicroservice, db, config)
			if errHandled {
				switch createServiceError.(type) {
				case *MSMissingVariableConfigError:
					glog.Errorf(apiLogString(fmt.Sprintf("Configstate autoconfig received error (%T) %v", createServiceError, createServiceError)))
					msErr := createServiceError.(*MSMissingVariableConfigError)
					// Cannot autoconfig this microservice because it has variables that need to be configured.
					return errorhandler(NewAPIUserInputError(fmt.Sprintf("Configstate autoconfig, microservice %v %v %v, %v", apiSpec.SpecRef, apiSpec.Org, apiSpec.Version, msErr.Err), "configstate.state")), nil, nil

				case *DuplicateServiceError:
					glog.V(3).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig found duplicate microservice %v %v %v, continuing.", apiSpec.SpecRef, apiSpec.Org, apiSpec.Version)))
					// If the microservice is already registered, that's ok because the node user is allowed to configure any of the
					// required microservices before calling the configstate API.

				default:
					return errorhandler(NewSystemError(fmt.Sprintf("unexpected error returned from service create (%T) %v", createServiceError, createServiceError))), nil, nil
				}
			} else {
				glog.V(5).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig created service %v", newService)))
				msgs = append(msgs, msg)
			}
		}

		glog.V(3).Infof(apiLogString(fmt.Sprintf("Configstate autoconfig of microservices complete")))

	}

	// Update the state in the local database
	updatedDev, err := pDevice.SetConfigstate(db, pDevice.Id, *cfg.State)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting new config state: %v", err))), nil, nil
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Update configstate: updated device: %v", updatedDev)))

	exDev := ConvertFromPersistentHorizonDevice(updatedDev)
	return false, exDev.Config, msgs

}

// This function verifies that if the given workload needs variable configuration, that there is a workloadconfig
// object holding that config.
func workloadConfigPresent(workloadDef *exchange.WorkloadDefinition, wUrl string, wVersion string, db *bolt.DB) (bool, error) {

	// If the workload needs no config, exit early.
	if !workloadDef.NeedsUserInput() {
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
	} else if len(cfgs) == 0 {
		return false, nil
	}
	return true, nil

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
