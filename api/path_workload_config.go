package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"sort"
)

func FindWorkloadConfigForOutput(db *bolt.DB) (map[string][]persistence.WorkloadConfig, error) {

	// Only "get all" is supported
	wrap := make(map[string][]persistence.WorkloadConfig)

	// Retrieve all workload configs from the db
	cfgs, err := persistence.FindWorkloadConfigs(db, []persistence.WCFilter{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read workloadconfig objects, error %v", err))
	}

	wrap["config"] = cfgs

	// Sort the output by workload URL and then within that by version
	sort.Sort(WorkloadConfigByWorkloadURLAndVersion(wrap["config"]))

	return wrap, nil

}

// Given a demarshalled workloadconfig object, validate it and save it, returning any errors.
func CreateWorkloadconfig(cfg *WorkloadConfig,
	existingDevice *persistence.ExchangeDevice,
	errorhandler ErrorHandler,
	getWorkload exchange.WorkloadHandler,
	db *bolt.DB) (bool, *persistence.WorkloadConfig) {

	glog.V(5).Infof(apiLogString(fmt.Sprintf("WorkloadConfig POST input: %v", cfg)))

	// Validate the input strings. The variables map can be empty if the device owner wants
	// the workload to use all default values, so we wont validate that map.
	if cfg.WorkloadURL == "" {
		return errorhandler(NewAPIUserInputError("not specified", "workload_url")), nil
	}

	// If version is omitted, the default is all versions.
	if cfg.Version == "" {
		cfg.Version = "0.0.0"
	}

	if !policy.IsVersionString(cfg.Version) && !policy.IsVersionExpression(cfg.Version) {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("workload_version %v is not a valid version string or expression", cfg.Version), "workload_version")), nil
	}

	// Convert the input version to a full version expression if it is not already a full expression.
	vExp, verr := policy.Version_Expression_Factory(cfg.Version)
	if verr != nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("workload_version %v error converting to full version expression, error: %v", cfg.Version, verr), "workload_version")), nil
	}

	// Use the device org if not explicitly specified. We cant verify whether or not the org exists because the node
	// we are running on might not have authority to read other orgs in the exchange.
	org := cfg.Org
	if cfg.Org == "" {
		org = existingDevice.Org
	}

	// Reject the POST if there is already a config for this workload and version range
	existingCfg, err := persistence.FindWorkloadConfig(db, cfg.WorkloadURL, org, vExp.Get_expression())
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read workloadconfig object, error %v", err))), nil
	} else if existingCfg != nil {
		return errorhandler(NewConflictError("workloadconfig already exists")), nil
	}

	// Get the workload metadata from the exchange
	workloadDef, _, err := getWorkload(cfg.WorkloadURL, org, vExp.Get_expression(), cutil.ArchString(), existingDevice.GetId(), existingDevice.Token)
	if err != nil || workloadDef == nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("unable to find the workload definition using %v %v %v %v in the exchange.", cfg.WorkloadURL, org, vExp.Get_expression(), cutil.ArchString()), "workload_url")), nil
	}

	// Only the UserInputAttribute is supported. It must include a value for all non-default userInputs in the workload definition.
	workloadAttributeVerifier := func(attr persistence.Attribute) (bool, error) {

		// Verfiy that all non-defaulted userInput variables in the workload definition are specified in a mapped attribute
		// of this service invocation.
		if attr.GetMeta().Type == "UserInputAttributes" {

			// Loop through each input variable and verify that it is defined in the workload's user input section, and that the
			// type matches.
			for varName, varValue := range attr.GetGenericMappings() {
				glog.V(5).Infof(apiLogString(fmt.Sprintf("WorkloadConfig checking input variable: %v", varName)))
				if ui := workloadDef.GetUserInputName(varName); ui != nil {
					if err := cutil.VerifyWorkloadVarTypes(varValue, ui.Type); err != nil {
						return errorhandler(NewAPIUserInputError(fmt.Sprintf("WorkloadConfig variable %v is %v", varName, err), "variables")), nil
					}
				} else {
					return errorhandler(NewAPIUserInputError(fmt.Sprintf("unable to find the workload config variable %v in workload definition %v %v %v %v", varName, cfg.WorkloadURL, org, vExp.Get_expression(), cutil.ArchString()), "variables")), nil
				}
			}

			// Loop through each userInput variable in the workload definition to make sure variables without default values have been set.
			for _, ui := range workloadDef.UserInputs {
				glog.V(5).Infof(apiLogString(fmt.Sprintf("WorkloadConfig checking workload userInput: %v", ui)))
				if _, ok := attr.GetGenericMappings()[ui.Name]; !ok && ui.DefaultValue == "" {
					// User Input variable is not defined in the workload config request and doesnt have a default, that's a problem.
					return errorhandler(NewAPIUserInputError(fmt.Sprintf("WorkloadConfig does not set %v, which has no default value", ui.Name), "variables")), nil
				}
			}

		} else {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("attribute %v is not supported on workload/config", attr.GetMeta().Type), "workload.[attribute]")), nil
		}

		return false, nil
	}

	// Verify the input attributes and convert to persistent attributes. If a UserInputAttribute is not returned for workloads that need user input,
	// there is a problem.
	var attributes []persistence.Attribute
	var inputErrWritten bool

	attributes, inputErrWritten, err = toPersistedAttributes(errorhandler, false, existingDevice, cfg.Attributes, []AttributeVerifier{workloadAttributeVerifier})
	if !inputErrWritten && err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Failure validating attributes: %v", err))), nil
	} else if inputErrWritten {
		return true, nil
	} else if workloadDef.NeedsUserInput() {
		uia := attributesContains(attributes, "", "UserInputAttributes")
		if uia == nil {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("workload requires userInput variables to be set, but there are no UserInputAttributes"), "workload.[attribute].UserInputAttributes")), nil
		}
	}

	// Persist the workload configuration to the database
	glog.V(5).Infof(apiLogString(fmt.Sprintf("WorkloadConfig persisting variables: %v (%T)", attributes, attributes)))

	wc, err := persistence.NewWorkloadConfig(db, cfg.WorkloadURL, org, vExp.Get_expression(), attributes)
	if err != nil {
		glog.Error(apiLogString(err))
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to save workloadconfig object, error: %v", err))), nil
	}

	return false, wc
}

// Delete a workloadconfig object.
func DeleteWorkloadconfig(cfg *WorkloadConfig,
	errorhandler ErrorHandler,
	db *bolt.DB) bool {

	glog.V(5).Infof(apiLogString(fmt.Sprintf("WorkloadConfig DELETE: %v", &cfg)))

	// Validate the input strings. The variables map is ignored.
	if cfg.WorkloadURL == "" {
		return errorhandler(NewAPIUserInputError("not specified", "workload_url"))
	} else if cfg.Version == "" {
		return errorhandler(NewAPIUserInputError("not specified", "workload_version"))
	} else if !policy.IsVersionString(cfg.Version) && !policy.IsVersionExpression(cfg.Version) {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("workload_version %v is not a valid version string", cfg.Version), "workload_version"))
	}

	// Convert the input version to a full version expression if it is not already a full expression.
	vExp, verr := policy.Version_Expression_Factory(cfg.Version)
	if verr != nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("workload_version %v error converting to full version expression, error: %v", cfg.Version, verr), "workload_version"))
	}

	// Find the target record
	existingCfg, err := persistence.FindWorkloadConfig(db, cfg.WorkloadURL, cfg.Org, vExp.Get_expression())
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read workloadconfig object, error: %v", err)))
	} else if existingCfg == nil {
		return errorhandler(NewNotFoundError("WorkloadConfig not found", "workloadconfig"))
	} else {
		glog.V(5).Infof(apiLogString(fmt.Sprintf("WorkloadConfig deleting: %v", &cfg)))
		persistence.DeleteWorkloadConfig(db, cfg.WorkloadURL, cfg.Org, vExp.Get_expression())
		return false
	}

}
