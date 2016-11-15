package api

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/persistence"
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
	return fmt.Sprintf("Id: %v, ShortType: %v, SensorUrls: %v, Label: %v, Publishable: %v, Mappings: %v", *a.Id, *a.ShortType, *a.SensorUrls, *a.Label, *a.Publishable, *a.Mappings)
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

		default:
			glog.Errorf("Failed to find expected id for given input attribute: %v", given)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "mappings", Error: "Unmappable id field"})
		}
	}

	return attributes, nil, false
}
