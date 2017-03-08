package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

func Test_reads_and_writes_file(t *testing.T) {

	var saved_pf *Policy

	if pf, err := ReadPolicyFile("./test/pftest/test1.policy"); err != nil {
		t.Error(err)
	} else if pf.Header.Name != "test policy" {
		t.Errorf("Demarshalled file has incorrect name: %v", pf.Header.Name)
	} else if pf.DeviceType != "12345-54321-abcdef-fedcba" {
		t.Errorf("Demarshalled file has incorrect DeviceType: %v", pf.DeviceType)
	} else if pf.ResourceLimits.CPUs != 2 {
		t.Errorf("Demarshalled file has incorrect ResourceLimits section: %v", pf.ResourceLimits.CPUs)
	} else if pf.DataVerify.Interval != 300 {
		t.Errorf("Demarshalled file has incorrect DataVerifiy section, interval: %v", pf.DataVerify.Interval)
	} else if pf.ProposalReject.Number != 5 {
		t.Errorf("Demarshalled file has incorrect Proposal Rejections: %v", pf.ProposalReject.Number)
	} else {
		saved_pf = pf
	}

	if _, err := os.Stat("./test/pftest/echo.policy"); !os.IsNotExist(err) {
		os.Remove("./test/pftest/echo.policy")
	}

	if err := WritePolicyFile(saved_pf, "./test/pftest/echo.policy"); err != nil {
		t.Error(err)
	} else if policyFile1, err := os.Open("./test/pftest/test1.policy"); err != nil {
		t.Errorf("Unable to open test1.policy policy file, error: %v", err)
	} else if pf1bytes, err := ioutil.ReadAll(policyFile1); err != nil {
		t.Errorf("Unable to read test1.policy policy file, error: %v", err)
	} else if policyFile2, err := os.Open("./test/pftest/echo.policy"); err != nil {
		t.Errorf("Unable to open echo.policy policy file, error: %v", err)
	} else if pf2bytes, err := ioutil.ReadAll(policyFile2); err != nil {
		t.Errorf("Unable to read echo.policy policy file, error: %v", err)
	} else if bytes.Compare(pf1bytes, pf2bytes) != 0 {
		t.Errorf("Echoed policy file %v does not match original file %v", string(pf2bytes), string(pf1bytes))
	}

}

func Test_getPolicyFiles(t *testing.T) {

	if files, err := getPolicyFiles("./test/pftest/"); err != nil {
		t.Error(err)
	} else if len(files) != 2 || files[0].Name() != "echo.policy" || files[1].Name() != "test1.policy" {
		t.Errorf("Did not see all policy files, saw %v", files)
	}
}

func Test_getPolicyFiles_NoDir(t *testing.T) {

	if _, err := getPolicyFiles("./test/notexist/"); err == nil {
		t.Error("Expected no such directory error, but no error was returned.")
	} else if !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("Expected 'no such directory' error, but received %v", err)
	}
}

func Test_getPolicyFiles_EmptyDir(t *testing.T) {

	if files, err := getPolicyFiles("./test/pfempty/"); err != nil {
		t.Error(err)
	} else if len(files) != 0 {
		t.Errorf("Expected an empty directory, but received %v", files)
	}
}

func Test_PolicyFileChangeWatcher(t *testing.T) {

	var deleteDetected = 0
	var changeDetected = 0
	var errorDetected = 0
	var checkInterval = 1

	changeNotify := func(fileName string, policy *Policy) {
		changeDetected += 1
		// fmt.Printf("Change to %v\n", fileName)
	}

	deleteNotify := func(fileName string, policy *Policy) {
		deleteDetected += 1
		// fmt.Printf("Delete for %v\n", fileName)
	}

	errorNotify := func(fileName string, err error) {
		errorDetected += 1
		fmt.Printf("Error for %v error %v\n", fileName, err)
	}

	// Test a single call into the watcher
	if err := PolicyFileChangeWatcher("./test/pfwatchtest/", changeNotify, deleteNotify, errorNotify, 0); err != nil {
		t.Error(err)
	} else if changeDetected != 1 || deleteDetected != 0 || errorDetected != 0 {
		t.Errorf("Incorrect number of events fired. Expected 1 change, saw %v, expected 0 deletes, saw %v, expected 0 errors, saw %v", changeDetected, deleteDetected, errorDetected)
	} else {
		changeDetected = 0
	}

	// Test a continously running watcher
	go PolicyFileChangeWatcher("./test/pfwatchtest/", changeNotify, deleteNotify, errorNotify, checkInterval)

	// Give the watcher a chance to read the contents of the pfwatchtest directory and fire events
	time.Sleep(2 * time.Second)

	// Add a new policy file
	newPolicyContent := `{"header":{"name":"new policy","version":"1.0"}}`
	newPolicy := new(Policy)
	if err := json.Unmarshal([]byte(newPolicyContent), newPolicy); err != nil {
		t.Errorf("Error demarshalling new policy: %v", err)
	} else if err := WritePolicyFile(newPolicy, "./test/pfwatchtest/new.policy"); err != nil {
		t.Errorf("Error writing new policy: %v", err)
	}

	// Give the watcher a chance to read the new policy
	time.Sleep(2 * time.Second)

	// Change the newly created file
	if err := WritePolicyFile(newPolicy, "./test/pfwatchtest/new.policy"); err != nil {
		t.Errorf("Error writing new policy: %v", err)
	}

	// Give the watcher a chance to read the change policy
	time.Sleep(2 * time.Second)

	// Remove the new file and give the watcher a chance to see it
	os.Remove("./test/pfwatchtest/new.policy")
	time.Sleep(2 * time.Second)

	if changeDetected != 3 || deleteDetected != 1 || errorDetected != 0 {
		t.Errorf("Incorrect number of events fired. Expected 3 changes, saw %v, expected 1 delete, saw %v, expected 0 errors, saw %v", changeDetected, deleteDetected, errorDetected)
	}

}

func Test_PolicyFileChangeWatcher_Empty(t *testing.T) {

	var deleteDetected = 0
	var changeDetected = 0
	var errorDetected = 0

	changeNotify := func(fileName string, policy *Policy) {
		changeDetected += 1
		// fmt.Printf("Change to %v\n", fileName)
	}

	deleteNotify := func(fileName string, policy *Policy) {
		deleteDetected += 1
		// fmt.Printf("Delete for %v\n", fileName)
	}

	errorNotify := func(fileName string, err error) {
		errorDetected += 1
		// fmt.Printf("Error for %v error %v\n", fileName, err)
	}

	// Test a single call into the watcher
	if err := PolicyFileChangeWatcher("./test/pfempty/", changeNotify, deleteNotify, errorNotify, 0); err != nil {
		t.Error(err)
	} else if changeDetected != 0 || deleteDetected != 0 || errorDetected != 0 {
		t.Errorf("Incorrect number of events fired. Expected 0 changes, saw %v, expected 0 deletes, saw %v, expected 0 errors, saw %v", changeDetected, deleteDetected, errorDetected)
	}
}

func Test_PolicyFileChangeWatcher_NoDir(t *testing.T) {

	var deleteDetected = 0
	var changeDetected = 0
	var errorDetected = 0

	changeNotify := func(fileName string, policy *Policy) {
		changeDetected += 1
		// fmt.Printf("Change to %v\n", fileName)
	}

	deleteNotify := func(fileName string, policy *Policy) {
		deleteDetected += 1
		// fmt.Printf("Delete for %v\n", fileName)
	}

	errorNotify := func(fileName string, err error) {
		errorDetected += 1
		// fmt.Printf("Error for %v error %v\n", fileName, err)
	}

	// Test a single call into the watcher
	if err := PolicyFileChangeWatcher("./test/notexist/", changeNotify, deleteNotify, errorNotify, 0); err == nil {
		t.Error("Expected 'no such directory error', but no error was returned.")
	} else if !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("Expected 'no such directory' error, but received %v", err)
	}
}

// And now for some full policy file compatibility tests.
// Let's start with a compatible test between a producer and consumer.
func Test_Policy_Compatible(t *testing.T) {

	if pf_prod, err := ReadPolicyFile("./test/pfcompat1/device.policy"); err != nil {
		t.Error(err)
	} else if pf_con, err := ReadPolicyFile("./test/pfcompat1/agbot.policy"); err != nil {
		t.Error(err)
	} else if err := Are_Compatible(pf_prod, pf_con); err != nil {
		t.Error(err)
	}
}

// Let's try an incompatible test between a producer and consumer.
func Test_Policy_Incompatible(t *testing.T) {

	if pf_prod, err := ReadPolicyFile("./test/pfincompat1/device.policy"); err != nil {
		t.Error(err)
	} else if pf_con, err := ReadPolicyFile("./test/pfincompat1/agbot.policy"); err != nil {
		t.Error(err)
	} else if err := Are_Compatible(pf_prod, pf_con); err == nil {
		t.Errorf("Error: %v is not compatible with %v\n", pf_prod, pf_con)
	}
}

// Finally, merge 2 policy files (producer and consumer.) together and make sure the merged
// policy is what we would expect.
//
func Test_Policy_Merge(t *testing.T) {

	if _, err := os.Stat("./test/pfmerge1/merged.policy"); !os.IsNotExist(err) {
		os.Remove("./test/pfmerge1/merged.policy")
	}

	if pf_prod, err := ReadPolicyFile("./test/pfmerge1/device.policy"); err != nil {
		t.Error(err)
	} else if pf_con, err := ReadPolicyFile("./test/pfmerge1/agbot.policy"); err != nil {
		t.Error(err)
	} else if pf_merged, err := Create_Terms_And_Conditions(pf_prod, pf_con, "12345", ""); err != nil {
		t.Error(err)
	} else if err := WritePolicyFile(pf_merged, "./test/pfmerge1/merged.policy"); err != nil {
		t.Error(err)
	} else if mpPolicy, err := os.Open("./test/pfmerge1/merged.policy"); err != nil {
		t.Errorf("Unable to open merged.policy policy file, error: %v", err)
	} else if mpbytes, err := ioutil.ReadAll(mpPolicy); err != nil {
		t.Errorf("Unable to read merged.policy policy file, error: %v", err)
	} else if expectingPolicy, err := os.Open("./test/pfmerge1/expecting.policy"); err != nil {
		t.Errorf("Unable to open expecting.policy policy file, error: %v", err)
	} else if epbytes, err := ioutil.ReadAll(expectingPolicy); err != nil {
		t.Errorf("Unable to read expecting.policy policy file, error: %v", err)
	} else if bytes.Compare(mpbytes, epbytes) != 0 {
		t.Errorf("Merged policy file %v does not match expected file %v", string(mpbytes), string(epbytes))
	}
}

// Now let's try a compatibility test between 2 producers.
func Test_Producer_Policy_Compatible(t *testing.T) {

	if _, err := os.Stat("./test/pfcompat2/merged.policy"); !os.IsNotExist(err) {
		os.Remove("./test/pfcompat2/merged.policy")
	}

	if pf_prod1, err := ReadPolicyFile("./test/pfcompat2/device1.policy"); err != nil {
		t.Error(err)
	} else if pf_prod2, err := ReadPolicyFile("./test/pfcompat2/device2.policy"); err != nil {
		t.Error(err)
	} else if pf_merged, err := Are_Compatible_Producers(pf_prod1, pf_prod2); err != nil {
		t.Error(err)
	} else if err := WritePolicyFile(pf_merged, "./test/pfcompat2/merged.policy"); err != nil {
		t.Error(err)
	} else if mpPolicy, err := os.Open("./test/pfcompat2/merged.policy"); err != nil {
		t.Errorf("Unable to open merged.policy policy file, error: %v", err)
	} else if mpbytes, err := ioutil.ReadAll(mpPolicy); err != nil {
		t.Errorf("Unable to read merged.policy policy file, error: %v", err)
	} else if expectingPolicy, err := os.Open("./test/pfcompat2/expecting.policy"); err != nil {
		t.Errorf("Unable to open expecting.policy policy file, error: %v", err)
	} else if epbytes, err := ioutil.ReadAll(expectingPolicy); err != nil {
		t.Errorf("Unable to read expecting.policy policy file, error: %v", err)
	} else if bytes.Compare(mpbytes, epbytes) != 0 {
		t.Errorf("Merged policy file %v does not match expected file %v", string(mpbytes), string(epbytes))
	}
}

// Now let's create a Policy through the APIs and then verify that we got what we expected.
func Test_Policy_Creation(t *testing.T) {

	if _, err := os.Stat("./test/pfcreate/created.policy"); !os.IsNotExist(err) {
		os.Remove("./test/pfcreate/created.policy")
	}

	pf_created := Policy_Factory("test creation")

	pf_created.Add_API_Spec(APISpecification_Factory("http://mycompany.com/dm/cpu_temp", "1.0.0", "arm"))
	pf_created.Add_API_Spec(APISpecification_Factory("http://mycompany.com/dm/gps", "1.0.0", "arm"))

	pf_created.Add_Agreement_Protocol(AgreementProtocol_Factory(CitizenScientist))
	pf_created.Add_Agreement_Protocol(AgreementProtocol_Factory("2Party Bitcoin"))

	var details interface{}
	details_bc1 := make(map[string][]string)
	details_bc1["bootnodes"] = append(details_bc1["bootnodes"], "http://bhnetwork.com/bootnodes")
	details_bc1["directory"] = append(details_bc1["directory"], "http://bhnetwork.com/directory")
	details_bc1["genesis"] = append(details_bc1["genesis"], "http://bhnetwork.com/genesis")
	details_bc1["networkid"] = append(details_bc1["networkid"], "http://bhnetwork.com/networkid")
	details = details_bc1
	bc1 := Blockchain_Factory(Ethereum_bc, details)
	details_bc2 := make(map[string][]string)
	details_bc2["bootnodes"] = append(details_bc2["bootnodes"], "http://bhnetwork.staging.com/bootnodes")
	details_bc2["directory"] = append(details_bc2["directory"], "http://bhnetwork.staging.com/directory")
	details_bc2["genesis"] = append(details_bc2["genesis"], "http://bhnetwork.staging.com/genesis")
	details_bc2["networkid"] = append(details_bc2["networkid"], "http://bhnetwork.staging.com/networkid")
	details = details_bc2
	bc2 := Blockchain_Factory(Ethereum_bc, details)
	pf_created.Add_Blockchain(bc1)
	pf_created.Add_Blockchain(bc2)

	pf_created.Add_Property(Property_Factory("rpiprop1", "rpival1"))
	pf_created.Add_Property(Property_Factory("rpiprop2", "rpival2"))
	pf_created.Add_Property(Property_Factory("rpiprop3", "rpival3"))
	pf_created.Add_Property(Property_Factory("rpiprop4", "rpival4"))
	pf_created.Add_Property(Property_Factory("rpiprop5", "rpival5"))

	if err := WritePolicyFile(pf_created, "./test/pfcreate/created.policy"); err != nil {
		t.Error(err)
	} else if mpPolicy, err := os.Open("./test/pfcreate/created.policy"); err != nil {
		t.Errorf("Unable to open created.policy policy file, error: %v", err)
	} else if mpbytes, err := ioutil.ReadAll(mpPolicy); err != nil {
		t.Errorf("Unable to read created.policy policy file, error: %v", err)
	} else if expectingPolicy, err := os.Open("./test/pfcreate/expecting.policy"); err != nil {
		t.Errorf("Unable to open expecting.policy policy file, error: %v", err)
	} else if epbytes, err := ioutil.ReadAll(expectingPolicy); err != nil {
		t.Errorf("Unable to read expecting.policy policy file, error: %v", err)
	} else if bytes.Compare(mpbytes, epbytes) != 0 {
		t.Errorf("Generated policy file %v does not match expected file %v", string(mpbytes), string(epbytes))
	}

}

// Now let's make sure we are obscuring the workload password
func Test_Policy_Workload_obscure1(t *testing.T) {
	if pf_prod1, err := ReadPolicyFile("./test/pftest/test1.policy"); err != nil {
		t.Error(err)
	} else {
		pf_prod1.Workloads[0].WorkloadPassword = "abcdefg"
		if err := pf_prod1.ObscureWorkloadPWs("123456",""); err != nil {
			t.Error(err)
		} else if pf_prod1.Workloads[0].WorkloadPassword == "abcdefg" {
			t.Errorf("Password was not obscured in %v", pf_prod1.Workloads[0])
		}
	}
}

func Test_Policy_Workload_obscure2(t *testing.T) {
	if pf_prod1, err := ReadPolicyFile("./test/pftest/test1.policy"); err != nil {
		t.Error(err)
	} else {
		pf_prod1.Workloads[0].WorkloadPassword = "abcdefg"
		if err := pf_prod1.ObscureWorkloadPWs("","098765"); err != nil {
			t.Error(err)
		} else if pf_prod1.Workloads[0].WorkloadPassword == "abcdefg" {
			t.Errorf("Password was not obscured in %v", pf_prod1.Workloads[0])
		}
	}
}

// ================================================================================================================
// Helper functions
//
// Create an APISpecification array from a JSON serialization. The JSON serialization
// does not have to be a valid APISpecification serialization, just has to be a valid
// JSON serialization.
func create_APISpecification(jsonString string, t *testing.T) *APISpecList {
	as := new(APISpecList)

	if err := json.Unmarshal([]byte(jsonString), &as); err != nil {
		t.Errorf("Error unmarshalling APISpecification json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return as
	}
}

// Create a Blockchain array from a JSON serialization. The JSON serialization
// does not have to be a valid Blockchain serialization, just has to be a valid
// JSON serialization.
func create_BlockchainList(jsonString string, t *testing.T) *BlockchainList {
	bl := new(BlockchainList)

	if err := json.Unmarshal([]byte(jsonString), &bl); err != nil {
		t.Errorf("Error unmarshalling BlockchainList json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return bl
	}
}

// Create a Property array from a JSON serialization. The JSON serialization
// does not have to be a valid Property serialization, just has to be a valid
// JSON serialization.
func create_PropertyList(jsonString string, t *testing.T) *PropertyList {
	pl := new(PropertyList)

	if err := json.Unmarshal([]byte(jsonString), &pl); err != nil {
		t.Errorf("Error unmarshalling PropertyList json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return pl
	}
}

// Create an AgreementProtocol array from a JSON serialization. The JSON serialization
// does not have to be a valid AgreementProtocol serialization, just has to be a valid
// JSON serialization.
func create_AgreementProtocolList(jsonString string, t *testing.T) *AgreementProtocolList {
	pl := new(AgreementProtocolList)

	if err := json.Unmarshal([]byte(jsonString), &pl); err != nil {
		t.Errorf("Error unmarshalling AgreementProtocolList json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return pl
	}
}
