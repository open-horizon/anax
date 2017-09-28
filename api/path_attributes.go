package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
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
		Type:        typeName,
	}
}

func parseCompute(w http.ResponseWriter, permitEmpty bool, given *Attribute) (*persistence.ComputeAttributes, bool, error) {
	if permitEmpty {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "compute.mappings", Error: "partial update unsupported"})
	}

	var err error
	var ram int64
	r, exists := (*given.Mappings)["ram"]
	if !exists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "compute.mappings.ram", Error: "missing key"})
		return nil, true, nil
	}
	if ram, err = r.(json.Number).Int64(); err != nil {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "compute.mappings.ram", Error: "expected integer"})
		return nil, true, nil
	}
	var cpus int64
	c, exists := (*given.Mappings)["cpus"]
	if !exists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "compute.mappings.cpus", Error: "missing key"})
		return nil, true, nil
	}
	if cpus, err = c.(json.Number).Int64(); err != nil {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "compute.mappings.cpus", Error: "expected integer"})
		return nil, true, nil
	}

	return &persistence.ComputeAttributes{
		Meta: generateAttributeMetadata(*given, reflect.TypeOf(persistence.ComputeAttributes{}).Name()),
		CPUs: cpus,
		RAM:  ram,
	}, false, nil
}

func parseLocation(w http.ResponseWriter, permitEmpty bool, given *Attribute) (*persistence.LocationAttributes, bool, error) {
	if permitEmpty {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings", Error: "partial update unsupported"})
		return nil, true, nil
	}
	var ok bool

	var lat string
	la, exists := (*given.Mappings)["lat"]
	if !exists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.lat", Error: "missing key"})
		return nil, true, nil
	}
	if lat, ok = la.(string); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.lat", Error: "expected string"})
		return nil, true, nil
	}
	var lon string
	lo, exists := (*given.Mappings)["lon"]
	if !exists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.lon", Error: "missing key"})
		return nil, true, nil
	}
	if lon, ok = lo.(string); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.lon", Error: "expected string"})
		return nil, true, nil
	}

	var userProvidedCoords bool
	up, exists := (*given.Mappings)["user_provided_coords"]
	if !exists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.user_provided_coords", Error: "missing key"})
		return nil, true, nil
	} else if userProvidedCoords, ok = up.(bool); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.user_provided_coords", Error: "non-boolean value"})
		return nil, true, nil
	}
	var useGps bool
	ug, exists := (*given.Mappings)["use_gps"]
	if !exists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.use_gps", Error: "missing key"})
		return nil, true, nil
	} else if useGps, ok = ug.(bool); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.use_gps", Error: "non-boolean value"})
		return nil, true, nil
	}

	return &persistence.LocationAttributes{
		Meta:               generateAttributeMetadata(*given, reflect.TypeOf(persistence.LocationAttributes{}).Name()),
		Lat:                lat,
		Lon:                lon,
		UserProvidedCoords: userProvidedCoords,
		UseGps:             useGps,
	}, false, nil
}

func parseMapped(w http.ResponseWriter, permitEmpty bool, given *Attribute) (*persistence.MappedAttributes, bool, error) {
	// convert all to string representations
	mappedStr := map[string]string{}

	if given.Mappings == nil {
		if !permitEmpty {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "mappings", Error: "missing mappings"})
			return nil, true, nil
		}
	} else {
		for k, v := range *given.Mappings {
			mappedStr[k] = fmt.Sprintf("%v", v)
		}
	}

	return &persistence.MappedAttributes{
		Meta:     generateAttributeMetadata(*given, reflect.TypeOf(persistence.MappedAttributes{}).Name()),
		Mappings: mappedStr,
	}, false, nil
}

func parseHTTPSBasicAuth(w http.ResponseWriter, permitEmpty bool, given *Attribute) (*persistence.HTTPSBasicAuthAttributes, bool, error) {
	var ok bool

	var username string
	us, exists := (*given.Mappings)["username"]
	if !exists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "httpsbasic.mappings.username", Error: "missing key"})
		return nil, true, nil
	}
	if username, ok = us.(string); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "httpsbasic.mappings.username", Error: "expected string"})
		return nil, true, nil
	}

	var password string
	pa, exists := (*given.Mappings)["password"]
	if !exists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "httpsbasic.mappings.password", Error: "missing key"})
		return nil, true, nil
	}
	if password, ok = pa.(string); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "httpsbasic.mappings.password", Error: "expected string"})
		return nil, true, nil
	}

	return &persistence.HTTPSBasicAuthAttributes{
		Meta:     generateAttributeMetadata(*given, reflect.TypeOf(persistence.HTTPSBasicAuthAttributes{}).Name()),
		Username: username,
		Password: password,
	}, false, nil
}

func parseHA(w http.ResponseWriter, permitEmpty bool, given *Attribute) (*persistence.HAAttributes, bool, error) {
	if permitEmpty {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "ha.mappings", Error: "partial update unsupported"})
		return nil, true, nil
	}

	pID, exists := (*given.Mappings)["partnerID"]
	if !exists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "ha.mappings.partnerID", Error: "missing key"})
		return nil, true, nil
	} else if partnerIDs, ok := pID.([]interface{}); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "ha.mappings.partnerID", Error: fmt.Sprintf("expected []interface{} received %T", pID)})
		return nil, true, nil
	} else {
		// convert partner values to proper array type
		strPartners := make([]string, 0, 5)
		for _, val := range partnerIDs {
			p, ok := val.(string)
			if !ok {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "ha.mappings.partnerID", Error: fmt.Sprintf("array value is not a string, it is %T", val)})
				return nil, true, nil
			}
			strPartners = append(strPartners, p)

		}
		return &persistence.HAAttributes{
			Meta:     generateAttributeMetadata(*given, reflect.TypeOf(persistence.HAAttributes{}).Name()),
			Partners: strPartners,
		}, false, nil
	}
}

func parseMetering(w http.ResponseWriter, permitEmpty bool, given *Attribute) (*persistence.MeteringAttributes, bool, error) {
	if permitEmpty {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings", Error: "partial update unsupported"})
		return nil, true, nil
	}

	var err error

	// Check for valid combinations of input parameters
	t, tokensExists := (*given.Mappings)["tokens"]
	p, perTimeUnitExists := (*given.Mappings)["perTimeUnit"]
	n, notificationIntervalExists := (*given.Mappings)["notificationInterval"]

	if tokensExists && !perTimeUnitExists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.perTimeUnit", Error: "missing key"})
		return nil, true, nil
	} else if !tokensExists && perTimeUnitExists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.tokens", Error: "missing key"})
		return nil, true, nil
	} else if notificationIntervalExists && !tokensExists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.notificationInterval", Error: "missing tokens and perTimeUnit keys"})
		return nil, true, nil
	}

	// Deserialize the attribute pieces
	var ok bool
	var tokens int64
	if _, ok = t.(json.Number); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.tokens", Error: "expected integer"})
		return nil, true, nil
	} else if tokens, err = t.(json.Number).Int64(); err != nil {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.tokens", Error: "could not convert to integer"})
		return nil, true, nil
	}

	var perTimeUnit string
	if perTimeUnit, ok = p.(string); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.perTimeUnit", Error: "expected string"})
		return nil, true, nil
	}

	// Make sure the attribute values make sense together
	if tokens == 0 && perTimeUnit != "" {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.tokens", Error: "must be non-zero"})
		return nil, true, nil
	} else if tokens != 0 && perTimeUnit == "" {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.perTimeUnit", Error: "must be non-empty"})
		return nil, true, nil
	}

	// Deserialize and validate the last piece of the attribute
	var notificationInterval int64

	if _, ok = n.(json.Number); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.notificationInterval", Error: "expected integer"})
		return nil, true, nil
	} else if notificationInterval, err = n.(json.Number).Int64(); err != nil {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.notificationInterval", Error: "could not convert to integer"})
		return nil, true, nil
	}

	if notificationInterval != 0 && tokens == 0 {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.notificationInterval", Error: "cannot be non-zero without tokens and perTimeUnit"})
		return nil, true, nil
	}

	return &persistence.MeteringAttributes{
		Meta:                  generateAttributeMetadata(*given, reflect.TypeOf(persistence.MeteringAttributes{}).Name()),
		Tokens:                uint64(tokens),
		PerTimeUnit:           perTimeUnit,
		NotificationIntervalS: int(notificationInterval),
	}, false, nil
}

func parseProperty(w http.ResponseWriter, permitEmpty bool, given *Attribute) (*persistence.PropertyAttributes, bool, error) {
	if permitEmpty {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "property.mappings", Error: "partial update unsupported"})
		return nil, true, nil
	}

	return &persistence.PropertyAttributes{
		Meta:     generateAttributeMetadata(*given, reflect.TypeOf(persistence.PropertyAttributes{}).Name()),
		Mappings: (*given.Mappings)}, false, nil
}

func parseCounterPartyProperty(w http.ResponseWriter, permitEmpty bool, given *Attribute) (*persistence.CounterPartyPropertyAttributes, bool, error) {
	if permitEmpty {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "counterpartyproperty.mappings", Error: "partial update unsupported"})
		return nil, true, nil
	}

	rawExpression, exists := (*given.Mappings)["expression"]
	if !exists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "counterpartyproperty.mappings.expression", Error: "missing key"})
		return nil, true, nil
	}

	if exp, ok := rawExpression.(map[string]interface{}); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "counterpartyproperty.mappings.expression", Error: fmt.Sprintf("expected map[string]interface{}, is %T", rawExpression)})
		return nil, true, nil
	} else if rp := policy.RequiredProperty_Factory(); rp == nil {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "counterpartyproperty.mappings.expression", Error: "could not construct RequiredProperty"})
		return nil, true, nil
	} else if err := rp.Initialize(&exp); err != nil {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "counterpartyproperty.mappings.expression", Error: fmt.Sprintf("could not initialize RequiredProperty: %v", err)})
		return nil, true, nil
	} else if err := rp.IsValid(); err != nil {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "counterpartyproperty.mappings.expression", Error: fmt.Sprintf("not a valid expression: %v", err)})
		return nil, true, nil
	} else {
		return &persistence.CounterPartyPropertyAttributes{
			Meta:       generateAttributeMetadata(*given, reflect.TypeOf(persistence.CounterPartyPropertyAttributes{}).Name()),
			Expression: rawExpression.(map[string]interface{}),
		}, false, nil
	}
}

func parseAgreementProtocol(w http.ResponseWriter, permitEmpty bool, given *Attribute) (*persistence.AgreementProtocolAttributes, bool, error) {
	if permitEmpty {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings", Error: "partial update unsupported"})
		return nil, true, nil
	}

	p, exists := (*given.Mappings)["protocols"]
	if !exists {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols", Error: "missing key"})
		return nil, true, nil
	} else if protocols, ok := p.([]interface{}); !ok {
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols", Error: fmt.Sprintf("expected []interface{} received %T", p)})
		return nil, true, nil
	} else {
		// convert protocol values to proper agreement protocol object
		allProtocols := make([]policy.AgreementProtocol, 0, 5)
		for _, val := range protocols {
			protoDef, ok := val.(map[string]interface{})
			if !ok {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols", Error: fmt.Sprintf("array value is not a map[string]interface{}, it is %T", val)})
				return nil, true, nil
			}

			for protocolName, bcValue := range protoDef {
				if !policy.SupportedAgreementProtocol(protocolName) {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.protocolName", Error: fmt.Sprintf("protocol name %v is not supported", protocolName)})
					return nil, true, nil
				} else if bcDefArray, ok := bcValue.([]interface{}); !ok {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.blockchain", Error: fmt.Sprintf("blockchain value is not []interface{}, it is %T", bcValue)})
					return nil, true, nil
				} else {
					agp := policy.AgreementProtocol_Factory(protocolName)
					for _, bcEle := range bcDefArray {
						if bcDef, ok := bcEle.(map[string]interface{}); !ok {
							writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.blockchain", Error: fmt.Sprintf("blockchain array element is not map[string]interface{}, it is %T", bcEle)})
							return nil, true, nil
						} else if _, ok := bcDef["type"].(string); bcDef["type"] != nil && !ok {
							writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.blockchain.type", Error: fmt.Sprintf("blockchain type is not string, it is %T", bcDef["type"])})
							return nil, true, nil
						} else if _, ok := bcDef["name"].(string); bcDef["name"] != nil && !ok {
							writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.blockchain.name", Error: fmt.Sprintf("blockchain name is not string, it is %T", bcDef["name"])})
							return nil, true, nil
						} else if bcDef["type"] != nil && bcDef["type"].(string) != "" && bcDef["type"].(string) != policy.RequiresBlockchainType(protocolName) {
							writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.blockchain.type", Error: fmt.Sprintf("blockchain type %v is not supported for protocol %v", bcDef["type"].(string), protocolName)})
							return nil, true, nil
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
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols", Error: "array value is empty"})
			return nil, true, nil
		}

		return &persistence.AgreementProtocolAttributes{
			Meta:      generateAttributeMetadata(*given, reflect.TypeOf(persistence.AgreementProtocolAttributes{}).Name()),
			Protocols: allProtocols,
		}, false, nil
	}
}

// AttributeVerifier returns true if there is a handled inputError (one that caused a write to the http responsewriter) and error if there is a system processing problem
type AttributeVerifier func(w http.ResponseWriter, attr persistence.Attribute) (bool, error)

func toPersistedAttributesAttachedToService(w http.ResponseWriter, persistedDevice *persistence.ExchangeDevice, defaultRAM int64, attrs []Attribute, sensorURL string, additionalVerifiers []AttributeVerifier) ([]persistence.Attribute, bool, error) {

	additionalVerifiers = append(additionalVerifiers, func(w http.ResponseWriter, attr persistence.Attribute) (bool, error) {
		// can't specify sensorURLs in attributes that are a part of a service
		sensorURLs := attr.GetMeta().SensorUrls
		if sensorURLs != nil {
			if len(sensorURLs) > 1 || (len(sensorURLs) == 1 && sensorURLs[0] != sensorURL) {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "service.[attribute].sensor_urls", Error: "sensor_urls not permitted on attributes specified on a service"})
				return true, nil
			}
		}

		return false, nil
	})

	persistenceAttrs, inputErr, err := toPersistedAttributes(w, false, persistedDevice, attrs, additionalVerifiers)
	if inputErr || err != nil {
		return persistenceAttrs, inputErr, err
	}

	persistenceAttrs = finalizeAttributesSpecifiedInService(defaultRAM, sensorURL, persistenceAttrs)

	return persistenceAttrs, inputErr, err
}

func toPersistedAttributes(w http.ResponseWriter, permitEmpty bool, persistedDevice *persistence.ExchangeDevice, attrs []Attribute, additionalVerifiers []AttributeVerifier) ([]persistence.Attribute, bool, error) {
	attributes := []persistence.Attribute{}

	for _, given := range attrs {

		// ----------------------

		if permitEmpty && given.Label == nil {
			glog.V(4).Infof("Allowing unspecified label in partial update of %v", given)
		} else if bail := checkInputString(w, "label", given.Label); bail {
			return nil, true, nil
		}

		if given.Publishable == nil {
			if permitEmpty {
				glog.V(4).Infof("Allowing unspecified publishable flag in partial update of %v", given)
			} else {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "publishable", Error: "nil value"})
				return nil, true, nil
			}
		}

		// always ok if this one is nil
		if given.SensorUrls != nil {
			for _, url := range *given.SensorUrls {
				if bail := checkInputString(w, "sensorurl", &url); bail {
					return nil, true, nil
				}
			}
		}

		if given.Mappings == nil {
			if permitEmpty {
				glog.V(4).Infof("Allowing unspecified mappings in partial update of %v", given)
			} else {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "mappings", Error: "nil value"})
				return nil, true, nil
			}
		} else {

			// check each mapping
			if value, inputErr, err := MapInputIsIllegal(*given.Mappings); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return nil, true, fmt.Errorf("Failed to check input: %v", err)
			} else if inputErr != "" {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: fmt.Sprintf("mappings.%v", value), Error: inputErr})
				return nil, true, nil
			}
		}

		if given.Type == nil && permitEmpty {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "type", Error: "partial update with missing type is not supported"})
			return nil, true, nil
		} else if bail := checkInputString(w, "type", given.Type); bail {
			return nil, true, nil
		} else {

			// attribute meta is good, deserialize (except architecture, we add our own for that)
			switch *given.Type {

			case reflect.TypeOf(persistence.ComputeAttributes{}).Name():
				attr, inputErr, err := parseCompute(w, permitEmpty, &given)
				if err != nil || inputErr {
					return nil, inputErr, err
				}
				attributes = append(attributes, attr)

			case reflect.TypeOf(persistence.LocationAttributes{}).Name():
				attr, inputErr, err := parseLocation(w, permitEmpty, &given)
				if err != nil || inputErr {
					return nil, inputErr, err
				}
				attributes = append(attributes, attr)

			case reflect.TypeOf(persistence.MappedAttributes{}).Name():
				attr, inputErr, err := parseMapped(w, permitEmpty, &given)
				if err != nil || inputErr {
					return nil, inputErr, err
				}
				attributes = append(attributes, attr)

			case reflect.TypeOf(persistence.HAAttributes{}).Name():
				attr, inputErr, err := parseHA(w, permitEmpty, &given)
				if err != nil || inputErr {
					return nil, inputErr, err
				}
				attributes = append(attributes, attr)

			case reflect.TypeOf(persistence.MeteringAttributes{}).Name():
				attr, inputErr, err := parseMetering(w, permitEmpty, &given)
				if err != nil || inputErr {
					return nil, inputErr, err
				}
				attributes = append(attributes, attr)

			case reflect.TypeOf(persistence.PropertyAttributes{}).Name():
				attr, inputErr, err := parseProperty(w, permitEmpty, &given)
				if err != nil || inputErr {
					return attributes, inputErr, err
				}
				attributes = append(attributes, attr)

			case reflect.TypeOf(persistence.CounterPartyPropertyAttributes{}).Name():
				attr, inputErr, err := parseCounterPartyProperty(w, permitEmpty, &given)
				if err != nil || inputErr {
					return attributes, inputErr, err
				}
				attributes = append(attributes, attr)

			case reflect.TypeOf(persistence.AgreementProtocolAttributes{}).Name():
				attr, inputErr, err := parseAgreementProtocol(w, permitEmpty, &given)
				if err != nil || inputErr {
					return attributes, inputErr, err
				}
				attributes = append(attributes, attr)

			case reflect.TypeOf(persistence.HTTPSBasicAuthAttributes{}).Name():
				attr, inputErr, err := parseHTTPSBasicAuth(w, permitEmpty, &given)
				if err != nil || inputErr {
					return attributes, inputErr, err
				}
				attributes = append(attributes, attr)

			default:
				glog.Errorf("Failed to find expected id for given input attribute: %v", given)
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "mappings", Error: "Unmappable id field"})
			}
		}
	}

	// do validation on concrete types (make sure conflicting options aren't specified, etc.)
	if inputErr, err := validateConcreteAttributes(w, persistedDevice, attributes, additionalVerifiers); err != nil || inputErr {
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
		Type:        &persisted.GetMeta().Type,
		Mappings:    &mappings,
	}
}

func finalizeAttributesSpecifiedInService(defaultRAM int64, sensorURL string, attributes []persistence.Attribute) []persistence.Attribute {

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
		glog.Infof("SensorUrls for %v: %v", attr.GetMeta().Id, attr.GetMeta().SensorUrls)
	}

	// return updated
	return attributes
}

func validateConcreteAttributes(w http.ResponseWriter, persistedDevice *persistence.ExchangeDevice, attributes []persistence.Attribute, additionalVerifiers []AttributeVerifier) (bool, error) {

	// check for errors in attribute input, like specifying a sensorUrl or specifying HA Partner on a non-HA device
	for _, attr := range attributes {
		for _, verifier := range additionalVerifiers {
			if inputErr, err := verifier(w, attr); inputErr || err != nil {
				return inputErr, err
			}
		}

		if attr.GetMeta().Type == reflect.TypeOf(persistence.HAAttributes{}).Name() {
			// if the device is not HA enabled then the HA partner attribute is illegal
			if !persistedDevice.HADevice {
				glog.Errorf("Non-HA device %v does not support HA enabled service", persistedDevice)
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "service.[attribute].Id", Error: "HA partner not permitted on non-HA devices"})
				return true, nil
			}

			// Make sure that a device doesn't specify itself in the HA partner list
			if _, ok := attr.GetGenericMappings()["partnerID"]; ok {
				switch attr.GetGenericMappings()["partnerID"].(type) {
				case []string:
					partners := attr.GetGenericMappings()["partnerID"].([]string)
					for _, partner := range partners {
						if partner == persistedDevice.Id {
							glog.Errorf("HA device %v cannot refer to itself in partner list %v", persistedDevice, partners)
							writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "service.[attribute].ha", Error: "partner list cannot refer to itself."})
							return true, nil
						}
					}
				}
			}
		}
	}

	return false, nil
}

func payloadToAttributes(w http.ResponseWriter, body io.Reader, permitPartial bool, existingDevice *persistence.ExchangeDevice) ([]persistence.Attribute, bool, error) {

	by, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, false, fmt.Errorf("Failed to read request bytes: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(by))
	decoder.UseNumber()

	var attribute Attribute
	if err := decoder.Decode(&attribute); err != nil {
		glog.Errorf("User submitted data that couldn't be deserialized to attribute. Error: %v", err)
		writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "attribute", Error: fmt.Sprintf("could not be demarshalled, error: %v", err)})
		return nil, true, err
	}

	// N.B. remove the id from the input doc; it won't be checked and it shouldn't be trusted, prefer the path param id instead
	attribute.Id = nil

	// we allow the user to send partial updates that leave out some members of an attribute
	if permitPartial && attribute.Mappings == nil {
		attribute.Mappings = new(map[string]interface{})
	}

	return toPersistedAttributes(w, permitPartial, existingDevice, []Attribute{attribute}, []AttributeVerifier{})
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
