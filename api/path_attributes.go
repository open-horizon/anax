package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
)

func attributesContains(given []persistence.Attribute, sensorURL string, typeString string) *persistence.Attribute {
	// only returns the first match and doesn't look in the db; this is sufficient for looking at POST services, but not sufficient for supporting PUT and PATCH mechanisms

	for _, attr := range given {
		if attr.GetMeta().Type == typeString {

			if len(attr.GetMeta().SensorUrls) == 0 {
				return &attr
			}

			for _, url := range attr.GetMeta().SensorUrls {
				if sensorURL == url {
					return &attr
				}
			}
		}
	}

	return nil
}

func generateAttributeMetadata(given Attribute, typeName string) *persistence.AttributeMeta {
	var sensorUrls []string
	if given.SensorUrls == nil {
		sensorUrls = []string{}
	} else {
		sensorUrls = *given.SensorUrls
	}

	return &persistence.AttributeMeta{
		SensorUrls:  sensorUrls,
		Label:       *given.Label,
		Publishable: given.Publishable,
		HostOnly:    given.HostOnly,
		Type:        typeName,
	}
}

func parseCompute(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.ComputeAttributes, bool, error) {
	if permitEmpty {
		return nil, errorhandler(NewAPIUserInputError("partial update unsupported", "compute.mappings")), nil
	}

	var err error
	var ram int64
	r, exists := (*given.Mappings)["ram"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "compute.mappings.ram")), nil
	}
	if _, ok := r.(json.Number); !ok {
		return nil, errorhandler(NewAPIUserInputError("expected integer", "compute.mappings.ram")), nil
	} else if ram, err = r.(json.Number).Int64(); err != nil {
		return nil, errorhandler(NewAPIUserInputError("expected integer", "compute.mappings.ram")), nil
	}
	var cpus int64
	c, exists := (*given.Mappings)["cpus"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "compute.mappings.cpus")), nil
	}
	if _, ok := c.(json.Number); !ok {
		return nil, errorhandler(NewAPIUserInputError("expected integer", "compute.mappings.cpus")), nil
	} else if cpus, err = c.(json.Number).Int64(); err != nil {
		return nil, errorhandler(NewAPIUserInputError("expected integer", "compute.mappings.cpus")), nil
	}

	return &persistence.ComputeAttributes{
		Meta: generateAttributeMetadata(*given, reflect.TypeOf(persistence.ComputeAttributes{}).Name()),
		CPUs: cpus,
		RAM:  ram,
	}, false, nil
}

func parseLocation(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.LocationAttributes, bool, error) {
	if permitEmpty {
		return nil, errorhandler(NewAPIUserInputError("partial update unsupported", "location.mappings")), nil
	}
	var ok bool
	var err error

	var lat float64
	la, exists := (*given.Mappings)["lat"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "location.mappings.lat")), nil
	}
	if _, ok := la.(json.Number); !ok {
		return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("expected float but is %T", la), "location.mappings.lat")), nil
	} else if lat, err = la.(json.Number).Float64(); err != nil {
		return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("expected float but is %T", la), "location.mappings.lat")), nil
	}

	var lon float64
	lo, exists := (*given.Mappings)["lon"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "location.mappings.lon")), nil
	}
	if _, ok := lo.(json.Number); !ok {
		return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("expected float but is %T", la), "location.mappings.lon")), nil
	} else if lon, err = lo.(json.Number).Float64(); err != nil {
		return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("expected float but is %T", lo), "location.mappings.lon")), nil
	}

	var locationAccuracyKM float64
	lacc, exists := (*given.Mappings)["location_accuracy_km"]
	if exists {
		if locationAccuracyKM, err = lacc.(json.Number).Float64(); err != nil {
			return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("expected float but is %T", lacc), "location.mappings.location_accuracy_km")), nil
		}
	}

	var useGps bool
	ug, exists := (*given.Mappings)["use_gps"]
	if exists {
		if useGps, ok = ug.(bool); !ok {
			return nil, errorhandler(NewAPIUserInputError("non-boolean value", "location.mappings.use_gps")), nil
		}
	}

	return &persistence.LocationAttributes{
		Meta:               generateAttributeMetadata(*given, reflect.TypeOf(persistence.LocationAttributes{}).Name()),
		Lat:                lat,
		Lon:                lon,
		LocationAccuracyKM: locationAccuracyKM,
		UseGps:             useGps,
	}, false, nil
}

func parseUserInput(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.UserInputAttributes, bool, error) {

	if given.Mappings == nil {
		if !permitEmpty {
			return nil, errorhandler(NewAPIUserInputError("missing mappings", "mappings")), nil
		}
	}

	return &persistence.UserInputAttributes{
		Meta:     generateAttributeMetadata(*given, reflect.TypeOf(persistence.UserInputAttributes{}).Name()),
		Mappings: (*given.Mappings),
	}, false, nil
}

func parseHTTPSBasicAuth(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.HTTPSBasicAuthAttributes, bool, error) {
	var ok bool

	var username string
	us, exists := (*given.Mappings)["username"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "httpsbasic.mappings.username")), nil
	}
	if username, ok = us.(string); !ok {
		return nil, errorhandler(NewAPIUserInputError("expected string", "httpsbasic.mappings.username")), nil
	}

	var password string
	pa, exists := (*given.Mappings)["password"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "httpsbasic.mappings.password")), nil
	}
	if password, ok = pa.(string); !ok {
		return nil, errorhandler(NewAPIUserInputError("expected string", "httpsbasic.mappings.password")), nil
	}

	return &persistence.HTTPSBasicAuthAttributes{
		Meta:     generateAttributeMetadata(*given, reflect.TypeOf(persistence.HTTPSBasicAuthAttributes{}).Name()),
		Username: username,
		Password: password,
	}, false, nil
}

func parseBXDockerRegistryAuth(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.BXDockerRegistryAuthAttributes, bool, error) {
	var ok bool

	var token string
	tk, exists := (*given.Mappings)["token"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "dockerregistry.mappings.token")), nil
	}
	if token, ok = tk.(string); !ok {
		return nil, errorhandler(NewAPIUserInputError("expected string", "dockerregistry.mappings.token")), nil
	}

	return &persistence.BXDockerRegistryAuthAttributes{
		Meta:  generateAttributeMetadata(*given, reflect.TypeOf(persistence.BXDockerRegistryAuthAttributes{}).Name()),
		Token: token,
	}, false, nil
}

func parseHA(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.HAAttributes, bool, error) {
	if permitEmpty {
		return nil, errorhandler(NewAPIUserInputError("partial update unsupported", "ha.mappings")), nil
	}

	pID, exists := (*given.Mappings)["partnerID"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "ha.mappings.partnerID")), nil
	} else if partnerIDs, ok := pID.([]interface{}); !ok {
		return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("expected []interface{} received %T", pID), "ha.mappings.partnerID")), nil
	} else {
		// convert partner values to proper array type
		strPartners := make([]string, 0, 5)
		for _, val := range partnerIDs {
			p, ok := val.(string)
			if !ok {
				return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("array value is not a string, it is %T", val), "ha.mappings.partnerID")), nil
			}
			strPartners = append(strPartners, p)

		}
		return &persistence.HAAttributes{
			Meta:     generateAttributeMetadata(*given, reflect.TypeOf(persistence.HAAttributes{}).Name()),
			Partners: strPartners,
		}, false, nil
	}
}

func parseMetering(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.MeteringAttributes, bool, error) {
	if permitEmpty {
		return nil, errorhandler(NewAPIUserInputError("partial update unsupported", "metering.mappings")), nil
	}

	var err error

	// Check for valid combinations of input parameters
	t, tokensExists := (*given.Mappings)["tokens"]
	p, perTimeUnitExists := (*given.Mappings)["perTimeUnit"]
	n, notificationIntervalExists := (*given.Mappings)["notificationInterval"]

	if tokensExists && !perTimeUnitExists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "metering.mappings.perTimeUnit")), nil
	} else if !tokensExists && perTimeUnitExists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "metering.mappings.tokens")), nil
	} else if notificationIntervalExists && !tokensExists {
		return nil, errorhandler(NewAPIUserInputError("missing tokens and perTimeUnit keys", "metering.mappings.notificationInterval")), nil
	}

	// Deserialize the attribute pieces
	var ok bool
	var tokens int64
	if _, ok = t.(json.Number); !ok {
		return nil, errorhandler(NewAPIUserInputError("expected integer", "metering.mappings.tokens")), nil
	} else if tokens, err = t.(json.Number).Int64(); err != nil {
		return nil, errorhandler(NewAPIUserInputError("could not convert to integer", "metering.mappings.tokens")), nil
	}

	var perTimeUnit string
	if perTimeUnit, ok = p.(string); !ok {
		return nil, errorhandler(NewAPIUserInputError("expected string", "metering.mappings.perTimeUnit")), nil
	}

	// Make sure the attribute values make sense together
	if tokens == 0 && perTimeUnit != "" {
		return nil, errorhandler(NewAPIUserInputError("must be non-zero", "metering.mappings.tokens")), nil
	} else if tokens != 0 && perTimeUnit == "" {
		return nil, errorhandler(NewAPIUserInputError("must be non-empty", "metering.mappings.perTimeUnit")), nil
	}

	// Deserialize and validate the last piece of the attribute
	var notificationInterval int64

	if _, ok = n.(json.Number); !ok {
		return nil, errorhandler(NewAPIUserInputError("expected integer", "metering.mappings.notificationInterval")), nil
	} else if notificationInterval, err = n.(json.Number).Int64(); err != nil {
		return nil, errorhandler(NewAPIUserInputError("could not convert to integer", "metering.mappings.notificationInterval")), nil
	}

	if notificationInterval != 0 && tokens == 0 {
		return nil, errorhandler(NewAPIUserInputError("cannot be non-zero without tokens and perTimeUnit", "metering.mappings.notificationInterval")), nil
	}

	return &persistence.MeteringAttributes{
		Meta:                  generateAttributeMetadata(*given, reflect.TypeOf(persistence.MeteringAttributes{}).Name()),
		Tokens:                uint64(tokens),
		PerTimeUnit:           perTimeUnit,
		NotificationIntervalS: int(notificationInterval),
	}, false, nil
}

func parseProperty(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.PropertyAttributes, bool, error) {
	if permitEmpty {
		return nil, errorhandler(NewAPIUserInputError("partial update unsupported", "property.mappings")), nil
	}

	return &persistence.PropertyAttributes{
		Meta:     generateAttributeMetadata(*given, reflect.TypeOf(persistence.PropertyAttributes{}).Name()),
		Mappings: (*given.Mappings)}, false, nil
}

func parseCounterPartyProperty(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.CounterPartyPropertyAttributes, bool, error) {
	if permitEmpty {
		return nil, errorhandler(NewAPIUserInputError("partial update unsupported", "counterpartyproperty.mappings")), nil
	}

	rawExpression, exists := (*given.Mappings)["expression"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "counterpartyproperty.mappings.expression")), nil
	}

	if exp, ok := rawExpression.(map[string]interface{}); !ok {
		return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("expected map[string]interface{}, is %T", rawExpression), "counterpartyproperty.mappings.expression")), nil
	} else if rp := policy.RequiredProperty_Factory(); rp == nil {
		return nil, errorhandler(NewAPIUserInputError("could not construct RequiredProperty", "counterpartyproperty.mappings.expression")), nil
	} else if err := rp.Initialize(&exp); err != nil {
		return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("could not initialize RequiredProperty: %v", err), "counterpartyproperty.mappings.expression")), nil
	} else if err := rp.IsValid(); err != nil {
		return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("not a valid expression: %v", err), "counterpartyproperty.mappings.expression")), nil
	} else {
		return &persistence.CounterPartyPropertyAttributes{
			Meta:       generateAttributeMetadata(*given, reflect.TypeOf(persistence.CounterPartyPropertyAttributes{}).Name()),
			Expression: rawExpression.(map[string]interface{}),
		}, false, nil
	}
}

func parseAgreementProtocol(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.AgreementProtocolAttributes, bool, error) {
	if permitEmpty {
		return nil, errorhandler(NewAPIUserInputError("partial update unsupported", "agreementprotocol.mappings")), nil
	}

	p, exists := (*given.Mappings)["protocols"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "agreementprotocol.mappings.protocols")), nil
	} else if protocols, ok := p.([]interface{}); !ok {
		return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("expected []interface{} received %T", p), "agreementprotocol.mappings.protocols")), nil
	} else {
		// convert protocol values to proper agreement protocol object
		allProtocols := make([]policy.AgreementProtocol, 0, 5)
		for _, val := range protocols {
			protoDef, ok := val.(map[string]interface{})
			if !ok {
				return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("array value is not a map[string]interface{}, it is %T", val), "agreementprotocol.mappings.protocols")), nil
			}

			for protocolName, bcValue := range protoDef {
				if !policy.SupportedAgreementProtocol(protocolName) {
					return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("protocol name %v is not supported", protocolName), "agreementprotocol.mappings.protocols.protocolName")), nil
				} else if bcDefArray, ok := bcValue.([]interface{}); !ok {
					return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("blockchain value is not []interface{}, it is %T", bcValue), "agreementprotocol.mappings.protocols.blockchain")), nil
				} else {
					agp := policy.AgreementProtocol_Factory(protocolName)
					for _, bcEle := range bcDefArray {
						if bcDef, ok := bcEle.(map[string]interface{}); !ok {
							return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("blockchain array element is not map[string]interface{}, it is %T", bcEle), "agreementprotocol.mappings.protocols.blockchain")), nil
						} else if _, ok := bcDef["type"].(string); bcDef["type"] != nil && !ok {
							return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("blockchain type is not string, it is %T", bcDef["type"]), "agreementprotocol.mappings.protocols.blockchain.type")), nil
						} else if _, ok := bcDef["name"].(string); bcDef["name"] != nil && !ok {
							return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("blockchain name is not string, it is %T", bcDef["name"]), "agreementprotocol.mappings.protocols.blockchain.name")), nil
						} else if bcDef["type"] != nil && bcDef["type"].(string) != "" && bcDef["type"].(string) != policy.RequiresBlockchainType(protocolName) {
							return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("blockchain type %v is not supported for protocol %v", bcDef["type"].(string), protocolName), "agreementprotocol.mappings.protocols.blockchain.type")), nil
						} else {
							bcType := ""
							if bcDef["type"] != nil {
								bcType = bcDef["type"].(string)
							}
							bcName := ""
							if bcDef["name"] != nil {
								bcName = bcDef["name"].(string)
							}
							bcOrg := ""
							if bcDef["organization"] != nil {
								bcOrg = bcDef["organization"].(string)
							}
							(&agp.Blockchains).Add_Blockchain(policy.Blockchain_Factory(bcType, bcName, bcOrg))
						}
					}
					agp.Initialize()
					allProtocols = append(allProtocols, *agp)
				}
			}
		}
		if len(allProtocols) == 0 {
			return nil, errorhandler(NewAPIUserInputError("array value is empty", "agreementprotocol.mappings.protocols")), nil
		}

		return &persistence.AgreementProtocolAttributes{
			Meta:      generateAttributeMetadata(*given, reflect.TypeOf(persistence.AgreementProtocolAttributes{}).Name()),
			Protocols: allProtocols,
		}, false, nil
	}
}

// AttributeVerifier returns true if there is a handled inputError (one that caused a write to the http responsewriter) and error if there is a system processing problem
type AttributeVerifier func(attr persistence.Attribute) (bool, error)

func toPersistedAttributesAttachedToService(errorhandler ErrorHandler, persistedDevice *persistence.ExchangeDevice, defaultRAM int64, attrs []Attribute, sensorURL string, additionalVerifiers []AttributeVerifier) ([]persistence.Attribute, bool, error) {

	additionalVerifiers = append(additionalVerifiers, func(attr persistence.Attribute) (bool, error) {
		// can't specify sensorURLs in attributes that are a part of a service
		sensorURLs := attr.GetMeta().SensorUrls
		if sensorURLs != nil {
			if len(sensorURLs) > 1 || (len(sensorURLs) == 1 && sensorURLs[0] != sensorURL) {
				return errorhandler(NewAPIUserInputError("sensor_urls not permitted on attributes specified on a service", "service.[attribute].sensor_urls")), nil
			}
		}

		return false, nil
	})

	persistenceAttrs, inputErr, err := toPersistedAttributes(errorhandler, false, persistedDevice, attrs, additionalVerifiers)
	if inputErr || err != nil {
		return persistenceAttrs, inputErr, err
	}

	persistenceAttrs = FinalizeAttributesSpecifiedInService(defaultRAM, sensorURL, persistenceAttrs)

	return persistenceAttrs, inputErr, err
}

func ValidateAndConvertAPIAttribute(errorhandler ErrorHandler, permitEmpty bool, given Attribute) (persistence.Attribute, bool, error) {
	var attribute persistence.Attribute

	// ----------------------

	if permitEmpty && given.Label == nil {
		glog.V(4).Infof(apiLogString(fmt.Sprintf("Allowing unspecified label in partial update of %v", given)))
	} else if bail := checkInputString(errorhandler, "label", given.Label); bail {
		return nil, true, nil
	}

	if given.Publishable == nil {
		if permitEmpty {
			glog.V(4).Infof(apiLogString(fmt.Sprintf("Allowing unspecified publishable flag in partial update of %v", given)))
		} else {
			return nil, errorhandler(NewAPIUserInputError("nil value", "publishable")), nil
		}
	}

	// always ok if this one is nil
	if given.SensorUrls != nil {
		for _, url := range *given.SensorUrls {
			if bail := checkInputString(errorhandler, "sensorurl", &url); bail {
				return nil, true, nil
			}
		}
	}

	if given.Mappings == nil {
		if permitEmpty {
			glog.V(4).Infof(apiLogString(fmt.Sprintf("Allowing unspecified mappings in partial update of %v", given)))
		} else {
			return nil, errorhandler(NewAPIUserInputError("nil value", "mappings")), nil
		}
	} else {

		// check each mapping
		if value, inputErr, err := MapInputIsIllegal(*given.Mappings); err != nil {
			return nil, true, fmt.Errorf("Failed to check input: %v", err)
		} else if inputErr != "" {
			return nil, errorhandler(NewAPIUserInputError(inputErr, fmt.Sprintf("mappings.%v", value))), nil
		}
	}

	if given.Type == nil && permitEmpty {
		return nil, errorhandler(NewAPIUserInputError("partial update with missing type is not supported", "type")), nil
	} else if bail := checkInputString(errorhandler, "type", given.Type); bail {
		return nil, true, nil
	} else {

		// attribute meta is good, deserialize (except architecture, we add our own for that)
		switch *given.Type {

		case reflect.TypeOf(persistence.ComputeAttributes{}).Name():
			attr, inputErr, err := parseCompute(errorhandler, permitEmpty, &given)
			if err != nil || inputErr {
				return nil, inputErr, err
			}
			attribute = attr

		case reflect.TypeOf(persistence.LocationAttributes{}).Name():
			attr, inputErr, err := parseLocation(errorhandler, permitEmpty, &given)
			if err != nil || inputErr {
				return nil, inputErr, err
			}
			attribute = attr

		case reflect.TypeOf(persistence.UserInputAttributes{}).Name():
			attr, inputErr, err := parseUserInput(errorhandler, permitEmpty, &given)
			if err != nil || inputErr {
				return nil, inputErr, err
			}
			attribute = attr

		case reflect.TypeOf(persistence.HAAttributes{}).Name():
			attr, inputErr, err := parseHA(errorhandler, permitEmpty, &given)
			if err != nil || inputErr {
				return nil, inputErr, err
			}
			attribute = attr

		case reflect.TypeOf(persistence.MeteringAttributes{}).Name():
			attr, inputErr, err := parseMetering(errorhandler, permitEmpty, &given)
			if err != nil || inputErr {
				return nil, inputErr, err
			}
			attribute = attr

		case reflect.TypeOf(persistence.PropertyAttributes{}).Name():
			attr, inputErr, err := parseProperty(errorhandler, permitEmpty, &given)
			if err != nil || inputErr {
				return attribute, inputErr, err
			}
			attribute = attr

		case reflect.TypeOf(persistence.CounterPartyPropertyAttributes{}).Name():
			attr, inputErr, err := parseCounterPartyProperty(errorhandler, permitEmpty, &given)
			if err != nil || inputErr {
				return attribute, inputErr, err
			}
			attribute = attr

		case reflect.TypeOf(persistence.AgreementProtocolAttributes{}).Name():
			attr, inputErr, err := parseAgreementProtocol(errorhandler, permitEmpty, &given)
			if err != nil || inputErr {
				return attribute, inputErr, err
			}
			attribute = attr

		case reflect.TypeOf(persistence.HTTPSBasicAuthAttributes{}).Name():
			attr, inputErr, err := parseHTTPSBasicAuth(errorhandler, permitEmpty, &given)
			if err != nil || inputErr {
				return attribute, inputErr, err
			}
			attribute = attr

		case reflect.TypeOf(persistence.BXDockerRegistryAuthAttributes{}).Name():
			attr, inputErr, err := parseBXDockerRegistryAuth(errorhandler, permitEmpty, &given)
			if err != nil || inputErr {
				return attribute, inputErr, err
			}
			attribute = attr

		default:
			return nil, errorhandler(NewAPIUserInputError("Unmappable type field", "mappings")), nil
		}
	}
	return attribute, false, nil
}

func toPersistedAttributes(errorhandler ErrorHandler, permitEmpty bool, persistedDevice *persistence.ExchangeDevice, attrs []Attribute, additionalVerifiers []AttributeVerifier) ([]persistence.Attribute, bool, error) {

	attributes := []persistence.Attribute{}

	for _, given := range attrs {
		attr, errorHandled, err := ValidateAndConvertAPIAttribute(errorhandler, permitEmpty, given)
		if errorHandled || err != nil {
			return nil, errorHandled, err
		}
		attributes = append(attributes, attr)
	}

	// do validation on concrete types (make sure conflicting options aren't specified, etc.)
	if inputErr, err := validateConcreteAttributes(errorhandler, persistedDevice, attributes, additionalVerifiers); err != nil || inputErr {
		return nil, inputErr, err
	}

	return attributes, false, nil
}

func toOutModel(persisted persistence.Attribute) *Attribute {
	mappings := persisted.GetGenericMappings()

	return &Attribute{
		Id:          &persisted.GetMeta().Id,
		SensorUrls:  &persisted.GetMeta().SensorUrls,
		Label:       &persisted.GetMeta().Label,
		Publishable: persisted.GetMeta().Publishable,
		HostOnly:    persisted.GetMeta().HostOnly,
		Type:        &persisted.GetMeta().Type,
		Mappings:    &mappings,
	}
}

func FinalizeAttributesSpecifiedInService(defaultRAM int64, sensorURL string, attributes []persistence.Attribute) []persistence.Attribute {

	// check for required
	cType := reflect.TypeOf(persistence.ComputeAttributes{}).Name()
	if attributesContains(attributes, sensorURL, cType) == nil {
		computePub := true

		attributes = append(attributes, &persistence.ComputeAttributes{
			Meta: &persistence.AttributeMeta{
				Id:          "compute",
				SensorUrls:  []string{sensorURL},
				Label:       "Compute Resources",
				Publishable: &computePub,
				Type:        cType,
			},
			CPUs: 1,
			RAM:  defaultRAM,
		})
	}

	aType := reflect.TypeOf(persistence.ArchitectureAttributes{}).Name()
	// a little weird; could a user give us an alternate architecture than the one we're going to publising in the prop?
	if attributesContains(attributes, sensorURL, aType) == nil {
		// make a default

		archPub := true
		attributes = append(attributes, &persistence.ArchitectureAttributes{
			Meta: &persistence.AttributeMeta{
				Id:          "architecture",
				SensorUrls:  []string{sensorURL},
				Label:       "Architecture",
				Publishable: &(archPub),
				Type:        aType,
			},
			Architecture: cutil.ArchString(),
		})
	}

	for _, attr := range attributes {
		attr.GetMeta().AppendSensorUrl(sensorURL)
		glog.V(3).Infof(apiLogString(fmt.Sprintf("SensorUrls for %v: %v", attr.GetMeta().Id, attr.GetMeta().SensorUrls)))
	}

	// return updated
	return attributes
}

func validateConcreteAttributes(errorhandler ErrorHandler, persistedDevice *persistence.ExchangeDevice, attributes []persistence.Attribute, additionalVerifiers []AttributeVerifier) (bool, error) {

	// check for errors in attribute input, like specifying a sensorUrl or specifying HA Partner on a non-HA device
	for _, attr := range attributes {
		for _, verifier := range additionalVerifiers {
			if inputErr, err := verifier(attr); inputErr || err != nil {
				return inputErr, err
			}
		}

		if attr.GetMeta().Type == reflect.TypeOf(persistence.HAAttributes{}).Name() {
			// if the device is not HA enabled then the HA partner attribute is illegal
			if !persistedDevice.HA {
				return errorhandler(NewAPIUserInputError("HA partner not permitted on non-HA devices", "service.[attribute].type")), nil
			}

			// Make sure that a device doesn't specify itself in the HA partner list
			if _, ok := attr.GetGenericMappings()["partnerID"]; ok {
				switch attr.GetGenericMappings()["partnerID"].(type) {
				case []string:
					partners := attr.GetGenericMappings()["partnerID"].([]string)
					for _, partner := range partners {
						if partner == persistedDevice.Id {
							return errorhandler(NewAPIUserInputError("partner list cannot refer to itself.", "service.[attribute].mappings.partnerID")), nil
						}
					}
				}
			}
		}
	}

	return false, nil
}

func payloadToAttributes(errorhandler ErrorHandler, body io.Reader, permitPartial bool, existingDevice *persistence.ExchangeDevice) ([]persistence.Attribute, bool, error) {

	by, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, false, fmt.Errorf("Failed to read request bytes: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(by))
	decoder.UseNumber()

	var attribute Attribute
	if err := decoder.Decode(&attribute); err != nil {
		return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("attribute could not be demarshalled, error: %v", err), "attribute")), err
	}
	glog.V(6).Infof(apiLogString(fmt.Sprintf("Decoded Attribute from payload: %v", attribute)))

	// N.B. remove the id from the input doc; it won't be checked and it shouldn't be trusted, prefer the path param id instead
	attribute.Id = nil

	// we allow the user to send partial updates that leave out some members of an attribute
	if permitPartial && attribute.Mappings == nil {
		attribute.Mappings = new(map[string]interface{})
	}

	return toPersistedAttributes(errorhandler, permitPartial, existingDevice, []Attribute{attribute}, []AttributeVerifier{})
}

// serializeAttributeForOutput retrieves attributes by url from the DB and then
// serializes then as JSON, returning a byte array for convenient writing to an
// HTTP response.
func FindAndWrapAttributesForOutput(db *bolt.DB, id string) (map[string][]Attribute, error) {

	attributes, err := persistence.FindApplicableAttributes(db, "")
	if err != nil {
		return nil, fmt.Errorf("Failed fetching existing service attributes. Error: %v", err)
	}

	return wrapAttributesForOutput(attributes, id), nil
}

func wrapAttributesForOutput(attributes []persistence.Attribute, id string) map[string][]Attribute {

	outAttrs := []Attribute{}
	for _, persisted := range attributes {
		// convert persistence model to API model

		if id == "" || persisted.GetMeta().Id == id {
			outAttr := toOutModel(persisted)
			outAttrs = append(outAttrs, *outAttr)
		}
	}

	wrap := map[string][]Attribute{}
	wrap["attributes"] = outAttrs

	return wrap
}
