// +build unit

package exchangesync

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"github.com/open-horizon/anax/persistence"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

var ExchangeNodePolicy *exchange.ExchangeNodePolicy

const NUM_BUILT_INS = 7
const CLUSTER_NUM_BUILT_INS = 4

// Verify that a Node Policy Object can be created and saved the first time.
func Test_UpdateNodePolicy(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	pDevice, err := persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "device", false, "myOrg", "", persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	propName := "prop1"
	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName, "val1"), false)

	extPol := externalpolicy.ExternalPolicy{
		Properties:  *propList,
		Constraints: []string{`prop3 == "some value"`},
	}
	nodePolicy := &exchangecommon.NodePolicy{ExternalPolicy: extPol}

	ExchangeNodePolicy = nil

	_, _, err = UpdateNodePolicy(pDevice, db, nodePolicy, getDummyNodePolicyHandler(), getDummyPutNodePolicyHandler())

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if fnp, err := persistence.FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != 1+NUM_BUILT_INS {
		t.Errorf("incorrect node policy, there should be %v property defined, found: %v", 1+NUM_BUILT_INS, *fnp)
	} else if fnp.Properties[0].Name != propName {
		t.Errorf("expected property %v, but received %v", propName, fnp.Properties[0].Name)
	}

}

// Verify that a Node Policy Object can be created and deleted.
func Test_DeleteNodePolicy(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	pDevice, err := persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "cluster", false, "myOrg", "", persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	propName := "prop1"
	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName, "val1"), false)

	extPol := externalpolicy.ExternalPolicy{
		Properties:  *propList,
		Constraints: []string{`prop3 == "some value"`},
	}
	nodePolicy := &exchangecommon.NodePolicy{ExternalPolicy: extPol}

	ExchangeNodePolicy = nil

	_, _, err = UpdateNodePolicy(pDevice, db, nodePolicy, getDummyNodePolicyHandler(), getDummyPutNodePolicyHandler())

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if fnp, err := persistence.FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != CLUSTER_NUM_BUILT_INS+1 {
		t.Errorf("incorrect node policy, there should be %v property defined, found: %v", CLUSTER_NUM_BUILT_INS+1, *fnp)
	} else if fnp.Properties[0].Name != propName {
		t.Errorf("expected property %v, but received %v", propName, fnp.Properties[0].Name)
	}

	err = DeleteNodePolicy(pDevice, db, getDummyNodePolicyHandler(), getDummyDeleteNodePolicyHandler())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if fnp, err := persistence.FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if fnp != nil {
		t.Errorf("should have gotten nil but found: %v", fnp)
	}
}

// Verify ExchangeNodePolicyChanged works.
func Test_ExchangeNodePolicyChanged(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	pDevice, err := persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "", false, "myOrg", "", persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	propName := "prop1"
	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName, "val1"), false)

	extPol := externalpolicy.ExternalPolicy{
		Properties:  *propList,
		Constraints: []string{`prop3 == "some value"`},
	}
	nodePolicy := &exchangecommon.NodePolicy{ExternalPolicy: extPol}

	ExchangeNodePolicy = nil

	rc_d, rc_m, err := UpdateNodePolicy(pDevice, db, nodePolicy, getDummyNodePolicyHandler(), getDummyPutNodePolicyHandler())

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if fnp, err := persistence.FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != 1+NUM_BUILT_INS {
		t.Errorf("incorrect node policy, there should be %v property defined, found: %v", 1+NUM_BUILT_INS, *fnp)
	} else if fnp.Properties[0].Name != propName {
		t.Errorf("expected property %v, but received %v", propName, fnp.Properties[0].Name)
	} else if rc_d != externalpolicy.EP_COMPARE_PROPERTY_CHANGED|externalpolicy.EP_COMPARE_CONSTRAINT_CHANGED ||
		rc_m != externalpolicy.EP_COMPARE_PROPERTY_CHANGED|externalpolicy.EP_COMPARE_CONSTRAINT_CHANGED {
		t.Errorf("UpdateNodePolicy should have returned (%v, %v), but got (%v, %v).",
			externalpolicy.EP_COMPARE_PROPERTY_CHANGED|externalpolicy.EP_COMPARE_CONSTRAINT_CHANGED,
			externalpolicy.EP_COMPARE_PROPERTY_CHANGED|externalpolicy.EP_COMPARE_CONSTRAINT_CHANGED,
			rc_d, rc_m)
	}

	changed, _, err := ExchangeNodePolicyChanged(pDevice, db, getDummyNodePolicyHandler())
	if err != nil {
		t.Errorf("Unexpected error calling ExchangeNodePolicyChanged: %v", err)
	} else if changed {
		t.Errorf("Should return false but got true")
	}

	// update the exchange only
	getDummyPutNodePolicyHandler()(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), nodePolicy)
	changed1, _, err := ExchangeNodePolicyChanged(pDevice, db, getDummyNodePolicyHandler())
	if err != nil {
		t.Errorf("Unexpected error calling ExchangeNodePolicyChanged: %v", err)
	} else if !changed1 {
		t.Errorf("Should return true but got false")
	}

}

// Verify setting node policy from file.
func Test_SetDefaultNodePolicy(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	pDevice, err := persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "device", false, "myOrg", "", persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	ExchangeNodePolicy = nil

	// bad file name
	var config config.HorizonConfig
	config.Edge.DefaultNodePolicyFile = "fake file name"
	_, err = SetDefaultNodePolicy(&config, pDevice, db, getDummyNodePolicyHandler(), getDummyPutNodePolicyHandler())
	if err != nil {
		t.Errorf("Should not have returned error but got: %v", err)
	}

	// bad file format
	config.Edge.DefaultNodePolicyFile = "./test/nodepolicy_bad.json"
	_, err = SetDefaultNodePolicy(&config, pDevice, db, getDummyNodePolicyHandler(), getDummyPutNodePolicyHandler())
	if err == nil {
		t.Errorf("Should have returned but not")
	} else if !strings.Contains(err.Error(), "Failed to unmarshal") {
		t.Errorf("Wrong error, should say 'Failed to unmarshal' but got: %v", err)
	}

	// good file format
	config.Edge.DefaultNodePolicyFile = "./test/nodepolicy_test1.json"
	_, err = SetDefaultNodePolicy(&config, pDevice, db, getDummyNodePolicyHandler(), getDummyPutNodePolicyHandler())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if fnp, err := persistence.FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != 2+NUM_BUILT_INS {
		t.Errorf("incorrect node policy, there should be %v property defined, found: %v", 2+NUM_BUILT_INS, *fnp)
	} else if fnp.Properties[0].Name != "purpose" {
		t.Errorf("expected property %v, but received %v", "purpose", fnp.Properties[0].Name)
	}
}

// Verify initial setup. It will read from the default node policy file.
func Test_NodePolicyInitalSetup(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	ExchangeNodePolicy = nil

	var config config.HorizonConfig
	config.Edge.DefaultNodePolicyFile = "./test/nodepolicy_test1.json"

	// device does not exist yet
	_, err = NodePolicyInitalSetup(db, &config, getDummyNodePolicyHandler(), getDummyPutNodePolicyHandler())
	if err == nil {
		t.Errorf("Should have returned but not")
	} else if !strings.Contains(err.Error(), "Exchange registration not recorded") {
		t.Errorf("Wrong error, should say 'Exchange registration not recorded' but got: %v", err)
	}

	pDevice, err := persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "", false, "myOrg", "", persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	_, err = NodePolicyInitalSetup(db, &config, getDummyNodePolicyHandler(), getDummyPutNodePolicyHandler())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if fnp, err := persistence.FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != 2+NUM_BUILT_INS {
		t.Errorf("incorrect node policy, there should be %v property defined, found: %v", 2+NUM_BUILT_INS, *fnp)
	} else if fnp.Properties[0].Name != "purpose" {
		t.Errorf("expected property %v, but received %v", "purpose", fnp.Properties[0].Name)
	}

	// make change to the exchange
	propName := "prop1"
	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName, "val1"), false)

	extPol := externalpolicy.ExternalPolicy{
		Properties:  *propList,
		Constraints: []string{`prop3 == "some value"`},
	}
	nodePolicy := &exchangecommon.NodePolicy{ExternalPolicy: extPol}
	getDummyPutNodePolicyHandler()(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), nodePolicy)

	// delete the local node policy
	err = persistence.DeleteNodePolicy(db)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	//now run NodePolicyInitalSetup and see that the exchange is the master
	_, err = NodePolicyInitalSetup(db, &config, getDummyNodePolicyHandler(), getDummyPutNodePolicyHandler())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if fnp, err := persistence.FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if fnp == nil {
		t.Errorf("FindNodePolicy returned nil but should not.")
	} else if len(fnp.Properties) != 1+NUM_BUILT_INS && len(fnp.Properties) != NUM_BUILT_INS {
		// openhorizon.hardwareId will be present only if the device serial number can be read
		t.Errorf("incorrect node policy, there should be %v property defined, found: %v", 1+NUM_BUILT_INS, *fnp)
	} else if fnp.Properties[0].Name != propName {
		t.Errorf("expected property %v, but received %v", propName, fnp.Properties[0].Name)
	}
}

func getDummyPutNodePolicyHandler() exchange.PutNodePolicyHandler {
	return func(deviceId string, ep *exchangecommon.NodePolicy) (*exchange.PutDeviceResponse, error) {
		if ep == nil {
			ExchangeNodePolicy = nil
		} else {
			if ExchangeNodePolicy == nil {
				ExchangeNodePolicy = &exchange.ExchangeNodePolicy{NodePolicy: *ep}
			} else {
				ExchangeNodePolicy.NodePolicy = *ep
			}
			ExchangeNodePolicy.LastUpdated += "blah"
		}
		return nil, nil
	}
}

func getDummyNodePolicyHandler() exchange.NodePolicyHandler {
	return func(deviceId string) (*exchange.ExchangeNodePolicy, error) {
		if ExchangeNodePolicy != nil {
			return ExchangeNodePolicy, nil
		} else {
			return nil, nil
		}
	}
}

func getDummyDeleteNodePolicyHandler() exchange.DeleteNodePolicyHandler {
	return func(deviceId string) error {
		return nil
	}
}

func utsetup() (string, *bolt.DB, error) {
	dir, err := ioutil.TempDir("", "utdb-")
	if err != nil {
		return "", nil, err
	}

	db, err := bolt.Open(path.Join(dir, "anax-int.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		return dir, nil, err
	}

	return dir, db, nil
}

// Make a deferred call to this function after calling setup(), passing the output dirpath of the setup() function.
func cleanTestDir(dirPath string) error {
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(dirPath); err != nil {
			return err
		}
	}
	return nil
}
