package persistence

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/policy"
)

// ==========================================================================================================
// Interface for service/microservice definition
// ==========================================================================================================
type MicroserviceDefInterface interface {
	ShortString() string
	GetKey() string
	GetOrg() string
	GetURL() string
	GetVersion() string
	GetArch() string
	GetServiceType() string // device, cluster or both
	GetRequiredServices() []exchangecommon.ServiceDependency
	GetUserInputs() []exchangecommon.UserInput
	GetDeploymentString() string
	GetExtendedDeploymentString() string
	IsPublic() bool
	GetSharable() string
	IsArchived() bool
	GetUpgradeVersionRange() string
	GetUpgradeStartedTime() uint64
	GetUpgradeFailedTime() uint64
	GetUpgradeUngradeFailureReason() uint64
	GetUngradeFailureDescription() string

	Archive(db *bolt.DB) error

	/*
		GetUpgradeNewDefId() string
		GetUpgradeUnregisteredTime() uint64
		GetUpgradeExecutionStartedTime() uint64
		GetUpgradeReregisteredTime() uint64

		HasDeployment() bool // check if the service has deployment
		HasRequiredServices() bool
		NeedsUserInput() bool
		GetNewUpgradeVersionRange() string
		IsUpgradeAgreementsCleared() bool

		// functions to persist it
		Save() error // save current settings
		Delete() error // delete it from the db
		Unarchive() error
		SetUpgradeStarted() error
		SetUpgradeUnregistered() error
		SetUpgradeExecutionStarted() error
		SetUpgradeReregistered() error
		SetUpgradeFailed(reasonCode uint64, reasonString string) error
		SetUpgradeNewDefId() error
		SetNewUpgradeVersionRange() error
	*/
}

// ==========================================================================================================
// Interface for service/microservice instance
// ==========================================================================================================
type MicroserviceInstInterface interface {
	ShortString() string
	GetInstanceId() string
	GetKey() string
	GetServiceDefId() string
	GetOrg() string
	GetURL() string
	GetVersion() string
	GetArch() string
	IsArchived() bool
	IsTopLevelService() bool
	IsAgreementLess() bool
	GetEnvVars() map[string]string
	GetAssociatedAgreements() []string
	GetParentPath() [][]ServiceInstancePathElement
	GetInstanceCreationTime() uint64
	GetExecutionStartTime() uint64
	GetExecutionFailureCode() uint
	GetExecutionFailureDesc() string
	GetCleanupStartTime() uint64
	GetMaxRetries() uint
	GetMaxRetryDuration() uint
	GetCurrentRetryCount() uint
	GetRetryStartTime() uint64

	Archive(db *bolt.DB) error

	/*
			HasDeployment() bool
		    // Check if this service instance has the given service as a direct parent.
			HasDirectParent(parent *ServiceInstancePathElement) bool
		    // It returns a an array of direct parents for this service instance.
			GetDirectParents() []ServiceInstancePathElement


			// ---------- functions to persist it ---------- //
			Save() error // save current settings
			Delete() error // delete it from the db
			Unarchive() error
			// set the state to execution started or failed
			SetExecutionState(started bool, failure_code uint, failure_desc string) error
		    // add or delete an associated agreement id to/from the service instance
		    UpdateAssociatedAgreements(add bool, agreement_id string) error
		    SetEnvVars(env_vars map[string]string) error
			SetMaxRetries(new_value uint) error
			// The number of seconds in which the specified number of retries must occur in order for next retry cycle.
			SetMaxRetryDuration(new_value uint) error
			SetCurrentRetryCount(new_value uint) error
			SetRetryStartTime(new_value uint64) error
			SetCleanupStartTime(new_value uint64) error
			// Add the given path to the ParentPath. It will not be added if there is duplicate path.
			AddDependencyPath(dp *[]ServiceInstancePathElement) error
			// remove the given path to the ParentPath.
			RemoveDependencyPath(dp *[]ServiceInstancePathElement) error
			// Remove all the paths with the given top parent from the ParentPath
			RemoveDependencyPath2(top_parent *ServiceInstancePathElement) error
			SetAgreementLess() error
		    // This function is call when the retry starts or retry is done. When it is done, this function resets the retry counts
			UpdateRetryState(started bool, max_retries uint, max_retry_duration uint) error
	*/
}

// Get all the MicroserviceInstInterface objects including the archived ones if includeArchived is true.
// It gets both MicroserviceInstance objects and EstablishedAgreement objects.
// If convertToMI is true, it will be EstablishedAgreement objects to MicroserviceIntance objects.
func GetAllMicroserviceInstances(db *bolt.DB, includeArchived bool, convertToMI bool) ([]MicroserviceInstInterface, error) {
	// get microservice definitions
	filter := []MIFilter{}
	if !includeArchived {
		filter = append(filter, UnarchivedMIFilter())
	}

	// Get all the service instances that we know about from the dependent service database.
	msinsts, err := FindMicroserviceInstances(db, filter)
	if err != nil {
		return nil, fmt.Errorf("Unable to read service instances from database, error %v", err)
	}

	// Get all the agreement (top-level) services
	filter2 := []EAFilter{}
	if !includeArchived {
		filter2 = append(filter2, UnarchivedEAFilter())
	}
	agInsts, err := FindEstablishedAgreementsAllProtocols(db, policy.AllAgreementProtocols(), filter2)
	if err != nil {
		return nil, fmt.Errorf("Unable to read agreement services from database, error %v", err)
	}

	ret := []MicroserviceInstInterface{}
	for i := 0; i < len(msinsts); i++ {
		ret = append(ret, &msinsts[i])
	}
	for i, ag := range agInsts {
		if convertToMI {
			ret = append(ret, AgreementToMicroserviceInstance(ag))
		} else {
			ret = append(ret, &agInsts[i])
		}
	}

	return ret, nil
}

// Get all the MicroserviceInstInterface objects that reference the given microservice definition id.
// It gets both MicroserviceInstance objects and EstablishedAgreement objects.
// If includeArchived is true, it will return both archived and unarchived.
// If convertToMI is true, it will be EstablishedAgreement objects to MicroserviceIntance objects.
func GetAllMicroserviceInstancesWithDefId(db *bolt.DB, msdefId string, includeArchived bool, convertToMI bool) ([]MicroserviceInstInterface, error) {

	ret := []MicroserviceInstInterface{}

	// find all the agreement (top-level) services that reference this service def id
	filter1 := []EAFilter{ServiceDefEAFilter(msdefId)}
	if !includeArchived {
		filter1 = append(filter1, UnarchivedEAFilter())
	}
	agInsts, err := FindEstablishedAgreementsAllProtocols(db, policy.AllAgreementProtocols(), filter1)
	if err != nil {
		return nil, fmt.Errorf("Unable to get agreement with service def id %v from database. %v", msdefId, err)
	} else if agInsts != nil && len(agInsts) != 0 {
		for i, ag := range agInsts {
			if convertToMI {
				ret = append(ret, AgreementToMicroserviceInstance(ag))
			} else {
				ret = append(ret, &agInsts[i])
			}
		}
	}

	// find all the dependent service instances that reference this service def id
	filter2 := []MIFilter{ServiceDefMIFilter(msdefId)}
	if !includeArchived {
		filter2 = append(filter2, UnarchivedMIFilter())
	}
	msInsts, err := FindMicroserviceInstances(db, filter2)
	if err != nil {
		return nil, fmt.Errorf("Error finding service instances with service def id %v. %v", msdefId, err)
	} else if msInsts != nil && len(msInsts) != 0 {
		for i := 0; i < len(msInsts); i++ {
			ret = append(ret, &msInsts[i])
		}
	}

	return ret, nil
}

// Create an microservice instance object out of an agreement. The object is not be saved into the db.
func AgreementToMicroserviceInstance(ag EstablishedAgreement) *MicroserviceInstance {
	sipe := NewServiceInstancePathElement(ag.RunningWorkload.URL, ag.RunningWorkload.Org, ag.RunningWorkload.Version)
	return &MicroserviceInstance{
		SpecRef:              ag.RunningWorkload.URL,
		Org:                  ag.RunningWorkload.Org,
		Version:              ag.RunningWorkload.Version,
		Arch:                 ag.RunningWorkload.Arch,
		InstanceId:           ag.CurrentAgreementId,
		Archived:             ag.Archived,
		TopLevelService:      true,
		InstanceCreationTime: ag.AgreementCreationTime,
		ExecutionStartTime:   ag.AgreementExecutionStartTime,
		ExecutionFailureCode: uint(ag.TerminatedReason),
		ExecutionFailureDesc: ag.TerminatedDescription,
		CleanupStartTime:     ag.AgreementTerminatedTime,
		AssociatedAgreements: []string{ag.CurrentAgreementId},
		MicroserviceDefId:    ag.ServiceDefId,
		ParentPath:           [][]ServiceInstancePathElement{[]ServiceInstancePathElement{*sipe}},
	}
}

// This function returns an object with MicroserviceInstInterface.
// It could be *MicroserviceInstance or *EstablishedAgreement.
func GetMicroserviceInstIWithKey(db *bolt.DB, msinst_key string) (MicroserviceInstInterface, error) {
	if inst, err := FindMicroserviceInstanceWithKey(db, msinst_key); err != nil {
		return nil, fmt.Errorf("Error getting service instance %v from db. %v", msinst_key, err)
	} else if inst != nil {
		return inst, nil
	} else if ags, err := FindEstablishedAgreementsAllProtocols(db, policy.AllAgreementProtocols(), []EAFilter{IdEAFilter(msinst_key)}); err != nil {
		return nil, fmt.Errorf("Unable to get agreement %v from database. %v", msinst_key, err)
	} else if len(ags) != 0 {
		return &ags[0], nil
	}
	return nil, nil
}

// This function archives the microservice instance with the given key.
// The key could be a key for the MicroseviceInstance or EstablishedAgreement
// It will archive the related microservice defintion if no more instances referencing it.
func ArchiveMicroserviceInstAndDef(db *bolt.DB, msinst_key string) error {
	// find the microservice instance
	msi, err := GetMicroserviceInstIWithKey(db, msinst_key)
	if err != nil {
		return err
	}
	if msi == nil {
		return fmt.Errorf("Cannot find service with given key %v from the database.", msinst_key)
	}

	// archive it if not archived
	if !msi.IsArchived() {
		if err = msi.Archive(db); err != nil {
			return err
		}
	}

	msdefId := msi.GetServiceDefId()
	if msdefId == "" {
		// no microservice def found, do nothing.
		// this is to support backward compatibility for Established agreement that
		// did not have ServiceDefId attribute.
		return nil
	}

	// find the microservice definiton
	msdef, err := FindMicroserviceDefWithKey(db, msdefId)
	if err != nil {
		return fmt.Errorf("Error finding service definition from db for %v/%v version %v key %v. %v", msi.GetOrg(), msi.GetURL(), msi.GetVersion(), msdefId, err)
	} else if msdef == nil {
		return nil
	} else if msdef.IsArchived() {
		return nil
	}

	// find all the microservice instances that reference this service def id
	if ms_insts, err := FindMicroserviceInstances(db, []MIFilter{UnarchivedMIFilter(), ServiceDefMIFilter(msdefId)}); err != nil {
		return fmt.Errorf("Error finding service instances with service def id %v. %v", msdefId, err)
	} else if ms_insts != nil && len(ms_insts) != 0 {
		// there are other instances that reference the service def.
		// keep the service def
		return nil
	} else if ags, err := FindEstablishedAgreementsAllProtocols(db, policy.AllAgreementProtocols(), []EAFilter{UnarchivedEAFilter(), ServiceDefEAFilter(msdefId)}); err != nil {
		return fmt.Errorf("Unable to get agreement with service def id %v from database. %v", msdefId, err)
	} else if ags != nil && len(ags) != 0 {
		// there are other top leverl service instances that reference the service def.
		// keep the service def
		return nil
	}

	// archive the microservice definiton
	if err := msdef.Archive(db); err != nil {
		return err
	}

	return nil
}
