package persistence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"strconv"
	"strings"
)

// workload variable configuration table name
const WORKLOAD_CONFIG = "workload_config"

type WorkloadConfig struct {
	WorkloadURL       string                 `json:"workload_url"`
	VersionExpression string                 `json:"version"` // This is a version range
	Variables         map[string]interface{} `json:"variables"`
}

func (w WorkloadConfig) String() string {
	return fmt.Sprintf("WorkloadURL: %v, "+
		"VersionExpression: %v, "+
		"Variables: %v",
		w.WorkloadURL, w.VersionExpression, w.Variables)
}

func (w *WorkloadConfig) GetKey() string {
	catNull := func(str string) string {
		return fmt.Sprintf("%s\x00", str)
	}

	var sb bytes.Buffer
	sb.WriteString(catNull(w.WorkloadURL))
	sb.WriteString(w.VersionExpression)

	return sb.String()
}

// create a new workload config object and save it to db.
func NewWorkloadConfig(db *bolt.DB, workloadURL string, version string, variables map[string]interface{}) (*WorkloadConfig, error) {

	if workloadURL == "" || version == "" {
		return nil, errors.New("WorkloadConfig, workload URL or version is empty, cannot persist")
	}

	if wcfg, err := FindWorkloadConfig(db, workloadURL, version); err != nil {
		return nil, err
	} else if wcfg != nil {
		return nil, fmt.Errorf("Not expecting any records with WorkloadURL %v, and version %v, found %v", workloadURL, version, wcfg)
	}

	new_cfg := &WorkloadConfig{
		WorkloadURL:       workloadURL,
		VersionExpression: version,
		Variables:         variables,
	}

	return new_cfg, db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(WORKLOAD_CONFIG)); err != nil {
			return err
		} else if bytes, err := json.Marshal(new_cfg); err != nil {
			return fmt.Errorf("Unable to marshal new record: %v", err)
		} else if err := b.Put([]byte(new_cfg.GetKey()), []byte(bytes)); err != nil {
			return fmt.Errorf("Unable to persist workload config: %v", err)
		}
		// success, close tx
		return nil
	})
}

// find the workload config variables in the db
func FindWorkloadConfig(db *bolt.DB, url string, version string) (*WorkloadConfig, error) {
	var cfg *WorkloadConfig
	cfg = nil

	// fetch workload config objects
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(WORKLOAD_CONFIG)); b != nil {
			b.ForEach(func(k, v []byte) error {

				var w WorkloadConfig

				if err := json.Unmarshal(v, &w); err != nil {
					glog.Errorf("Unable to deserialize workload config db record: %v", v)
				} else if w.WorkloadURL == url && w.VersionExpression == version {
					cfg = &w
					return nil
				}
				return nil
			})
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return cfg, nil
	}
}

// filter on WorkloadConfig
type WCFilter func(WorkloadConfig) bool

// filter for all workload config objects
func AllWCFilter() WCFilter {
	return func(e WorkloadConfig) bool { return true }
}

// filter for all the workload config objects for the given url
func AllWorkloadWCFilter(workload_url string) WCFilter {
	return func(e WorkloadConfig) bool {
		if e.WorkloadURL == workload_url {
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

		if b := tx.Bucket([]byte(WORKLOAD_CONFIG)); b != nil {
			b.ForEach(func(k, v []byte) error {

				var e WorkloadConfig

				if err := json.Unmarshal(v, &e); err != nil {
					glog.Errorf("Unable to deserialize db record: %v", v)
				} else {
					glog.V(5).Infof("Demarshalled workload config object in DB: %v", e)
					exclude := false
					for _, filterFn := range filters {
						if !filterFn(e) {
							exclude = true
						}
					}
					if !exclude {
						cfg_instances = append(cfg_instances, e)
					}
				}
				return nil
			})
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return cfg_instances, nil
	}
}

func DeleteWorkloadConfig(db *bolt.DB, url string, version string) error {

	if url == "" || version == "" {
		return errors.New("workload URL or version is empty, cannot delete")
	} else {

		if cfg, err := FindWorkloadConfig(db, url, version); err != nil {
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

// Sorting functions used with the sort package
type WorkloadConfigByVersion []WorkloadConfig

func (s WorkloadConfigByVersion) Len() int {
	return len(s)
}

func (s WorkloadConfigByVersion) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s WorkloadConfigByVersion) Less(i, j int) bool {

	// Just compare the starting version in the two ranges
	first := s[i].VersionExpression[1:strings.Index(s[i].VersionExpression,",")]
	second := s[j].VersionExpression[1:strings.Index(s[j].VersionExpression,",")]
	return strings.Compare(first, second) == -1
}

// Grab configured userInput variables for the workload and pass them into the
// workload container. The namespace of these env vars is defined by the workload
// so there is no need for us to prefix them with the HZN prefix.
func ConfigToEnvvarMap(db *bolt.DB, cfg *WorkloadConfig, prefix string) (map[string]string, error) {

	pf := func(str string, prefix string) string {
		return fmt.Sprintf("%v%v", prefix, str)
	}

	envvars := map[string]string{}

	// Get the location attributes and set them into the envvar map. We think this is a
	// temporary measure until all workloads are taught to use a GPS microservice.
	if allAttrs, err := FindApplicableAttributes(db, ""); err != nil {
		return nil, err
	} else {
		for _, attr := range allAttrs {

			// Extract location property
			switch attr.(type) {
			case LocationAttributes:
				s := attr.(LocationAttributes)
				envvars[pf("LAT", prefix)] = s.Lat
				envvars[pf("LON", prefix)] = s.Lon
			case ComputeAttributes:
				s := attr.(ComputeAttributes)
				envvars[pf("CPUS", prefix)] = strconv.FormatInt(s.CPUs, 10)
				envvars[pf("RAM", prefix)] = strconv.FormatInt(s.RAM, 10)
			case ArchitectureAttributes:
				s := attr.(ArchitectureAttributes)
				envvars[pf("ARCH", prefix)] = s.Architecture
			}
		}
	}

	if cfg == nil {
		return envvars, nil
	}

	// workload config values are saved as strings except for the list of strings case.
	for v, varValue := range cfg.Variables {
		switch varValue.(type) {
		case []interface{}:
			los := ""
			for _, e := range varValue.([]interface{}) {
				if _, ok := e.(string); ok {
					los = los + e.(string) + " "
				}
			}
			los = los[:len(los)-1]
			envvars[v] = los
		default:
			envvars[v] = varValue.(string)
		}
	}

	return envvars, nil
}
