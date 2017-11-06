package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"sort"
	"strings"
)

func FindWorkloadConfigForOutput(db *bolt.DB) (map[string][]persistence.WorkloadConfig, error) {

	// Only "get all" is supported
	wrap := make(map[string][]persistence.WorkloadConfig)

	// Retrieve all workload configs from the db
	cfgs, err := persistence.FindWorkloadConfigs(db, []persistence.WCFilter{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read workloadconfig objects, error %v", err))
	}

	wrap["active"] = cfgs

	// Sort the output by workload URL and then within that by version
	sort.Sort(WorkloadConfigByWorkloadURLAndVersion(wrap["active"]))

	return wrap, nil

}

// Given a demarshalled workloadconfig object, validate it and save it, returning any errors.
func CreateWorkloadconfig(cfg *WorkloadConfig,
	existingDevice *persistence.ExchangeDevice,
	errorhandler ErrorHandler,
	getWorkload WorkloadHandler,
	db *bolt.DB) (bool, *persistence.WorkloadConfig) {

	glog.V(5).Infof("WorkloadConfig POST input: %v", cfg)

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
	// we are running on migh tnot have authority to read other orgs in the exchange.
	org := cfg.Org
	if cfg.Org == "" {
		org = existingDevice.Org
	}

	// Reject the POST if there is already a config for this workload and version range
	existingCfg, err := persistence.FindWorkloadConfig(db, cfg.WorkloadURL, vExp.Get_expression())
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read workloadconfig object, error %v", err))), nil
	} else if existingCfg != nil {
		return errorhandler(NewConflictError("workloadconfig already exists")), nil
	}

	// Get the workload metadata from the exchange and verify the userInput against the variables in the POST body.
	workloadDef, err := getWorkload(cfg.WorkloadURL, org, vExp.Get_expression(), cutil.ArchString(), existingDevice.GetId(), existingDevice.Token)
	if err != nil || workloadDef == nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("unable to find the workload definition using version %v in the exchange.", vExp.Get_expression()), "workload_url")), nil
	}

	// Loop through each input variable and verify that it is defined in the workload's user input section, and that the
	// type matches.
	for varName, varValue := range cfg.Variables {
		glog.V(5).Infof("WorkloadConfig checking input variable: %v", varName)
		if ui := workloadDef.GetUserInputName(varName); ui != nil {
			errMsg := ""
			switch varValue.(type) {
			case string:
				if ui.Type != "string" {
					errMsg = fmt.Sprintf("WorkloadConfig variable %v is type %T, expecting %v", varName, varValue, ui.Type)
				}
			case json.Number:
				strNum := varValue.(json.Number).String()
				if ui.Type != "int" && ui.Type != "float" {
					errMsg = fmt.Sprintf("WorkloadConfig variable %v is a number, expecting %v", varName, ui.Type)
				} else if strings.Contains(strNum, ".") && ui.Type == "int" {
					errMsg = fmt.Sprintf("WorkloadConfig variable %v is a float, expecting int", varName)
				}
				cfg.Variables[varName] = strNum
			case []interface{}:
				if ui.Type != "list of strings" {
					errMsg = fmt.Sprintf("WorkloadConfig variable %v is type %T, expecting %v", varName, varValue, ui.Type)
				} else {
					for _, e := range varValue.([]interface{}) {
						if _, ok := e.(string); !ok {
							errMsg = fmt.Sprintf("WorkloadConfig variable %v is not []string", varName)
							break
						}
					}
				}
			default:
				errMsg = fmt.Sprintf("WorkloadConfig variable %v is type %T, but is an unexpected type.", varName, varValue)
			}
			if errMsg != "" {
				return errorhandler(NewAPIUserInputError(errMsg, "variables")), nil
			}
		} else {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("unable to find the workload config variable %v in workload definition", varName), "variables")), nil
		}
	}

	// Loop through each userInput variable in the workload definition to make sure variables without default values have been set.
	for _, ui := range workloadDef.UserInputs {
		glog.V(5).Infof("WorkloadConfig checking workload userInput: %v", ui)
		if _, ok := cfg.Variables[ui.Name]; ok {
			// User Input variable is defined in the workload config request
			continue
		} else if !ok && ui.DefaultValue != "" {
			// User Input variable is not defined in the workload config request but it has a default in the workload definition. Save
			// the default into the workload config so that we dont have to query the exchange for the value when the workload starts.
			cfg.Variables[ui.Name] = ui.DefaultValue
		} else {
			// User Input variable is not defined in the workload config request and doesnt have a default, that's a problem.
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("WorkloadConfig does not set %v, which has no default value", ui.Name), "variables")), nil
		}
	}

	// Persist the workload configuration to the database
	glog.V(5).Infof("WorkloadConfig persisting variables: %v", cfg.Variables)

	wc, err := persistence.NewWorkloadConfig(db, cfg.WorkloadURL, vExp.Get_expression(), cfg.Variables)
	if err != nil {
		glog.Error(err)
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to save workloadconfig object, error: %v", err))), nil
	}

	return false, wc
}

// Delete a workloadconfig object.
func DeleteWorkloadconfig(cfg *WorkloadConfig,
	errorhandler ErrorHandler,
	db *bolt.DB) bool {

	glog.V(5).Infof("WorkloadConfig DELETE: %v", &cfg)

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
	existingCfg, err := persistence.FindWorkloadConfig(db, cfg.WorkloadURL, vExp.Get_expression())
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read workloadconfig object, error: %v", err)))
	} else if existingCfg == nil {
		return errorhandler(NewNotFoundError("WorkloadConfig not found", "workloadconfig"))
	} else {
		glog.V(5).Infof("WorkloadConfig deleting: %v", &cfg)
		persistence.DeleteWorkloadConfig(db, cfg.WorkloadURL, vExp.Get_expression())
		return false
	}

}
