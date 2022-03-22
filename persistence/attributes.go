package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/satori/go.uuid"
	"reflect"
	"strconv"
	"strings"
)

// a "service is a v2.1.0 transitional concept: it stores patterned service-related information from v1 PoCs
const ATTRIBUTES = "attributes"

type AttributeMeta struct {
	Id          string `json:"id"` // should correspond to something meangingful to the caller
	Type        string `json:"type"`
	Label       string `json:"label"`       // for humans only, never computable
	HostOnly    *bool  `json:"host_only"`   // determines whether or not the attribute will be published inside workload containers or exists only for Host use
	Publishable *bool  `json:"publishable"` // means sent to exchange or otherwise published; whether or not an attr ends up in a workload depends on the value of HostOnly
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
	return fmt.Sprintf("Id: %v, Type: %v, Label: %v, HostOnly: %v, Publishable: %v", a.Id, a.Type, a.Label, ho, pub)
}

// Update *selectively* updates the content of this AttributeMeta (m) with non-empty values in the given meta.
func (m *AttributeMeta) Update(meta AttributeMeta) {
	// cannot change id, others are replaced if not empty / nil
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

type ServiceSpec struct {
	Url string `json:"url"`
	Org string `json:"organization,omitempty"` // default is the node org
}

func NewServiceSpec(url string, org string) *ServiceSpec {
	return &ServiceSpec{
		Url: url,
		Org: org,
	}
}

type ServiceSpecs []ServiceSpec // empty if applicable to all services

type ServiceAttribute interface {
	GetServiceSpecs() *ServiceSpecs
}

func (s ServiceSpec) IsSame(sp ServiceSpec) bool {
	return s.Url == sp.Url && s.Org == sp.Org
}

// check if the two service spec arrays point to the same set of services.
// assume no duplicates in either of the array.
func (s ServiceSpecs) IsSame(sps ServiceSpecs) bool {
	if len(s) != len(sps) {
		return false
	}
	for _, s_me := range s {
		found := false
		for _, s_in := range sps {
			if s_me.IsSame(s_in) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// check if the service attrubute supports the given service
// If the ServiceSpecs is an empty array, it supports all services.
// Empty string for service or org means all.
func (s ServiceSpecs) SupportService(serviceUrl string, serviceOrg string) bool {
	if serviceUrl == "" {
		return true
	} else {
		if len(s) == 0 {
			return true
		} else {
			for _, sp := range s {
				if sp.Url == serviceUrl && (sp.Org == "" || sp.Org == serviceOrg) {
					return true
				}
			}
		}
	}
	return false
}

// important to use this for additions to prevent duplicates and keep slice ordered
func (s *ServiceSpecs) AppendServiceSpec(sp ServiceSpec) {

	contains := false
	for _, val := range *s {
		if val.IsSame(sp) {
			contains = true
		}
	}

	if !contains {
		(*s) = append(*s, sp)
	}
}

// update the service specs. no duplicates
func (s *ServiceSpecs) Update(sps ServiceSpecs) {
	for _, sp := range sps {
		contains := false
		for _, val := range *s {
			if val.IsSame(sp) {
				contains = true
			}
		}

		if !contains {
			(*s) = append(*s, sp)
		}
	}
}

func GetAttributeServiceSpecs(attribute *Attribute) *ServiceSpecs {
	if s, ok := (*attribute).(ServiceAttribute); ok {
		return s.GetServiceSpecs()
	} else {
		return nil
	}
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
	case "UserInputAttributes":
		var ui UserInputAttributes
		if err := json.Unmarshal(v, &ui); err != nil {
			return nil, err
		}
		attr = ui

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

		// for backward compatibility
	case "LocationAttributes", "ArchitectureAttributes", "ComputeAttributes", "PropertyAttributes":
		return nil, nil

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
				} else if attr == nil {
					return nil
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

// get all the attribute that the given this service can use.
// If the given serviceUrl is an empty string, all attributes will be returned.
// For an attribute, if the a.ServiceSpecs is empty, it will be included.
// Otherwise, if an element in the attrubute's ServiceSpecs array equals to ServiceSpec{serviceUrl, org}
// the attribute will be included.
func FindApplicableAttributes(db *bolt.DB, serviceUrl string, org string) ([]Attribute, error) {

	filteredAttrs := []Attribute{}

	return filteredAttrs, db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ATTRIBUTES))

		if bucket == nil {
			return nil
		}

		return bucket.ForEach(func(k, v []byte) error {
			attr, err := HydrateConcreteAttribute(v)
			if err != nil {
				return err
			} else if attr != nil {
				serviceSpecs := GetAttributeServiceSpecs(&attr)
				if serviceSpecs == nil {
					filteredAttrs = append(filteredAttrs, attr)
				} else if serviceSpecs.SupportService(serviceUrl, org) {
					filteredAttrs = append(filteredAttrs, attr)
				}
			}
			return nil
		})
	})
}

// This function is used to convert the persistent attributes for a service to an env var map.
// This will include *all* values for which HostOnly is false, include those marked to not publish.
func AttributesToEnvvarMap(attributes []Attribute, envvars map[string]string, prefix string, defaultRAM int64, nodePol *externalpolicy.ExternalPolicy, isCluster bool) (map[string]string, error) {

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

	// Write the default
	writePrefix("CPUS", strconv.FormatInt(1, 10))
	writePrefix("RAM", strconv.FormatInt(defaultRAM, 10))
	writePrefix("ARCH", cutil.ArchString())

	// Override with the built-in properties
	externalPol, externalPolReadWrite := externalpolicy.CreateNodeBuiltInPolicy(false, true, nodePol, isCluster)
	if externalPol != nil {
		externalPol.MergeWith(externalPolReadWrite, false)
	} else if externalPolReadWrite != nil {
		externalPol = externalPolReadWrite
	}
	if externalPol != nil {
		for _, ele := range externalPol.Properties {
			if ele.Name == externalpolicy.PROP_NODE_CPU {
				writePrefix("CPUS", strconv.FormatFloat(ele.Value.(float64), 'f', -1, 64))
			} else if ele.Name == externalpolicy.PROP_NODE_MEMORY {
				writePrefix("RAM", strconv.FormatFloat(ele.Value.(float64), 'f', -1, 64))
			} else if ele.Name == externalpolicy.PROP_NODE_ARCH {
				writePrefix("ARCH", ele.Value.(string))
			} else if ele.Name == externalpolicy.PROP_NODE_HARDWAREID {
				writePrefix("HARDWAREID", ele.Value.(string))
			} else if ele.Name == externalpolicy.PROP_NODE_PRIVILEGED {
				writePrefix("PRIVILEGED", fmt.Sprintf("%t", ele.Value))
			}
		}
	}

	// Set the Host IPv4 addresses, omit interfaces that are down.
	if ips, err := cutil.GetAllHostIPv4Addresses([]cutil.NetFilter{cutil.OmitDown}); err != nil {
		return nil, fmt.Errorf("error obtaining host IP addresses: %v", err)
	} else {
		writePrefix("HOST_IPS", strings.Join(ips, ","))
	}

	// TODO: consider extracting this type-processing out for generalization
	for _, serv := range attributes {
		meta := serv.GetMeta()
		if meta.HostOnly != nil && (*meta.HostOnly) {
			glog.V(4).Infof("Not creating an envvar from attribute %v b/c it's marked HostOnly", serv)
			continue
		}

		switch serv.(type) {
		case UserInputAttributes:
			s := serv.(UserInputAttributes)
			for k, v := range s.Mappings {
				cutil.NativeToEnvVariableMap(envvars, k, v)
			}

		case HAAttributes:
			s := serv.(HAAttributes)
			writePrefix("HA_PARTNERS", strings.Join(s.Partners, ","))

		case MeteringAttributes:
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
	serviceSpecs := GetAttributeServiceSpecs(attribute)

	if serviceSpecs == nil || len(*serviceSpecs) == 0 {
		common, err = FindApplicableAttributes(db, "", "")
		if err != nil {
			return nil, err
		}
	} else {
		for _, sp := range *serviceSpecs {
			common, err = FindApplicableAttributes(db, sp.Url, sp.Org)
			if err != nil {
				return nil, err
			}
		}
	}

	attributeMeta := (*attribute).GetMeta()
	attributeServiceSpecs := GetAttributeServiceSpecs(attribute)
	for _, possiblyConflicting := range common {
		conflictingMeta := possiblyConflicting.GetMeta()
		conflictingServiceMeta := GetAttributeServiceSpecs(&possiblyConflicting)
		if attributeMeta.Type == conflictingMeta.Type &&
			((conflictingServiceMeta == nil && attributeServiceSpecs == nil) || conflictingServiceMeta.IsSame(*attributeServiceSpecs)) &&
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

// N.B. It's the caller's responsibility to ensure the attr.ServiceSpecs are deduplicated; use the ServiceSpecs.AddServiceSpec() function to keep the slice clean
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
