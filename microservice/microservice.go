package microservice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
	"golang.org/x/crypto/sha3"
	"strconv"
	"strings"
)

// microservice defaults
const MS_DEFAULT_AUTOUPGRADE = true
const MS_DEFAULT_ACTIVEUPGRADE = false

// microservice instance terminated reason code
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
		MS_UNREG_EXCH_FAILED:            "Service un-registration on exchange failed",
		MS_CLEAR_OLD_AGS_FAILED:         "Clearing old agreements failed",
		MS_EXEC_FAILED:                  "Execution failed",
		MS_REREG_EXCH_FAILED:            "Service registration on exchange failed",
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

// This function converts the structure from exchange service to persistence.
func ConvertServiceToPersistent(es *exchange.ServiceDefinition, org string) (*persistence.MicroserviceDefinition, error) {
	pms := new(persistence.MicroserviceDefinition)

	pms.Owner = es.Owner
	pms.Label = es.Label
	pms.Description = es.Description
	pms.SpecRef = es.URL
	pms.Org = org
	pms.Version = es.Version
	pms.Arch = es.Arch

	pms.Sharable = strings.ToLower(es.Sharable)
	if pms.Sharable != exchangecommon.SERVICE_SHARING_MODE_EXCLUSIVE &&
		pms.Sharable != exchangecommon.SERVICE_SHARING_MODE_SINGLE &&
		pms.Sharable != exchangecommon.SERVICE_SHARING_MODE_SINGLETON &&
		pms.Sharable != exchangecommon.SERVICE_SHARING_MODE_MULTIPLE {
		pms.Sharable = exchangecommon.SERVICE_SHARING_MODE_EXCLUSIVE // default
	}

	pms.MatchHardware = make(persistence.HardwareMatch)
	cutil.CopyMap(es.MatchHardware, pms.MatchHardware)

	user_inputs := make([]exchangecommon.UserInput, 0)
	for _, ui := range es.UserInputs {
		new_ui := exchangecommon.NewUserInput(ui.Name, ui.Label, ui.Type, ui.DefaultValue)
		user_inputs = append(user_inputs, *new_ui)
	}
	pms.UserInputs = user_inputs

	pms.Public = es.Public
	pms.Deployment = es.Deployment
	pms.DeploymentSignature = es.DeploymentSignature
	pms.ClusterDeployment = es.ClusterDeployment
	pms.ClusterDeploymentSignature = es.ClusterDeploymentSignature

	reqServs := make([]exchangecommon.ServiceDependency, 0)
	for _, r := range es.RequiredServices {
		sd := exchangecommon.NewServiceDependency(r.URL, r.Org, r.Version, r.Arch)
		reqServs = append(reqServs, *sd)
	}
	pms.RequiredServices = reqServs

	pms.LastUpdated = es.LastUpdated

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
	if serial, err := json.Marshal(*es); err != nil {
		return nil, fmt.Errorf("Failed to marshal service metadata: %v. %v", *es, err)
	} else {
		hash := sha3.Sum256(serial)
		pms.MetadataHash = hash[:]
	}

	return pms, nil
}

func ConvertRequiredServicesToExchange(m *persistence.MicroserviceDefinition) *[]exchangecommon.ServiceDependency {
	reqServs := make([]exchangecommon.ServiceDependency, 0)
	for _, rs := range m.RequiredServices {
		sd := exchangecommon.ServiceDependency{URL: rs.URL, Org: rs.Org, Version: rs.Version, Arch: rs.Arch}
		reqServs = append(reqServs, sd)
	}
	return &reqServs
}

// check if the given msdef is eligible for a upgrade
func MicroserviceReadyForUpgrade(msdef *persistence.MicroserviceDefinition, db *bolt.DB) bool {
	glog.V(5).Infof("Check if service %v/%v is available for a upgrade.", msdef.Org, msdef.SpecRef)

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

	// For inactive upgrade, make sure there are no agreements associated with the service instances. If there are,
	// the upgrade cannot proceed.
	//
	// For agreement-less services, never upgrade. The agreement-less indicator is only in the instance object
	// (not in the def object) because an agreement-less service is defined by the node's pattern which can
	// change on a lifecycle boundary that is different from the lifecycle of the service definition itself.
	//
	// For top level service, never initiate upgrade from the agent.
	//
	// Service's that are managed by an agreement do not have a record in the microservice instance table, so they
	// will never be found by this function and will therefore never be upgraded (which is the behavior we want).

	// Use a filter that only returns unarchived, non-terminating instances that match the input service definition.
	if ms_insts, err := persistence.GetAllMicroserviceInstancesWithDefId(db, msdef.Id, false, false); err != nil {
		glog.Errorf("Error retrieving all the service instances from db for %v/%v version %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, err)
		return false
	} else if ms_insts != nil && len(ms_insts) > 0 {
		for _, msi := range ms_insts {
			// Agreement-less services are never upgraded.
			if msi.IsAgreementLess() {
				return false
				// never upgrade top level services, they are controlled by the agbot.
			} else if msi.IsTopLevelService() {
				return false
			} else if !msdef.ActiveUpgrade && msi.GetServiceDefId() == msdef.Id && msi.GetCleanupStartTime() == 0 {
				// If the service can only be upgraded when there are no agreements, check for agreements.
				if ags := msi.GetAssociatedAgreements(); ags != nil && len(ags) > 0 {
					return false
				}
			}
		}
	}

	glog.V(5).Infof("Service is available for a upgrade.")
	return true
}

// Get the new microservice def that the given msdef need to upgrade to.
// This function gets the msdef with highest version within defined version range from the exchange and
// compare the version and content with the current msdef and decide if it needs to upgrade.
// It returns the new msdef if the old one needs to be upgraded, otherwide return nil.
func GetUpgradeMicroserviceDef(getService exchange.ServiceResolverHandler, msdef *persistence.MicroserviceDefinition, db *bolt.DB) (*persistence.MicroserviceDefinition, error) {
	glog.V(3).Infof("Get new service def for upgrading service %v/%v version %v key %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id)

	// convert the sensor version to a version expression
	if vExp, err := semanticversion.Version_Expression_Factory(msdef.UpgradeVersionRange); err != nil {
		return nil, fmt.Errorf("Unable to convert %v to a version expression, error %v", msdef.UpgradeVersionRange, err)
	} else if _, e_sdef, _, err := getService(msdef.SpecRef, msdef.Org, vExp.Get_expression(), msdef.Arch); err != nil {
		return nil, fmt.Errorf("Failed to find a highest version for service %v/%v version range %v: %v", msdef.Org, msdef.SpecRef, msdef.UpgradeVersionRange, err)
	} else if e_sdef == nil {
		return nil, fmt.Errorf("Could not find any services for %v/%v within the version range %v.", msdef.Org, msdef.SpecRef, msdef.UpgradeVersionRange)
	} else if new_msdef, err := ConvertServiceToPersistent(e_sdef, msdef.Org); err != nil {
		return nil, fmt.Errorf("Failed to convert service metadata to persistent.MicroserviceDefinition for %v/%v. %v", msdef.Org, msdef.SpecRef, err)
	} else {
		// if the newer version is smaller than the old one, do nothing
		if c, err := semanticversion.CompareVersions(e_sdef.GetVersion(), msdef.Version); err != nil {
			return nil, fmt.Errorf("error compairing version %v with version %v. %v", e_sdef.GetVersion(), msdef.Version, err)
		} else if c < 0 {
			return nil, nil
		} else if c == 0 && bytes.Equal(msdef.MetadataHash, new_msdef.MetadataHash) {
			return nil, nil // no change, do nothing
		} else {
			if msdefs, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{persistence.UrlOrgVersionMSFilter(new_msdef.SpecRef, new_msdef.Org, new_msdef.Version), persistence.ArchivedMSFilter()}); err != nil {
				return nil, fmt.Errorf("Failed to get archived service definition for %v/%v version %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, err)
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

		glog.V(5).Infof("New upgrade msdef is %v", new_msdef.ShortString())
		return new_msdef, nil
	}
}

// Get a msdef with a lower version compared to the given msdef version and return the new microservice def.
func GetRollbackMicroserviceDef(getService exchange.ServiceResolverHandler, msdef *persistence.MicroserviceDefinition, db *bolt.DB) (*persistence.MicroserviceDefinition, error) {
	glog.V(3).Infof("Get next highest service def for rolling back service %v/%v version %v key %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id)

	// convert the sensor version to a version expression
	if vExp, err := semanticversion.Version_Expression_Factory(msdef.UpgradeVersionRange); err != nil {
		return nil, fmt.Errorf("Unable to convert %v to a version expression, error %v", msdef.UpgradeVersionRange, err)
	} else if err := vExp.ChangeCeiling(msdef.Version, false); err != nil { //modify the version range in order to searh for new ms
		return nil, nil
	} else if _, e_sdef, _, err := getService(msdef.SpecRef, msdef.Org, vExp.Get_expression(), msdef.Arch); err != nil {
		return nil, fmt.Errorf("Failed to find a highest version for service %v/%v version range %v: %v", msdef.Org, msdef.SpecRef, vExp.Get_expression(), err)
	} else if e_sdef == nil {
		return nil, nil
	} else if new_msdef, err := ConvertServiceToPersistent(e_sdef, msdef.Org); err != nil {
		return nil, fmt.Errorf("Failed to convert service metadata to persistent.MicroserviceDefinition for %v/%v. %v", msdef.Org, msdef.SpecRef, err)
	} else {

		// copy some attributes from the old over to the new
		new_msdef.Name = msdef.Name
		new_msdef.UpgradeVersionRange = msdef.UpgradeVersionRange
		new_msdef.AutoUpgrade = msdef.AutoUpgrade
		new_msdef.ActiveUpgrade = msdef.ActiveUpgrade
		new_msdef.RequestedArch = msdef.RequestedArch

		glog.V(5).Infof("New rollback msdef is %v", new_msdef.ShortString())
		return new_msdef, nil
	}
}

// Remove the policy for the given microservice and rename the policy file name.
func RemoveMicroservicePolicy(spec_ref string, org string, version string, msdef_id string, policy_path string, pm *policy.PolicyManager) error {

	glog.V(3).Infof("Remove policy for %v/%v version %v, key %v", org, spec_ref, version, msdef_id)

	policies := pm.GetAllPolicies(org)
	if len(policies) > 0 {
		for _, pol := range policies {
			// skip the node policy which does not have APISpecs
			if len(pol.APISpecs) == 0 {
				continue
			}
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

// Generate a new policy file for given ms and then register the microservice in the exchange.
func GenMicroservicePolicy(msdef *persistence.MicroserviceDefinition, policyPath string, db *bolt.DB, e chan events.Message, deviceOrg string, pattern string) error {
	glog.V(3).Infof("Generate policy for the given service %v/%v version %v key %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id)

	var haPartner []string
	var serviceAgreementProtocols []interface{}

	props := make(map[string]interface{})

	// parse the service attributes and assign them to the correct variables defined above
	handleServiceAttributes := func(attributes []persistence.Attribute) {
		for _, attr := range attributes {
			switch attr.(type) {
			case persistence.HAAttributes:
				haPartner = attr.(persistence.HAAttributes).Partners

			case persistence.AgreementProtocolAttributes:
				agpl := attr.(persistence.AgreementProtocolAttributes).Protocols
				serviceAgreementProtocols = agpl.([]interface{})

			default:
				glog.V(4).Infof("Unhandled attr type (%T): %v", attr, attr)
			}
		}
	}

	// add node built-in properties
	existingPol, err := persistence.FindNodePolicy(db)
	if err != nil {
		glog.V(2).Infof("Failed to retrieve node policy from local db: %v", err)
	}
	isCluster := false
	exchDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		glog.V(2).Infof("Failed to retrieve node from local db: %v", err)
	} else {
		isCluster = exchDevice.IsEdgeCluster()
	}
	externalPol, externalReadWritePol := externalpolicy.CreateNodeBuiltInPolicy(false, false, existingPol, isCluster)
	externalPol.MergeWith(externalReadWritePol, false)
	if externalPol != nil {
		for _, ele := range externalPol.Properties {
			if ele.Name == externalpolicy.PROP_NODE_CPU {
				props["cpus"] = strconv.FormatFloat(ele.Value.(float64), 'f', -1, 64)
			} else if ele.Name == externalpolicy.PROP_NODE_MEMORY {
				props["ram"] = strconv.FormatFloat(ele.Value.(float64), 'f', -1, 64)
			} else if ele.Name == externalpolicy.PROP_NODE_HARDWAREID {
				props["hardwareId"] = ele.Value.(string)
			}
		}
	}

	// get the attributes for the microservice from the service_attribute table
	if orig_attributes, err := persistence.FindApplicableAttributes(db, msdef.SpecRef, msdef.Org); err != nil {
		return fmt.Errorf("Failed to get the service attributes for %v/%v from db. %v", msdef.Org, msdef.SpecRef, err)
	} else {
		// divide the attributes into 2 groups, common and specific
		common_attribs := make([]persistence.Attribute, 0)
		specific_attribs := make([]persistence.Attribute, 0)

		for _, attr := range orig_attributes {
			serviceSpecs := persistence.GetAttributeServiceSpecs(&attr)
			if serviceSpecs == nil || len(*serviceSpecs) == 0 {
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
		if msdef.Sharable == exchangecommon.SERVICE_SHARING_MODE_SINGLETON || msdef.Sharable == exchangecommon.SERVICE_SHARING_MODE_MULTIPLE || msdef.Sharable == exchangecommon.SERVICE_SHARING_MODE_SINGLE {
			maxAgreements = 5 // hard coded 2 for now, will change to 0 later
		}

		if polFileName, err := policy.GeneratePolicy(msdef.SpecRef, msdef.Org, msdef.Name, msdef.Version, msdef.RequestedArch, &props, haPartner, *list, maxAgreements, policyPath, deviceOrg); err != nil {
			return fmt.Errorf("Failed to generate policy for %v/%v version %v. Error: %v", msdef.Org, msdef.SpecRef, msdef.Version, err)
		} else {
			e <- events.NewPolicyCreatedMessage(events.NEW_POLICY, polFileName)
		}
	}

	return nil
}

// Unregisters the given microservice from the exchange
func UnregisterMicroserviceExchange(getExchangeDevice exchange.DeviceHandler,
	patchExchangeDevice exchange.PatchDeviceHandler,
	spec_ref string, org string, version string,
	device_id string, device_token string, db *bolt.DB) error {

	glog.V(3).Infof("Unregister service %v/%v from exchange for %v.", org, spec_ref, device_id)

	if eDevice, err := getExchangeDevice(device_id, device_token); err != nil {
		return fmt.Errorf("Error getting device %v from the exchange. %v", device_id, err)
	} else if eDevice.RegisteredServices == nil || len(eDevice.RegisteredServices) == 0 {
		return nil // no registered services/microservices, nothing to do
	} else {
		services := eDevice.RegisteredServices

		// remove the service with the given spec_ref
		ms_put := make([]exchange.Microservice, 0, 10)
		for _, ms := range services {
			if ms.Url != cutil.FormOrgSpecUrl(spec_ref, org) || (version != "" && ms.Version != "" && version != ms.Version) {
				ms_put = append(ms_put, ms)
			}
		}

		// modify the registeredServices for node
		pdr := exchange.PatchDeviceRequest{}
		pdr.RegisteredServices = &ms_put

		glog.V(3).Infof("Unregistering service: %v/%v", org, spec_ref)
		if err := patchExchangeDevice(device_id, device_token, &pdr); err != nil {
			return fmt.Errorf("Received error unregistering service %v/%v from the exchange: %v", org, spec_ref, err)
		} else {
			glog.V(3).Infof("Unregistered service %v/%v in exchange", org, spec_ref)
		}
	}
	return nil
}

// Find the MicroserviceDefinition object in the local db for the given service spec.
// If not found, create one with highest version within the range.
// If exactVersion is true, the service_version is a single version string,
// the microservice definition for the exact version will be returned.
// If exactVersion is false, the service_version is treated as a version range,
// the microservice definiton within the range will be returned if found instances. If not found,
// the microservice definition for the highest version within the range will be returned.
func FindOrCreateMicroserviceDef(db *bolt.DB, service_name string, service_org string, service_version string, service_arch string,
	exactVersion bool, forPattern bool, getService exchange.ServiceHandler) (*persistence.MicroserviceDefinition, error) {
	glog.V(5).Infof("Find or create MicroserviceDefinition object for %v/%v version %v", service_org, service_name, service_version)

	var backupMsdef *persistence.MicroserviceDefinition

	if exactVersion {
		// convert the single version string to a version range
		if !semanticversion.IsVersionString(service_version) {
			return nil, fmt.Errorf("The input service version %v must be a version string when exactVersion is set to true.", service_version)
		} else {
			service_version = fmt.Sprintf("[%v,%v]", service_version, service_version)
		}
	}

	// validate the version range
	vExp, err := semanticversion.Version_Expression_Factory(service_version)
	if err != nil {
		return nil, fmt.Errorf("Error converting APISpec version %v for %v/%v to version range. %v", service_version, service_org, service_name, err)
	}

	msdefs, err := persistence.FindUnarchivedMicroserviceDefs(db, service_name, service_org)
	if err != nil {
		return nil, fmt.Errorf("Error finding dependent service definition from the local db for %v/%v version range %v. %v", service_org, service_name, service_version, err)

	} else if msdefs != nil && len(msdefs) != 0 {
		glog.V(5).Infof("found service definitions locally: %v", msdefs)

		// go through each msdefs and find the exact match for top level services.
		// For dependent sevices, it will find the one that within the range;
		// it is further optimized to use the one that is used by dependent services if possible.
		for i, msdef := range msdefs {

			// check if this def is witin the given version range
			inRange, err := vExp.Is_within_range(msdef.Version)
			if err != nil {
				return nil, fmt.Errorf("Error checking if service version %v is within APISpec version range %v for %v/%v. %v", msdef.Version, vExp, service_org, service_name, err)
			}

			if exactVersion {
				// top level service case
				if inRange {
					return &msdefs[i], nil
				}
			} else {
				// dependent servivce case with existing dependents.
				// for the singleton case use the same version as other dependents.
				// for the pattern case, always use the same version.
				if forPattern || (msdef.Sharable == exchangecommon.SERVICE_SHARING_MODE_SINGLETON || msdef.Sharable == exchangecommon.SERVICE_SHARING_MODE_SINGLE) {
					if inRange {
						return &msdefs[i], nil
					} else {
						return nil, fmt.Errorf("Failed to create service definition for %v/%v version range %v because the service is in 'singleton' sharing mode."+
							" There is another dependent service with version %v exist."+
							" But it is not within the version range of this service.", service_org, service_name, service_version, msdef.Version)
					}
				}

				if inRange {
					// keep the higest version whtin the range in backupMsdef
					if backupMsdef == nil {
						backupMsdef = &msdefs[i]
					} else if c, err := semanticversion.CompareVersions(backupMsdef.Version, msdef.Version); err != nil {
						return nil, fmt.Errorf("Error compairing version %v with version %v for service %v/%v. %v", backupMsdef.Version, msdef.Version, service_org, service_name, err)
					} else if c < 0 {
						backupMsdef = &msdefs[i]
					}
				}
			}
		}
	}

	// get the highest version of this service from the exchange
	var sdef *exchange.ServiceDefinition
	sdef, sId, err := getService(service_name, service_org, vExp.Get_expression(), service_arch)
	if err != nil {
		return nil, fmt.Errorf("Error finding the service definition using  %v/%v %v %v in the exchange. %v", service_org, service_name, vExp.Get_expression(), service_arch, err)
	} else if sdef == nil {
		return nil, fmt.Errorf("Unable to find the service definition using  %v/%v %v %v in the exchange.", service_org, service_name, vExp.Get_expression(), service_arch)
	}

	// backupMsdef is used for dependent services only
	// create a MicroserviceDefinition object with the hightest version within the range if it is not created yet
	if backupMsdef != nil {
		if c, err := semanticversion.CompareVersions(backupMsdef.Version, sdef.Version); err != nil {
			return nil, fmt.Errorf("Error compairing version %v with version %v for service %v/%v. %v", backupMsdef.Version, sdef.Version, service_org, service_name, err)
		} else if c >= 0 {
			return backupMsdef, nil
		}
	}

	if msdef_new, err := CreateMicroserviceDefWithServiceDef(db, sdef, sId, vExp.Get_expression()); err != nil {
		return nil, err
	} else {
		return msdef_new, nil
	}
}

// Create and save the MicroserviceDefiniton for given service. The service_version is a version range.
// Please make sure there is no MicroserviceDefinition for this service before calling this function.
func CreateMicroserviceDef(db *bolt.DB, service_name string, service_org string, service_version string, service_arch string,
	getService exchange.ServiceHandler) (*persistence.MicroserviceDefinition, error) {
	glog.V(3).Infof("Create service definition for local db for %v/%v version range %v.", service_org, service_name, service_version)

	// Convert the version to a version expression.
	vExp, err := semanticversion.Version_Expression_Factory(service_version)
	if err != nil {
		return nil, fmt.Errorf("VersionRange %v cannot be converted to a version expression, error %v", service_version, err)
	}

	// get service.
	var sdef *exchange.ServiceDefinition
	sdef, sId, err := getService(service_name, service_org, vExp.Get_expression(), service_arch)
	if err != nil {
		return nil, fmt.Errorf("Error finding the service definition using  %v/%v %v %v in the exchange. %v", service_org, service_name, vExp.Get_expression(), service_arch, err)
	} else if sdef == nil {
		return nil, fmt.Errorf("Unable to find the service definition using  %v/%v %v %v in the exchange.", service_org, service_name, vExp.Get_expression(), service_arch)
	}

	if msdef_new, err := CreateMicroserviceDefWithServiceDef(db, sdef, sId, vExp.Get_expression()); err != nil {
		return nil, err
	} else {
		return msdef_new, nil
	}
}

// Create and save the MicroserviceDefiniton for given service. This function is used
// for top level service when agreement is formated. The version is not a version range.
// upgradeVerExpr is a full semantic version expression, representing upgrade or downgrade version range.
// Please make sure there is no MicroserviceDefinition for this service before calling this function.
func CreateMicroserviceDefWithServiceDef(db *bolt.DB, sdef *exchange.ServiceDefinition, sId string, upgradeVerExpr string) (*persistence.MicroserviceDefinition, error) {
	glog.V(3).Infof("Create service definition in local db for for %v", sId)

	// Convert the service definition to a persistent format so that it can be saved to the db.
	msdef, err := ConvertServiceToPersistent(sdef, exchange.GetOrg(sId))
	if err != nil {
		return nil, fmt.Errorf("Error converting the service def to persistent.MicroserviceDefinition for %v. %v", sId, err)
	}

	msdef.Name = sId
	msdef.RequestedArch = sdef.Arch
	msdef.UpgradeVersionRange = upgradeVerExpr

	// Save the service definition in the local database.
	if err := persistence.SaveOrUpdateMicroserviceDef(db, msdef); err != nil {
		return nil, fmt.Errorf("Error saving service definition %v into db: %v", *msdef, err)
	}

	return msdef, nil
}
