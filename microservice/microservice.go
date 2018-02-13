package microservice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"golang.org/x/crypto/sha3"
	"strconv"
	"strings"
)

// microservice defaults
const MS_DEFAULT_AUTOUPGRADE = true
const MS_DEFAULT_ACTIVEUPGRADE = false

// microservice instance termiated reason code
const MS_UNREG_EXCH_FAILED = 200
const MS_CLEAR_OLD_AGS_FAILED = 201
const MS_EXEC_FAILED = 202
const MS_REREG_EXCH_FAILED = 203
const MS_IMAGE_LOAD_FAILED = 204
const MS_DELETED_BY_UPGRADE_PROCESS = 205
const MS_DELETED_FOR_AG_ENDED = 206
const MS_IMAGE_FETCH_FAILED = 207
const MS_DELETED_BY_DOWNGRADE_PROCESS = 208

func DecodeReasonCode(code uint64) string {
	// microservice termiated deccription
	codeMeanings := map[uint64]string{
		MS_UNREG_EXCH_FAILED:            "Unregistering microservice on exchange failed",
		MS_CLEAR_OLD_AGS_FAILED:         "Clearing old agreements failed",
		MS_EXEC_FAILED:                  "Execution failed",
		MS_REREG_EXCH_FAILED:            "Reregistering microservice on exchange failed",
		MS_IMAGE_LOAD_FAILED:            "Image loading failed",
		MS_DELETED_BY_UPGRADE_PROCESS:   "Deleted by upgrading process",
		MS_DELETED_BY_DOWNGRADE_PROCESS: "Deleted by downgrading process",
		MS_DELETED_FOR_AG_ENDED:         "Deleted for agreement ended",
		MS_IMAGE_FETCH_FAILED:           "Image fetching failed",
	}

	if reasonString, ok := codeMeanings[code]; !ok {
		return "unknown reason code, device might be downlevel"
	} else {
		return reasonString
	}
}

// Currently the MicroserviceDefiniton structures in exchange and persistence packages are identical.
// But we need to make them separate structures in case we need to put more in persistence structure.
// This function converts the structure from exchange to persistence.
func ConvertToPersistent(ems *exchange.MicroserviceDefinition, org string) (*persistence.MicroserviceDefinition, error) {
	pms := new(persistence.MicroserviceDefinition)

	pms.Owner = ems.Owner
	pms.Label = ems.Label
	pms.Description = ems.Description
	pms.SpecRef = ems.SpecRef
	pms.Org = org
	pms.Version = ems.Version
	pms.Arch = ems.Arch
	pms.DownloadURL = ems.DownloadURL

	pms.Sharable = strings.ToLower(ems.Sharable)
	if pms.Sharable != exchange.MS_SHARING_MODE_EXCLUSIVE &&
		pms.Sharable != exchange.MS_SHARING_MODE_SINGLE &&
		pms.Sharable != exchange.MS_SHARING_MODE_MULTIPLE {
		pms.Sharable = exchange.MS_SHARING_MODE_EXCLUSIVE // default
	}

	hwmatch := persistence.NewHardwareMatch(ems.MatchHardware.USBDeviceIds, ems.MatchHardware.Devfiles)
	pms.MatchHardware = *hwmatch

	user_inputs := make([]persistence.UserInput, 0)
	for _, ui := range ems.UserInputs {
		new_ui := persistence.NewUserInput(ui.Name, ui.Label, ui.Type, ui.DefaultValue)
		user_inputs = append(user_inputs, *new_ui)
	}
	pms.UserInputs = user_inputs

	workloads := make([]persistence.WorkloadDeployment, 0)
	for _, wl := range ems.Workloads {
		new_wl := persistence.NewWorkloadDeployment(wl.Deployment, wl.DeploymentSignature, wl.Torrent)
		workloads = append(workloads, *new_wl)
	}
	pms.Workloads = workloads
	pms.LastUpdated = ems.LastUpdated

	// set defaults
	pms.UpgradeStartTime = 0
	pms.UpgradeMsUnregisteredTime = 0
	pms.UpgradeAgreementsClearedTime = 0
	pms.UpgradeExecutionStartTime = 0
	pms.UpgradeFailedTime = 0
	pms.UngradeFailureReason = 0
	pms.UngradeFailureDescription = ""
	pms.UpgradeNewMsId = ""

	pms.Name = ""
	pms.UpgradeVersionRange = "0.0.0"
	pms.AutoUpgrade = MS_DEFAULT_AUTOUPGRADE
	pms.ActiveUpgrade = MS_DEFAULT_ACTIVEUPGRADE

	// Hash the metadata and save it
	if serial, err := json.Marshal(*ems); err != nil {
		return nil, fmt.Errorf("Failed to marshal microservice metadata: %v. %v", *ems, err)
	} else {
		hash := sha3.Sum256(serial)
		pms.MetadataHash = hash[:]
	}

	return pms, nil
}

// check if the given msdef is eligible for a upgrade
func MicroserviceReadyForUpgrade(msdef *persistence.MicroserviceDefinition, db *bolt.DB) bool {
	glog.V(5).Infof("Check if microservice is available for a upgrade: %v.", msdef.SpecRef)

	if msdef.Archived {
		return false
	}

	// user does not want upgrade
	if !msdef.AutoUpgrade {
		return false
	}

	// in the middle of a upgrade, do not disturb
	if msdef.UpgradeStartTime != 0 && msdef.UpgradeMsReregisteredTime == 0 && msdef.UpgradeFailedTime == 0 {
		return false
	}

	// for inactive upgrade make sure there are no agreements associated with it
	if !msdef.ActiveUpgrade {
		if ms_insts, err := persistence.FindMicroserviceInstances(db, []persistence.MIFilter{persistence.AllInstancesMIFilter(msdef.SpecRef, msdef.Version), persistence.UnarchivedMIFilter()}); err != nil {
			glog.Errorf("Error retrieving all the microservice instaces from db for %v version %v. %v", msdef.SpecRef, msdef.Version, err)
			return false
		} else if ms_insts != nil && len(ms_insts) > 0 {
			for _, msi := range ms_insts {
				if msi.MicroserviceDefId == msdef.Id {
					if ags := msi.AssociatedAgreements; ags != nil && len(ags) > 0 {
						return false
					}
				}
			}
		}
	}

	glog.V(5).Infof("Microservice is available for a upgrade.")
	return true
}

// Get the new microservice def that the given msdef need to upgrade to.
// This function gets the msdef with highest version within defined version range from the exchange and
// compare the version and content with the current msdef and decide if it needs to upgrade.
// It returns the new msdef if the old one needs to be upgraded, otherwide return nil.
func GetUpgradeMicroserviceDef(getMicroservice exchange.MicroserviceHandler, msdef *persistence.MicroserviceDefinition, deviceId string, deviceToken string, db *bolt.DB) (*persistence.MicroserviceDefinition, error) {
	glog.V(3).Infof("Get new microservice def for upgrading microservice %v version %v key %v", msdef.SpecRef, msdef.Version, msdef.Id)

	// convert the sensor version to a version expression
	if vExp, err := policy.Version_Expression_Factory(msdef.UpgradeVersionRange); err != nil {
		return nil, fmt.Errorf("Unable to convert %v to a version expression, error %v", msdef.UpgradeVersionRange, err)
	} else if e_msdef, err := getMicroservice(msdef.SpecRef, msdef.Org, vExp.Get_expression(), msdef.Arch, deviceId, deviceToken); err != nil {
		return nil, fmt.Errorf("Filed to find a highest version for microservice %v version range %v: %v", msdef.SpecRef, msdef.UpgradeVersionRange, err)
	} else if e_msdef == nil {
		return nil, fmt.Errorf("Could not find any microservices for %v within the version range %v.", msdef.SpecRef, msdef.UpgradeVersionRange)
	} else if new_msdef, err := ConvertToPersistent(e_msdef, msdef.Org); err != nil {
		return nil, fmt.Errorf("Failed to convert microservice metadata to persistent.MicroserviceDefinition for %v. %v", msdef.SpecRef, err)
	} else {
		// if the newer version is smaller than the old one, do nothing
		if c, err := policy.CompareVersions(e_msdef.Version, msdef.Version); err != nil {
			return nil, fmt.Errorf("error compairing version %v with version %v. %v", e_msdef.Version, msdef.Version, err)
		} else if c < 0 {
			return nil, nil
		} else if c == 0 && bytes.Equal(msdef.MetadataHash, new_msdef.MetadataHash) {
			return nil, nil // no change, do nothing
		} else {
			if msdefs, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{persistence.UrlVersionMSFilter(new_msdef.SpecRef, new_msdef.Version), persistence.ArchivedMSFilter()}); err != nil {
				return nil, fmt.Errorf("Failed to get archived microservice definition for %v version %v. %v", msdef.SpecRef, msdef.Version, err)
			} else if msdefs != nil && len(msdefs) > 0 {
				for _, ms := range msdefs {
					if ms.UpgradeNewMsId != "" && bytes.Equal(ms.MetadataHash, new_msdef.MetadataHash) {
						return nil, nil // do nothing because upgrade failed before
					}
				}
			}
		}

		// copy some attributes from the old over to the new
		new_msdef.Name = msdef.Name
		new_msdef.UpgradeVersionRange = msdef.UpgradeVersionRange
		new_msdef.AutoUpgrade = msdef.AutoUpgrade
		new_msdef.ActiveUpgrade = msdef.ActiveUpgrade
		new_msdef.RequestedArch = msdef.RequestedArch

		glog.V(5).Infof("New upgrade msdef is %v", new_msdef)
		return new_msdef, nil
	}
}

// Get a msdef with a lower version compared to the given msdef version and return the new microservice def.
func GetRollbackMicroserviceDef(getMicroservice exchange.MicroserviceHandler, msdef *persistence.MicroserviceDefinition, deviceId string, deviceToken string, db *bolt.DB) (*persistence.MicroserviceDefinition, error) {
	glog.V(3).Infof("Get next highest microservice def for rolling back microservice %v version %v key %v", msdef.SpecRef, msdef.Version, msdef.Id)

	// convert the sensor version to a version expression
	if vExp, err := policy.Version_Expression_Factory(msdef.UpgradeVersionRange); err != nil {
		return nil, fmt.Errorf("Unable to convert %v to a version expression, error %v", msdef.UpgradeVersionRange, err)
	} else if err := vExp.ChangeCeiling(msdef.Version, false); err != nil { //modify the version range in order to searh for new ms
		return nil, nil
	} else if e_msdef, err := getMicroservice(msdef.SpecRef, msdef.Org, vExp.Get_expression(), msdef.Arch, deviceId, deviceToken); err != nil {
		return nil, fmt.Errorf("Filed to find a highest version for microservice %v version range %v: %v", msdef.SpecRef, vExp.Get_expression(), err)
	} else if e_msdef == nil {
		return nil, nil
	} else if new_msdef, err := ConvertToPersistent(e_msdef, msdef.Org); err != nil {
		return nil, fmt.Errorf("Failed to convert microservice metadata to persistent.MicroserviceDefinition for %v. %v", msdef.SpecRef, err)
	} else {

		// copy some attributes from the old over to the new
		new_msdef.Name = msdef.Name
		new_msdef.UpgradeVersionRange = msdef.UpgradeVersionRange
		new_msdef.AutoUpgrade = msdef.AutoUpgrade
		new_msdef.ActiveUpgrade = msdef.ActiveUpgrade
		new_msdef.RequestedArch = msdef.RequestedArch

		glog.V(5).Infof("New rollback msdef is %v", new_msdef)
		return new_msdef, nil
	}
}

// Remove the policy for the given microservice and rename the policy file name.
func RemoveMicroservicePolicy(spec_ref string, org string, version string, msdef_id string, policy_path string, pm *policy.PolicyManager) error {

	glog.V(3).Infof("Remove policy for %v/%v version %v, key %v", org, spec_ref, version, msdef_id)

	policies := pm.GetAllPolicies(org)
	if len(policies) > 0 {
		for _, pol := range policies {
			apiSpec := pol.APISpecs[0]
			if apiSpec.SpecRef == spec_ref && apiSpec.Org == org && apiSpec.Version == version {
				pm.DeletePolicy(org, &pol)

				// get the policy file name
				a_tmp := strings.Split(spec_ref, "/")
				fileName := a_tmp[len(a_tmp)-1]

				if err := policy.RenamePolicyFile(policy_path, org, fileName, "."+msdef_id); err != nil {
					return err
				}

				return nil
			}
		}
	}
	return nil
}

// Generate a new policy file for given ms and the register the microservice on the exchange.
func GenMicroservicePolicy(msdef *persistence.MicroserviceDefinition, policyPath string, db *bolt.DB, e chan events.Message, deviceOrg string) error {
	glog.V(3).Infof("Genarate policy for the given microservice %v version %v key %v", msdef.SpecRef, msdef.Version, msdef.Id)

	var policyArch string
	var haPartner []string
	var meterPolicy policy.Meter
	var counterPartyProperties policy.RequiredProperty
	var properties map[string]interface{}
	var serviceAgreementProtocols []interface{}

	props := make(map[string]interface{})

	// parse the service attributes and assign them to the correct variables defined above
	handleServiceAttributes := func(attributes []persistence.Attribute) {
		for _, attr := range attributes {
			switch attr.(type) {
			case persistence.ComputeAttributes:
				compute := attr.(persistence.ComputeAttributes)
				props["cpus"] = strconv.FormatInt(compute.CPUs, 10)
				props["ram"] = strconv.FormatInt(compute.RAM, 10)

			case persistence.ArchitectureAttributes:
				policyArch = attr.(persistence.ArchitectureAttributes).Architecture

			case persistence.HAAttributes:
				haPartner = attr.(persistence.HAAttributes).Partners

			case persistence.MeteringAttributes:
				meterPolicy = policy.Meter{
					Tokens:                attr.(persistence.MeteringAttributes).Tokens,
					PerTimeUnit:           attr.(persistence.MeteringAttributes).PerTimeUnit,
					NotificationIntervalS: attr.(persistence.MeteringAttributes).NotificationIntervalS,
				}

			case persistence.CounterPartyPropertyAttributes:
				counterPartyProperties = attr.(persistence.CounterPartyPropertyAttributes).Expression

			case persistence.PropertyAttributes:
				properties = attr.(persistence.PropertyAttributes).Mappings

			case persistence.AgreementProtocolAttributes:
				agpl := attr.(persistence.AgreementProtocolAttributes).Protocols
				serviceAgreementProtocols = agpl.([]interface{})

			default:
				glog.V(4).Infof("Unhandled attr type (%T): %v", attr, attr)
			}
		}

		// add the PropertyAttributes to props
		if len(properties) > 0 {
			for key, val := range properties {
				glog.V(5).Infof("Adding property %v=%v with value type %T", key, val, val)
				props[key] = val
			}
		}
	}

	// get the attributes for the microservice from the service_attribute table
	if orig_attributes, err := persistence.FindApplicableAttributes(db, msdef.SpecRef); err != nil {
		return fmt.Errorf("Failed to get the microservice attributes for %v from db. %v", msdef.SpecRef, err)
	} else {
		// device the attributes into 2 groups, common and specific
		common_attribs := make([]persistence.Attribute, 0)
		specific_attribs := make([]persistence.Attribute, 0)

		for _, attr := range orig_attributes {
			sensorUrls := attr.GetMeta().SensorUrls
			if sensorUrls == nil || len(sensorUrls) == 0 {
				common_attribs = append(common_attribs, attr)
			} else {
				specific_attribs = append(specific_attribs, attr)
			}
		}

		// now we parse the common attributes first, then parse the specific ones to override the common attributes
		handleServiceAttributes(common_attribs)
		handleServiceAttributes(specific_attribs)

		list, err := policy.ConvertToAgreementProtocolList(serviceAgreementProtocols)
		if err != nil {
			return fmt.Errorf("Error converting agreement protocol list attribute %v to agreement protocol list, error: %v", serviceAgreementProtocols, err)
		}

		//Generate a policy based on all the attributes and the service definition
		maxAgreements := 1
		if msdef.Sharable == exchange.MS_SHARING_MODE_SINGLE || msdef.Sharable == exchange.MS_SHARING_MODE_MULTIPLE {
			maxAgreements = 2 // hard coded 2 for now, will change to 0 later
		}

		if msg, err := policy.GeneratePolicy(msdef.SpecRef, msdef.Org, msdef.Name, msdef.Version, msdef.RequestedArch, &props, haPartner, meterPolicy, counterPartyProperties, *list, maxAgreements, policyPath, deviceOrg); err != nil {
			return fmt.Errorf("Failed to generate policy for %v version %v. Error: %v", msdef.SpecRef, msdef.Version, err)
		} else {
			e <- msg
		}
	}

	return nil
}

// Unregisters the given microservice from the exchange
func UnregisterMicroserviceExchange(getExchangeDevice exchange.DeviceHandler,
	putExchangeDevice exchange.PutDeviceHandler,
	spec_ref string, device_id string, device_token string, db *bolt.DB) error {

	glog.V(3).Infof("Unregister microservice %v from exchange for %v.", spec_ref, device_id)

	var deviceName string

	if dev, err := persistence.FindExchangeDevice(db); err != nil {
		return fmt.Errorf("Received error getting device name: %v", err)
	} else if dev == nil {
		return fmt.Errorf("Could not get device name because no device was registered yet.")
	} else {
		deviceName = dev.Name
	}

	if eDevice, err := getExchangeDevice(device_id, device_token); err != nil {
		return fmt.Errorf("Error getting device %v from the exchange. %v", device_id, err)
	} else if eDevice.RegisteredMicroservices == nil || len(eDevice.RegisteredMicroservices) == 0 {
		return nil // no registered microservices, nothing to do
	} else {
		// remove the microservice with the given spec_ref
		ms_put := make([]exchange.Microservice, 0, 10)
		for _, ms := range eDevice.RegisteredMicroservices {
			if ms.Url != spec_ref {
				ms_put = append(ms_put, ms)
			}
		}

		// create PUT body
		pdr := exchange.CreateDevicePut(device_token, deviceName)
		pdr.RegisteredMicroservices = ms_put

		glog.V(3).Infof("Unregistering microservice: %v", spec_ref)

		if resp, err := putExchangeDevice(device_id, device_token, pdr); err != nil {
			return fmt.Errorf("Received error unregistering microservice %v from the exchange: %v", spec_ref, err)
		} else {
			glog.V(3).Infof("Unregistered microservice %v in exchange: %v", spec_ref, resp)
		}
	}
	return nil
}
