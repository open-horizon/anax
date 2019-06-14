package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/satori/go.uuid"
	"strconv"
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
}

func NewWorkloadDeployment(deployment string, deploy_sig string) *WorkloadDeployment {
	return &WorkloadDeployment{
		Deployment:          deployment,
		DeploymentSignature: deploy_sig,
	}
}

type HardwareMatch map[string]interface{}

type ServiceDependency struct {
	URL     string `json:"url"`
	Org     string `json:"org"`
	Version string `json:"version"`
	Arch    string `json:"arch"`
}

func NewServiceDependency(url string, org string, version string, arch string) *ServiceDependency {
	return &ServiceDependency{
		URL:     url,
		Org:     org,
		Version: version,
		Arch:    arch,
	}
}

type MicroserviceDefinition struct {
	Id                           string               `json:"record_id"` // unique primary key for records
	Owner                        string               `json:"owner"`
	Label                        string               `json:"label"`
	Description                  string               `json:"description"`
	SpecRef                      string               `json:"specRef"`
	Org                          string               `json:"organization"`
	Version                      string               `json:"version"`
	Arch                         string               `json:"arch"`
	Sharable                     string               `json:"sharable"`
	DownloadURL                  string               `json:"downloadUrl"`
	MatchHardware                HardwareMatch        `json:"matchHardware"`
	UserInputs                   []UserInput          `json:"userInput"`
	Workloads                    []WorkloadDeployment `json:"workloads"`            // Only used by old microservice definitions
	Public                       bool                 `json:"public"`               // Used by only services, indicates if the definition is public or not.
	RequiredServices             []ServiceDependency  `json:"requiredServices"`     // Used only by services, the list of services that this service depends on.
	Deployment                   string               `json:"deployment"`           // Used only by services, the deployment configuration of the implementation packages.
	DeploymentSignature          string               `json:"deployment_signature"` // Used only by services, the signature of the deployment configuration.
	LastUpdated                  string               `json:"lastUpdated"`
	Archived                     bool                 `json:"archived"`
	Name                         string               `json:"name"`                  //the sensor_name passed in from the POST /service call
	RequestedArch                string               `json:"requested_arch"`        //the arch from user input or from the ms referenced by a workload, it can be a synonym of the node arch.
	UpgradeVersionRange          string               `json:"upgrade_version_range"` //the sensor_version passed in from the POST service call
	AutoUpgrade                  bool                 `json:"auto_upgrade"`          // passed in from the POST service call
	ActiveUpgrade                bool                 `json:"active_upgrade"`        // passed in from the POST service call
	UpgradeStartTime             uint64               `json:"upgrade_start_time"`
	UpgradeMsUnregisteredTime    uint64               `json:"upgrade_ms_unregistered_time"`
	UpgradeAgreementsClearedTime uint64               `json:"upgrade_agreements_cleared_time"`
	UpgradeExecutionStartTime    uint64               `json:"upgrade_execution_start_time"`
	UpgradeMsReregisteredTime    uint64               `json:"upgrade_ms_reregistered_time"`
	UpgradeFailedTime            uint64               `json:"upgrade_failed_time"`
	UngradeFailureReason         uint64               `json:"upgrade_failure_reason"`
	UngradeFailureDescription    string               `json:"upgrade_failure_description"`
	UpgradeNewMsId               string               `json:"upgrade_new_ms_id"`
	MetadataHash                 []byte               `json:"metadata_hash"` // the hash of the whole exchange.MicroserviceDefinition

}

func (w MicroserviceDefinition) String() string {
	return fmt.Sprintf("ID: %v, "+
		"Owner: %v, "+
		"Label: %v, "+
		"Description: %v, "+
		"SpecRef: %v, "+
		"Org: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"Sharable: %v, "+
		"DownloadURL: %v, "+
		"MatchHardware: %v, "+
		"UserInputs: %v, "+
		"Workloads: %v, "+
		"Public: %v, "+
		"RequiredServices: %v, "+
		"Deployment: %v, "+
		"DeploymentSignature: %v, "+
		"LastUpdated: %v, "+
		"Archived: %v, "+
		"Name: %v, "+
		"RequestedArch: %v, "+
		"UpgradeVersionRange: %v, "+
		"AutoUpgrade: %v, "+
		"ActiveUpgrade: %v, "+
		"UpgradeStartTime: %v, "+
		"UpgradeMsUnregisteredTime: %v, "+
		"UpgradeAgreementsClearedTime: %v, "+
		"UpgradeExecutionStartTime: %v, "+
		"UpgradeMsReregisteredTime: %v, "+
		"UpgradeFailedTime: %v, "+
		"UngradeFailureReason: %v, "+
		"UngradeFailureDescription: %v, "+
		"UpgradeNewMsId: %v, "+
		"MetadataHash: %v",
		w.Id, w.Owner, w.Label, w.Description, w.SpecRef, w.Org, w.Version, w.Arch, w.Sharable, w.DownloadURL,
		w.MatchHardware, w.UserInputs, w.Workloads, w.Public, w.RequiredServices, w.Deployment, w.DeploymentSignature, w.LastUpdated,
		w.Archived, w.Name, w.RequestedArch, w.UpgradeVersionRange, w.AutoUpgrade, w.ActiveUpgrade,
		w.UpgradeStartTime, w.UpgradeMsUnregisteredTime, w.UpgradeAgreementsClearedTime, w.UpgradeExecutionStartTime, w.UpgradeMsReregisteredTime,
		w.UpgradeFailedTime, w.UngradeFailureReason, w.UngradeFailureDescription, w.UpgradeNewMsId, w.MetadataHash)
}

func (w MicroserviceDefinition) ShortString() string {
	return fmt.Sprintf("Owner: %v, "+
		"Label: %v, "+
		"Description: %v, "+
		"SpecRef: %v, "+
		"Org: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"Archived: %v, "+
		"Name: %v, "+
		"RequestedArch: %v, "+
		"UpgradeVersionRange: %v, "+
		"AutoUpgrade: %v, "+
		"ActiveUpgrade: %v, "+
		"UpgradeStartTime: %v, "+
		"UpgradeMsUnregisteredTime: %v, "+
		"UpgradeAgreementsClearedTime: %v, "+
		"UpgradeExecutionStartTime: %v, "+
		"UpgradeMsReregisteredTime: %v, "+
		"UpgradeFailedTime: %v, "+
		"UngradeFailureReason: %v, "+
		"UngradeFailureDescription: %v, "+
		"UpgradeNewMsId: %v, "+
		"MetadataHash: %v",
		w.Owner, w.Label, w.Description, w.SpecRef, w.Org, w.Version, w.Arch,
		w.Archived, w.Name, w.RequestedArch, w.UpgradeVersionRange, w.AutoUpgrade, w.ActiveUpgrade,
		w.UpgradeStartTime, w.UpgradeMsUnregisteredTime, w.UpgradeAgreementsClearedTime, w.UpgradeExecutionStartTime, w.UpgradeMsReregisteredTime,
		w.UpgradeFailedTime, w.UngradeFailureReason, w.UngradeFailureDescription, w.UpgradeNewMsId, w.MetadataHash)
}

func (m *MicroserviceDefinition) HasDeployment() bool {
	if (m.Workloads == nil || len(m.Workloads) == 0) && m.Deployment == "" {
		return false
	}
	return true
}

// Returns the deployment string and signature of a ms def.
func (m *MicroserviceDefinition) GetDeployment() (string, string) {
	if m.HasDeployment() {
		// Microservice definitions never have more than 1 element in the workloads array.
		if m.Workloads != nil && len(m.Workloads) > 0 {
			return m.Workloads[0].Deployment, m.Workloads[0].DeploymentSignature
		} else if m.Deployment != "" {
			return m.Deployment, m.DeploymentSignature
		}
	}
	return "", ""
}

func (m *MicroserviceDefinition) NeedsUserInput() string {
	for _, ui := range m.UserInputs {
		if ui.DefaultValue == "" {
			return ui.Name
		}
	}
	return ""
}

func (w *MicroserviceDefinition) GetUserInputName(name string) *UserInput {
	for _, ui := range w.UserInputs {
		if ui.Name == name {
			return &ui
		}
	}
	return nil
}

func (m *MicroserviceDefinition) HasRequiredServices() bool {
	return len(m.RequiredServices) != 0
}

// save the microservice record. update if it already exists in the db
func SaveOrUpdateMicroserviceDef(db *bolt.DB, msdef *MicroserviceDefinition) error {
	writeErr := db.Update(func(tx *bolt.Tx) error {
		if bucket, err := tx.CreateBucketIfNotExists([]byte(MICROSERVICE_DEFINITIONS)); err != nil {
			return err
		} else if nextKey, err := bucket.NextSequence(); err != nil {
			return fmt.Errorf("Unable to get sequence key for new msdef %v. Error: %v", msdef, err)
		} else {
			strKey := strconv.FormatUint(nextKey, 10)
			msdef.Id = strKey

			glog.V(5).Infof("saving service definition %v to db", *msdef)

			serial, err := json.Marshal(*msdef)
			if err != nil {
				return fmt.Errorf("Failed to serialize service: %v. Error: %v", *msdef, err)
			}
			return bucket.Put([]byte(strKey), serial)
		}
	})

	return writeErr
}

// find the unarchived microservice definitions for the given url and org
func FindUnarchivedMicroserviceDefs(db *bolt.DB, url string, org string) ([]MicroserviceDefinition, error) {
	return FindMicroserviceDefs(db, []MSFilter{UnarchivedMSFilter(), UrlOrgMSFilter(url, org)})
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
				glog.Errorf("Unable to deserialize service definition db record: %v. Error: %v", v, err)
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

// filter for all unarchived msdefs
func UnarchivedMSFilter() MSFilter {
	return func(e MicroserviceDefinition) bool { return !e.Archived }
}

// filter for all archived msdefs
func ArchivedMSFilter() MSFilter {
	return func(e MicroserviceDefinition) bool { return e.Archived }
}

// filter on the url + version + org
func UrlOrgVersionMSFilter(spec_url string, org string, version string) MSFilter {
	return func(e MicroserviceDefinition) bool {
		return (e.SpecRef == spec_url && e.Org == org && e.Version == version)
	}
}

// filter on the url + + org
func UrlOrgMSFilter(spec_url string, org string) MSFilter {
	return func(e MicroserviceDefinition) bool {
		return (e.SpecRef == spec_url && e.Org == org)
	}
}

// filter for all the microservice defs for the given url
func UrlMSFilter(spec_url string) MSFilter {
	return func(e MicroserviceDefinition) bool { return (e.SpecRef == spec_url) }
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
					glog.V(5).Infof("Demarshalled service definition in DB: %v", e.ShortString())
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

// set the msdef to archived
func MsDefArchived(db *bolt.DB, key string) (*MicroserviceDefinition, error) {
	return microserviceDefStateUpdate(db, key, func(c MicroserviceDefinition) *MicroserviceDefinition {
		c.Archived = true
		return &c
	})
}

// set the msdef to un-archived
func MsDefUnarchived(db *bolt.DB, key string) (*MicroserviceDefinition, error) {
	return microserviceDefStateUpdate(db, key, func(c MicroserviceDefinition) *MicroserviceDefinition {
		c.Archived = false
		return &c
	})
}

func MSDefUpgradeStarted(db *bolt.DB, key string) (*MicroserviceDefinition, error) {
	return microserviceDefStateUpdate(db, key, func(c MicroserviceDefinition) *MicroserviceDefinition {
		c.UpgradeStartTime = uint64(time.Now().Unix())
		return &c
	})
}

func MSDefUpgradeMsUnregistered(db *bolt.DB, key string) (*MicroserviceDefinition, error) {
	return microserviceDefStateUpdate(db, key, func(c MicroserviceDefinition) *MicroserviceDefinition {
		c.UpgradeMsUnregisteredTime = uint64(time.Now().Unix())
		return &c
	})
}

func MsDefUpgradeAgreementsCleared(db *bolt.DB, key string) (*MicroserviceDefinition, error) {
	return microserviceDefStateUpdate(db, key, func(c MicroserviceDefinition) *MicroserviceDefinition {
		c.UpgradeAgreementsClearedTime = uint64(time.Now().Unix())
		return &c
	})
}

func MSDefUpgradeExecutionStarted(db *bolt.DB, key string) (*MicroserviceDefinition, error) {
	return microserviceDefStateUpdate(db, key, func(c MicroserviceDefinition) *MicroserviceDefinition {
		c.UpgradeExecutionStartTime = uint64(time.Now().Unix())
		return &c
	})
}

func MSDefUpgradeMsReregistered(db *bolt.DB, key string) (*MicroserviceDefinition, error) {
	return microserviceDefStateUpdate(db, key, func(c MicroserviceDefinition) *MicroserviceDefinition {
		c.UpgradeMsReregisteredTime = uint64(time.Now().Unix())
		return &c
	})
}

func MSDefUpgradeFailed(db *bolt.DB, key string, reason uint64, reasonString string) (*MicroserviceDefinition, error) {
	return microserviceDefStateUpdate(db, key, func(c MicroserviceDefinition) *MicroserviceDefinition {
		c.UpgradeFailedTime = uint64(time.Now().Unix())
		c.UngradeFailureReason = reason
		c.UngradeFailureDescription = reasonString
		return &c
	})
}

func MSDefUpgradeNewMsId(db *bolt.DB, key string, new_id string) (*MicroserviceDefinition, error) {
	return microserviceDefStateUpdate(db, key, func(c MicroserviceDefinition) *MicroserviceDefinition {
		c.UpgradeNewMsId = new_id
		return &c
	})
}

func MSDefNewUpgradeVersionRange(db *bolt.DB, key string, version_range string) (*MicroserviceDefinition, error) {
	return microserviceDefStateUpdate(db, key, func(c MicroserviceDefinition) *MicroserviceDefinition {
		c.UpgradeVersionRange = version_range
		return &c
	})
}

// update the micorserive definition
func microserviceDefStateUpdate(db *bolt.DB, key string, fn func(MicroserviceDefinition) *MicroserviceDefinition) (*MicroserviceDefinition, error) {

	if ms, err := FindMicroserviceDefWithKey(db, key); err != nil {
		return nil, err
	} else if ms == nil {
		return nil, fmt.Errorf("No record with key: %v", key)
	} else {
		// run this single contract through provided update function and persist it
		updated := fn(*ms)
		return updated, persistUpdatedMicroserviceDef(db, key, updated)
	}
}

// does whole-member replacements of values that are legal to change
func persistUpdatedMicroserviceDef(db *bolt.DB, key string, update *MicroserviceDefinition) error {
	return db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(MICROSERVICE_DEFINITIONS)); err != nil {
			return err
		} else {
			current := b.Get([]byte(key))
			var mod MicroserviceDefinition

			if current == nil {
				return fmt.Errorf("No service with given key available to update: %v", key)
			} else if err := json.Unmarshal(current, &mod); err != nil {
				return fmt.Errorf("Failed to unmarshal service DB data: %v. Error: %v", string(current), err)
			} else {

				// This code is running in a database transaction. Within the tx, the current record is
				// read and then updated according to the updates within the input update record. It is critical
				// to check for correct data transitions within the tx.
				if mod.UpgradeStartTime == 0 { // 1 transition from zero to non-zero
					mod.UpgradeStartTime = update.UpgradeStartTime
				}
				if mod.UpgradeMsUnregisteredTime == 0 {
					mod.UpgradeMsUnregisteredTime = update.UpgradeMsUnregisteredTime
				}
				if mod.UpgradeAgreementsClearedTime == 0 {
					mod.UpgradeAgreementsClearedTime = update.UpgradeAgreementsClearedTime
				}
				if mod.UpgradeExecutionStartTime == 0 {
					mod.UpgradeExecutionStartTime = update.UpgradeExecutionStartTime
				}
				if mod.UpgradeMsReregisteredTime == 0 {
					mod.UpgradeMsReregisteredTime = update.UpgradeMsReregisteredTime
				}
				if mod.UpgradeFailedTime == 0 {
					mod.UpgradeFailedTime = update.UpgradeFailedTime
				}
				if mod.UngradeFailureReason == 0 {
					mod.UngradeFailureReason = update.UngradeFailureReason
				}
				if mod.UngradeFailureDescription == "" {
					mod.UngradeFailureDescription = update.UngradeFailureDescription
				}

				if mod.Archived != update.Archived {
					mod.Archived = update.Archived
				}

				if mod.UpgradeNewMsId != update.UpgradeNewMsId {
					mod.UpgradeNewMsId = update.UpgradeNewMsId
				}

				if mod.UpgradeVersionRange != update.UpgradeVersionRange {
					mod.UpgradeVersionRange = update.UpgradeVersionRange
				}

				if serialized, err := json.Marshal(mod); err != nil {
					return fmt.Errorf("Failed to serialize contract record: %v. Error: %v", mod, err)
				} else if err := b.Put([]byte(key), serialized); err != nil {
					return fmt.Errorf("Failed to write service definition %v version %v key %v. Error: %v", mod.SpecRef, mod.Version, key, err)
				} else {
					glog.V(2).Infof("Succeeded updating service definition record to %v", mod.ShortString())
					return nil
				}
			}
		}
	})
}

// ==========================================================================================================
// Service/Microservice instance object
//

type MicroserviceInstance struct {
	SpecRef              string                         `json:"ref_url"`
	Org                  string                         `json:"organization"`
	Version              string                         `json:"version"`
	Arch                 string                         `json:"arch"`
	InstanceId           string                         `json:"instance_id"`
	Archived             bool                           `json:"archived"`
	InstanceCreationTime uint64                         `json:"instance_creation_time"`
	ExecutionStartTime   uint64                         `json:"execution_start_time"`
	ExecutionFailureCode uint                           `json:"execution_failure_code"`
	ExecutionFailureDesc string                         `json:"execution_failure_desc"`
	CleanupStartTime     uint64                         `json:"cleanup_start_time"`
	AssociatedAgreements []string                       `json:"associated_agreements"`
	MicroserviceDefId    string                         `json:"microservicedef_id"`
	ParentPath           [][]ServiceInstancePathElement `json:"service_instance_path"` // Set when instance is created
	AgreementLess        bool                           `json:"agreement_less"`        // Set when the service instance was started because it is an agreement-less service (as defined in the pattern)
	MaxRetries           uint                           `json:"max_retries"`           // maximum retries allowed
	MaxRetryDuration     uint                           `json:"max_retry_duration"`    // The number of seconds in which the specified number of retries must occur in order for next retry cycle.
	CurrentRetryCount    uint                           `json:"current_retry_count"`
	RetryStartTime       uint64                         `json:"retry_start_time"`
	EnvVars              map[string]string              `json:"env_vars"`
}

func (w MicroserviceInstance) String() string {
	return fmt.Sprintf("SpecRef: %v, "+
		"Org: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"InstanceId: %v, "+
		"Archived: %v, "+
		"InstanceCreationTime: %v, "+
		"ExecutionStartTime: %v, "+
		"ExecutionFailureCode: %v, "+
		"ExecutionFailureDesc: %v, "+
		"CleanupStartTime: %v, "+
		"AssociatedAgreements: %v, "+
		"MicroserviceDefId: %v, "+
		"ParentPath: %v, "+
		"AgreementLess: %v, "+
		"MaxRetries: %v, "+
		"MaxRetryDuration: %v, "+
		"CurrentRetryCount: %v, "+
		"RetryStartTime: %v, "+
		"EnvVars: %v",
		w.SpecRef, w.Org, w.Version, w.Arch, w.InstanceId, w.Archived, w.InstanceCreationTime,
		w.ExecutionStartTime, w.ExecutionFailureCode, w.ExecutionFailureDesc,
		w.CleanupStartTime, w.AssociatedAgreements, w.MicroserviceDefId, w.ParentPath, w.AgreementLess,
		w.MaxRetries, w.MaxRetryDuration, w.CurrentRetryCount, w.RetryStartTime, w.EnvVars)
}

// create a unique name for a microservice def
// If SpecRef is https://bluehorizon.network/microservices/network, Org is myorg, version is 2.3.1 and the instance id is "abcd1234"
// the output string will be "myorg_bluehorizon.network-microservices-network_2.3.1_abcd1234"
func (m MicroserviceInstance) GetKey() string {
	return cutil.MakeMSInstanceKey(m.SpecRef, m.Org, m.Version, m.InstanceId)
}

// Check if this microservice instance has a container dpeloyment.
// If it does not, then there is no nothing to execute.
func (m MicroserviceInstance) HasWorkload(db *bolt.DB) (bool, error) {
	if msdef, err := FindMicroserviceDefWithKey(db, m.MicroserviceDefId); err != nil {
		return false, err
	} else if msdef.HasDeployment() {
		return true, nil
	}

	return false, nil
}

// Check if this microservice instance has the given service as a direct parent.
func (m *MicroserviceInstance) HasDirectParent(parent *ServiceInstancePathElement) bool {
	for _, pathList := range m.ParentPath {
		for ix, element := range pathList {
			if parent != nil && parent.IsSame(&element) && (len(pathList) > (ix + 1)) && (&pathList[ix+1]).IsSame(NewServiceInstancePathElement(m.SpecRef, m.Org, m.Version)) {
				return true
			}
		}
	}
	return false
}

// It returns a an array of direct parents for this service instance.
func (m *MicroserviceInstance) GetDirectParents() []ServiceInstancePathElement {
	parents := make([]ServiceInstancePathElement, 0)

	for _, pathList := range m.ParentPath {
		if len(pathList) > 1 {
			item := pathList[len(pathList)-2]

			// no duplicates
			found := false
			for _, s := range parents {
				if s.IsSame(&item) {
					found = true
					break
				}
			}

			if !found {
				parents = append(parents, item)
			}
		}
	}
	return parents
}

// create a new microservice instance and save it to db.
func NewMicroserviceInstance(db *bolt.DB, ref_url string, org string, version string, msdef_id string, dependencyPath []ServiceInstancePathElement) (*MicroserviceInstance, error) {

	if ref_url == "" || org == "" || version == "" {
		return nil, errors.New("Microservice ref url id, org or version is empty, cannot persist")
	}

	instance_id, err := createInstanceId()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error creating an instance id for the service instance %v/%v version %v", org, ref_url, version))
	}

	if ms_instance, err := FindMicroserviceInstance(db, ref_url, org, version, instance_id); err != nil {
		return nil, err
	} else if ms_instance != nil {
		return nil, fmt.Errorf("Not expecting any records with Org %v, SpecRef %v, version %v and instance id %v, found %v", org, ref_url, version, instance_id, ms_instance)
	}

	new_inst := &MicroserviceInstance{
		SpecRef:              ref_url,
		Org:                  org,
		Version:              version,
		Arch:                 cutil.ArchString(),
		InstanceId:           instance_id,
		Archived:             false,
		InstanceCreationTime: uint64(time.Now().Unix()),
		ExecutionStartTime:   0, // execution started and running
		ExecutionFailureCode: 0,
		ExecutionFailureDesc: "",
		CleanupStartTime:     0,
		AssociatedAgreements: make([]string, 0),
		MicroserviceDefId:    msdef_id,
		ParentPath:           [][]ServiceInstancePathElement{dependencyPath},
		MaxRetries:           0,
		MaxRetryDuration:     0,
		CurrentRetryCount:    1, // the original execution is counted as the first one.
		RetryStartTime:       0,
	}

	return saveMicroserviceInstance(db, new_inst)
}

// Create an microservice instance object out of an agreement. The object is not be saved into the db.
func AgreementToMicroserviceInstance(ag EstablishedAgreement, msdef_id string) *MicroserviceInstance {
	sipe := NewServiceInstancePathElement(ag.RunningWorkload.URL, ag.RunningWorkload.Org, ag.RunningWorkload.Version)
	return &MicroserviceInstance{
		SpecRef:              ag.RunningWorkload.URL,
		Org:                  ag.RunningWorkload.Org,
		Version:              ag.RunningWorkload.Version,
		Arch:                 ag.RunningWorkload.Arch,
		InstanceId:           ag.CurrentAgreementId,
		Archived:             ag.Archived,
		InstanceCreationTime: ag.AgreementCreationTime,
		ExecutionStartTime:   ag.AgreementExecutionStartTime,
		ExecutionFailureCode: uint(ag.TerminatedReason),
		ExecutionFailureDesc: ag.TerminatedDescription,
		CleanupStartTime:     ag.AgreementTerminatedTime,
		AssociatedAgreements: []string{ag.CurrentAgreementId},
		MicroserviceDefId:    msdef_id,
		ParentPath:           [][]ServiceInstancePathElement{[]ServiceInstancePathElement{*sipe}},
	}
}

// find the microservice instance from the db
func FindMicroserviceInstance(db *bolt.DB, url string, org string, version string, instance_id string) (*MicroserviceInstance, error) {
	var pms *MicroserviceInstance
	pms = nil

	// fetch microservice instances
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(MICROSERVICE_INSTANCES)); b != nil {
			b.ForEach(func(k, v []byte) error {

				var ms MicroserviceInstance

				if err := json.Unmarshal(v, &ms); err != nil {
					glog.Errorf("Unable to deserialize service_instance db record: %v", v)
				} else if ms.SpecRef == url && ms.Version == version && ms.InstanceId == instance_id {
					// ms.Org == "" is for ms instances created by older versions
					if ms.Org == "" || ms.Org == org {
						pms = &ms
					}
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
				glog.Errorf("Unable to deserialize service instance db record: %v. Error: %v", v, err)
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

// filter for all the microservice instances for the given url and org and version
func AllInstancesMIFilter(spec_url string, org string, version string) MIFilter {
	return func(e MicroserviceInstance) bool {
		if e.SpecRef == spec_url && e.Version == version {
			// e.Org == "" is for ms instances created by older versions
			if e.Org == "" || e.Org == org {
				return true
			}
		}
		return false
	}
}

func UnarchivedMIFilter() MIFilter {
	return func(e MicroserviceInstance) bool { return !e.Archived }
}

func NotCleanedUpMIFilter() MIFilter {
	return func(e MicroserviceInstance) bool { return e.CleanupStartTime == 0 }
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
					glog.V(5).Infof("Demarshalled service instance in DB: %v", e)
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
		return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
			c.ExecutionStartTime = uint64(time.Now().Unix())
			c.ExecutionFailureCode = 0
			c.ExecutionFailureDesc = ""
			return &c
		})

	} else {
		return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
			c.ExecutionFailureCode = failure_code
			c.ExecutionFailureDesc = failure_desc
			return &c
		})
	}
}

// add or delete an associated agreement id to/from the microservice instance in the db
func UpdateMSInstanceAssociatedAgreements(db *bolt.DB, key string, add bool, agreement_id string) (*MicroserviceInstance, error) {
	return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
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
	return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		c.Archived = true
		return &c
	})
}

func UpdateMSInstanceEnvVars(db *bolt.DB, key string, env_vars map[string]string) (*MicroserviceInstance, error) {
	return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		c.EnvVars = env_vars
		return &c
	})
}

func MicroserviceInstanceCleanupStarted(db *bolt.DB, key string) (*MicroserviceInstance, error) {
	return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		c.CleanupStartTime = uint64(time.Now().Unix())
		return &c
	})
}

// Add the given path to the ParentPath. It will not be added if there is duplicate path.
func UpdateMSInstanceAddDependencyPath(db *bolt.DB, key string, dp *[]ServiceInstancePathElement) (*MicroserviceInstance, error) {
	return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		if dp != nil && len(*dp) != 0 {
			found := false
			for _, p := range c.ParentPath {
				// compare two arrays
				if CompareServiceInstancePath(p, *dp) {
					found = true
				}
			}

			if !found {
				c.ParentPath = append(c.ParentPath, *dp)
			}
		}
		return &c
	})
}

// remove the given path to the ParentPath.
func UpdateMSInstanceRemoveDependencyPath(db *bolt.DB, key string, dp *[]ServiceInstancePathElement) (*MicroserviceInstance, error) {
	return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		if dp != nil && len(*dp) != 0 {
			new_pp := make([][]ServiceInstancePathElement, 0)
			for _, p := range c.ParentPath {
				// compare two arrays
				if !CompareServiceInstancePath(p, *dp) {
					new_pp = append(new_pp, p)
				}
			}
			c.ParentPath = new_pp
		}
		return &c
	})
}

// Remove all the paths with the given top parent from the ParentPath
func UpdateMSInstanceRemoveDependencyPath2(db *bolt.DB, key string, top_parent *ServiceInstancePathElement) (*MicroserviceInstance, error) {
	return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		if top_parent != nil {
			new_pp := make([][]ServiceInstancePathElement, 0)
			for _, p := range c.ParentPath {
				if !top_parent.IsSame(&p[0]) {
					new_pp = append(new_pp, p)
				}
			}
			c.ParentPath = new_pp
		}
		return &c
	})
}

func UpdateMSInstanceAgreementLess(db *bolt.DB, key string) (*MicroserviceInstance, error) {
	return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		c.AgreementLess = true
		return &c
	})
}

// This function is call when the retry starts or retry is done. When it is done, this function resets the retry counts
func UpdateMSInstanceRetryState(db *bolt.DB, key string, started bool, max_retries uint, max_retry_duration uint) (*MicroserviceInstance, error) {
	return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		if started {
			c.RetryStartTime = uint64(time.Now().Unix())
			c.MaxRetries = max_retries
			c.MaxRetryDuration = max_retry_duration
			c.CurrentRetryCount = 1 // the original execution is counted as the first one.
		} else {
			c.RetryStartTime = 0
			c.MaxRetries = 0
			c.MaxRetryDuration = 0
			c.CurrentRetryCount = 1 // the original execution is counted as the first one.
		}
		return &c
	})
}

func UpdateMSInstanceCurrentRetryCount(db *bolt.DB, key string, current_retry uint) (*MicroserviceInstance, error) {
	return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		c.CurrentRetryCount = current_retry
		return &c
	})
}

func ResetMsInstanceExecutionStatus(db *bolt.DB, key string) (*MicroserviceInstance, error) {
	return microserviceInstanceStateUpdate(db, key, func(c MicroserviceInstance) *MicroserviceInstance {
		c.ExecutionStartTime = 0
		c.ExecutionFailureCode = 0
		c.ExecutionFailureDesc = ""
		c.CleanupStartTime = 0
		return &c
	})
}

// update the micorserive instance
func microserviceInstanceStateUpdate(db *bolt.DB, key string, fn func(MicroserviceInstance) *MicroserviceInstance) (*MicroserviceInstance, error) {

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

// does whole-member replacements of values that are legal to change
func persistUpdatedMicroserviceInstance(db *bolt.DB, key string, update *MicroserviceInstance) error {
	return db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(MICROSERVICE_INSTANCES)); err != nil {
			return err
		} else {
			current := b.Get([]byte(key))
			var mod MicroserviceInstance

			if current == nil {
				return fmt.Errorf("No service with given key available to update: %v", key)
			} else if err := json.Unmarshal(current, &mod); err != nil {
				return fmt.Errorf("Failed to unmarshal service DB data: %v. Error: %v", string(current), err)
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
				mod.ExecutionStartTime = update.ExecutionStartTime
				mod.ExecutionFailureCode = update.ExecutionFailureCode
				mod.ExecutionFailureDesc = update.ExecutionFailureDesc
				mod.CleanupStartTime = update.CleanupStartTime
				mod.AssociatedAgreements = update.AssociatedAgreements
				mod.RetryStartTime = update.RetryStartTime
				mod.MaxRetries = update.MaxRetries
				mod.MaxRetryDuration = update.MaxRetryDuration
				mod.CurrentRetryCount = update.CurrentRetryCount
				mod.EnvVars = update.EnvVars

				if len(mod.ParentPath) != len(update.ParentPath) {
					mod.ParentPath = update.ParentPath
				}

				if !mod.AgreementLess { // 1 transition from false to true
					mod.AgreementLess = update.AgreementLess
				}

				if serialized, err := json.Marshal(mod); err != nil {
					return fmt.Errorf("Failed to serialize contract record: %v. Error: %v", mod, err)
				} else if err := b.Put([]byte(key), serialized); err != nil {
					return fmt.Errorf("Failed to write service instance with key: %v. Error: %v", key, err)
				} else {
					glog.V(2).Infof("Succeeded updating service instance record to %v", mod)
					return nil
				}
			}
		}
	})
}

// delete associated agreement id from all the microservice instances
func DeleteAsscAgmtsFromMSInstances(db *bolt.DB, agreement_id string) error {
	if ms_instances, err := FindMicroserviceInstances(db, []MIFilter{UnarchivedMIFilter()}); err != nil {
		return fmt.Errorf("Error retrieving all service instances from database, error: %v", err)
	} else if ms_instances != nil {
		for _, msi := range ms_instances {
			if msi.AssociatedAgreements != nil && len(msi.AssociatedAgreements) > 0 {
				for _, id := range msi.AssociatedAgreements {
					if id == agreement_id {
						if _, err := UpdateMSInstanceAssociatedAgreements(db, msi.GetKey(), false, agreement_id); err != nil {
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
					return fmt.Errorf("Unable to delete service instance %v: %v", key, err)
				} else {
					return nil
				}
			})
		}
	}
}

// Service dependencies can be described by a directed graph, starting from the agreement service as the root node of
// the graph all the way to the services which are leaf nodes because they have no dependencies. Services can be
// defined such that instances of a service are sharable by more than 1 caller (sharable = singleton), not sharable by
// more than 1 caller (sharable = multiple), or exclusive to 1 caller (sharable = exclusive).
//
// When a service is defined as sharable=multiple AND it has more than 1 parent in the dependency graph, then each
// instance of such a service has a unique dependency path. To illustrate, suppose A depends on B and C, and B and C
// both depend on D such that the dependency graph forms a diamond. Further suppose that D is defined as sharable=multiple.
// This means that at runtime, there will be 2 instances of the D service running, one is supporting B and the other is
// supporting C. Thus, when an instance of D is started, it is important to know if the instance is supporting B or C so
// so that the correct docker network configuration can be established. Service B should not be able to access the instance
// of D that is supporting C, it should only be able to access the instance of D that supports B. Likewise for C.
//
// Therefore, we can express an abstract "path" to each instance of D as follows: /A/B/D and /A/C/D, where '/' is used to
// separate path elements. Each element in the "path" is described by a ServiceInstancePathElement object. The path is an
// array of ServiceInstancePathElements and is stored within the database record of a service instance.
//

type ServiceInstancePathElement struct {
	URL     string `json:"url"`
	Org     string `json:"org"`
	Version string `json:"version"`
}

func NewServiceInstancePathElement(url string, org string, version string) *ServiceInstancePathElement {
	return &ServiceInstancePathElement{
		URL:     url,
		Org:     org,
		Version: version,
	}
}

func (s *ServiceInstancePathElement) IsSame(other *ServiceInstancePathElement) bool {
	return (s.URL == other.URL) && (s.Org == other.Org) && (s.Version == other.Version)
}

// create an instance
func createInstanceId() (string, error) {
	if id, err := uuid.NewV4(); err != nil {
		return "", errors.New(fmt.Sprintf("Unable to generate UUID, error: %v", err))
	} else {
		return id.String(), nil
	}
}

// save the given microservice instance into the db
func saveMicroserviceInstance(db *bolt.DB, new_inst *MicroserviceInstance) (*MicroserviceInstance, error) {
	return new_inst, db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(MICROSERVICE_INSTANCES)); err != nil {
			return err
		} else if bytes, err := json.Marshal(new_inst); err != nil {
			return fmt.Errorf("Unable to marshal new record: %v", err)
		} else if err := b.Put([]byte(new_inst.GetKey()), []byte(bytes)); err != nil {
			return fmt.Errorf("Unable to persist service instance: %v", err)
		}
		// success, close tx
		return nil
	})
}

func CompareServiceInstancePath(a, b []ServiceInstancePathElement) bool {
	if a == nil && b == nil {
		return true
	}

	if a != nil && b != nil && len(a) == len(b) {
		for i, a_elem := range a {
			if !a_elem.IsSame(&b[i]) {
				return false
			}
		}
		return true
	}

	return false
}
