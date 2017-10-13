package api

import (
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
)

// Given a demarshalled Service object, validate it and save it, returning any errors.
func CreateService(service *Service,
	errorhandler ErrorHandler,
	getMicroservice MicroserviceHandler,
	db *bolt.DB,
	config *config.HorizonConfig) (bool, *Service, *events.PolicyCreatedMessage) {

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
	if bail := checkInputString(errorhandler, "service.sensor_url", service.SensorUrl); bail {
		return true, nil, nil
	}

	// Use the device's org if org not specified in the service object.
	if service.SensorOrg == nil {
		service.SensorOrg = &pDevice.Org
	} else if bail := checkInputString(errorhandler, "service.sensor_org", service.SensorOrg); bail {
		return true, nil, nil
	}

	if bail := checkInputString(errorhandler, "service.sensor_name", service.SensorName); bail {
		return true, nil, nil
	}

	// The sensor_version field is checked for valid characters by the Version_Expression_Factory, it has a very
	// specific syntax and allows a subset of normally valid characters.

	// Use a default sensor version that allows all version if not specified.
	if service.SensorVersion == nil {
		def := "0.0.0"
		service.SensorVersion = &def
	}

	// Convert the sensor version to a version expression.
	vExp, err := policy.Version_Expression_Factory(*service.SensorVersion)
	if err != nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("sensor_version %v cannot be converted to a version expression, error %v", *service.SensorVersion, err), "service.sensor_version")), nil, nil
	}

	// Verify with the exchange to make sure the microservice definition is readable by this node.
	var msdef *persistence.MicroserviceDefinition
	e_msdef, err := getMicroservice(*service.SensorUrl, *service.SensorOrg, vExp.Get_expression(), cutil.ArchString(), pDevice.GetId(), pDevice.Token)
	if err != nil || e_msdef == nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("Unable to find the microservice definition for '%v' on the exchange. Please verify sensor_url and sensor_version.", *service.SensorName), "service")), nil, nil
	}

	// Convert the microservice definition to a persistent format so that it can be saved to the db.
	msdef, err = microservice.ConvertToPersistent(e_msdef, *service.SensorOrg)
	if err != nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("Error converting the microservice metadata to persistent.MicroserviceDefinition for %v version %v, error %v", e_msdef.SpecRef, e_msdef.Version, err), "service")), nil, nil
	}

	// Save some of the items in the MicroserviceDefinition object for use in the upgrading process.
	msdef.Name = *service.SensorName
	msdef.UpgradeVersionRange = *service.SensorVersion
	if service.AutoUpgrade != nil {
		msdef.AutoUpgrade = *service.AutoUpgrade
	}
	if service.ActiveUpgrade != nil {
		msdef.ActiveUpgrade = *service.ActiveUpgrade
	}

	// The microservice definition returned by the exchange might be newer than what was specified in the input service object, so we save
	// the actual version of the microservice so that we know if we need to upgrade in the future.
	service.SensorVersion = &msdef.Version

	// Check if the microservice has been registered or not (currently only support one microservice registration)
	if pms, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{persistence.UrlMSFilter(*service.SensorUrl)}); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error accessing db to find microservice definition: %v", err))), nil, nil
	} else if pms != nil && len(pms) > 0 {
		//return errorhandler(NewAPIUserInputError(fmt.Sprintf("Duplicate registration for %v. Only one registration per microservice is supported.", *service.SensorUrl), "service")), nil, nil
		return errorhandler(NewDuplicateServiceError(fmt.Sprintf("Duplicate registration for %v. Only one registration per microservice is supported.", *service.SensorUrl), "service")), nil, nil
	}

	// If there are no attributes associated with this request but the MS requires some configuration, return an error.
	if service.Attributes == nil || (service.Attributes != nil && len(*service.Attributes) == 0) {
		if varname := msdef.NeedsUserInput(); varname != "" {
			return errorhandler(NewMSMissingVariableConfigError(fmt.Sprintf("variable %v is missing from mappings", varname), "service.[attribute].mapped")), nil, nil
		}
	}

	// Validate any attributes specified in the attribute list and convert them to persistent objects.
	// This attribute verifier makes sure that there is a mapped attribute which specifies values for all the non-default
	// user inputs in the specific microservice selected earlier.
	msdefAttributeVerifier := func(attr persistence.Attribute) (bool, error) {

		// Verfiy that all non-defaulted userInput variables in the microservice definition are specified in a mapped property attribute
		// of this service invocation.
		if msdef != nil && attr.GetMeta().Type == "MappedAttributes" {
			for _, ui := range msdef.UserInputs {
				if ui.DefaultValue != "" {
					continue
				} else if _, ok := attr.GetGenericMappings()[ui.Name]; !ok {
					// return errorhandler(NewAPIUserInputError(fmt.Sprintf("variable %v is missing from mappings", ui.Name), "service.[attribute].mapped")), nil
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

		attributes, inputErrWritten, err = toPersistedAttributesAttachedToService(errorhandler, pDevice, config.Edge.DefaultServiceRegistrationRAM, *service.Attributes, *service.SensorUrl, []AttributeVerifier{msdefAttributeVerifier, patternedDeviceAttributeVerifier})
		if !inputErrWritten && err != nil {
			return errorhandler(NewSystemError(fmt.Sprintf("Failure deserializing attributes: %v", err))), nil, nil
		} else if inputErrWritten {
			return true, nil, nil
		}
	}

	// Information advertised in the edge node policy file
	var policyArch string
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
	if pDevice.HADevice && len(haPartner) == 0 {
		if attr := attributesContains(attributes, *service.SensorUrl, haType); attr == nil {
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
			policyArch = attr.(*persistence.ArchitectureAttributes).Architecture

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
			glog.V(4).Infof("Unhandled attr type (%T): %v", attr, attr)
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

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Complete Attr list for registration of service %v: %v", *service.SensorUrl, attributes)))

	// Establish the correct agreement protocol list. The AGP list from this service overrides any global list that might exist.
	var agpList *[]policy.AgreementProtocol
	if len(serviceAgreementProtocols) != 0 {
		agpList = &serviceAgreementProtocols
	} else if list, err := policy.ConvertToAgreementProtocolList(globalAgreementProtocols); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error converting global agreement protocol list attribute %v to agreement protocol list, error: %v", globalAgreementProtocols, err))), nil, nil
	} else {
		agpList = list
	}

	// Save the microservice definition in the local database.
	if err := persistence.SaveOrUpdateMicroserviceDef(db, msdef); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error saving microservice definition %v into db: %v", *msdef, err))), nil, nil
	}

	// Set max number of agreements for this microservice's policy.
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
	if msg, genErr := policy.GeneratePolicy(*service.SensorUrl, *service.SensorOrg, *service.SensorName, *service.SensorVersion, policyArch, &props, haPartner, meterPolicy, counterPartyProperties, *agpList, maxAgreements, config.Edge.PolicyPath, pDevice.Org); genErr != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error generating policy, error: %v", genErr))), nil, nil
	} else {
		return false, service, msg
	}

}
