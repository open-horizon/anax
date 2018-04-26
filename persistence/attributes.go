package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/satori/go.uuid"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// a "service is a v2.1.0 transitional concept: it stores patterned service-related information from v1 PoCs
const ATTRIBUTES = "attributes"

type AttributeMeta struct {
	Id          string   `json:"id"` // should correspond to something meangingful to the caller
	Type        string   `json:"type"`
	SensorUrls  []string `json:"sensor_urls"` // empty if applicable to all services
	Label       string   `json:"label"`       // for humans only, never computable
	HostOnly    *bool    `json:"host_only"`   // determines whether or not the attribute will be published inside workload containers or exists only for Host use
	Publishable *bool    `json:"publishable"` // means sent to exchange or otherwise published; whether or not an attr ends up in a workload depends on the value of HostOnly
}

func (a AttributeMeta) String() string {
	ho := "true"
	if a.HostOnly == nil || !*a.HostOnly {
		ho = "false"
	}
	pub := "true"
	if a.Publishable == nil || !*a.Publishable {
		pub = "false"
	}
	return fmt.Sprintf("Id: %v, Type: %v, SensorUrls: %v, Label: %v, HostOnly: %v, Publishable: %v", a.Id, a.Type, a.SensorUrls, a.Label, ho, pub)
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

// Update *selectively* updates the content of this AttributeMeta (m) with non-empty values in the given meta.
func (m *AttributeMeta) Update(meta AttributeMeta) {
	// cannot change id, type; SensorUrls are merged, others are replaced if not empty / nil

	for _, updateSensorUrl := range meta.SensorUrls {
		alreadyPresent := false
		for _, sensorUrl := range (*m).SensorUrls {
			if updateSensorUrl == sensorUrl {
				alreadyPresent = true
			}
		}

		if !alreadyPresent {
			m.SensorUrls = append(m.SensorUrls, updateSensorUrl)
		}
	}

	if m.Label != "" {
		m.Label = meta.Label
	}

	if m.HostOnly != nil {
		m.HostOnly = meta.HostOnly
	}

	if m.Publishable != nil {
		m.Publishable = meta.Publishable
	}
}

type Attribute interface {
	GetMeta() *AttributeMeta
	GetGenericMappings() map[string]interface{}
	Update(other Attribute) error
	String() string
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

// N.B. The concrete attributes are to be found in a different file in this module

func HydrateConcreteAttribute(v []byte) (Attribute, error) {
	var meta MetaAttributesOnly // for easy deserialization

	if err := json.Unmarshal(v, &meta); err != nil {
		glog.Errorf("Unable to deserialize db record: %v", v)
		return nil, err
	}

	var attr Attribute

	switch meta.GetMeta().Type {
	case "LocationAttributes":
		var location LocationAttributes
		if err := json.Unmarshal(v, &location); err != nil {
			return nil, err
		}
		attr = location

	case "ArchitectureAttributes":
		var arch ArchitectureAttributes
		if err := json.Unmarshal(v, &arch); err != nil {
			return nil, err
		}
		attr = arch

	case "UserInputAttributes":
		var ui UserInputAttributes
		if err := json.Unmarshal(v, &ui); err != nil {
			return nil, err
		}
		attr = ui

	case "ComputeAttributes":
		var compute ComputeAttributes
		if err := json.Unmarshal(v, &compute); err != nil {
			return nil, err
		}
		attr = compute

	case "HAAttributes":
		var ha HAAttributes
		if err := json.Unmarshal(v, &ha); err != nil {
			return nil, err
		}
		attr = ha

	case "MeteringAttributes":
		var ma MeteringAttributes
		if err := json.Unmarshal(v, &ma); err != nil {
			return nil, err
		}
		attr = ma

	case "PropertyAttributes":
		var pa PropertyAttributes
		if err := json.Unmarshal(v, &pa); err != nil {
			return nil, err
		}
		attr = pa

	case "CounterPartyPropertyAttributes":
		var ca CounterPartyPropertyAttributes
		if err := json.Unmarshal(v, &ca); err != nil {
			return nil, err
		}
		attr = ca

	case "AgreementProtocolAttributes":
		var agp AgreementProtocolAttributes
		if err := json.Unmarshal(v, &agp); err != nil {
			return nil, err
		}
		attr = agp

	case "HTTPSBasicAuthAttributes":
		var hba HTTPSBasicAuthAttributes
		if err := json.Unmarshal(v, &hba); err != nil {
			return nil, err
		}
		attr = hba

	case "DockerRegistryAuthAttributes":
		var dra DockerRegistryAuthAttributes
		if err := json.Unmarshal(v, &dra); err != nil {
			return nil, err
		}
		attr = dra

	default:
		return nil, fmt.Errorf("Unknown attr type: %v", meta.GetMeta().Type)
	}

	glog.V(5).Infof("Deserialized Attribute: %v", attr)
	return attr, nil
}

// FindAttributeByKey is used to fetch a single attribute by its primary key
func FindAttributeByKey(db *bolt.DB, id string) (*Attribute, error) {
	var attr Attribute
	var bucket *bolt.Bucket

	readErr := db.View(func(tx *bolt.Tx) error {
		bucket = tx.Bucket([]byte(ATTRIBUTES))
		if bucket != nil {

			v := bucket.Get([]byte(id))
			if v != nil {
				var err error
				attr, err = HydrateConcreteAttribute(v)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})

	if readErr != nil {
		if bucket == nil {
			// no bucket created yet so record not found
			return nil, nil
		}

		return nil, readErr
	}

	return &attr, nil
}

func FindApplicableAttributes(db *bolt.DB, serviceUrl string) ([]Attribute, error) {

	filteredAttrs := []Attribute{}

	return filteredAttrs, db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ATTRIBUTES))

		if bucket == nil {
			return nil
		}

		return bucket.ForEach(func(k, v []byte) error {
			// TODO: optimization: do this only if the sensorurls match
			attr, err := HydrateConcreteAttribute(v)
			if err != nil {
				return err
			}

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
	})
}

// Workloads dont see the same system level env vars that microservices see. This function picks out just
// the attributes that are applicable to workloads.
func ConvertWorkloadPersistentNativeToEnv(allAttrs []Attribute, envvars map[string]string, prefix string) (map[string]string, error) {
	var lat, lon, cpus, ram, arch string
	for _, attr := range allAttrs {

		// Extract location property
		switch attr.(type) {
		case LocationAttributes:
			s := attr.(LocationAttributes)
			lat = strconv.FormatFloat(s.Lat, 'f', 6, 64)
			lon = strconv.FormatFloat(s.Lon, 'f', 6, 64)
		case ComputeAttributes:
			s := attr.(ComputeAttributes)
			cpus = strconv.FormatInt(s.CPUs, 10)
			ram = strconv.FormatInt(s.RAM, 10)
		case ArchitectureAttributes:
			s := attr.(ArchitectureAttributes)
			arch = s.Architecture
		}
	}
	cutil.SetSystemEnvvars(envvars, prefix, lat, lon, cpus, ram, arch)
	return envvars, nil
}

// This function is used to convert the persistent attributes for a microservice to an env var map.
// This will include *all* values for which HostOnly is false, include those marked to not publish.
func AttributesToEnvvarMap(attributes []Attribute, envvars map[string]string, prefix string) (map[string]string, error) {

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
		meta := serv.GetMeta()
		if meta.HostOnly != nil && (*meta.HostOnly) {
			glog.V(4).Infof("Not creating an envvar from attribute %v b/c it's marked HostOnly", serv)
			continue
		}

		switch serv.(type) {
		case ComputeAttributes:
			s := serv.(ComputeAttributes)
			writePrefix("CPUS", strconv.FormatInt(s.CPUs, 10))
			writePrefix("RAM", strconv.FormatInt(s.RAM, 10))

		case UserInputAttributes:
			s := serv.(UserInputAttributes)
			for k, v := range s.Mappings {
				cutil.NativeToEnvVariableMap(envvars, k, v)
			}

		case LocationAttributes:
			s := serv.(LocationAttributes)
			writePrefix("LAT", strconv.FormatFloat(s.Lat, 'f', 6, 64))
			writePrefix("LON", strconv.FormatFloat(s.Lon, 'f', 6, 64))
			writePrefix("USE_GPS", strconv.FormatBool(s.UseGps))
			writePrefix("LOCATION_ACCURACY_KM", strconv.FormatFloat(s.LocationAccuracyKM, 'f', 2, 64))

		case ArchitectureAttributes:
			s := serv.(ArchitectureAttributes)
			writePrefix("ARCH", s.Architecture)

		case HAAttributes:
			s := serv.(HAAttributes)
			writePrefix("HA_PARTNERS", strings.Join(s.Partners, ","))

		case MeteringAttributes:
			// Nothing to do

		case PropertyAttributes:
			// Nothing to do

		case CounterPartyPropertyAttributes:
			// Nothing to do

		case AgreementProtocolAttributes:
			// Nothing to do

		default:
			return nil, fmt.Errorf("Unhandled service attribute: %v", serv)
		}
	}

	return envvars, nil
}

func FindConflictingAttributes(db *bolt.DB, attribute *Attribute) (*Attribute, error) {
	var err error
	var common []Attribute
	urls := (*attribute).GetMeta().SensorUrls

	if len(urls) == 0 {
		common, err = FindApplicableAttributes(db, "")
		if err != nil {
			return nil, err
		}
	} else {
		for _, url := range urls {
			common, err = FindApplicableAttributes(db, url)
			if err != nil {
				return nil, err
			}
		}
	}

	attributeMeta := (*attribute).GetMeta()
	for _, possiblyConflicting := range common {
		conflictingMeta := possiblyConflicting.GetMeta()
		if attributeMeta.Type == conflictingMeta.Type &&
			reflect.DeepEqual(attributeMeta.SensorUrls, conflictingMeta.SensorUrls) &&
			attributeMeta.Label == conflictingMeta.Label &&
			reflect.DeepEqual((*attribute).GetGenericMappings(), possiblyConflicting.GetGenericMappings()) {

			return &possiblyConflicting, nil
		}
	}
	return nil, nil
}

type OverwriteCandidateNotFound struct {
	msg string // description of error
}

func (e OverwriteCandidateNotFound) Error() string { return e.msg }

type ConflictingAttributeFound struct {
	msg string // description of error
}

func (e ConflictingAttributeFound) Error() string { return e.msg }

// N.B. It's the caller's responsibility to ensure the attr.SensorUrls are deduplicated; use the Attribute.AddSensorUrl() function to keep the slice clean
func SaveOrUpdateAttribute(db *bolt.DB, attr Attribute, id string, permitPartialOverwrite bool) (*Attribute, error) {
	var ret *Attribute

	if id == "" {
		// an empty id means this is a new record and we'll generate a unique id before saving

		if possiblyConflicting, err := FindConflictingAttributes(db, &attr); err != nil {
			return nil, err
		} else if possiblyConflicting != nil {
			glog.Infof("Found conflicting attribute during save of new one. Existing: %v. New: %v", *possiblyConflicting, attr)
			return nil, &ConflictingAttributeFound{}
		}

		id = uuid.NewV4().String()
		ret = &attr
	} else {
		// updating and existing must be found; may be a partial overwrite

		existing, err := FindAttributeByKey(db, id)
		if err != nil {
			return nil, fmt.Errorf("Failed to search for existing attribute: %v", err)
		}

		if *existing == nil {
			return nil, &OverwriteCandidateNotFound{}
		} else {

			if permitPartialOverwrite {
				err := (*existing).Update(attr)
				if err != nil {
					return nil, err
				}

				ret = existing
			} else {
				ret = &attr
			}

		}
	}

	// make sure we set the id in-doc
	(*ret).GetMeta().Id = id

	// make sure nil-able fields are set to conservative defaults
	pF := false
	if (*ret).GetMeta().HostOnly == nil {
		(*ret).GetMeta().HostOnly = &pF
	}

	pT := true
	if (*ret).GetMeta().Publishable == nil {
		(*ret).GetMeta().Publishable = &pT
	}

	writeErr := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(ATTRIBUTES))
		if err != nil {
			return err
		}
		serial, err := json.Marshal(ret)
		if err != nil {
			return fmt.Errorf("Failed to serialize attribute: %v. Error: %v", ret, err)
		}
		return bucket.Put([]byte(id), serial)
	})

	return ret, writeErr
}

func DeleteAttribute(db *bolt.DB, id string) (*Attribute, error) {

	existing, err := FindAttributeByKey(db, id)
	if err != nil {
		return nil, fmt.Errorf("Failed to search for existing attribute: %v", err)
	}

	if *existing == nil {
		return nil, nil
	}

	delError := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(ATTRIBUTES))
		if err != nil {
			return err
		}
		return bucket.Delete([]byte(id))
	})

	return existing, delError
}
