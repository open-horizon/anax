//go:build unit
// +build unit

package microservice

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func TestConvertToPersistent(t *testing.T) {
	pms := createService(t)

	// check defaults
	assert.True(t, pms.AutoUpgrade, "The default AutoUpgrade should be true")
	assert.False(t, pms.ActiveUpgrade, "The default ActiveUpgrade should be false")
	assert.False(t, pms.Archived, "The default Archived should be false")
	assert.Equal(t, uint64(0), pms.UpgradeStartTime, "The default should be 0")
	assert.Equal(t, uint64(0), pms.UpgradeMsUnregisteredTime, "The default should be 0")
	assert.Equal(t, uint64(0), pms.UpgradeAgreementsClearedTime, "The default should be 0")
	assert.Equal(t, uint64(0), pms.UpgradeExecutionStartTime, "The default should be 0")
	assert.Equal(t, uint64(0), pms.UpgradeFailedTime, "The default should be 0")
	assert.Equal(t, uint64(0), pms.UngradeFailureReason, "The default should be 0")
	assert.Equal(t, "", pms.UngradeFailureDescription, "The default should be an empty string")
	assert.Equal(t, "", pms.UpgradeNewMsId, "The default should be an empty string")
	assert.Equal(t, "0.0.0", pms.UpgradeVersionRange, "The default UpgradeVersionRange should be 0.0.0")
	assert.Equal(t, "", pms.Name, "The default should be an empty string")

	// check hash
	assert.NotNil(t, pms.MetadataHash, "The MetadataHash should not be nil")

}

func TestMicroserviceReadyForUpgrade(t *testing.T) {
	dir, db, err := setupDB()
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))

	pms := createService(t)

	assert.True(t, MicroserviceReadyForUpgrade(pms, db), "")

	pms.Archived = true
	assert.False(t, MicroserviceReadyForUpgrade(pms, db), "Archived ms is not ready for update")
	pms.Archived = false

	pms.AutoUpgrade = false
	assert.False(t, MicroserviceReadyForUpgrade(pms, db), "AutoUpgrade must be true in order for update")
	pms.AutoUpgrade = true

	pms.UpgradeStartTime = uint64(1)
	pms.UpgradeMsReregisteredTime = uint64(0)
	pms.UpgradeFailedTime = uint64(0)
	assert.False(t, MicroserviceReadyForUpgrade(pms, db), "Should not upgrade if in the middle of an upgrade")
	pms.UpgradeStartTime = uint64(0)

	pms.Id = "1"
	msi, err := persistence.NewMicroserviceInstance(db, pms.SpecRef, pms.Org, pms.Version, pms.Id, []persistence.ServiceInstancePathElement{}, false)
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))

	pms.AutoUpgrade = true
	pms.ActiveUpgrade = false
	assert.True(t, MicroserviceReadyForUpgrade(pms, db), "Should allow upgrade for inactive upgrade when there are no agreements associated with it.")

	msi, err = persistence.UpdateMSInstanceAssociatedAgreements(db, msi.GetKey(), true, "agreementid1")
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))
	assert.False(t, MicroserviceReadyForUpgrade(pms, db), "Should not allow upgrade for inactive upgrade when there are agreements associated with it.")

	pms.ActiveUpgrade = true
	assert.True(t, MicroserviceReadyForUpgrade(pms, db), "Should allow upgrade for active upgrade even when there are agreements associated with it.")

	err = cleanupDB(dir)
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))
}

func TestGetUpgradeMicroserviceDef(t *testing.T) {
	dir, db, err := setupDB()
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))

	pms := createService(t)

	// invalide verision range
	saved_vr := pms.UpgradeVersionRange
	pms.UpgradeVersionRange = "abc"
	_, err = GetUpgradeMicroserviceDef(getVariableExchangeDefinitionHandler("2.0"), pms, db)
	assert.NotNil(t, err, "Invalid version range fromat should result in error")
	pms.UpgradeVersionRange = saved_vr

	// higher version
	new_ms, err := GetUpgradeMicroserviceDef(getVariableExchangeDefinitionHandler("2.0"), pms, db)
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))
	assert.NotNil(t, new_ms, "should return a new ms")
	assert.Equal(t, "2.0", new_ms.Version, "should have a higher version")
	assert.Equal(t, pms.AutoUpgrade, new_ms.AutoUpgrade, "")
	assert.Equal(t, pms.ActiveUpgrade, new_ms.ActiveUpgrade, "")
	assert.Equal(t, pms.Name, new_ms.Name, "")
	assert.Equal(t, pms.UpgradeVersionRange, new_ms.UpgradeVersionRange, "")

	// lower version
	new_ms, err = GetUpgradeMicroserviceDef(getVariableExchangeDefinitionHandler("0.5"), pms, db)
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))
	assert.Nil(t, new_ms, fmt.Sprintf("should return a nil ms, but got this: %v", new_ms))

	// same version but different hash
	new_ms, err = GetUpgradeMicroserviceDef(getVariableExchangeDefinitionHandler("1.0.0"), pms, db)
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))
	assert.NotNil(t, new_ms, "should return a new ms")
	assert.Equal(t, "1.0.0", new_ms.Version, "should have the same version")

	err = cleanupDB(dir)
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))
}

func TestGetRollbackMicroserviceDef(t *testing.T) {
	dir, db, err := setupDB()
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))

	pms := createService(t)

	// invalide verision range
	saved_vr := pms.UpgradeVersionRange
	pms.UpgradeVersionRange = "abc"
	_, err = GetRollbackMicroserviceDef(getVariableExchangeDefinitionHandler("2.0"), pms, db)
	assert.NotNil(t, err, "Invalid version range fromat should result in error")
	pms.UpgradeVersionRange = saved_vr

	// always return lower version
	new_ms, err := GetRollbackMicroserviceDef(getVariableExchangeDefinitionHandler("0.5"), pms, db)
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))
	assert.NotNil(t, new_ms, "should return a new ms")
	assert.Equal(t, "0.5", new_ms.Version, "should have a lower version")
	assert.Equal(t, pms.AutoUpgrade, new_ms.AutoUpgrade, "")
	assert.Equal(t, pms.ActiveUpgrade, new_ms.ActiveUpgrade, "")
	assert.Equal(t, pms.Name, new_ms.Name, "")
	assert.Equal(t, pms.UpgradeVersionRange, new_ms.UpgradeVersionRange, "")

	err = cleanupDB(dir)
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))
}

func TestUnregisterServiceExchange(t *testing.T) {

	checkPatchDeviceHandler := func(t *testing.T, mss []exchange.Microservice, url string) exchange.PatchDeviceHandler {
		return func(id string, token string, pdr *exchange.PatchDeviceRequest) error {

			assert.Equal(t, len(mss)-1, len(*pdr.RegisteredServices), "one service should have been removed")

			for _, ms := range *pdr.RegisteredServices {
				assert.False(t, ms.Url == url, fmt.Sprintf("%v should have been removed", url))
			}

			return nil
		}
	}

	dir, db, err := setupDB()
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))

	m1 := exchange.Microservice{
		Url:           "myorg/gps",
		Version:       "1.0",
		Properties:    nil,
		NumAgreements: 0,
		Policy:        "blahblah",
	}
	m2 := exchange.Microservice{
		Url:           "myorg/network",
		Version:       "1.0",
		Properties:    nil,
		NumAgreements: 0,
		Policy:        "blahblah",
	}
	m3 := exchange.Microservice{
		Url:           "myorg/pwsms",
		Version:       "1.0",
		Properties:    nil,
		NumAgreements: 0,
		Policy:        "blahblah",
	}
	mss := []exchange.Microservice{m1, m2, m3}

	org := "myorg"
	device_id := "mydevice"
	device_token := "mytoken"
	device_name := "mydevicename"
	url := "network"
	version := "1.0"

	// save device in db
	_, err = persistence.SaveNewExchangeDevice(db, "mydevice", device_token, device_name, "device", false, org, "netspeed-amd64", "configuring", persistence.SoftwareVersion{persistence.AGENT_VERSION: "1.0.0"})
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))

	err = UnregisterMicroserviceExchange(getVariableDeviceHandler(nil, nil),
		checkPatchDeviceHandler(t, nil, url),
		url, org, version, device_id, device_token, db)
	assert.Nil(t, err, "no registered ms, nothing to do")

	err = UnregisterMicroserviceExchange(getVariableDeviceHandler(nil, mss),
		checkPatchDeviceHandler(t, mss, url),
		url, org, version, device_id, device_token, db)
	assert.Nil(t, err, "eveything should have worked")

	err = cleanupDB(dir)
	assert.Nil(t, err, fmt.Sprintf("should not return error, but got this: %v", err))
}

func createService(t *testing.T) *persistence.MicroserviceDefinition {
	hwm := exchange.HardwareRequirement{
		"USBDeviceIds": "1546:01a7",
		"Devfiles":     "/dev/ttyUSB*,/dev/ttyACM*",
	}
	ut1 := exchangecommon.UserInput{
		Name:         "foo1",
		Label:        "The Foo1 Value",
		Type:         "string",
		DefaultValue: "bar1",
	}
	ut2 := exchangecommon.UserInput{
		Name:         "foo2",
		Label:        "The Foo2 Value",
		Type:         "string",
		DefaultValue: "bar2",
	}
	sd := exchangecommon.ServiceDependency{
		URL:     "https://bluehorizon.network/services/other",
		Org:     "myorg",
		Version: "1.0.0",
		Arch:    cutil.ArchString(),
	}

	ems := exchange.ServiceDefinition{
		Owner:               "bob",
		Label:               "GPS for ARM",
		Description:         "my services",
		Public:              false,
		URL:                 "https://bluehorizon.network/microservices/gps",
		Version:             "1.0.0",
		Arch:                cutil.ArchString(),
		Sharable:            "singleton",
		MatchHardware:       hwm,
		RequiredServices:    []exchangecommon.ServiceDependency{sd},
		UserInputs:          []exchangecommon.UserInput{ut1, ut2},
		Deployment:          "",
		DeploymentSignature: "",
		LastUpdated:         "2017-11-14T22:36:52.748Z[UTC]",
	}

	pms, err := ConvertServiceToPersistent(&ems, "mycompany")

	// check error
	assert.Nil(t, err, fmt.Sprintf("Shold return no error, but got %v", err))

	// check a few attributes
	assert.Equal(t, ems.URL, pms.SpecRef, "The assignment should work")
	assert.Equal(t, ems.Version, pms.Version, "The assignment should work")
	assert.Equal(t, ems.Arch, pms.Arch, "The assignment should work")
	assert.Equal(t, "mycompany", pms.Org, "The assignment should work")
	assert.Equal(t, len(ems.UserInputs), len(pms.UserInputs), "The assignment should work")
	assert.Equal(t, len(ems.RequiredServices), len(pms.RequiredServices), "The assignment should work")

	return pms
}

func getVariableServiceHandler(version string) exchange.ServiceHandler {
	return func(mUrl string, mOrg string, mVersion string, mArch string) (*exchange.ServiceDefinition, string, error) {
		md := exchange.ServiceDefinition{
			Owner:         "owner",
			Label:         "label",
			Description:   "desc",
			URL:           mUrl,
			Version:       version,
			Arch:          mArch,
			Sharable:      exchangecommon.SERVICE_SHARING_MODE_EXCLUSIVE,
			MatchHardware: exchange.HardwareRequirement{},
			UserInputs:    []exchangecommon.UserInput{},
			LastUpdated:   "today",
		}
		return &md, "service-id", nil
	}
}

func getVariableExchangeDefinitionHandler(version string) exchange.ServiceResolverHandler {
	return func(mUrl string, mOrg string, mVersion string, mArch string) (*policy.APISpecList, *exchange.ServiceDefinition, []string, error) {
		md := exchange.ServiceDefinition{
			Owner:         "owner",
			Label:         "label",
			Description:   "desc",
			URL:           mUrl,
			Version:       version,
			Arch:          mArch,
			Sharable:      exchangecommon.SERVICE_SHARING_MODE_EXCLUSIVE,
			MatchHardware: exchange.HardwareRequirement{},
			UserInputs:    []exchangecommon.UserInput{},
			LastUpdated:   "today",
		}

		return nil, &md, []string{}, nil
	}
}

func getVariableDeviceHandler(mss []exchange.Microservice, ss []exchange.Microservice) exchange.DeviceHandler {
	return func(id string, token string) (*exchange.Device, error) {
		d := exchange.Device{
			Token:              token,
			Name:               id,
			Owner:              "bob",
			Pattern:            "netspeed-amd64",
			RegisteredServices: ss,
			MsgEndPoint:        "",
			SoftwareVersions:   nil,
			LastHeartbeat:      "now",
			PublicKey:          "",
		}
		return &d, nil
	}
}

func setupDB() (string, *bolt.DB, error) {
	dir, err := ioutil.TempDir("", "container-")
	if err != nil {
		return "", nil, err
	}

	db, err := bolt.Open(path.Join(dir, "anax-int.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		return dir, nil, err
	}

	return dir, db, nil
}

func cleanupDB(dir string) error {
	return os.RemoveAll(dir)
}
