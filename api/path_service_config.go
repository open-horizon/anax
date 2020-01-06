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
	"github.com/open-horizon/anax/exchangesync"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/microservice"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
	"strconv"
	"strings"
)

func LogServiceEvent(db *bolt.DB, severity string, message *persistence.MessageMeta, event_code string, service *Service) {
	surl := ""
	org := ""
	version := "[0.0.0,INFINITY)"
	arch := ""
	if service != nil {
		if service.Url != nil {
			surl = *service.Url
		}
		if service.Org != nil {
			org = *service.Org
		}
		if service.Arch != nil {
			arch = *service.Arch
		}
		if service.VersionRange != nil {
			version = *service.VersionRange
		}
	}

	eventlog.LogServiceEvent2(db, severity, message, event_code, "", surl, org, version, arch, []string{})
}

func findPoliciesForOutput(pm *policy.PolicyManager, db *bolt.DB) (map[string]policy.Policy, error) {

	out := make(map[string]policy.Policy)

	// Policies are kept in org specific directories
	allOrgs := pm.GetAllPolicyOrgs()
	for _, org := range allOrgs {

		allPolicies := pm.GetAllPolicies(org)
		for _, pol := range allPolicies {

			// the arch of SPecRefs have been converted to canonical arch in the pm, we will switch to the ones defined in the pattern or by user for output
			if pol.APISpecs != nil {
				for i := 0; i < len(pol.APISpecs); i++ {
					api_spec := &pol.APISpecs[i]
					if pmsdef, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{persistence.UnarchivedMSFilter(), persistence.UrlOrgMSFilter(api_spec.SpecRef, org)}); err != nil {
						glog.Warningf(apiLogString(fmt.Sprintf("Failed to get service %v/%v from local db. %v", api_spec.Org, api_spec.SpecRef, err)))
					} else if pmsdef != nil && len(pmsdef) > 0 {
						api_spec.Arch = pmsdef[0].Arch
					}
				}
			}

			out[pol.Header.Name] = pol
		}
	}

	return out, nil
}

func FindServiceConfigForOutput(pm *policy.PolicyManager, db *bolt.DB) (map[string][]MicroserviceConfig, error) {

	outConfig := make([]MicroserviceConfig, 0, 10)

	// Get all the policies so that we can grab the pieces we need from there
	policies, err := findPoliciesForOutput(pm, db)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to get local policies, error %v", err))
	}

	// Each policy has some data we need for creating the output object. There is also data
	// in the microservice definition database and the attibutes in the attribute database.
	for _, pol := range policies {
		// skip the node policy which does not have api specs.
		if len(pol.APISpecs) == 0 {
			continue
		}
		msURL := pol.APISpecs[0].SpecRef
		msOrg := pol.APISpecs[0].Org
		msVer := pol.APISpecs[0].Version
		mc := NewMicroserviceConfig(msURL, msOrg, msVer)

		// Find the microservice definition in our database so that we can get the upgrade settings.
		msDefs, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{persistence.UrlOrgVersionMSFilter(msURL, msOrg, msVer), persistence.UnarchivedMSFilter()})
		if err != nil {
			return nil, errors.New(fmt.Sprintf("unable to get service definitions from the database, error %v", err))
		} else if msDefs != nil && len(msDefs) > 0 {
			mc.AutoUpgrade = msDefs[0].AutoUpgrade
			mc.ActiveUpgrade = msDefs[0].ActiveUpgrade
		} else {
			// take the default
			mc.AutoUpgrade = microservice.MS_DEFAULT_AUTOUPGRADE
			mc.ActiveUpgrade = microservice.MS_DEFAULT_ACTIVEUPGRADE
		}

		// Get the attributes for this service from the attributes database
		if attrs, err := persistence.FindApplicableAttributes(db, msURL, msOrg); err != nil {
			return nil, errors.New(fmt.Sprintf("unable to get service attributes from the database, error %v", err))
		} else {
			mc.Attributes = attrs
		}

		// Add the microservice config to the output array
		outConfig = append(outConfig, *mc)
	}

	out := make(map[string][]MicroserviceConfig)
	out["config"] = outConfig

	return out, nil
}

// Given a demarshalled Service object, validate it and save it, returning any errors.
func CreateService(service *Service,
	errorhandler ErrorHandler,
	getPatterns exchange.PatternHandler,
	resolveService exchange.ServiceResolverHandler,
	getService exchange.ServiceHandler,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler,
	mergedUserInput *policy.UserInput, //nil for /service/config case. non-nil for auto-complete case to save some getPatterns calls.
	db *bolt.DB,
	config *config.HorizonConfig,
	from_user bool) (bool, *Service, *events.PolicyCreatedMessage) {

	org_forlog := ""
	if service.Org != nil {
		org_forlog = *service.Org
	}
	url_forlog := ""
	if service.Url != nil {
		url_forlog = *service.Url
	}
	if from_user {
		LogServiceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_START_SVC_CONFIG, org_forlog, url_forlog), persistence.EC_START_SERVICE_CONFIG, service)
	} else {
		LogServiceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_START_SVC_AUTO_CONFIG, org_forlog, url_forlog), persistence.EC_START_SERVICE_CONFIG, service)
	}

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read horizondevice object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(NewAPIUserInputError("Exchange registration not recorded. Complete account and device registration with an exchange and then record device registration using this API's /horizondevice path.", "service")), nil, nil
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Create service payload: %v", service)))

	// Validate all the inputs in the service object.
	if *service.Url == "" {
		return errorhandler(NewAPIUserInputError("not specified", "service.url")), nil, nil
	}
	if bail := checkInputString(errorhandler, "service.url", service.Url); bail {
		return true, nil, nil
	}

	// Use the device's org if org not specified in the service object.
	if service.Org == nil || *service.Org == "" {
		service.Org = &pDevice.Org
	} else if bail := checkInputString(errorhandler, "service.organization", service.Org); bail {
		return true, nil, nil
	}

	// Return error if the arch in the service object is not a synonym of the node's arch.
	// Use the device's arch if not specified in the service object.
	thisArch := cutil.ArchString()
	if service.Arch == nil || *service.Arch == "" {
		service.Arch = &thisArch
	} else if *service.Arch != thisArch && config.ArchSynonyms.GetCanonicalArch(*service.Arch) != thisArch {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("arch %v is not supported by this node.", *service.Arch), "service.arch")), nil, nil
	} else if bail := checkInputString(errorhandler, "service.arch", service.Arch); bail {
		return true, nil, nil
	}

	// save the version range for later use
	vr_saved := "[0.0.0,INFINITY)"
	if service.VersionRange != nil && *service.VersionRange != "" {
		vr_saved = *service.VersionRange
	}

	if pDevice.Pattern != "" {
		pattern_org, pattern_name, _ := persistence.GetFormatedPatternString(pDevice.Pattern, pDevice.Org)

		if from_user {
			// We might be registering a dependent service, so look through the pattern and get a list of all dependent services, then
			// come up with a common version for all references. If the service we're registering is one of these, then use the
			// common version range in our service instead of the version range that was passed as input.
			common_apispec_list, exchPattern, err := getSpecRefsForPattern(pattern_name, pattern_org, getPatterns, resolveService, db, config, false)
			if err != nil {
				return errorhandler(err), nil, nil
			}

			if len(*common_apispec_list) != 0 {
				for _, apiSpec := range *common_apispec_list {
					if apiSpec.SpecRef == *service.Url && apiSpec.Org == *service.Org {
						service.VersionRange = &apiSpec.Version
						service.Arch = &apiSpec.Arch
						break
					}
				}
			}

			// get the user input from the pattern so that we can merge it with the given service attributes to make sure all the necessary user inputs are set.
			if mergedUserInput == nil {
				var err1 error
				mergedUserInput, err1 = getMergedUserInput(exchPattern.UserInput, *service.Url, *service.Org, *service.Arch, db)
				if err1 != nil {
					return errorhandler(NewSystemError(fmt.Sprintf("Failed to get the service config from the merged node user input with pattern user input. %v", err1))), nil, nil
				}
			}
		}
	} else {
		// this is the case where /service/config is called for policy
		if mergedUserInput == nil {
			var err1 error
			mergedUserInput, err1 = getMergedUserInput([]policy.UserInput{}, *service.Url, *service.Org, *service.Arch, db)
			if err1 != nil {
				return errorhandler(NewSystemError(fmt.Sprintf("Failed to get the service config from the node user input. %v", err1))), nil, nil
			}
		}
	}

	// The versionRange field is checked for valid characters by the Version_Expression_Factory, it has a very
	// specific syntax and allows a subset of normally valid characters.

	// Use a default sensor version that allows all version if not specified.
	if service.VersionRange == nil || *service.VersionRange == "" {
		def := "0.0.0"
		service.VersionRange = &def
	}

	// Convert the sensor version to a version expression.
	vExp, err := semanticversion.Version_Expression_Factory(*service.VersionRange)
	if err != nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("versionRange %v cannot be converted to a version expression, error %v", *service.VersionRange, err), "service.versionRange")), nil, nil
	}

	// Verify with the exchange to make sure the service definition is readable by this node.
	var msdef *persistence.MicroserviceDefinition
	var sdef *exchange.ServiceDefinition
	var err1 error
	sdef, _, err1 = getService(*service.Url, *service.Org, vExp.Get_expression(), *service.Arch)
	if err1 != nil || sdef == nil {
		if *service.Arch == thisArch {
			// failed with user defined arch
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("Unable to find the service definition using  %v/%v %v %v in the exchange.", *service.Org, *service.Url, vExp.Get_expression(), *service.Arch), "service")), nil, nil
		} else {
			// try node's arch
			sdef, _, err1 = getService(*service.Url, *service.Org, vExp.Get_expression(), thisArch)
			if err1 != nil || sdef == nil {
				if pDevice.Pattern != "" {
					return errorhandler(NewAPIUserInputError(fmt.Sprintf("Unable to find the service definition using  %v/%v %v %v in the exchange. Please ensure all services referenced in the user input file are included in pattern %v.", *service.Org, *service.Url, vExp.Get_expression(), thisArch, pDevice.Pattern), "service")), nil, nil
				}
				return errorhandler(NewAPIUserInputError(fmt.Sprintf("Unable to find the service definition using  %v/%v %v %v in the exchange.", *service.Org, *service.Url, vExp.Get_expression(), thisArch), "service")), nil, nil
			}
		}
	}

	// Convert the service definition to a persistent format so that it can be saved to the db.
	msdef, err = microservice.ConvertServiceToPersistent(sdef, *service.Org)
	if err != nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("Error converting the service metadata to persistent.MicroserviceDefinition for %v/%v version %v, error %v", *service.Org, sdef.URL, sdef.Version, err), "service")), nil, nil
	}

	// Save some of the items in the MicroserviceDefinition object for use in the upgrading process.
	if service.Name != nil {
		msdef.Name = *service.Name
	} else {
		names := strings.Split(*service.Url, "/")
		msdef.Name = names[len(names)-1]
		service.Name = &msdef.Name
	}
	msdef.RequestedArch = *service.Arch
	msdef.UpgradeVersionRange = vExp.Get_expression()
	if service.AutoUpgrade != nil {
		msdef.AutoUpgrade = *service.AutoUpgrade
	}
	if service.ActiveUpgrade != nil {
		msdef.ActiveUpgrade = *service.ActiveUpgrade
	}

	// The service definition returned by the exchange might be newer than what was specified in the input service object, so we save
	// the actual version of the service so that we know if we need to upgrade in the future.
	service.VersionRange = &msdef.Version

	// Check if the service has been registered or not (currently only support one service registration)
	if pms, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{persistence.UnarchivedMSFilter(), persistence.UrlOrgMSFilter(*service.Url, *service.Org)}); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error accessing db to find service definition: %v", err))), nil, nil
	} else if pms != nil && len(pms) > 0 {
		// this is for the auto service registration case.
		if !from_user {
			LogServiceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_COMPLETE_SVC_AUTO_CONFIG, *service.Org, *service.Url), persistence.EC_SERVICE_CONFIG_COMPLETE, service)
		}
		return errorhandler(NewDuplicateServiceError(fmt.Sprintf("Duplicate registration for %v/%v %v %v. Only one registration per service is supported.", *service.Org, *service.Url, vExp.Get_expression(), cutil.ArchString()), "service")), nil, nil
	}

	// Validate any attributes specified in the attribute list and convert them to persistent objects.
	// This attribute verifier makes sure that there is a mapped attribute which specifies values for all the non-default
	// user inputs in the specific service selected earlier.
	msdefAttributeVerifier := func(attr persistence.Attribute) (bool, error) {

		// Verfiy that all userInput variables are correctly typed and that non-defaulted userInput variables are specified
		// in a mapped property attribute.
		if msdef != nil && attr.GetMeta().Type == "UserInputAttributes" {

			// Loop through each input variable and verify that it is defined in the service's user input section, and that the
			// type matches.
			for varName, varValue := range attr.GetGenericMappings() {
				glog.V(5).Infof(apiLogString(fmt.Sprintf("checking input variable: %v", varName)))
				if ui := msdef.GetUserInputName(varName); ui != nil {
					if err := cutil.VerifyWorkloadVarTypes(varValue, ui.Type); err != nil {
						return errorhandler(NewAPIUserInputError(fmt.Sprintf(cutil.ANAX_SVC_WRONG_TYPE+"%v", varName, cutil.FormOrgSpecUrl(*service.Url, *service.Org), err), "variables")), nil
					}
				}
			}
		}

		return false, nil
	}

	// This attribute verifier makes sure that nodes using a pattern dont try to use policy. When patterns are in use, all policy
	// comes from the pattern.
	patternedDeviceAttributeVerifier := func(attr persistence.Attribute) (bool, error) {

		// If the device declared itself to be using a pattern, then it CANNOT specify any attributes that generate policy settings.
		if pDevice.Pattern != "" {
			if attr.GetMeta().Type == "MeteringAttributes" || attr.GetMeta().Type == "PropertyAttributes" || attr.GetMeta().Type == "AgreementProtocolAttributes" {
				return errorhandler(NewAPIUserInputError(fmt.Sprintf("device is using a pattern %v, policy attributes are not supported.", pDevice.Pattern), "service.[attribute].type")), nil
			}
		}

		return false, nil
	}

	var attributes []persistence.Attribute
	if service.Attributes != nil {
		// build a serviceAttribute for each one
		var err error
		var inputErrWritten bool

		attributes, inputErrWritten, err = toPersistedAttributesAttachedToService(errorhandler, pDevice, *service.Attributes, persistence.NewServiceSpec(*service.Url, *service.Org), []AttributeVerifier{msdefAttributeVerifier, patternedDeviceAttributeVerifier})
		if !inputErrWritten && err != nil {
			return errorhandler(NewSystemError(fmt.Sprintf("Failure deserializing attributes: %v", err))), nil, nil
		} else if inputErrWritten {
			return true, nil, nil
		}
	}

	// Information advertised in the edge node policy file
	var haPartner []string
	var globalAgreementProtocols []interface{}

	props := make(map[string]interface{})

	// There might be node wide global attributes. Check for them and grab the values to use as defaults for later.
	allAttrs, aerr := persistence.FindApplicableAttributes(db, "", "")
	if aerr != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to fetch global attributes, error %v", err))), nil, nil
	}

	// For each node wide attribute, extract the value and save it for use later in this function.
	for _, attr := range allAttrs {
		// Extract HA property
		if attr.GetMeta().Type == "HAAttributes" {
			haPartner = attr.(persistence.HAAttributes).Partners
			glog.V(5).Infof(apiLogString(fmt.Sprintf("Found default global HA attribute %v", attr)))
		}
	}

	// If an HA device has no HA attribute then the configuration is invalid.
	if pDevice.HA && len(haPartner) == 0 {
		return errorhandler(NewAPIUserInputError("services on an HA device must specify an HA partner.", "service.[attribute].type")), nil, nil
	}

	// Persist all attributes on this service, and while we're at it, fetch the attribute values we need for the node side policy file.
	// Any policy attributes we find will overwrite values set in a global attribute of the same type.
	var serviceAgreementProtocols []policy.AgreementProtocol

	userInput := []policy.UserInput{}
	for _, attr := range attributes {
		bSave := true
		switch attr.(type) {
		case *persistence.UserInputAttributes:
			// do not save UserInputAttributes because it will be converted to UserInput and saved.
			bSave = false
			if from_user {
				ui := convertAttributeToExchangeUserInput(service, vr_saved, attr.(*persistence.UserInputAttributes))
				if ui != nil {
					userInput = append(userInput, *ui)
				}
			}

		case *persistence.HAAttributes:
			haPartner = attr.(*persistence.HAAttributes).Partners

		case *persistence.AgreementProtocolAttributes:
			agpl := attr.(*persistence.AgreementProtocolAttributes).Protocols
			serviceAgreementProtocols = agpl.([]policy.AgreementProtocol)

		default:
			glog.V(4).Infof(apiLogString(fmt.Sprintf("Unhandled attr type (%T): %v", attr, attr)))
		}

		if bSave {
			_, err := persistence.SaveOrUpdateAttribute(db, attr, "", false)
			if err != nil {
				return errorhandler(NewSystemError(fmt.Sprintf("error saving attribute %v, error %v", attr, err))), nil, nil
			}
		}
	}

	// merge the user input with the pattern and existing node user input to get a whole user input for this service
	merged_ui := mergedUserInput
	if len(userInput) > 0 {
		if mergedUserInput == nil {
			merged_ui = &userInput[0]
		} else {
			// there should be only one for this service
			merged_ui, _ = policy.MergeUserInput(*mergedUserInput, userInput[0], false)
		}
	}

	// make sure we have all the required user settings for this service. We can only check for the pattern case.
	if present, missingVarName := validateUserInput(sdef, merged_ui); !present {
		if pDevice.Pattern != "" {
			return errorhandler(NewMSMissingVariableConfigError(fmt.Sprintf(cutil.ANAX_SVC_MISSING_VARIABLE, missingVarName, cutil.FormOrgSpecUrl(*service.Url, *service.Org)), "service.[attribute].mappings")), nil, nil
		} else {
			// For policy case, we do not know what business policy will form agreement with it, so we just give warning for the missing variable name
			glog.Warningf(apiLogString(fmt.Sprintf("Variable %v is missing in the service configuration for %v/%v. It may cause agreement not formed if the business policy does not contain the setting for the missing variable.", missingVarName, *service.Org, *service.Url)))
			LogServiceEvent(db, persistence.SEVERITY_WARN, persistence.NewMessageMeta(EL_API_ERR_MISS_VAR_IN_SVC_CONFIG, missingVarName, *service.Org, *service.Url), persistence.EC_WARNING_SERVICE_CONFIG, service)
		}
	}

	if from_user && len(userInput) > 0 {
		if err := exchangesync.PatchNodeUserInput(pDevice, db, userInput, getDevice, patchDevice); err != nil {
			return errorhandler(NewSystemError(fmt.Sprintf("Failed to add the user input %v to node. %v", userInput, err))), nil, nil
		}
	}

	// add node built-in properties
	existingPol, err := persistence.FindNodePolicy(db)
	if err != nil {
		glog.V(2).Infof("Failed to retrieve node policy from local db: %v", err)
	}
	externalPol := externalpolicy.CreateNodeBuiltInPolicy(false, false, existingPol)
	if externalPol != nil {
		for _, ele := range externalPol.Properties {
			if ele.Name == externalpolicy.PROP_NODE_CPU {
				props["cpus"] = strconv.FormatFloat(ele.Value.(float64), 'f', -1, 64)
			} else if ele.Name == externalpolicy.PROP_NODE_MEMORY {
				props["ram"] = strconv.FormatFloat(ele.Value.(float64), 'f', -1, 64)
			}
		}
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Complete Attr list for registration of service %v/%v: %v", *service.Org, *service.Url, attributes)))

	// Save the service definition in the local database.
	if err := persistence.SaveOrUpdateMicroserviceDef(db, msdef); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error saving service definition %v into db: %v", *msdef, err))), nil, nil
	}

	if pDevice.Pattern == "" {
		// non pattern case, do not generate policies
		LogServiceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_COMPLETE_SVC_CONFIG, *service.Org, *service.Url), persistence.EC_SERVICE_CONFIG_COMPLETE, service)
		return false, service, nil
	} else {
		// Establish the correct agreement protocol list. The AGP list from this service overrides any global list that might exist.
		var agpList *[]policy.AgreementProtocol
		if len(serviceAgreementProtocols) != 0 {
			agpList = &serviceAgreementProtocols
		} else if list, err := policy.ConvertToAgreementProtocolList(globalAgreementProtocols); err != nil {
			return errorhandler(NewSystemError(fmt.Sprintf("Error converting global agreement protocol list attribute %v to agreement protocol list, error: %v", globalAgreementProtocols, err))), nil, nil
		} else {
			agpList = list
		}

		// Set max number of agreements for this service's policy.
		maxAgreements := 1
		if msdef.Sharable == exchange.MS_SHARING_MODE_SINGLETON || msdef.Sharable == exchange.MS_SHARING_MODE_MULTIPLE || msdef.Sharable == exchange.MS_SHARING_MODE_SINGLE {
			maxAgreements = 0 // no limites for pattern
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Create service policy: %v", service)))

		// Generate a policy based on all the attributes and the service definition.
		if polFileName, genErr := policy.GeneratePolicy(*service.Url, *service.Org, *service.Name, *service.VersionRange, *service.Arch, &props, haPartner, *agpList, maxAgreements, config.Edge.PolicyPath, pDevice.Org); genErr != nil {
			return errorhandler(NewSystemError(fmt.Sprintf("Error generating policy, error: %v", genErr))), nil, nil
		} else {
			if from_user {
				LogServiceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_COMPLETE_SVC_CONFIG, *service.Org, *service.Url), persistence.EC_SERVICE_CONFIG_COMPLETE, service)
			} else {
				LogServiceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_COMPLETE_SVC_AUTO_CONFIG, *service.Org, *service.Url), persistence.EC_SERVICE_CONFIG_COMPLETE, service)
			}
			// Create the new policy event
			msg := events.NewPolicyCreatedMessage(events.NEW_POLICY, polFileName)

			return false, service, msg
		}
	}
}

// Convert the UserInputAttributes to UserInput of policy.
func convertAttributeToExchangeUserInput(service *Service, vr string, attr *persistence.UserInputAttributes) *policy.UserInput {
	userInput := new(policy.UserInput)
	if attr.ServiceSpecs != nil && len(*attr.ServiceSpecs) > 0 {
		userInput.ServiceUrl = (*attr.ServiceSpecs)[0].Url
		userInput.ServiceOrgid = (*attr.ServiceSpecs)[0].Org
	} else {
		if service.Url != nil {
			userInput.ServiceUrl = *service.Url
		}
		if service.Org != nil {
			userInput.ServiceOrgid = *service.Org
		}
	}
	if service.Arch != nil {
		userInput.ServiceArch = *service.Arch
	}

	userInput.ServiceVersionRange = vr

	if len(attr.Mappings) == 0 {
		return nil
	} else {
		ui := []policy.Input{}
		for k, v := range attr.Mappings {
			ui = append(ui, policy.Input{Name: k, Value: v})
		}
		userInput.Inputs = ui
	}
	return userInput
}

// check if the given merged user input satisfies the service requirement. It is only called in the pattern case.
func validateUserInput(sdef *exchange.ServiceDefinition, mergedUserInput *policy.UserInput) (bool, string) {
	if !sdef.NeedsUserInput() {
		return true, ""
	}

	if mergedUserInput == nil || mergedUserInput.Inputs == nil || len(mergedUserInput.Inputs) == 0 {
		for _, ui := range sdef.UserInputs {
			if ui.DefaultValue == "" {
				return false, ui.Name
			}
		}
	} else {
		// check if the user input has all the necessary values
		for _, ui := range sdef.UserInputs {
			if ui.DefaultValue != "" {
				continue
			} else if mergedUserInput.FindInput(ui.Name) == nil {
				return false, ui.Name
			}
		}
	}

	return true, ""
}

// get the pattern from exchange
func getExchangePattern(patOrg string, patName string, getPatterns exchange.PatternHandler) (*exchange.Pattern, error) {
	pattern, err := getPatterns(patOrg, patName)
	if err != nil {
		return nil, fmt.Errorf("Unable to read pattern object %v from exchange, error %v", patName, err)
	} else if len(pattern) != 1 {
		return nil, fmt.Errorf("Expected only 1 pattern from exchange, received %v", len(pattern))
	}

	// Get the pattern definition that we need to analyze.
	patId := fmt.Sprintf("%v/%v", patOrg, patName)
	exchPattern, ok := pattern[patId]
	if !ok {
		return nil, fmt.Errorf("Expected pattern id not found in GET pattern response: %v", pattern)
	}

	return &exchPattern, nil
}

// get the merged user input for a service from the pattern and node.
func getMergedUserInput(patternUserInput []policy.UserInput, svcUrl, svcOrg, svcArch string, db *bolt.DB) (*policy.UserInput, error) {

	// get node user input
	nodeUserInput, err := persistence.FindNodeUserInput(db)
	if err != nil {
		return nil, fmt.Errorf("Failed get user input from local db. %v", err)
	}

	// merge node user input it with pattern user input
	mergedUserInput := policy.MergeUserInputArrays(patternUserInput, nodeUserInput, true)
	if mergedUserInput == nil {
		mergedUserInput = []policy.UserInput{}
	}

	// get the user input for this service
	ui_merged, err := policy.FindUserInput(svcUrl, svcOrg, "", svcArch, mergedUserInput)
	if err != nil {
		return nil, fmt.Errorf("Failed to find preferences for service %v/%v from the merged user input, error: %v", svcOrg, svcUrl, err)
	}
	return ui_merged, nil
}
