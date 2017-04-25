package persistence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"sort"
	"strconv"
	"strings"
)

// a "service is a v2.1.0 transitional concept: it stores patterned service-related information from v1 PoCs
const SERVICE_ATTRIBUTES = "service_attributes"

type AttributeMeta struct {
	// Note: Id and SensorUrls content (sorted) are a composite key for this persisted entity
	Id          string   `json:"id"`          // should correspond to something meangingful to the caller
	SensorUrls  []string `json:"sensor_urls"` // empty if applicable to all services
	Label       string   `json:"label"`       // for humans only, never computable
	Publishable bool     `json:"publishable"` // means sent to exchange or otherwise published; all attrs end up inside workload containers
	Type        string   `json:"type"`
}

func (a AttributeMeta) String() string {
	return fmt.Sprintf("Id: %v, SensorUrls (%p): %v, Label: %v, Publishable: %v, Type: %v", a.Id, a.SensorUrls, a.SensorUrls, a.Label, a.Publishable, a.Type)
}

// important to use this for additions to prevent duplicates and keep slice ordered
func (m *AttributeMeta) AppendSensorUrl(url string) *AttributeMeta {

	contains := false
	for _, val := range m.SensorUrls {
		if val == url {
			contains = true
		}
	}

	if !contains {
		new := append(m.SensorUrls, url)
		m.SensorUrls = new

	}

	sort.Strings(m.SensorUrls)

	return m
}

type ServiceAttribute interface {
	GetMeta() *AttributeMeta
	GetGenericMappings() map[string]interface{}
}

type MetaAttributesOnly struct {
	Meta *AttributeMeta `json:"meta"`
}

func (a MetaAttributesOnly) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a MetaAttributesOnly) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{}
}

// particular service preferences
type LocationAttributes struct {
	Meta               *AttributeMeta `json:"meta"`
	Lat                string         `json:"lat"`
	Lon                string         `json:"lon"`
	UserProvidedCoords bool           `json:"user_provided_coords"` // if true, the lat / lon could have been edited by the user
	UseGps             bool           `json:"use_gps"`              // a statement of preference; does not indicate that there is a GPS device
}

func (a LocationAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a LocationAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"lat": a.Lat,
		"lon": a.Lon,
		"user_provided_coords": a.UserProvidedCoords,
		"use_gps":              a.UseGps,
	}
}

type ArchitectureAttributes struct {
	Meta         *AttributeMeta `json:"meta"`
	Architecture string         `json:"architecture"`
}

func (a ArchitectureAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a ArchitectureAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"architecture": a.Architecture,
	}
}

type ComputeAttributes struct {
	Meta *AttributeMeta `json:"meta"`
	CPUs int64          `json:"cpus"`
	RAM  int64          `json:"ram"`
}

func (a ComputeAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a ComputeAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"cpus": a.CPUs,
		"ram":  a.RAM,
	}
}

type HAAttributes struct {
	Meta     *AttributeMeta `json:"meta"`
	Partners []string       `json:"partners"`
}

func (a HAAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a HAAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"partnerID": a.Partners,
	}
}

func (a HAAttributes) PartnersContains(id string) bool {
	for _, p := range a.Partners {
		if p == id {
			return true
		}
	}
	return false
}

type MeteringAttributes struct {
	Meta                  *AttributeMeta `json:"meta"`
	Tokens                uint64         `json:"tokens"`
	PerTimeUnit           string         `json:"per_time_unit"`
	NotificationIntervalS int            `json:"notification_interval"`
}

func (a MeteringAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a MeteringAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"tokens":                a.Tokens,
		"per_time_unit":         a.PerTimeUnit,
		"notification_interval": a.NotificationIntervalS,
	}
}

type MappedAttributes struct {
	Meta     *AttributeMeta    `json:"meta"`
	Mappings map[string]string `json:"mappings"`
}

func (a MappedAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a MappedAttributes) GetGenericMappings() map[string]interface{} {
	out := map[string]interface{}{}

	for k, v := range a.Mappings {
		out[k] = v
	}

	return out
}

func FindApplicableAttributes(db *bolt.DB, serviceUrl string) ([]ServiceAttribute, error) {

	filteredAttrs := []ServiceAttribute{}

	return filteredAttrs, db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(SERVICE_ATTRIBUTES))

		if bucket == nil {
			return nil
		}

		return bucket.ForEach(func(k, v []byte) error {
			var meta MetaAttributesOnly // for easy deserialization

			if err := json.Unmarshal(v, &meta); err != nil {
				glog.Errorf("Unable to deserialize db record: %v", v)
				return err
			}

			var attr ServiceAttribute

			// TODO: optimization: do this only if the sensorurls match
			switch meta.GetMeta().Type {
			case "persistence.LocationAttributes":
				var location LocationAttributes
				if err := json.Unmarshal(v, &location); err != nil {
					return err
				}
				attr = location

			case "persistence.ArchitectureAttributes":
				var arch ArchitectureAttributes
				if err := json.Unmarshal(v, &arch); err != nil {
					return err
				}
				attr = arch

			case "persistence.MappedAttributes":
				var mapped MappedAttributes
				if err := json.Unmarshal(v, &mapped); err != nil {
					return err
				}
				attr = mapped

			case "persistence.ComputeAttributes":
				var compute ComputeAttributes
				if err := json.Unmarshal(v, &compute); err != nil {
					return err
				}
				attr = compute

			case "persistence.HAAttributes":
				var ha HAAttributes
				if err := json.Unmarshal(v, &ha); err != nil {
					return err
				}
				attr = ha

			case "persistence.MeteringAttributes":
				var ma MeteringAttributes
				if err := json.Unmarshal(v, &ma); err != nil {
					return err
				}
				attr = ma

			default:
				return fmt.Errorf("Unknown attr type: %v, just handling as meta", meta.GetMeta().Type)
			}

			glog.V(5).Infof("Deserialized ServiceAttribute: %v", attr)

			if serviceUrl == "" {
				// no need to discriminate
				filteredAttrs = append(filteredAttrs, attr)
			} else {
				sensorUrls := attr.GetMeta().SensorUrls
				if sensorUrls == nil || len(sensorUrls) == 0 {
					filteredAttrs = append(filteredAttrs, attr)
				} else {
					// O(2)
					for _, url := range sensorUrls {
						if url == "" || url == serviceUrl {
							filteredAttrs = append(filteredAttrs, attr)
						}
					}
				}
			}

			return nil
		})

		return nil
	})

	return filteredAttrs, nil
}

// this will include *all* values, include those marked to not publish
func AttributesToEnvvarMap(attributes []ServiceAttribute, prefix string) (map[string]string, error) {
	envvars := map[string]string{}

	pf := func(str string, prefix string) string {
		return fmt.Sprintf("%v%v", prefix, str)
	}

	write := func(k string, v string, skipPrefix bool) {
		if skipPrefix {
			envvars[k] = v
		} else {
			envvars[pf(k, prefix)] = v
		}
	}

	// a nil altPrefix is for skipping prefixing altogether, an empty signifies using the given
	writePrefix := func(k string, v string) {
		write(k, v, false)
	}

	// TODO: consider extracting this type-processing out for generalization
	for _, serv := range attributes {

		switch serv.(type) {
		case ComputeAttributes:
			s := serv.(ComputeAttributes)
			writePrefix("CPUS", strconv.FormatInt(s.CPUs, 10))
			writePrefix("RAM", strconv.FormatInt(s.RAM, 10))

		case MappedAttributes:
			s := serv.(MappedAttributes)
			for k, v := range s.Mappings {
				write(k, v, true)
			}

		case LocationAttributes:
			s := serv.(LocationAttributes)
			writePrefix("LAT", s.Lat)
			writePrefix("LON", s.Lon)
			writePrefix("USE_GPS", strconv.FormatBool(s.UseGps))
			writePrefix("USER_PROVIDED_COORDS", strconv.FormatBool(s.UserProvidedCoords))

		case ArchitectureAttributes:
			s := serv.(ArchitectureAttributes)
			writePrefix("ARCH", s.Architecture)

		case HAAttributes:
			s := serv.(HAAttributes)
			writePrefix("HA_PARTNERS", strings.Join(s.Partners,","))

		case MeteringAttributes:
			// Nothing to do

		default:
			return nil, fmt.Errorf("Unhandled service attribute: %v", serv)
		}
	}

	return envvars, nil
}

// N.B. It's the caller's responsibility to ensure the attr.SensorUrls are deduplicated; use the ServiceAttribute.AddSensorUrl() function to keep the slice clean
func SaveOrUpdateServiceAttribute(db *bolt.DB, attr ServiceAttribute) (*ServiceAttribute, error) {
	computePrimaryKey := func(id string, sensorUrls []string) string {
		catNull := func(str string) string {
			return fmt.Sprintf("%s\x00", str)
		}

		sort.Strings(sensorUrls)

		var sb bytes.Buffer
		sb.WriteString(catNull(id))
		for _, url := range sensorUrls {
			sb.WriteString(catNull(url))
		}

		return sb.String()
	}

	writeErr := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(SERVICE_ATTRIBUTES))
		if err != nil {
			return err
		}

		serial, err := json.Marshal(attr)
		if err != nil {
			return fmt.Errorf("Failed to serialize service attribute: %v. Error: %v", attr, err)
		}
		meta := attr.GetMeta()
		return bucket.Put([]byte(computePrimaryKey(meta.Id, meta.SensorUrls)), serial)
	})

	return &attr, writeErr
}
