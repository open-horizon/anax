// +build unit

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
		t.Errorf("Demarshalled file has incorrect DataVerify section, interval: %v", pf.DataVerify.Interval)
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

	if _, err := os.Stat("/tmp/pfempty"); !os.IsNotExist(err) {
		os.Remove("/tmp/pfempty")
	}

	if err := os.MkdirAll("/tmp/pfempty", 0644); err != nil {
		t.Error(err)
	}

	if files, err := getPolicyFiles("/tmp/pfempty"); err != nil {
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

	changeNotify := func(org string, fileName string, policy *Policy) {
		if org != "testorg" {
			errorDetected += 1
			fmt.Printf("Error for %v %v, wrong org\n", org, fileName)
		} else {
			changeDetected += 1
		}
	}

	deleteNotify := func(org string, fileName string, policy *Policy) {
		if org != "testorg" {
			errorDetected += 1
			fmt.Printf("Error for %v %v, wrong org\n", org, fileName)
		} else {
			deleteDetected += 1
		}
	}

	errorNotify := func(org string, fileName string, err error) {
		errorDetected += 1
		fmt.Printf("Error for %v %v error %v\n", org, fileName, err)
	}

	// Test a single call into the watcher
	contents := NewContents()
	if _, err := PolicyFileChangeWatcher("./test/pfwatchtest/", contents, changeNotify, deleteNotify, errorNotify, nil, 0); err != nil {
		t.Error(err)
	} else if changeDetected != 1 || deleteDetected != 0 || errorDetected != 0 {
		t.Errorf("Incorrect number of events fired. Expected 1 change, saw %v, expected 0 deletes, saw %v, expected 0 errors, saw %v", changeDetected, deleteDetected, errorDetected)
	} else {
		changeDetected = 0
	}

	// Test a continously running watcher
	contents = NewContents()
	go PolicyFileChangeWatcher("./test/pfwatchtest/", contents, changeNotify, deleteNotify, errorNotify, nil, checkInterval)

	// Give the watcher a chance to read the contents of the pfwatchtest directory and fire events
	time.Sleep(3 * time.Second)

	// Add a new policy file
	newPolicyContent := `{"header":{"name":"new policy","version":"1.0"}}`
	newPolicy := new(Policy)
	if err := json.Unmarshal([]byte(newPolicyContent), newPolicy); err != nil {
		t.Errorf("Error demarshalling new policy: %v", err)
	} else if err := WritePolicyFile(newPolicy, "./test/pfwatchtest/testorg/new.policy"); err != nil {
		t.Errorf("Error writing new policy: %v", err)
	}

	// Give the watcher a chance to read the new policy
	time.Sleep(3 * time.Second)

	// Change the newly created file
	if err := WritePolicyFile(newPolicy, "./test/pfwatchtest/testorg/new.policy"); err != nil {
		t.Errorf("Error writing new policy: %v", err)
	}

	// Give the watcher a chance to read the change policy
	time.Sleep(3 * time.Second)

	// Change the name of the existing policy
	newPolicyContent = `{"header":{"name":"new 2 policy","version":"1.0"}}`
	newPolicy = new(Policy)
	if err := json.Unmarshal([]byte(newPolicyContent), newPolicy); err != nil {
		t.Errorf("Error demarshalling new policy: %v", err)
	} else if err := WritePolicyFile(newPolicy, "./test/pfwatchtest/testorg/new.policy"); err != nil {
		t.Errorf("Error writing new policy: %v", err)
	}

	// Give the watcher a chance to read the change policy
	time.Sleep(3 * time.Second)

	// Remove the new file and give the watcher a chance to see it
	os.Remove("./test/pfwatchtest/testorg/new.policy")
	time.Sleep(3 * time.Second)

	if changeDetected != 4 || deleteDetected != 2 || errorDetected != 0 {
		t.Errorf("Incorrect number of events fired. Expected 4 changes, saw %v, expected 2 delete, saw %v, expected 0 errors, saw %v", changeDetected, deleteDetected, errorDetected)
	}

}

func Test_PolicyFileChangeWatcher_Empty(t *testing.T) {

	var deleteDetected = 0
	var changeDetected = 0
	var errorDetected = 0

	changeNotify := func(org string, fileName string, policy *Policy) {
		changeDetected += 1
		// fmt.Printf("Change to %v\n", fileName)
	}

	deleteNotify := func(org string, fileName string, policy *Policy) {
		deleteDetected += 1
		// fmt.Printf("Delete for %v\n", fileName)
	}

	errorNotify := func(org string, fileName string, err error) {
		errorDetected += 1
		// fmt.Printf("Error for %v error %v\n", fileName, err)
	}

	if _, err := os.Stat("/tmp/pfempty"); !os.IsNotExist(err) {
		os.Remove("/tmp/pfempty")
	}

	if err := os.MkdirAll("/tmp/pfempty", 0644); err != nil {
		t.Error(err)
	}

	// Test a single call into the watcher
	contents := NewContents()
	if _, err := PolicyFileChangeWatcher("/tmp/pfempty", contents, changeNotify, deleteNotify, errorNotify, nil, 0); err != nil {
		t.Error(err)
	} else if changeDetected != 0 || deleteDetected != 0 || errorDetected != 0 {
		t.Errorf("Incorrect number of events fired. Expected 0 changes, saw %v, expected 0 deletes, saw %v, expected 0 errors, saw %v", changeDetected, deleteDetected, errorDetected)
	}
}

func Test_PolicyFileChangeWatcher_NoDir(t *testing.T) {

	var deleteDetected = 0
	var changeDetected = 0
	var errorDetected = 0

	changeNotify := func(org string, fileName string, policy *Policy) {
		changeDetected += 1
		// fmt.Printf("Change to %v\n", fileName)
	}

	deleteNotify := func(org string, fileName string, policy *Policy) {
		deleteDetected += 1
		// fmt.Printf("Delete for %v\n", fileName)
	}

	errorNotify := func(org string, fileName string, err error) {
		errorDetected += 1
		// fmt.Printf("Error for %v error %v\n", fileName, err)
	}

	// Test a single call into the watcher
	contents := NewContents()
	if _, err := PolicyFileChangeWatcher("./test/notexist/", contents, changeNotify, deleteNotify, errorNotify, nil, 0); err == nil {
		t.Error("Expected 'no such directory error', but no error was returned.")
	} else if !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("Expected 'no such directory' error, but received %v", err)
	}
}

// And now for some full policy file compatibility tests.
// Let's start with a compatible test between a producer and consumer.
func Test_Policy_Compatible(t *testing.T) {

	if pf_prod, err := ReadPolicyFile("./test/pfcompat1/testorg/device.policy"); err != nil {
		t.Error(err)
	} else if pf_con, err := ReadPolicyFile("./test/pfcompat1/testorg/agbot.policy"); err != nil {
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
	} else if pf_merged, err := Create_Terms_And_Conditions(pf_prod, pf_con, &pf_con.Workloads[0], "12345", "", 600, 1); err != nil {
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
func Test_Producer_Policy_Compatible_basic(t *testing.T) {

	if _, err := os.Stat("./test/pfcompat2/merged.policy"); !os.IsNotExist(err) {
		os.Remove("./test/pfcompat2/merged.policy")
	}

	if pf_prod1, err := ReadPolicyFile("./test/pfcompat2/device1.policy"); err != nil {
		t.Error(err)
	} else if pf_prod2, err := ReadPolicyFile("./test/pfcompat2/device2.policy"); err != nil {
		t.Error(err)
	} else if pf_merged, err := Are_Compatible_Producers(pf_prod1, pf_prod2, 600); err != nil {
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

	pf_created.Add_API_Spec(APISpecification_Factory("http://mycompany.com/dm/cpu_temp", "myorg", "1.0.0", "arm"))
	pf_created.Add_API_Spec(APISpecification_Factory("http://mycompany.com/dm/gps", "myorg", "1.0.0", "arm"))

	agp1 := AgreementProtocol_Factory(CitizenScientist)
	agp1.Blockchains.Add_Blockchain(Blockchain_Factory(Ethereum_bc, "bc1", "myorg"))
	pf_created.Add_Agreement_Protocol(agp1)
	agp2 := AgreementProtocol_Factory("2Party Bitcoin")
	agp2.Blockchains.Add_Blockchain(Blockchain_Factory(Ethereum_bc, "bc2", "myorg"))
	pf_created.Add_Agreement_Protocol(agp2)

	pf_created.Add_Property(Property_Factory("rpiprop1", "rpival1"))
	pf_created.Add_Property(Property_Factory("rpiprop2", "rpival2"))
	pf_created.Add_Property(Property_Factory("rpiprop3", "rpival3"))
	pf_created.Add_Property(Property_Factory("rpiprop4", "rpival4"))
	pf_created.Add_Property(Property_Factory("rpiprop5", "rpival5"))

	pf_created.Add_NodeHealth(NodeHealth_Factory(600, 30))

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
		if err := pf_prod1.ObscureWorkloadPWs("123456", ""); err != nil {
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
		if err := pf_prod1.ObscureWorkloadPWs("", "098765"); err != nil {
			t.Error(err)
		} else if pf_prod1.Workloads[0].WorkloadPassword == "abcdefg" {
			t.Errorf("Password was not obscured in %v", pf_prod1.Workloads[0])
		}
	}
}

func Test_MinimumProtocolVersion(t *testing.T) {

	var p1, p2 *Policy

	pa := `{"agreementProtocols":[{"name":"` + CitizenScientist + `","blockchains":[{"name":"fred"}]}]}`
	pb := `{"agreementProtocols":[{"name":"` + CitizenScientist + `","blockchains":[{"name":"fred"}]}]}`

	if p1 = create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 = create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pv := p1.MinimumProtocolVersion(CitizenScientist, p2, 2); pv != 1 {
		t.Errorf("Error: the min version should be 1 but was %v\n", pv)
	}

	pa = `{"agreementProtocols":[{"name":"` + CitizenScientist + `","protocolVersion":2}]}`
	pb = `{"agreementProtocols":[{"name":"` + CitizenScientist + `","protocolVersion":1}]}`

	if p1 = create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 = create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pv := p1.MinimumProtocolVersion(CitizenScientist, p2, 1); pv != 1 {
		t.Errorf("Error: the min version should be 1 but was %v\n", pv)
	}

	pa = `{"agreementProtocols":[{"name":"` + CitizenScientist + `","protocolVersion":3}]}`
	pb = `{"agreementProtocols":[{"name":"` + CitizenScientist + `"}]}`

	if p1 = create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 = create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pv := p1.MinimumProtocolVersion(CitizenScientist, p2, 2); pv != 2 {
		t.Errorf("Error: the min version should be 2 but was %v\n", pv)
	}

	pa = `{"agreementProtocols":[{"name":"` + CitizenScientist + `","protocolVersion":3}]}`
	pb = `{"agreementProtocols":[{"name":"` + CitizenScientist + `"}]}`

	if p1 = create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 = create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pv := p1.MinimumProtocolVersion(CitizenScientist, p2, 4); pv != 3 {
		t.Errorf("Error: the min version should be 3 but was %v\n", pv)
	}

	pa = `{"agreementProtocols":[{"name":"` + CitizenScientist + `","protocolVersion":2}]}`
	pb = `{"agreementProtocols":[{"name":"` + CitizenScientist + `","protocolVersion":4}]}`

	if p1 = create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 = create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pv := p1.MinimumProtocolVersion(CitizenScientist, p2, 5); pv != 2 {
		t.Errorf("Error: the min version should be 2 but was %v\n", pv)
	}

}

// Additional producer policy compatibility tests
func Test_ProducerPolicy_empty_apiSpecs(t *testing.T) {

	pa := `{"apiSpec":[]}`
	pb := `{"apiSpec":[]}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 0 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 0)
	}

}

func Test_ProducerPolicy_empty_apiSpec1(t *testing.T) {

	pa := `{"apiSpec":[]}`
	pb := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 1)
	}

}

func Test_ProducerPolicy_empty_apiSpec2(t *testing.T) {

	pa := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`
	pb := `{"apiSpec":[]}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 1)
	}

}

func Test_ProducerPolicy_dup_apiSpec1(t *testing.T) {

	pa := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`
	pb := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 1)
	}

}

func Test_ProducerPolicy_dup_apiSpec2(t *testing.T) {

	pa := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`
	pb := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 2 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 2)
	}

}

func Test_ProducerPolicy_dup_apiSpec3(t *testing.T) {

	pa := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`
	pb := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 2 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 2)
	}

}

func Test_ProducerPolicy_dup_apiSpec4(t *testing.T) {

	pa := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`
	pb := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 2 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 2)
	}

}

func Test_ProducerPolicy_dup_apiSpec5(t *testing.T) {

	pa := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`
	pb := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 2 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 2)
	}

}

func Test_ProducerPolicy_dup_apiSpec6(t *testing.T) {

	pa := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`
	pb := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 2 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 2)
	}

}

func Test_ProducerPolicy_dup_apiSpec7(t *testing.T) {

	pa := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms3","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`
	pb := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms4","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 4 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 4)
	}

}

func Test_ProducerPolicy_dup_apiSpec8(t *testing.T) {

	pa := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms3","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`
	pb := `{"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"2.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms4","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}]}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 5 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 5)
	}

}

// Merge 2 producer policies and then create a TsAndCs policy
func Test_Merge_Producers_Create_TsAndCs1(t *testing.T) {

	pa := `{"header":{"name":"ms1 policy","version": "2.0"},` +
		`"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}],` +
		`"agreementProtocols":[{"name":"Basic","protocolVersion":1}],` +
		`"dataVerification":{"enabled":true,"URL":"","interval":0,"metering":{"tokens":2,"per_time_unit":"hour","notification_interval":3600}},` +
		`"ha_group":{"partners":["12345"]},` +
		`"maxAgreements":1}`
	pb := `{"header":{"name":"ms2 policy","version": "2.0"},` +
		`"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}],` +
		`"agreementProtocols":[{"name":"Basic","protocolVersion":1}],` +
		`"dataVerification":{"enabled":true,"URL":"","interval":0,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":120}},` +
		`"ha_group":{"partners":["12345"]},` +
		`"maxAgreements":1}`

	pc := `{"header":{"name":"split netspeed policy","version":"2.0"},` +
		`"agreementProtocols":[{"name":"Basic"}],` +
		`"workloads":[{"workloadUrl":"https://bluehorizon.network/workloads/netspeed","version":"2.3.0","arch":"amd64"}],` +
		`"dataVerification":{"enabled":true,"URL":"","interval":600,"metering":{"tokens":4,"per_time_unit":"min","notification_interval":30}},` +
		`"maxAgreements":2,"nodeHealth":{"missing_heartbeat_interval":600,"check_agreement_status":15}}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if p3 := create_Policy(pc, t); p3 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p3, pc)
	} else if pf_merged, err := Are_Compatible_Producers(p1, p2, 600); err != nil {
		t.Error(err)
	} else if len(pf_merged.APISpecs) != 2 {
		t.Errorf("Error: returned %v, should have returned %v\n", len(pf_merged.APISpecs), 2)
	} else if len(pf_merged.HAGroup.Partners) != 1 {
		t.Errorf("Error: HA group not merged, is %v\n", pf_merged.HAGroup.Partners)
	} else if pf_merged.DataVerify.Metering.Tokens != 60 {
		t.Errorf("Error: returned DataVerify.Tokens %v, should have returned %v\n", pf_merged.DataVerify.Metering.Tokens, 60)
	} else {
		// The runtime does this before creating terms and conditions
		p3.APISpecs = pf_merged.APISpecs

		if tcPolicy, err := Create_Terms_And_Conditions(pf_merged, p3, &p3.Workloads[0], "agreementId", "defaultPW", 600, 1); err != nil {
			t.Error(err)
		} else if tcPolicy.DataVerify.Metering.Tokens != 240 {
			t.Errorf("Error: returned DataVerify.Tokens %v, should have returned %v\n", tcPolicy.DataVerify.Metering.Tokens, 240)
		} else if len(tcPolicy.APISpecs) != 2 {
			t.Errorf("Error: returned %v APISpecs, should have returned %v\n", len(tcPolicy.APISpecs), 2)
		} else if tcPolicy.NodeH.MissingHBInterval != 600 {
			t.Errorf("Error: missing heartbeat interval, should be %v but is %v", 600, tcPolicy.NodeH.MissingHBInterval)
		} else {
			t.Logf("Merged Policy from 2 producer policies: %v", tcPolicy)
		}
	}
}

// Merge an empty producer policy with a pattern generated policy
func Test_Merge_EmptyProducer_and_Create_TsAndCs1(t *testing.T) {

	pa := `{"header":{"name":"producer","version": "2.0"}}`
	pb := `{"header":{"name":"pws_bluehorizon.network-workloads-weather_e2edev_amd64","version": "2.0"},` +
		`"paternId": "e2edev/pws",` +
		`"agreementProtocols":[{"name":"Basic","protocolVersion":1}],` +
		`"workloads":[{"torrent":{},"priority":{"priority_value":3,"retries":1,"retry_durations":3600,"verified_durations":52},` +
		`"workloadUrl":"https://bluehorizon.network/workloads/weather",` +
		`"organization":"e2edev","version":"1.5.0","arch":"amd64"}` +
		`],"valueExchange":{},"resourceLimits":{},` +
		`"dataVerification":{"enabled":true,"interval":240,"check_rate":15,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":30}},` +
		`"proposalRejection":{},"ha_group":{}}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if err := Are_Compatible(p1, p2); err != nil {
		t.Errorf(err.Error())
	} else if mergedPF, err := Create_Terms_And_Conditions(p1, p2, &p2.Workloads[0], "agreementId", "defaultPW", 300, 1); err != nil {
		t.Errorf(err.Error())
	} else {
		t.Log(mergedPF)
	}
}

// Add a workload to a policy object
func Test_Add_Workload1(t *testing.T) {

	pa := `{"header":{"name":"policy1","version": "2.0"},` +
		`"agreementProtocols":[{"name":"Basic","protocolVersion":1}],` +
		`"dataVerification":{"enabled":true,"URL":"","interval":0,"metering":{"tokens":2,"per_time_unit":"hour","notification_interval":3600}},` +
		`"maxAgreements":1}`

	wa := `{"workloadUrl":"url1","organization":"myorg","version":"1.0.0","arch":"amd64"}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if w1 := create_Workload(wa, t); w1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", w1, wa)
	} else if err := p1.Add_Workload(w1); err != nil {
		t.Errorf("Error: %v adding workload %v to %v\n", err, wa, p1)
	} else if len(p1.Workloads) != 1 {
		t.Errorf("Error: workload list length should be 1, but is %v\n", len(p1.Workloads))
	} else if err := p1.Add_Workload(w1); err == nil {
		t.Errorf("Duplicate add should have raised an error for %v and %v", wa, p1)
	}
}

// ================================================================================================================
// Helper functions
//
// Create an APISpecification array from a JSON serialization. The JSON serialization
// does not have to be a valid APISpecification serialization, just has to be a valid
// JSON serialization.
func create_Policy(jsonString string, t *testing.T) *Policy {
	p := new(Policy)

	if err := json.Unmarshal([]byte(jsonString), p); err != nil {
		t.Errorf("Error unmarshalling Policy json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return p
	}
}

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

// Create a Blockchain object from a JSON serialization. The JSON serialization
// does not have to be a valid Blockchain serialization, just has to be a valid
// JSON serialization.
func create_Blockchain(jsonString string, t *testing.T) *Blockchain {
	bl := new(Blockchain)

	if err := json.Unmarshal([]byte(jsonString), &bl); err != nil {
		t.Errorf("Error unmarshalling Blockchain json string: %v error:%v\n", jsonString, err)
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

// Create a workload array from a JSON serialization. The JSON serialization
// does not have to be a valid workload serialization, just has to be a valid
// JSON serialization.
func create_WorkloadList(jsonString string, t *testing.T) *WorkloadList {
	pl := new(WorkloadList)

	if err := json.Unmarshal([]byte(jsonString), &pl); err != nil {
		t.Errorf("Error unmarshalling WorkloadList json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return pl
	}
}
