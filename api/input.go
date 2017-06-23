package api

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"net/http"
	"reflect"
)

type HorizonAccount struct {
	Id    *string `json:"id"`
	Email *string `json:"email"`
}

func (h HorizonAccount) String() string {
	return fmt.Sprintf("Id: %v, Email: %v", *h.Id, *h.Email)
}

type HorizonDevice struct {
	Id                 *string         `json:"id"`
	Account            *HorizonAccount `json:"account,omitempty"`
	Name               *string         `json:"name,omitempty"`
	Token              *string         `json:"token,omitempty"`
	TokenLastValidTime *uint64         `json:"token_last_valid_time,omitempty"`
	TokenValid         *bool           `json:"token_valid,omitempty"`
	HADevice           *bool           `json:"ha_device,omitempty"`
}

func (h HorizonDevice) String() string {
	cred := "not set"
	if h.Token != nil && *h.Token != "" {
		cred = "set"
	}

	return fmt.Sprintf("Account: %v, Id: %v, Name: %v, Token: [%v], TokenLastValidTime: %v, TokenValid: %v", *h.Account, h.Id, h.Name, cred, h.TokenLastValidTime, h.TokenValid)
}

type Attribute struct {
	Id          *string                 `json:"id"`
	ShortType   *string                 `json:"short_type,omitempty"`
	SensorUrls  *[]string               `json:"sensor_urls"`
	Label       *string                 `json:"label"`
	Publishable *bool                   `json:"publishable"`
	Mappings    *map[string]interface{} `json:"mappings"`
}

func (a Attribute) String() string {
	// function to make sure the nil pointers get printed without 'invalid memory address' error
	getString := func(v interface{}) string {
		if reflect.ValueOf(v).IsNil() {
			return "<nil>"
		} else {
			return fmt.Sprintf("%v", reflect.Indirect(reflect.ValueOf(v)))
		}
	}

	return fmt.Sprintf("Id: %v, ShortType: %v, SensorUrls: %v, Label: %v, Publishable: %v, Mappings: %v",
		getString(a.Id), getString(a.ShortType), getString(a.SensorUrls), getString(a.Label), getString(a.Publishable), getString(a.Mappings))
}

// uses pointers for members b/c it allows nil-checking at deserialization; !Important!: the json field names here must not change w/out changing the error messages returned from the API, they are not programmatically dete  rmined
type Service struct {
	SensorUrl  *string      `json:"sensor_url"`  // uniquely identifying
	SensorName *string      `json:"sensor_name"` // may not be uniquely identifying
	Attributes *[]Attribute `json:"attributes"`
}

func (s *Service) String() string {
	return fmt.Sprintf("SensorUrl: %v, SensorName: %v, Attributes: %s", *s.SensorUrl, *s.SensorName, s.Attributes)
}

func attributesContains(given []persistence.ServiceAttribute, sensorUrl string, typeString string) *persistence.ServiceAttribute {
	// only returns the first match and doesn't look in the db; this is sufficient for looking at POST services, but not sufficient for supporting PUT and PATCH mechanisms

	for _, attr := range given {
		if attr.GetMeta().Type == typeString {

			if len(attr.GetMeta().SensorUrls) == 0 {
				return &attr
			}

			for _, url := range attr.GetMeta().SensorUrls {
				if sensorUrl == url {
					return &attr
				}
			}
		}
	}

	return nil
}

func deserializeAttributes(w http.ResponseWriter, attrs []Attribute) ([]persistence.ServiceAttribute, error, bool) {

	attributes := []persistence.ServiceAttribute{}

	generateAttributeMetadata := func(given Attribute, typeString string) *persistence.AttributeMeta {
		var sensorUrls []string
		if given.SensorUrls == nil {
			sensorUrls = []string{}
		} else {
			sensorUrls = *given.SensorUrls
		}

		return &persistence.AttributeMeta{
			Id:          *given.Id,
			SensorUrls:  sensorUrls,
			Label:       *given.Label,
			Publishable: *given.Publishable,
			Type:        typeString,
		}
	}

	for _, given := range attrs {
		if bail := checkInputString(w, "id", given.Id); bail {
			return nil, nil, true
		}
		if bail := checkInputString(w, "label", given.Label); bail {
			return nil, nil, true
		}
		if bail := checkInputString(w, "short_type", given.ShortType); bail {
			return nil, nil, true
		}
		if given.Publishable == nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "publishable", Error: "nil value"})
			return nil, nil, true
		}
		// ok if this one is nil
		if given.SensorUrls != nil {
			for _, url := range *given.SensorUrls {
				if bail := checkInputString(w, "sensorurl", &url); bail {
					return nil, nil, true
				}
			}
		}
		if given.Mappings == nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "mappings", Error: "nil value"})
			return nil, nil, true
		}

		// check each mapping
		if value, inputErr, err := MapInputIsIllegal(*given.Mappings); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return nil, fmt.Errorf("Failed to check input: %v", err), true
		} else if inputErr != "" {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: fmt.Sprintf("mappings.%v", value), Error: inputErr})
			return nil, nil, true
		}

		// all good, deserialize (except architecture, we add our own for that)
		switch *given.ShortType {

		case "compute":
			var err error
			var ram int64
			r, exists := (*given.Mappings)["ram"]
			if !exists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "compute.mappings.ram", Error: "missing key"})
				return nil, nil, true
			}
			if ram, err = r.(json.Number).Int64(); err != nil {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "compute.mappings.ram", Error: "expected integer"})
				return nil, nil, true
			}
			var cpus int64
			c, exists := (*given.Mappings)["cpus"]
			if !exists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "compute.mappings.cpus", Error: "missing key"})
				return nil, nil, true
			}
			if cpus, err = c.(json.Number).Int64(); err != nil {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "compute.mappings.cpus", Error: "expected integer"})
				return nil, nil, true
			}

			attributes = append(attributes, persistence.ComputeAttributes{
				Meta: generateAttributeMetadata(given, reflect.TypeOf(persistence.ComputeAttributes{}).String()),
				CPUs: cpus,
				RAM:  ram,
			})

		case "location":
			var ok bool

			var lat string
			la, exists := (*given.Mappings)["lat"]
			if !exists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.lat", Error: "missing key"})
				return nil, nil, true
			}
			if lat, ok = la.(string); !ok {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.lat", Error: "expected string"})
				return nil, nil, true
			}
			var lon string
			lo, exists := (*given.Mappings)["lon"]
			if !exists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.lon", Error: "missing key"})
				return nil, nil, true
			}
			if lon, ok = lo.(string); !ok {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.lon", Error: "expected string"})
				return nil, nil, true
			}

			var userProvidedCoords bool
			up, exists := (*given.Mappings)["user_provided_coords"]
			if !exists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.user_provided_coords", Error: "missing key"})
				return nil, nil, true
			} else if userProvidedCoords, ok = up.(bool); !ok {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.user_provided_coords", Error: "non-boolean value"})
				return nil, nil, true
			}
			var useGps bool
			ug, exists := (*given.Mappings)["use_gps"]
			if !exists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.use_gps", Error: "missing key"})
				return nil, nil, true
			} else if useGps, ok = ug.(bool); !ok {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "location.mappings.use_gps", Error: "non-boolean value"})
				return nil, nil, true
			}

			attributes = append(attributes, persistence.LocationAttributes{
				Meta:               generateAttributeMetadata(given, reflect.TypeOf(persistence.LocationAttributes{}).String()),
				Lat:                lat,
				Lon:                lon,
				UserProvidedCoords: userProvidedCoords,
				UseGps:             useGps,
			})
		case "mapped":
			// convert all to string representations
			mappedStr := map[string]string{}
			for k, v := range *given.Mappings {
				mappedStr[k] = fmt.Sprintf("%v", v)
			}

			attributes = append(attributes, persistence.MappedAttributes{
				Meta:     generateAttributeMetadata(given, reflect.TypeOf(persistence.MappedAttributes{}).String()),
				Mappings: mappedStr,
			})

		case "ha":
			pID, exists := (*given.Mappings)["partnerID"]
			if !exists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "ha.mappings.partnerID", Error: "missing key"})
				return nil, nil, true
			} else if partnerIDs, ok := pID.([]interface{}); !ok {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "ha.mappings.partnerID", Error: fmt.Sprintf("expected []interface{} received %T", pID)})
				return nil, nil, true
			} else {
				// convert partner values to proper array type
				strPartners := make([]string, 0, 5)
				for _, val := range partnerIDs {
					if p, ok := val.(string); !ok {
						writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "ha.mappings.partnerID", Error: fmt.Sprintf("array value is not a string, it is %T",val)})
						return nil, nil, true
					} else {
						strPartners = append(strPartners, p)
					}
				}
				attributes = append(attributes, persistence.HAAttributes{
					Meta:     generateAttributeMetadata(given, reflect.TypeOf(persistence.HAAttributes{}).String()),
					Partners: strPartners,
					})
			}

		case "metering":
			var err error

			// Check for valid combinations of input parameters
			t, tokensExists := (*given.Mappings)["tokens"]
			p, perTimeUnitExists := (*given.Mappings)["perTimeUnit"]
			n, notificationIntervalExists := (*given.Mappings)["notificationInterval"]

			if tokensExists && !perTimeUnitExists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.perTimeUnit", Error: "missing key"})
				return nil, nil, true
			} else if !tokensExists && perTimeUnitExists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.tokens", Error: "missing key"})
				return nil, nil, true
			} else if notificationIntervalExists && !tokensExists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.notificationInterval", Error: "missing tokens and perTimeUnit keys"})
				return nil, nil, true
			}

			// Deserialize the attribute pieces
			var ok bool
			var tokens int64
			if _, ok = t.(json.Number); !ok {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.tokens", Error: "expected integer"})
				return nil, nil, true
			} else if tokens, err = t.(json.Number).Int64(); err != nil {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.tokens", Error: "could not convert to integer"})
				return nil, nil, true
			}

			var perTimeUnit string
			if perTimeUnit, ok = p.(string); !ok {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.perTimeUnit", Error: "expected string"})
				return nil, nil, true
			}

			// Make sure the attribute values make sense together
			if tokens == 0 && perTimeUnit != "" {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.tokens", Error: "must be non-zero"})
				return nil, nil, true
			} else if tokens != 0 && perTimeUnit == "" {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.perTimeUnit", Error: "must be non-empty"})
				return nil, nil, true
			}

			// Deserialize and validate the last piece of the attribute
			var notificationInterval int64
			if _, ok = n.(json.Number); !ok {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.notificationInterval", Error: "expected integer"})
				return nil, nil, true
			} else if notificationInterval, err = n.(json.Number).Int64(); err != nil {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.notificationInterval", Error: "could not convert to integer"})
				return nil, nil, true
			}

			if notificationInterval != 0 && tokens == 0 {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "metering.mappings.notificationInterval", Error: "cannot be non-zero without tokens and perTimeUnit"})
				return nil, nil, true
			}

			attributes = append(attributes, persistence.MeteringAttributes{
				Meta: generateAttributeMetadata(given, reflect.TypeOf(persistence.MeteringAttributes{}).String()),
				Tokens:                uint64(tokens),
				PerTimeUnit:           perTimeUnit,
				NotificationIntervalS: int(notificationInterval),
			})

		case "property":
			attributes = append(attributes, persistence.PropertyAttributes{
				Meta:     generateAttributeMetadata(given, reflect.TypeOf(persistence.PropertyAttributes{}).String()),
				Mappings: (*given.Mappings),
			})

		case "counterpartyproperty":
			rawExpression, exists := (*given.Mappings)["expression"]
			if !exists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "counterpartyproperty.mappings.expression", Error: "missing key"})
				return nil, nil, true
			} else {
				if exp, ok := rawExpression.(map[string]interface{}); !ok {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "counterpartyproperty.mappings.expression", Error: fmt.Sprintf("expected map[string]interface{}, is %T", rawExpression)})
					return nil, nil, true
				} else if rp := policy.RequiredProperty_Factory(); rp == nil {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "counterpartyproperty.mappings.expression", Error: "could not construct RequiredProperty"})
					return nil, nil, true
				} else if err := rp.Initialize(&exp); err != nil {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "counterpartyproperty.mappings.expression", Error: fmt.Sprintf("could not initialize RequiredProperty", err)})
					return nil, nil, true
				} else if err := rp.IsValid(); err != nil {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "counterpartyproperty.mappings.expression", Error: fmt.Sprintf("not a valid expression: %v", err)})
					return nil, nil, true
				} else {
					attributes = append(attributes, persistence.CounterPartyPropertyAttributes{
						Meta:     generateAttributeMetadata(given, reflect.TypeOf(persistence.CounterPartyPropertyAttributes{}).String()),
						Expression: rawExpression.(map[string]interface{}),
						})
				}
			}

		case "agreementprotocol":
			p, exists := (*given.Mappings)["protocols"]
			if !exists {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols", Error: "missing key"})
				return nil, nil, true
			} else if protocols, ok := p.([]interface{}); !ok {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols", Error: fmt.Sprintf("expected []interface{} received %T", p)})
				return nil, nil, true
			} else {
				// convert protocol values to proper agreement protocol object
				allProtocols := make([]policy.AgreementProtocol, 0, 5)
				for _, val := range protocols {
					if protoDef, ok := val.(map[string]interface{}); !ok {
						writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols", Error: fmt.Sprintf("array value is not a map[string]interface{}, it is %T",val)})
						return nil, nil, true
					} else {
						for protocolName, bcValue := range protoDef {
							if !policy.SupportedAgreementProtocol(protocolName) {
								writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.protocolName", Error: fmt.Sprintf("protocol name %v is not supported", protocolName)})
								return nil, nil, true
							} else if bcDefArray, ok := bcValue.([]interface{}); !ok {
								writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.blockchain", Error: fmt.Sprintf("blockchain value is not []interface{}, it is %T", bcValue)})
								return nil, nil, true
							} else {
								agp := policy.AgreementProtocol_Factory(protocolName)
								for _, bcEle := range bcDefArray {
									if bcDef, ok := bcEle.(map[string]interface{}); !ok {
										writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.blockchain", Error: fmt.Sprintf("blockchain array element is not map[string]interface{}, it is %T", bcEle)})
										return nil, nil, true
									} else if _, ok := bcDef["type"].(string); bcDef["type"] != nil && !ok {
										writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.blockchain.type", Error: fmt.Sprintf("blockchain type is not string, it is %T", bcDef["type"])})
										return nil, nil, true
									} else if _, ok := bcDef["name"].(string); bcDef["name"] != nil && !ok {
										writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.blockchain.name", Error: fmt.Sprintf("blockchain name is not string, it is %T", bcDef["name"])})
										return nil, nil, true
									} else if bcDef["type"] != nil && bcDef["type"].(string) != "" && bcDef["type"].(string) != policy.RequiresBlockchainType(protocolName) {
										writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols.blockchain.type", Error: fmt.Sprintf("blockchain type %v is not supported for protocol %v", bcDef["type"].(string), protocolName)})
										return nil, nil, true
									} else {
										bcType := ""
										if bcDef["type"] != nil {
											bcType = bcDef["type"].(string)
										}
										bcName := ""
										if bcDef["name"] != nil {
											bcName = bcDef["name"].(string)
										}
										(&agp.Blockchains).Add_Blockchain(policy.Blockchain_Factory(bcType, bcName))
									}
								}
								agp.Initialize()
								allProtocols = append(allProtocols, *agp)
							}
						}
					}
				}
				if len(allProtocols) != 0 {
					attributes = append(attributes, persistence.AgreementProtocolAttributes{
						Meta:     generateAttributeMetadata(given, reflect.TypeOf(persistence.AgreementProtocolAttributes{}).String()),
						Protocols: allProtocols,
						})
				} else {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementprotocol.mappings.protocols", Error: "array value is empty"})
					return nil, nil, true
				}
			}

		default:
			glog.Errorf("Failed to find expected id for given input attribute: %v", given)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "mappings", Error: "Unmappable id field"})
		}
	}

	return attributes, nil, false
}
