package persistence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
)

// workload variable configuration table name
const WORKLOAD_CONFIG = "workload_config"

type WorkloadConfig struct {
	WorkloadURL       string      `json:"workload_url"`
	Org               string      `json:"organization"`
	VersionExpression string      `json:"workload_version"` // This is a version range
	Attributes        []Attribute `json:"attributes"`
}

func (w WorkloadConfig) String() string {
	return fmt.Sprintf("WorkloadURL: %v, "+
		"Org: %v, "+
		"VersionExpression: %v, "+
		"Attributes: %v",
		w.WorkloadURL, w.Org, w.VersionExpression, w.Attributes)
}

func (w *WorkloadConfig) GetKey() string {
	catNull := func(str string) string {
		return fmt.Sprintf("%s\x00", str)
	}

	var sb bytes.Buffer
	sb.WriteString(catNull(w.WorkloadURL))
	sb.WriteString(catNull(w.Org))
	sb.WriteString(w.VersionExpression)

	return sb.String()
}

// create a new workload config object and save it to db.
func NewWorkloadConfig(db *bolt.DB, workloadURL string, org string, version string, variables []Attribute) (*WorkloadConfig, error) {

	if workloadURL == "" || org == "" || version == "" {
		return nil, errors.New("WorkloadConfig, workload URL, organization, or version is empty, cannot persist")
	}

	if wcfg, err := FindWorkloadConfig(db, workloadURL, org, version); err != nil {
		return nil, err
	} else if wcfg != nil {
		return nil, fmt.Errorf("Not expecting any records with WorkloadURL %v, org %v, and version %v, found %v", workloadURL, org, version, wcfg)
	}

	new_cfg := &WorkloadConfig{
		WorkloadURL:       workloadURL,
		Org:               org,
		VersionExpression: version,
		Attributes:        variables,
	}

	return new_cfg, db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(WORKLOAD_CONFIG)); err != nil {
			return err
		} else if bytes, err := json.Marshal(new_cfg); err != nil {
			return fmt.Errorf("Unable to marshal new record: %v", err)
		} else if err := b.Put([]byte(new_cfg.GetKey()), []byte(bytes)); err != nil {
			return fmt.Errorf("Unable to persist workload config: %v", err)
		} else {
			glog.Infof("serialized to db record: %v", string(bytes))
		}
		// success, close tx
		return nil
	})
}

// Used to assist in demarshalling just the workload config object. The attributes stored with the object
// could be of various types and schemas.
type WorkloadConfigOnly struct {
	WorkloadURL       string                   `json:"workload_url"`
	Org               string                   `json:"organization"`
	VersionExpression string                   `json:"workload_version"` // This is a version range
	Attributes        []map[string]interface{} `json:"attributes"`
}

func hydrateWorkloadConfig(cfgOnly *WorkloadConfigOnly) (*WorkloadConfig, error) {
	if cfgOnly == nil {
		return nil, nil
	}
	attrList := make([]Attribute, 0, 10)
	for _, intf := range cfgOnly.Attributes {
		if sa, err := json.Marshal(intf); err != nil {
			glog.Errorf("Unable to serialize workload config attribute %v, error %v", intf, err)
			return nil, err
		} else if attr, err := hydrateConcreteAttribute(sa); err != nil {
			glog.Errorf("Unable to hydrate workload config attribute %s, error %v", sa, err)
			return nil, err
		} else {
			attrList = append(attrList, attr)
		}
	}
	return &WorkloadConfig{
		WorkloadURL:       cfgOnly.WorkloadURL,
		Org:               cfgOnly.Org,
		VersionExpression: cfgOnly.VersionExpression,
		Attributes:        attrList,
	}, nil
}

// find the workload config variables in the db
func FindWorkloadConfig(db *bolt.DB, url string, org string, version string) (*WorkloadConfig, error) {
	var cfg *WorkloadConfig

	// fetch workload config objects
	readErr := db.View(func(tx *bolt.Tx) error {

		var cfgOnly *WorkloadConfigOnly

		if b := tx.Bucket([]byte(WORKLOAD_CONFIG)); b != nil {
			err := b.ForEach(func(k, v []byte) error {

				var w WorkloadConfigOnly

				if err := json.Unmarshal(v, &w); err != nil {
					glog.Errorf("Unable to deserialize workload config db record %v, error %v", string(v), err)
					return err
				} else if w.WorkloadURL == url && w.Org == org && w.VersionExpression == version {
					cfgOnly = &w
					return nil
				}
				return nil
			})
			if err != nil {
				return err
			}
		}

		// If we found an eligible object, deserialize the attribute list
		var err error
		cfg, err = hydrateWorkloadConfig(cfgOnly)

		return err // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return cfg, nil
	}
}

// filter on WorkloadConfig
type WCFilter func(WorkloadConfigOnly) bool

// filter for all workload config objects
func AllWCFilter() WCFilter {
	return func(e WorkloadConfigOnly) bool { return true }
}

// filter for all the workload config objects for the given url
func AllWorkloadWCFilter(workload_url string, org string) WCFilter {
	return func(e WorkloadConfigOnly) bool {
		if e.WorkloadURL == workload_url && e.Org == org {
			return true
		} else {
			return false
		}
	}
}

// find the microservice instance from the db
func FindWorkloadConfigs(db *bolt.DB, filters []WCFilter) ([]WorkloadConfig, error) {
	cfg_instances := make([]WorkloadConfig, 0)

	// fetch contracts
	readErr := db.View(func(tx *bolt.Tx) error {

		cfgOnly_instances := make([]WorkloadConfigOnly, 0)

		if b := tx.Bucket([]byte(WORKLOAD_CONFIG)); b != nil {
			err := b.ForEach(func(k, v []byte) error {

				var e WorkloadConfigOnly

				if err := json.Unmarshal(v, &e); err != nil {
					glog.Errorf("Unable to deserialize db record: %v", v)
					return err
				} else {
					glog.V(5).Infof("Demarshalled workload config object in DB: %v", e)
					exclude := false
					for _, filterFn := range filters {
						if !filterFn(e) {
							exclude = true
						}
					}
					if !exclude {
						cfgOnly_instances = append(cfgOnly_instances, e)
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		}

		// If we found eligible objects, deserialize the attribute list for each one
		for _, cfgOnly := range cfgOnly_instances {
			if cfg, err := hydrateWorkloadConfig(&cfgOnly); err != nil {
				return err
			} else {
				cfg_instances = append(cfg_instances, *cfg)
			}
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return cfg_instances, nil
	}
}

func DeleteWorkloadConfig(db *bolt.DB, url string, org string, version string) error {

	if url == "" || version == "" {
		return errors.New("workload URL or version is empty, cannot delete")
	} else {

		if cfg, err := FindWorkloadConfig(db, url, org, version); err != nil {
			return err
		} else if cfg == nil {
			return fmt.Errorf("could not find record for %v and %v", url, version)
		} else {

			return db.Update(func(tx *bolt.Tx) error {

				if b, err := tx.CreateBucketIfNotExists([]byte(WORKLOAD_CONFIG)); err != nil {
					return err
				} else if err := b.Delete([]byte(cfg.GetKey())); err != nil {
					return fmt.Errorf("Unable to delete workload config: %v", err)
				} else {
					return nil
				}
			})
		}
	}
}
