package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
)

func attributesContains(given []persistence.Attribute, sp *persistence.ServiceSpec, typeString string) *persistence.Attribute {
	// only returns the first match and doesn't look in the db; this is sufficient for looking at POST services, but not sufficient for supporting PUT and PATCH mechanisms

	for _, attr := range given {
		if attr.GetMeta().Type == typeString {

			sps := persistence.GetAttributeServiceSpecs(&attr)

			if sps == nil || len(*sps) == 0 {
				return &attr
			}

			for _, sp1 := range *sps {
				if sp.IsSame(sp1) {
					return &attr
				}
			}
		}
	}

	return nil
}

func generateAttributeMetadata(given Attribute, typeName string) *persistence.AttributeMeta {
	return &persistence.AttributeMeta{
		Label:       *given.Label,
		Publishable: given.Publishable,
		HostOnly:    given.HostOnly,
		Type:        typeName,
	}
}

func parseUserInput(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.UserInputAttributes, bool, error) {

	if given.Mappings == nil {
		if !permitEmpty {
			return nil, errorhandler(NewAPIUserInputError("missing mappings", "mappings")), nil
		}
	}

	sps := new(persistence.ServiceSpecs)
	if given.ServiceSpecs != nil {
		sps = given.ServiceSpecs
	}

	return &persistence.UserInputAttributes{
		Meta:         generateAttributeMetadata(*given, reflect.TypeOf(persistence.UserInputAttributes{}).Name()),
		ServiceSpecs: sps,
		Mappings:     (*given.Mappings),
	}, false, nil
}

func parseHTTPSBasicAuth(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.HTTPSBasicAuthAttributes, bool, error) {
	var ok bool

	var server_url string
	su, exists := (*given.Mappings)["url"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "httpsbasic.mappings.url")), nil
	}
	if server_url, ok = su.(string); !ok {
		return nil, errorhandler(NewAPIUserInputError("expected string", "httpsbasic.mappings.url")), nil
	}

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
		Url:      server_url,
		Username: username,
		Password: password,
	}, false, nil
}

func parseDockerRegistryAuth(errorhandler ErrorHandler, permitEmpty bool, given *Attribute) (*persistence.DockerRegistryAuthAttributes, bool, error) {
	auths, exists := (*given.Mappings)["auths"]
	if !exists {
		return nil, errorhandler(NewAPIUserInputError("missing key", "dockerregistry.mappings.auths")), nil
	} else if a_temp, ok := auths.([]interface{}); !ok {
		return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("expected []interface{} received %T", a_temp), "dockerregistry.mappings.auths")), nil
	} else {
		var auth_array []persistence.Auth
		for _, val := range a_temp {
			a_temp2, ok2 := val.(map[string]interface{})
			if !ok2 {
				return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("array value is not a map[string]interface{}, it is %T", val), "dockerregistry.mappings.auths")), nil
			}

			if a_temp2["registry"] == nil {
				return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("'registry' does not exist, it is %v", a_temp2), "dockerregistry.mappings.auths")), nil
			}
			registry, ok3 := a_temp2["registry"].(string)
			if !ok3 {
				return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("the registry value is not a string, it is %T", a_temp2["registry"]), "dockerregistry.mappings.auths")), nil
			}

			if a_temp2["token"] == nil {
				return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("'token' does not exist, it is %v", a_temp2), "dockerregistry.mappings.auths")), nil
			}
			token, ok4 := a_temp2["token"].(string)
			if !ok4 {
				return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("the token value is not a string, it is %T", a_temp2["token"]), "dockerregistry.mappings.auths")), nil
			}

			// username can be omitted, the default is "token"
			username := "token"
			if a_temp2["username"] != nil {
				username, ok = a_temp2["username"].(string)
				if !ok {
					return nil, errorhandler(NewAPIUserInputError(fmt.Sprintf("the username value is not a string, it is %T", a_temp2["username"]), "dockerregistry.mappings.auths")), nil
				}
			}

			auth_array = append(auth_array, persistence.Auth{Registry: registry, UserName: username, Token: token})
		}

		return &persistence.DockerRegistryAuthAttributes{
			Meta:  generateAttributeMetadata(*given, reflect.TypeOf(persistence.DockerRegistryAuthAttributes{}).Name()),
			Auths: auth_array,
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

	sps := new(persistence.ServiceSpecs)
	if given.ServiceSpecs != nil {
		sps = given.ServiceSpecs
	}

	return &persistence.MeteringAttributes{
		Meta:                  generateAttributeMetadata(*given, reflect.TypeOf(persistence.MeteringAttributes{}).Name()),
		Tokens:                uint64(tokens),
		ServiceSpecs:          sps,
		PerTimeUnit:           perTimeUnit,
		NotificationIntervalS: int(notificationInterval),
	}, false, nil
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

		sps := new(persistence.ServiceSpecs)
		if given.ServiceSpecs != nil {
			sps = given.ServiceSpecs
		}

		return &persistence.AgreementProtocolAttributes{
			Meta:         generateAttributeMetadata(*given, reflect.TypeOf(persistence.AgreementProtocolAttributes{}).Name()),
			ServiceSpecs: sps,
			Protocols:    allProtocols,
		}, false, nil
	}
}

// AttributeVerifier returns true if there is a handled inputError (one that caused a write to the http responsewriter) and error if there is a system processing problem
type AttributeVerifier func(attr persistence.Attribute) (bool, error)

func toPersistedAttributesAttachedToService(errorhandler ErrorHandler, persistedDevice *persistence.ExchangeDevice, attrs []Attribute, sp *persistence.ServiceSpec, additionalVerifiers []AttributeVerifier) ([]persistence.Attribute, bool, error) {

	additionalVerifiers = append(additionalVerifiers, func(attr persistence.Attribute) (bool, error) {
		// can't specify service specs in attributes that are a part of a service
		sps := persistence.GetAttributeServiceSpecs(&attr)
		if sps != nil {
			if len(*sps) > 1 || (len(*sps) == 1 && !(*sps)[0].IsSame(*sp)) {
				return errorhandler(NewAPIUserInputError("service_specs not permitted on attributes specified on a service", "service.[attribute].service_specs")), nil
			}
		}

		return false, nil
	})

	persistenceAttrs, inputErr, err := toPersistedAttributes(errorhandler, false, persistedDevice, attrs, additionalVerifiers)
	if inputErr || err != nil {
		return persistenceAttrs, inputErr, err
	}

	persistenceAttrs = FinalizeAttributesSpecifiedInService(sp, persistenceAttrs)

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
	if given.ServiceSpecs != nil {
		sps := *given.ServiceSpecs
		for _, sp := range sps {
			if bail := checkInputString(errorhandler, "service_specs", &sp.Url); bail {
				return nil, true, nil
			}

			if sp.Org != "" {
				if bail := checkInputString(errorhandler, "service_specs", &sp.Org); bail {
					return nil, true, nil
				}
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

		case reflect.TypeOf(persistence.UserInputAttributes{}).Name():
			attr, inputErr, err := parseUserInput(errorhandler, permitEmpty, &given)
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

		case reflect.TypeOf(persistence.DockerRegistryAuthAttributes{}).Name():
			attr, inputErr, err := parseDockerRegistryAuth(errorhandler, permitEmpty, &given)
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

	sps := persistence.GetAttributeServiceSpecs(&persisted)
	return &Attribute{
		Id:           &persisted.GetMeta().Id,
		Label:        &persisted.GetMeta().Label,
		Publishable:  persisted.GetMeta().Publishable,
		HostOnly:     persisted.GetMeta().HostOnly,
		Type:         &persisted.GetMeta().Type,
		ServiceSpecs: sps,
		Mappings:     &mappings,
	}
}

func FinalizeAttributesSpecifiedInService(sp *persistence.ServiceSpec, attributes []persistence.Attribute) []persistence.Attribute {

	sps := new(persistence.ServiceSpecs)
	sps.AppendServiceSpec(*sp)

	for _, attr := range attributes {
		sps := persistence.GetAttributeServiceSpecs(&attr)
		if sps != nil {
			sps.AppendServiceSpec(*sp)
			glog.V(3).Infof(apiLogString(fmt.Sprintf("ServiceSpecs for %v: %v, %v", attr.GetMeta(), *sps, *(persistence.GetAttributeServiceSpecs(&attr)))))
		}
	}

	// return updated
	return attributes
}

func validateConcreteAttributes(errorhandler ErrorHandler, persistedDevice *persistence.ExchangeDevice, attributes []persistence.Attribute, additionalVerifiers []AttributeVerifier) (bool, error) {

	// check for errors in attribute input, like specifying HA Partner on a non-HA device
	for _, attr := range attributes {
		for _, verifier := range additionalVerifiers {
			if inputErr, err := verifier(attr); inputErr || err != nil {
				return inputErr, err
			}
		}
	}

	return false, nil
}

func payloadToAttributes(errorhandler ErrorHandler, body io.Reader, permitPartial bool, existingDevice *persistence.ExchangeDevice) ([]persistence.Attribute, bool, error) {

	by, err := io.ReadAll(body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read request bytes: %v", err)
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

	attributes, err := persistence.FindApplicableAttributes(db, "", "")
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
