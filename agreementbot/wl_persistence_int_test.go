// +build integration

package agreementbot

import (
	"github.com/boltdb/bolt"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

var testDb *bolt.DB

func TestMain(m *testing.M) {
	testDbFile, err := ioutil.TempFile("", "agreementbot_test.db")
	if err != nil {
		panic(err)
	}
	defer os.Remove(testDbFile.Name())

	var dbErr error
	testDb, dbErr = bolt.Open(testDbFile.Name(), 0600, &bolt.Options{Timeout: 10 * time.Second})
	if dbErr != nil {
		panic(err)
	}

	m.Run()
}

func Test_SaveNewRecord1(t *testing.T) {

	deviceid := "an12345"
	pName := "test policy"

	if err := NewWorkloadUsage(testDb, deviceid, []string{}, "{some json serialized policy file}", pName, 1, 30, 180, false, "AG1"); err != nil {
		t.Errorf("Received error creating new workload usage: %v", err)
	} else if wlu, err := FindSingleWorkloadUsageByDeviceAndPolicyName(testDb, deviceid, pName); err != nil {
		t.Errorf("Received error finding new record: %v", err)
	} else if wlu.Priority != 1 {
		t.Errorf("Record received on read does not have the right priority, expecting 1, was %v", wlu.Priority)
	}

	if err := NewWorkloadUsage(testDb, deviceid, []string{}, "{some json serialized policy file}", pName, 1, 30, 180, false, "AG1"); err == nil {
		t.Errorf("Should have received error creating duplicate record")
	}

	deviceid = "an54321"
	if wlu, err := FindSingleWorkloadUsageByDeviceAndPolicyName(testDb, deviceid, pName); err != nil {
		t.Errorf("Received error finding non-existent record: %v", err)
	} else if wlu != nil {
		t.Errorf("Received record %v that should not have been returned.", wlu)
	}

}

func Test_SaveNewRecord2(t *testing.T) {

	deviceid := "an67890"
	pName := "test policy"

	if err := NewWorkloadUsage(testDb, deviceid, []string{}, "{some json serialized policy file}", pName, 2, 30, 180, false, "AG1"); err != nil {
		t.Errorf("Received error creating new workload usage: %v", err)
	} else if wlu, err := FindSingleWorkloadUsageByDeviceAndPolicyName(testDb, deviceid, pName); err != nil {
		t.Errorf("Received error finding new record: %v", err)
	} else if wlu.Priority != 2 {
		t.Errorf("Record received on read does not have the right priority, expecting 1, was %v", wlu.Priority)
	}

	if err := DeleteWorkloadUsage(testDb, deviceid, pName); err != nil {
		t.Errorf("Received error deleting workload usage: %v", err)
	} else if wlu, err := FindSingleWorkloadUsageByDeviceAndPolicyName(testDb, deviceid, pName); err != nil {
		t.Errorf("Received error finding new record: %v", err)
	} else if wlu != nil {
		t.Errorf("Received record %v that should not have been returned.", wlu)
	}

}
