package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/satori/go.uuid"
	"strings"
	"time"
)

// microdevice definition table name
const MICROSERVICE_INSTANCES = "microdevice_instances"
const MICROSERVICE_DEFINITIONS = "microdevice_definitions"

type UserInput struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	Type         string `json:"type"`
	DefaultValue string `json:"defaultValue"`
}

func NewUserInput(name string, label string, stype string, default_value string) *UserInput {
	return &UserInput{
		Name:         name,
		Label:        label,
		Type:         stype,
		DefaultValue: default_value,
	}
}

type WorkloadDeployment struct {
	Deployment          string `json:"deployment"`
	DeploymentSignature string `json:"deployment_signature"`
	Torrent             string `json:"torrent"`
}

func NewWorkloadDeployment(deployment string, deploy_sig string, torrent string) *WorkloadDeployment {
	return &WorkloadDeployment{
		Deployment:          deployment,
		DeploymentSignature: deploy_sig,
		Torrent:             torrent,
	}
}

type HardwareMatch struct {
	USBDeviceIds string `json:"usbDeviceIds"`
	Devfiles     string `json:"devFiles"`
}

func NewHardwareMatch(usb_dev_ids string, dev_files string) *HardwareMatch {
	return &HardwareMatch{
		USBDeviceIds: usb_dev_ids,
		Devfiles:     dev_files,
	}
}

type MicroserviceDefinition struct {
	Owner         string               `json:"owner"`
	Label         string               `json:"label"`
	Description   string               `json:"description"`
	SpecRef       string               `json:"specRef"`
	Version       string               `json:"version"`
	Arch          string               `json:"arch"`
	Sharable      string               `json:"sharable"`
	DownloadURL   string               `json:"downloadUrl"`
	MatchHardware HardwareMatch        `json:"matchHardware"`
	UserInputs    []UserInput          `json:"userInput"`
	Workloads     []WorkloadDeployment `json:"workloads"`
	LastUpdated   string               `json:"lastUpdated"`
}

func (w MicroserviceDefinition) String() string {
	return fmt.Sprintf("Owner: %v, "+
		"Label: %v, "+
		"Description: %v, "+
		"SpecRef: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"Sharable: %v, "+
		"DownloadURL: %v, "+
		"MatchHardware: %v, "+
		"UserInputs: %v, "+
		"Workloads: %v, "+
		"LastUpdated: %v",
		w.Owner, w.Label, w.Description, w.SpecRef, w.Version, w.Arch, w.Sharable, w.DownloadURL,
		w.MatchHardware, w.UserInputs, w.Workloads, w.LastUpdated)
}

// create a unique name for a microservice def
// If SpecRef is https://bluehorizon.network/microservices/network and version is 2.3.1,
// the output string will be "bluehorizon.network-microservices-network_2.3.1"
func (m MicroserviceDefinition) GetKey() string {
	s := m.SpecRef
	if strings.Contains(m.SpecRef, "://") {
		s = strings.Split(m.SpecRef, "://")[1]
	}
	new_s := strings.Replace(s, "/", "-", -1)

	return fmt.Sprintf("%v_%v", new_s, m.Version)
}

func NewMicroserviceDefinition(owner string, label string, description string, specRef string, version string, arch string, sharable string,
	download_url string, match_hardware HardwareMatch, user_inputs []UserInput, workloads []WorkloadDeployment, last_updated string) *MicroserviceDefinition {
	return &MicroserviceDefinition{
		Owner:         owner,
		Label:         label,
		Description:   description,
		SpecRef:       specRef,
		Version:       version,
		Arch:          arch,
		Sharable:      sharable,
		DownloadURL:   download_url,
		MatchHardware: match_hardware,
		UserInputs:    user_inputs,
		Workloads:     workloads,
		LastUpdated:   last_updated,
	}
}

// save the microservice record. update if it already exists in the db
func SaveOrUpdateMicroserviceDef(db *bolt.DB, msdef *MicroserviceDefinition) error {
	key := msdef.GetKey()

	writeErr := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(MICROSERVICE_DEFINITIONS))
		if err != nil {
			return err
		}

		serial, err := json.Marshal(*msdef)
		if err != nil {
			return fmt.Errorf("Failed to serialize microservice: %v. Error: %v", *msdef, err)
		}
		return bucket.Put([]byte(key), serial)
	})

	return writeErr
}

// find the microservice definition from the db
func FindMicroserviceDef(db *bolt.DB, url string, version string) (*MicroserviceDefinition, error) {
	var pms *MicroserviceDefinition
	pms = nil

	// fetch microservice instances
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(MICROSERVICE_DEFINITIONS)); b != nil {
			b.ForEach(func(k, v []byte) error {

				var ms MicroserviceDefinition

				if err := json.Unmarshal(v, &ms); err != nil {
					glog.Errorf("Unable to deserialize microservice db record: %v", v)
				} else if ms.SpecRef == url && ms.Version == version {
					pms = &ms
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
		return pms, nil
	}
}

// find the microservice definition from the db
func FindMicroserviceDefWithKey(db *bolt.DB, key string) (*MicroserviceDefinition, error) {
	var pms *MicroserviceDefinition
	pms = nil

	// fetch microservice definitions
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(MICROSERVICE_DEFINITIONS)); b != nil {
			v := b.Get([]byte(key))

			var ms MicroserviceDefinition

			if err := json.Unmarshal(v, &ms); err != nil {
				glog.Errorf("Unable to deserialize microservice_definition db record: %v. Error: %v", v, err)
				return err
			} else {
				pms = &ms
				return nil
			}
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return pms, nil
	}
}

// filter on MicroserviceDefinition
type MSFilter func(MicroserviceDefinition) bool

// filter for all microservice defs
func AllMSFilter() MSFilter {
	return func(e MicroserviceDefinition) bool { return true }
}

// filter for all the microservice defs for the given url
func AllDefsForUrlMSFilter(spec_url string) MSFilter {
	return func(e MicroserviceDefinition) bool {
		if e.SpecRef == spec_url {
			return true
		} else {
			return false
		}
	}
}

// find the microservice instance from the db
func FindMicroserviceDefs(db *bolt.DB, filters []MSFilter) ([]MicroserviceDefinition, error) {
	ms_defs := make([]MicroserviceDefinition, 0)

	// fetch contracts
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(MICROSERVICE_DEFINITIONS)); b != nil {
			b.ForEach(func(k, v []byte) error {

				var e MicroserviceDefinition

				if err := json.Unmarshal(v, &e); err != nil {
					glog.Errorf("Unable to deserialize db record: %v", v)
				} else {
					glog.V(5).Infof("Demarshalled microservice definition in DB: %v", e)
					exclude := false
					for _, filterFn := range filters {
						if !filterFn(e) {
							exclude = true
						}
					}
					if !exclude {
						ms_defs = append(ms_defs, e)
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
		return ms_defs, nil
	}
}

type MicroserviceInstance struct {
	SpecRef              string   `json:"ref_url"`
	Version              string   `json:"version"`
	Arch                 string   `json:"arch"`
	InstanceId           string   `json:"instance_id"`
	Archived             bool     `json:"archived"`
	InstanceCreationTime uint64   `json:"instance_creation_time"`
	ExecutionStartTime   uint64   `json:"execution_start_time"`
	ExecutionFailureCode uint     `json:"execution_failure_code"`
	ExecutionFailureDesc string   `json:"execution_failure_desc"`
	AssociatedAgreements []string `json:"associated_agreements"`
}

func (w MicroserviceInstance) String() string {
	return fmt.Sprintf("SpecRef: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"InstanceId: %v, "+
		"Archived: %v, "+
		"InstanceCreationTime: %v, "+
		"ExecutionStartTime: %v, "+
		"ExecutionFailureCode: %v, "+
		"ExecutionFailureDesc: %v, "+
		"AssociatedAgreements: %v",
		w.SpecRef, w.Version, w.Arch, w.InstanceId, w.Archived, w.InstanceCreationTime, w.ExecutionStartTime, w.ExecutionFailureCode, w.ExecutionFailureDesc, w.AssociatedAgreements)
}

// create a unique name for a microservice def
// If SpecRef is https://bluehorizon.network/microservices/network, version is 2.3.1 and the instance id is "abcd1234"
// the output string will be "bluehorizon.network-microservices-network_2.3.1_abcd1234"
func (m MicroserviceInstance) GetKey() string {
	s := m.SpecRef
	if strings.Contains(m.SpecRef, "://") {
		s = strings.Split(m.SpecRef, "://")[1]
	}
	new_s := strings.Replace(s, "/", "-", -1)

	return fmt.Sprintf("%v_%v_%v", new_s, m.Version, m.InstanceId)
}

// create a new microservice instance and save it to db.
func NewMicroserviceInstance(db *bolt.DB, ref_url string, version string) (*MicroserviceInstance, error) {

	if ref_url == "" || version == "" {
		return nil, errors.New("Microservice ref url id or version is empty, cannot persist")
	}

	instance_id := uuid.NewV4().String()

	if ms_instance, err := FindMicroserviceInstance(db, ref_url, version, instance_id); err != nil {
		return nil, err
	} else if ms_instance != nil {
		return nil, fmt.Errorf("Not expecting any records with SpecRef %v, version %v and instance id %v, found %v", ref_url, version, instance_id, ms_instance)
	}

	new_inst := &MicroserviceInstance{
		SpecRef:              ref_url,
		Version:              version,
		Arch:                 cutil.ArchString(),
		InstanceId:           instance_id,
		Archived:             false,
		InstanceCreationTime: uint64(time.Now().Unix()),
		ExecutionStartTime:   0, // execution started and running
		ExecutionFailureCode: 0,
		ExecutionFailureDesc: "",
		AssociatedAgreements: make([]string, 0),
	}

	return new_inst, db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(MICROSERVICE_INSTANCES)); err != nil {
			return err
		} else if bytes, err := json.Marshal(new_inst); err != nil {
			return fmt.Errorf("Unable to marshal new record: %v", err)
		} else if err := b.Put([]byte(new_inst.GetKey()), []byte(bytes)); err != nil {
			return fmt.Errorf("Unable to persist microservice instance: %v", err)
		}
		// success, close tx
		return nil
	})
}

// find the microservice instance from the db
func FindMicroserviceInstance(db *bolt.DB, url string, version string, instance_id string) (*MicroserviceInstance, error) {
	var pms *MicroserviceInstance
	pms = nil

	// fetch microservice instances
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(MICROSERVICE_INSTANCES)); b != nil {
			b.ForEach(func(k, v []byte) error {

				var ms MicroserviceInstance

				if err := json.Unmarshal(v, &ms); err != nil {
					glog.Errorf("Unable to deserialize microservice_instance db record: %v", v)
				} else if ms.SpecRef == url && ms.Version == version && ms.InstanceId == instance_id {
					pms = &ms
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
		return pms, nil
	}
}

// find the microservice instance from the db
func FindMicroserviceInstanceWithKey(db *bolt.DB, key string) (*MicroserviceInstance, error) {
	var pms *MicroserviceInstance
	pms = nil

	// fetch microservice instances
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(MICROSERVICE_INSTANCES)); b != nil {
			v := b.Get([]byte(key))

			var ms MicroserviceInstance

			if err := json.Unmarshal(v, &ms); err != nil {
				glog.Errorf("Unable to deserialize microservice_instance db record: %v. Error: %v", v, err)
				return err
			} else {
				pms = &ms
				return nil
			}
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return pms, nil
	}
}

// filter on MicroserviceInstance
type MIFilter func(MicroserviceInstance) bool

// filter for all microservice instances
func AllMIFilter() MIFilter {
	return func(e MicroserviceInstance) bool { return true }
}

// filter for all the microservice instances for the given url and version
func AllInstancesMIFilter(spec_url string, version string) MIFilter {
	return func(e MicroserviceInstance) bool {
		if e.SpecRef == spec_url && e.Version == version {
			return true
		} else {
			return false
		}
	}
}

func UnarchivedMIFilter() MIFilter {
	return func(e MicroserviceInstance) bool { return !e.Archived }
}

// find the microservice instance from the db
func FindMicroserviceInstances(db *bolt.DB, filters []MIFilter) ([]MicroserviceInstance, error) {
	ms_instances := make([]MicroserviceInstance, 0)

	// fetch contracts
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(MICROSERVICE_INSTANCES)); b != nil {
			b.ForEach(func(k, v []byte) error {

				var e MicroserviceInstance

				if err := json.Unmarshal(v, &e); err != nil {
					glog.Errorf("Unable to deserialize db record: %v", v)
				} else {
					glog.V(5).Infof("Demarshalled microservice instance in DB: %v", e)
					exclude := false
					for _, filterFn := range filters {
						if !filterFn(e) {
							exclude = true
						}
					}
					if !exclude {
						ms_instances = append(ms_instances, e)
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
		return ms_instances, nil
	}
}

// set microservice instance state to execution started or failed
func UpdateMSInstanceExecutionState(db *bolt.DB, key string, started bool, failure_code uint, failure_desc string) (*MicroserviceInstance, error) {
	if started {
		return microserviceInstancetStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
			c.ExecutionStartTime = uint64(time.Now().Unix())
			return &c
		})

	} else {
		return microserviceInstancetStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
			c.ExecutionFailureCode = failure_code
			c.ExecutionFailureDesc = failure_desc
			return &c
		})
	}
}

// add or delete an associated agreement id to/from the microservice instance in the db
func UpdateMSInstanceAssociaedAgreements(db *bolt.DB, key string, add bool, agreement_id string) (*MicroserviceInstance, error) {
	return microserviceInstancetStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		if c.AssociatedAgreements == nil {
			c.AssociatedAgreements = make([]string, 0)
		} else {
			// check existance
			for i, id := range c.AssociatedAgreements {
				if id == agreement_id {
					if !add { // remove
						c.AssociatedAgreements = append(c.AssociatedAgreements[:i], c.AssociatedAgreements[i+1:]...)
					}
					return &c
				}
			}

			// add
			if add {
				c.AssociatedAgreements = append(c.AssociatedAgreements, agreement_id)
			}
		}
		return &c
	})
}

func ArchiveMicroserviceInstance(db *bolt.DB, key string) (*MicroserviceInstance, error) {
	return microserviceInstancetStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		c.Archived = true
		return &c
	})
}

// update the micorserive instance
func microserviceInstancetStateUpdate(db *bolt.DB, key string, fn func(MicroserviceInstance) *MicroserviceInstance) (*MicroserviceInstance, error) {

	if ms, err := FindMicroserviceInstanceWithKey(db, key); err != nil {
		return nil, err
	} else if ms == nil {
		return nil, fmt.Errorf("No record with key: %v", key)
	} else {
		// run this single contract through provided update function and persist it
		updated := fn(*ms)
		return updated, persistUpdatedMicroserviceInstance(db, key, updated)
	}
}

// does whole-member replacements of values that are legal to change during the course of a contract's life
func persistUpdatedMicroserviceInstance(db *bolt.DB, key string, update *MicroserviceInstance) error {
	return db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(MICROSERVICE_INSTANCES)); err != nil {
			return err
		} else {
			current := b.Get([]byte(key))
			var mod MicroserviceInstance

			if current == nil {
				return fmt.Errorf("No microservice with given key available to update: %v", key)
			} else if err := json.Unmarshal(current, &mod); err != nil {
				return fmt.Errorf("Failed to unmarshal microservice DB data: %v. Error: %v", string(current), err)
			} else {

				// This code is running in a database transaction. Within the tx, the current record is
				// read and then updated according to the updates within the input update record. It is critical
				// to check for correct data transitions within the tx.
				if !mod.Archived { // 1 transition from false to true
					mod.Archived = update.Archived
				}
				if mod.InstanceCreationTime == 0 { // 1 transition from zero to non-zero
					mod.InstanceCreationTime = update.InstanceCreationTime
				}
				if mod.ExecutionStartTime == 0 {
					mod.ExecutionStartTime = update.ExecutionStartTime
				}
				if mod.ExecutionFailureCode == 0 {
					mod.ExecutionFailureCode = update.ExecutionFailureCode
				}
				if mod.ExecutionFailureDesc == "" {
					mod.ExecutionFailureDesc = update.ExecutionFailureDesc
				}

				mod.AssociatedAgreements = update.AssociatedAgreements

				if serialized, err := json.Marshal(mod); err != nil {
					return fmt.Errorf("Failed to serialize contract record: %v. Error: %v", mod, err)
				} else if err := b.Put([]byte(key), serialized); err != nil {
					return fmt.Errorf("Failed to write microservice instance with key: %v. Error: %v", key, err)
				} else {
					glog.V(2).Infof("Succeeded updating microservice instance record to %v", mod)
					return nil
				}
			}
		}
	})
}

// delete associated agreement id from all the microservice instances
func DeleteAsscAgmtsFromMSInstances(db *bolt.DB, agreement_id string) error {
	if ms_instances, err := FindMicroserviceInstances(db, []MIFilter{AllMIFilter(), UnarchivedMIFilter()}); err != nil {
		return fmt.Errorf("Error retrieving all microservice instances from database, error: %v", err)
	} else if ms_instances != nil {
		for _, msi := range ms_instances {
			if msi.AssociatedAgreements != nil && len(msi.AssociatedAgreements) > 0 {
				for _, id := range msi.AssociatedAgreements {
					if id == agreement_id {
						if _, err := UpdateMSInstanceAssociaedAgreements(db, msi.GetKey(), false, agreement_id); err != nil {
							return err
						}
						break
					}
				}
			}
		}
	}
	return nil
}

// delete a microservice instance from db. It will NOT return error if it does not exist in the db
func DeleteMicroserviceInstance(db *bolt.DB, key string) (*MicroserviceInstance, error) {

	if key == "" {
		return nil, errors.New("key is empty, cannot remove")
	} else {
		if ms, err := FindMicroserviceInstanceWithKey(db, key); err != nil {
			return nil, err
		} else if ms == nil {
			return nil, nil
		} else {
			return ms, db.Update(func(tx *bolt.Tx) error {

				if b, err := tx.CreateBucketIfNotExists([]byte(MICROSERVICE_INSTANCES)); err != nil {
					return err
				} else if err := b.Delete([]byte(key)); err != nil {
					return fmt.Errorf("Unable to delete microservice instance %v: %v", key, err)
				} else {
					return nil
				}
			})
		}
	}
}
