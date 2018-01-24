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
	"github.com/open-horizon/anax/microservice"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"reflect"
	"strconv"
	"strings"
)

func FindServiceConfigForOutput(pm *policy.PolicyManager, db *bolt.DB) (map[string][]MicroserviceConfig, error) {

	outConfig := make([]MicroserviceConfig, 0, 10)

	// Get all the policies so that we can grab the pieces we need from there
	policies, err := FindPoliciesForOutput(pm, db)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to get local policies, error %v", err))
	}

	// Each policy has some data we need for creating the output object. There is also data
	// in the microservice definition database and the attibutes in the attribute database.
	for _, pol := range policies {
		msURL := pol.APISpecs[0].SpecRef
		msOrg := pol.APISpecs[0].Org
		msVer := pol.APISpecs[0].Version
		mc := NewMicroserviceConfig(msURL, msOrg, msVer)

		// Find the microservice definition in our database so that we can get the upgrade settings.
		msDefs, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{persistence.UrlOrgVersionMSFilter(msURL, msOrg, msVer), persistence.UnarchivedMSFilter()})
		if err != nil {
			return nil, errors.New(fmt.Sprintf("unable to get microservice definitions from the database, error %v", err))
		} else if msDefs != nil && len(msDefs) > 0 {
			mc.AutoUpgrade = msDefs[0].AutoUpgrade
			mc.ActiveUpgrade = msDefs[0].ActiveUpgrade
		} else {
			// take the default
			mc.AutoUpgrade = microservice.MS_DEFAULT_AUTOUPGRADE
			mc.ActiveUpgrade = microservice.MS_DEFAULT_ACTIVEUPGRADE
		}

		// Get the attributes for this service from the attributes database
		if attrs, err := persistence.FindApplicableAttributes(db, msURL); err != nil {
			return nil, errors.New(fmt.Sprintf("unable to get microservice attributes from the database, error %v", err))
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
	db *bolt.DB,
	config *config.HorizonConfig,
	from_user bool) (bool, *Service, *events.PolicyCreatedMessage) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read horizondevice object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(NewAPIUserInputError("Exchange registration not recorded. Complete account and device registration with an exchange and then record device registration using this API's /horizondevice path.", "service")), nil, nil
	}

	// If the device is already set to use the workload/microservice model, then return an error.
	if pDevice.IsWorkloadBased() {
		return errorhandler(NewAPIUserInputError("The node is configured to use workloads and microservices, cannot configure a service.", "service")), nil, nil
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Create service payload: %v", service)))

	// Validate all the inputs in the service object.
	if bail := checkInputString(errorhandler, "service.url", service.Url); bail {
		return true, nil, nil
	}

	// We might be registering a dependent service, so look through the pattern and get a list of all dependent services, then
	// come up with a common version for all references. If the service we're registering is one of these, then use the
	// common version range in our service instead of the version range that was passed as input.
	if pDevice.Pattern != "" && from_user {
		common_apispec_list, _, err := getSpecRefsForPattern(pDevice.Pattern, pDevice.Org, getPatterns, nil, resolveService, db, config, false)
		if err != nil {
			return errorhandler(err), nil, nil
		}

		if len(*common_apispec_list) != 0 {
			for _, apiSpec := range *common_apispec_list {
				if apiSpec.SpecRef == *service.Url {
					service.Org = &apiSpec.Org
					service.VersionRange = &apiSpec.Version
					service.Arch = &apiSpec.Arch
					break
				}
			}
		}
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

	// The versionRange field is checked for valid characters by the Version_Expression_Factory, it has a very
	// specific syntax and allows a subset of normally valid characters.

	// Use a default sensor version that allows all version if not specified.
	if service.VersionRange == nil || *service.VersionRange == "" {
		def := "0.0.0"
		service.VersionRange = &def
	}

	// Convert the sensor version to a version expression.
	vExp, err := policy.Version_Expression_Factory(*service.VersionRange)
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
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("Unable to find the service definition using  %v %v %v %v in the exchange.", *service.Url, *service.Org, vExp.Get_expression(), *service.Arch), "service")), nil, nil
		} else {
			// try node's arch
			sdef, _, err1 = getService(*service.Url, *service.Org, vExp.Get_expression(), thisArch)
			if err1 != nil || sdef == nil {
				return errorhandler(NewAPIUserInputError(fmt.Sprintf("Unable to find the service definition using  %v %v %v %v in the exchange.", *service.Url, *service.Org, vExp.Get_expression(), thisArch), "service")), nil, nil
			}
		}
	}

	// Convert the service definition to a persistent format so that it can be saved to the db.
	msdef, err = microservice.ConvertServiceToPersistent(sdef, *service.Org)
	if err != nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("Error converting the service metadata to persistent.MicroserviceDefinition for %v version %v, error %v", sdef.URL, sdef.Version, err), "service")), nil, nil
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
	if pms, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{persistence.UnarchivedMSFilter(), persistence.UrlMSFilter(*service.Url)}); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error accessing db to find service definition: %v", err))), nil, nil
	} else if pms != nil && len(pms) > 0 {
		return errorhandler(NewDuplicateServiceError(fmt.Sprintf("Duplicate registration for %v %v %v %v. Only one registration per service is supported.", *service.Url, *service.Org, vExp.Get_expression(), cutil.ArchString()), "service")), nil, nil
	}

	// If there are no attributes associated with this request but the service requires some configuration, return an error.
	if service.Attributes == nil || (service.Attributes != nil && len(*service.Attributes) == 0) {
		if varname := msdef.NeedsUserInput(); varname != "" {
			return errorhandler(NewMSMissingVariableConfigError(fmt.Sprintf("variable %v is missing from mappings", varname), "service.[attribute].mapped")), nil, nil
		}
	}

	// Validate any attributes specified in the attribute list and convert them to persistent objects.
	// This attribute verifier makes sure that there is a mapped attribute which specifies values for all the non-default
	// user inputs in the specific service selected earlier.
	msdefAttributeVerifier := func(attr persistence.Attribute) (bool, error) {

		// Verfiy that all non-defaulted userInput variables in the service definition are specified in a mapped property attribute
		// of this service invocation.
		if msdef != nil && attr.GetMeta().Type == "UserInputAttributes" {
			for _, ui := range msdef.UserInputs {
				if ui.DefaultValue != "" {
					continue
				} else if _, ok := attr.GetGenericMappings()[ui.Name]; !ok {
					return errorhandler(NewMSMissingVariableConfigError(fmt.Sprintf("variable %v is missing from mappings", ui.Name), "service.[attribute].mapped")), nil
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
			if attr.GetMeta().Type == "MeteringAttributes" || attr.GetMeta().Type == "PropertyAttributes" || attr.GetMeta().Type == "CounterPartyPropertyAttributes" || attr.GetMeta().Type == "AgreementProtocolAttributes" {
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

		attributes, inputErrWritten, err = toPersistedAttributesAttachedToService(errorhandler, pDevice, config.Edge.DefaultServiceRegistrationRAM, *service.Attributes, *service.Url, []AttributeVerifier{msdefAttributeVerifier, patternedDeviceAttributeVerifier})
		if !inputErrWritten && err != nil {
			return errorhandler(NewSystemError(fmt.Sprintf("Failure deserializing attributes: %v", err))), nil, nil
		} else if inputErrWritten {
			return true, nil, nil
		}
	}

	// Information advertised in the edge node policy file
	var haPartner []string
	var meterPolicy policy.Meter
	var counterPartyProperties policy.RequiredProperty
	var properties map[string]interface{}
	var globalAgreementProtocols []interface{}

	props := make(map[string]interface{})

	// There might be node wide global attributes. Check for them and grab the values to use as defaults for later.
	allAttrs, aerr := persistence.FindApplicableAttributes(db, "")
	if aerr != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to fetch global attributes, error %v", err))), nil, nil
	}

	// For each node wide attribute, extract the value and save it for use later in this function.
	for _, attr := range allAttrs {

		// Extract HA property
		if attr.GetMeta().Type == "HAAttributes" && len(attr.GetMeta().SensorUrls) == 0 {
			haPartner = attr.(persistence.HAAttributes).Partners
			glog.V(5).Infof(apiLogString(fmt.Sprintf("Found default global HA attribute %v", attr)))
		}

		// Global policy attributes are ignored for devices that are using a pattern. All policy is controlled
		// by the pattern definition.
		if pDevice.Pattern == "" {

			// Extract global metering property
			if attr.GetMeta().Type == "MeteringAttributes" && len(attr.GetMeta().SensorUrls) == 0 {
				// found a global metering entry
				meterPolicy = policy.Meter{
					Tokens:                attr.(persistence.MeteringAttributes).Tokens,
					PerTimeUnit:           attr.(persistence.MeteringAttributes).PerTimeUnit,
					NotificationIntervalS: attr.(persistence.MeteringAttributes).NotificationIntervalS,
				}
				glog.V(5).Infof(apiLogString(fmt.Sprintf("Found default global metering attribute %v", attr)))
			}

			// Extract global counterparty property
			if attr.GetMeta().Type == "CounterPartyPropertyAttributes" && len(attr.GetMeta().SensorUrls) == 0 {
				counterPartyProperties = attr.(persistence.CounterPartyPropertyAttributes).Expression
				glog.V(5).Infof(apiLogString(fmt.Sprintf("Found default global counterpartyproperty attribute %v", attr)))
			}

			// Extract global properties
			if attr.GetMeta().Type == "PropertyAttributes" && len(attr.GetMeta().SensorUrls) == 0 {
				properties = attr.(persistence.PropertyAttributes).Mappings
				glog.V(5).Infof(apiLogString(fmt.Sprintf("Found default global properties %v", properties)))
			}

			// Extract global agreement protocol attribute
			if attr.GetMeta().Type == "AgreementProtocolAttributes" && len(attr.GetMeta().SensorUrls) == 0 {
				agpl := attr.(persistence.AgreementProtocolAttributes).Protocols
				globalAgreementProtocols = agpl.([]interface{})
				glog.V(5).Infof(apiLogString(fmt.Sprintf("Found default global agreement protocol attribute %v", globalAgreementProtocols)))
			}
		}
	}

	// If an HA device has no HA attribute from either node wide or service wide attributes, then the configuration is invalid.
	// This verification cannot be done in the attribute verifier above because those verifiers dont know about global attributes.
	haType := reflect.TypeOf(persistence.HAAttributes{}).Name()
	if pDevice.HA && len(haPartner) == 0 {
		if attr := attributesContains(attributes, *service.Url, haType); attr == nil {
			return errorhandler(NewAPIUserInputError("services on an HA device must specify an HA partner.", "service.[attribute].type")), nil, nil
		}
	}

	// Persist all attributes on this service, and while we're at it, fetch the attribute values we need for the node side policy file.
	// Any policy attributes we find will overwrite values set in a global attribute of the same type.
	var serviceAgreementProtocols []policy.AgreementProtocol
	for _, attr := range attributes {

		_, err := persistence.SaveOrUpdateAttribute(db, attr, "", false)
		if err != nil {
			return errorhandler(NewSystemError(fmt.Sprintf("error saving attribute %v, error %v", attr, err))), nil, nil
		}

		switch attr.(type) {
		case *persistence.ComputeAttributes:
			compute := attr.(*persistence.ComputeAttributes)
			props["cpus"] = strconv.FormatInt(compute.CPUs, 10)
			props["ram"] = strconv.FormatInt(compute.RAM, 10)

		case *persistence.ArchitectureAttributes:
			// save the sensor arch to db
			if attr.(*persistence.ArchitectureAttributes).Architecture != *service.Arch {
				attr.(*persistence.ArchitectureAttributes).Architecture = *service.Arch
				_, err := persistence.SaveOrUpdateAttribute(db, attr, attr.GetMeta().Id, false)
				if err != nil {
					return errorhandler(NewSystemError(fmt.Sprintf("error saving attribute %v, error %v", attr, err))), nil, nil
				}
			}

		case *persistence.HAAttributes:
			haPartner = attr.(*persistence.HAAttributes).Partners

		case *persistence.MeteringAttributes:
			meterPolicy = policy.Meter{
				Tokens:                attr.(*persistence.MeteringAttributes).Tokens,
				PerTimeUnit:           attr.(*persistence.MeteringAttributes).PerTimeUnit,
				NotificationIntervalS: attr.(*persistence.MeteringAttributes).NotificationIntervalS,
			}

		case *persistence.CounterPartyPropertyAttributes:
			counterPartyProperties = attr.(*persistence.CounterPartyPropertyAttributes).Expression

		case *persistence.PropertyAttributes:
			properties = attr.(*persistence.PropertyAttributes).Mappings

		case *persistence.AgreementProtocolAttributes:
			agpl := attr.(*persistence.AgreementProtocolAttributes).Protocols
			serviceAgreementProtocols = agpl.([]policy.AgreementProtocol)

		default:
			glog.V(4).Infof(apiLogString(fmt.Sprintf("Unhandled attr type (%T): %v", attr, attr)))
		}
	}

	// Add the PropertyAttributes to props. There are several attribute types that contribute properties to the properties
	// section of the policy file.
	if len(properties) > 0 {
		for key, val := range properties {
			glog.V(5).Infof(apiLogString(fmt.Sprintf("Adding property %v=%v with value type %T", key, val, val)))
			props[key] = val
		}
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Complete Attr list for registration of service %v: %v", *service.Url, attributes)))

	// Establish the correct agreement protocol list. The AGP list from this service overrides any global list that might exist.
	var agpList *[]policy.AgreementProtocol
	if len(serviceAgreementProtocols) != 0 {
		agpList = &serviceAgreementProtocols
	} else if list, err := policy.ConvertToAgreementProtocolList(globalAgreementProtocols); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error converting global agreement protocol list attribute %v to agreement protocol list, error: %v", globalAgreementProtocols, err))), nil, nil
	} else {
		agpList = list
	}

	// Save the service definition in the local database.
	if err := persistence.SaveOrUpdateMicroserviceDef(db, msdef); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error saving service definition %v into db: %v", *msdef, err))), nil, nil
	}

	// Indicate that this node is service based.
	if _, err := pDevice.SetServiceBased(db); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error setting service mode on device object: %v", err))), nil, nil
	}

	// Set max number of agreements for this service's policy.
	maxAgreements := 1
	if msdef.Sharable == exchange.MS_SHARING_MODE_SINGLE || msdef.Sharable == exchange.MS_SHARING_MODE_MULTIPLE {
		if pDevice.Pattern == "" {
			maxAgreements = 2 // hard coded to 2 for now. will change to 0 later
		} else {
			maxAgreements = 0 // no limites for pattern
		}
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Create service: %v", service)))

	// Generate a policy based on all the attributes and the service definition.
	if msg, genErr := policy.GeneratePolicy(*service.Url, *service.Org, *service.Name, *service.VersionRange, *service.Arch, &props, haPartner, meterPolicy, counterPartyProperties, *agpList, maxAgreements, config.Edge.PolicyPath, pDevice.Org); genErr != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error generating policy, error: %v", genErr))), nil, nil
	} else {
		return false, service, msg
	}

}
