// +build unit

package agreementbot

import (
	"errors"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/exchange"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_pattern_entry_success1(t *testing.T) {

	lab := "label"

	p := &exchange.Pattern{
		Label:              lab,
		Description:        "desc",
		Public:             true,
		Workloads:          []exchange.WorkloadReference{},
		AgreementProtocols: []exchange.AgreementProtocol{},
	}

	if np, err := NewPatternEntry(p); err != nil {
		t.Errorf("Error %v creating new pattern entry from %v", err, *p)
	} else if np.Pattern.Label != "label" {
		t.Errorf("Error: label should be %v but is %v", lab, np.Pattern.Label)
	} else if len(np.Hash) != 32 {
		t.Errorf("Error: hash should be length %v", 32)
	} else {
		t.Log(np)
	}

}

func Test_pattern_manager_success1(t *testing.T) {

	if np := NewPatternManager(); np == nil {
		t.Errorf("Error: pattern manager not created")
	} else {
		t.Log(np)
	}

}

// No existing served patterns, no new served patterns
func Test_pattern_manager_setpatterns0(t *testing.T) {

	policyPath := "/tmp/servedpatterntest/"
	servedPatterns := map[string]exchange.ServedPattern{}

	if np := NewPatternManager(); np == nil {
		t.Errorf("Error: pattern manager not created")
	} else if err := np.SetCurrentPatterns(servedPatterns, policyPath); err != nil {
		t.Errorf("Error %v consuming served patterns %v", err, servedPatterns)
	} else if len(np.OrgPatterns) != 0 {
		t.Errorf("Error: should have 0 org in the PatternManager, have %v", len(np.OrgPatterns))
	} else {
		t.Log(np)
	}

}

// Add a new served org and pattern
func Test_pattern_manager_setpatterns1(t *testing.T) {

	policyPath := "/tmp/servedpatterntest/"
	servedPatterns := map[string]exchange.ServedPattern{
		"myorg1_pattern1": {
			Org:     "myorg1",
			Pattern: "pattern1",
		},
	}

	if np := NewPatternManager(); np == nil {
		t.Errorf("Error: pattern manager not created")
	} else if err := np.SetCurrentPatterns(servedPatterns, policyPath); err != nil {
		t.Errorf("Error %v consuming served patterns %v", err, servedPatterns)
	} else if len(np.OrgPatterns) != 1 {
		t.Errorf("Error: should have 1 org in the PatternManager, have %v", len(np.OrgPatterns))
	} else {
		t.Log(np)
	}

}

// Remove an org and pattern, replace with a new org and pattern
func Test_pattern_manager_setpatterns2(t *testing.T) {

	policyPath := "/tmp/servedpatterntest/"
	myorg1 := "myorg1"
	myorg2 := "myorg2"
	pattern1 := "pattern1"
	pattern2 := "pattern2"

	servedPatterns1 := map[string]exchange.ServedPattern{
		"myorg1_pattern1": {
			Org:     myorg1,
			Pattern: pattern1,
		},
	}

	servedPatterns2 := map[string]exchange.ServedPattern{
		"myorg2_pattern2": {
			Org:     myorg2,
			Pattern: pattern2,
		},
	}

	definedPatterns1 := map[string]exchange.Pattern{
		"myorg1/pattern1": exchange.Pattern{
			Label:       "label",
			Description: "description",
			Public:      false,
			Workloads: []exchange.WorkloadReference{
				{
					WorkloadURL:  "http://mydomain.com/workload/test1",
					WorkloadOrg:  "testorg",
					WorkloadArch: "amd64",
					WorkloadVersions: []exchange.WorkloadChoice{
						{
							Version: "1.0.0",
						},
					},
				},
			},
			AgreementProtocols: []exchange.AgreementProtocol{
				{Name: "Basic"},
			},
		},
	}

	definedPatterns2 := map[string]exchange.Pattern{
		"myorg2/pattern2": exchange.Pattern{
			Label:       "label",
			Description: "description",
			Public:      false,
			Workloads: []exchange.WorkloadReference{
				{
					WorkloadURL:  "http://mydomain.com/workload/test2",
					WorkloadOrg:  "testorg",
					WorkloadArch: "amd64",
					WorkloadVersions: []exchange.WorkloadChoice{
						{
							Version: "1.5.0",
						},
					},
				},
			},
			AgreementProtocols: []exchange.AgreementProtocol{
				{Name: "Basic"},
			},
		},
	}

	// setup test
	if err := cleanTestDir(policyPath); err != nil {
		t.Errorf(err.Error())
	}

	// run test
	if np := NewPatternManager(); np == nil {
		t.Errorf("Error: pattern manager not created")
	} else if err := np.SetCurrentPatterns(servedPatterns1, policyPath); err != nil {
		t.Errorf("Error %v consuming served patterns %v", err, servedPatterns1)
	} else if err := np.UpdatePatternPolicies(myorg1, definedPatterns1, policyPath); err != nil {
		t.Errorf("Error: error updating pattern policies, %v", err)
	} else if len(np.OrgPatterns) != 1 {
		t.Errorf("Error: should have 1 org in the PatternManager, have %v", len(np.OrgPatterns))
	} else if !np.hasOrg(myorg1) {
		t.Errorf("Error: PM should have org %v but doesnt, has %v", myorg1, np)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg1][pattern1].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg1, pattern1, err)
	} else if err := np.SetCurrentPatterns(servedPatterns2, policyPath); err != nil {
		t.Errorf("Error %v consuming served patterns %v", err, servedPatterns2)
	} else if err := np.UpdatePatternPolicies(myorg2, definedPatterns2, policyPath); err != nil {
		t.Errorf("Error: error updating pattern policies, %v", err)
	} else if len(np.OrgPatterns) != 1 {
		t.Errorf("Error: should have 1 org in the PatternManager, have %v", len(np.OrgPatterns))
	} else if !np.hasOrg(myorg2) {
		t.Errorf("Error: PM should have org %v but doesnt, has %v", myorg2, np)
	} else if np.hasOrg(myorg1) {
		t.Errorf("Error: PM should NOT have org %v but does %v", myorg1, np)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg2][pattern2].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg2, pattern2, err)
	} else if files, err := getPolicyFiles(policyPath + myorg1); err != nil {
		t.Errorf(err.Error())
	} else if len(files) != 0 {
		t.Errorf("Error: found policy files for %v, %v", myorg1, files)
	} else {
		t.Log(np)
	}

}

// Remove an org with multiple patterns, add a pattern to existing org
func Test_pattern_manager_setpatterns3(t *testing.T) {

	policyPath := "/tmp/servedpatterntest/"
	myorg1 := "myorg1"
	myorg2 := "myorg2"
	pattern1 := "pattern1"
	pattern2 := "pattern2"

	servedPatterns1 := map[string]exchange.ServedPattern{
		"myorg1_pattern1": {
			Org:     myorg1,
			Pattern: pattern1,
		},
		"myorg1_pattern2": {
			Org:     myorg1,
			Pattern: pattern2,
		},
		"myorg2_pattern2": {
			Org:     myorg2,
			Pattern: pattern2,
		},
	}

	servedPatterns2 := map[string]exchange.ServedPattern{
		"myorg2_pattern1": {
			Org:     myorg2,
			Pattern: pattern1,
		},
		"myorg2_pattern2": {
			Org:     myorg2,
			Pattern: pattern2,
		},
	}

	definedPatterns1 := map[string]exchange.Pattern{
		"myorg1/pattern1": exchange.Pattern{
			Label:       "label",
			Description: "description",
			Public:      false,
			Workloads: []exchange.WorkloadReference{
				{
					WorkloadURL:  "http://mydomain.com/workload/test1",
					WorkloadOrg:  "testorg",
					WorkloadArch: "amd64",
					WorkloadVersions: []exchange.WorkloadChoice{
						{
							Version: "1.0.0",
						},
					},
				},
			},
			AgreementProtocols: []exchange.AgreementProtocol{
				{Name: "Basic"},
			},
		},
		"myorg1/pattern2": exchange.Pattern{
			Label:       "label",
			Description: "description",
			Public:      false,
			Workloads: []exchange.WorkloadReference{
				{
					WorkloadURL:  "http://mydomain.com/workload/test1",
					WorkloadOrg:  "testorg",
					WorkloadArch: "amd64",
					WorkloadVersions: []exchange.WorkloadChoice{
						{
							Version: "2.0.0",
						},
					},
				},
			},
			AgreementProtocols: []exchange.AgreementProtocol{
				{Name: "Basic"},
			},
		},
	}

	definedPatterns2 := map[string]exchange.Pattern{
		"myorg2/pattern1": exchange.Pattern{
			Label:       "label",
			Description: "description",
			Public:      false,
			Workloads: []exchange.WorkloadReference{
				{
					WorkloadURL:  "http://mydomain.com/workload/test2",
					WorkloadOrg:  "testorg",
					WorkloadArch: "amd64",
					WorkloadVersions: []exchange.WorkloadChoice{
						{
							Version: "1.4.0",
						},
					},
				},
			},
			AgreementProtocols: []exchange.AgreementProtocol{
				{Name: "Basic"},
			},
		},
		"myorg2/pattern2": exchange.Pattern{
			Label:       "label",
			Description: "description",
			Public:      false,
			Workloads: []exchange.WorkloadReference{
				{
					WorkloadURL:  "http://mydomain.com/workload/test2",
					WorkloadOrg:  "testorg",
					WorkloadArch: "amd64",
					WorkloadVersions: []exchange.WorkloadChoice{
						{
							Version: "1.5.0",
						},
					},
				},
			},
			AgreementProtocols: []exchange.AgreementProtocol{
				{Name: "Basic"},
			},
		},
	}

	// setup test
	if err := cleanTestDir(policyPath); err != nil {
		t.Errorf(err.Error())
	}

	// run test
	if np := NewPatternManager(); np == nil {
		t.Errorf("Error: pattern manager not created")
	} else if err := np.SetCurrentPatterns(servedPatterns1, policyPath); err != nil {
		t.Errorf("Error %v consuming served patterns %v", err, servedPatterns1)
	} else if err := np.UpdatePatternPolicies(myorg1, definedPatterns1, policyPath); err != nil {
		t.Errorf("Error: error updating pattern policies, %v", err)
	} else if err := np.UpdatePatternPolicies(myorg2, definedPatterns2, policyPath); err != nil {
		t.Errorf("Error: error updating pattern policies, %v", err)
	} else if len(np.OrgPatterns) != 2 {
		t.Errorf("Error: should have 2 orgs in the PatternManager, have %v", len(np.OrgPatterns))
	} else if !np.hasOrg(myorg1) {
		t.Errorf("Error: PM should have org %v but doesnt, has %v", myorg1, np)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg1][pattern1].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg1, pattern1, err)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg1][pattern2].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg1, pattern2, err)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg2][pattern2].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg2, pattern2, err)
	} else if err := np.SetCurrentPatterns(servedPatterns2, policyPath); err != nil {
		t.Errorf("Error %v consuming served patterns %v", err, servedPatterns2)
	} else if err := np.UpdatePatternPolicies(myorg2, definedPatterns2, policyPath); err != nil {
		t.Errorf("Error: error updating pattern policies, %v", err)
	} else if len(np.OrgPatterns) != 1 {
		t.Errorf("Error: should have 1 org in the PatternManager, have %v", len(np.OrgPatterns))
	} else if !np.hasOrg(myorg2) {
		t.Errorf("Error: PM should have org %v but doesnt, has %v", myorg2, np)
	} else if np.hasOrg(myorg1) {
		t.Errorf("Error: PM should NOT have org %v but does %v", myorg1, np)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg2][pattern1].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg2, pattern1, err)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg2][pattern2].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg2, pattern2, err)
	} else if files, err := getPolicyFiles(policyPath + myorg1); err != nil {
		t.Errorf(err.Error())
	} else if len(files) != 0 {
		t.Errorf("Error: found policy files for %v, %v", myorg1, files)
	} else {
		t.Log(np)
	}

}

// // Remove a pattern but org stays around, add a pattern to existing org
func Test_pattern_manager_setpatterns4(t *testing.T) {

	policyPath := "/tmp/servedpatterntest/"
	myorg1 := "myorg1"
	myorg2 := "myorg2"
	pattern1 := "pattern1"
	pattern2 := "pattern2"

	servedPatterns1 := map[string]exchange.ServedPattern{
		"myorg1_pattern1": {
			Org:     myorg1,
			Pattern: pattern1,
		},
		"myorg1_pattern2": {
			Org:     myorg1,
			Pattern: pattern2,
		},
		"myorg2_pattern2": {
			Org:     myorg2,
			Pattern: pattern2,
		},
	}

	servedPatterns2 := map[string]exchange.ServedPattern{
		"myorg1_pattern1": {
			Org:     myorg1,
			Pattern: pattern1,
		},
		"myorg2_pattern1": {
			Org:     myorg2,
			Pattern: pattern1,
		},
		"myorg2_pattern2": {
			Org:     myorg2,
			Pattern: pattern2,
		},
	}

	definedPatterns1 := map[string]exchange.Pattern{
		"myorg1/pattern1": exchange.Pattern{
			Label:       "label",
			Description: "description",
			Public:      false,
			Workloads: []exchange.WorkloadReference{
				{
					WorkloadURL:  "http://mydomain.com/workload/test1",
					WorkloadOrg:  "testorg",
					WorkloadArch: "amd64",
					WorkloadVersions: []exchange.WorkloadChoice{
						{
							Version: "1.0.0",
						},
					},
				},
			},
			AgreementProtocols: []exchange.AgreementProtocol{
				{Name: "Basic"},
			},
		},
		"myorg1/pattern2": exchange.Pattern{
			Label:       "label",
			Description: "description",
			Public:      false,
			Workloads: []exchange.WorkloadReference{
				{
					WorkloadURL:  "http://mydomain.com/workload/test1",
					WorkloadOrg:  "testorg",
					WorkloadArch: "amd64",
					WorkloadVersions: []exchange.WorkloadChoice{
						{
							Version: "2.0.0",
						},
					},
				},
			},
			AgreementProtocols: []exchange.AgreementProtocol{
				{Name: "Basic"},
			},
		},
	}

	definedPatterns2 := map[string]exchange.Pattern{
		"myorg2/pattern1": exchange.Pattern{
			Label:       "label",
			Description: "description",
			Public:      false,
			Workloads: []exchange.WorkloadReference{
				{
					WorkloadURL:  "http://mydomain.com/workload/test2",
					WorkloadOrg:  "testorg",
					WorkloadArch: "amd64",
					WorkloadVersions: []exchange.WorkloadChoice{
						{
							Version: "1.4.0",
						},
					},
				},
			},
			AgreementProtocols: []exchange.AgreementProtocol{
				{Name: "Basic"},
			},
		},
		"myorg2/pattern2": exchange.Pattern{
			Label:       "label",
			Description: "description",
			Public:      false,
			Workloads: []exchange.WorkloadReference{
				{
					WorkloadURL:  "http://mydomain.com/workload/test2",
					WorkloadOrg:  "testorg",
					WorkloadArch: "amd64",
					WorkloadVersions: []exchange.WorkloadChoice{
						{
							Version: "1.5.0",
						},
					},
				},
			},
			AgreementProtocols: []exchange.AgreementProtocol{
				{Name: "Basic"},
			},
		},
	}

	// setup the test
	if err := cleanTestDir(policyPath); err != nil {
		t.Errorf(err.Error())
	}

	// run the test
	if np := NewPatternManager(); np == nil {
		t.Errorf("Error: pattern manager not created")
	} else if err := np.SetCurrentPatterns(servedPatterns1, policyPath); err != nil {
		t.Errorf("Error %v consuming served patterns %v", err, servedPatterns1)
	} else if err := np.UpdatePatternPolicies(myorg1, definedPatterns1, policyPath); err != nil {
		t.Errorf("Error: error updating pattern policies, %v", err)
	} else if err := np.UpdatePatternPolicies(myorg2, definedPatterns2, policyPath); err != nil {
		t.Errorf("Error: error updating pattern policies, %v", err)
	} else if len(np.OrgPatterns) != 2 {
		t.Errorf("Error: should have 2 orgs in the PatternManager, have %v", len(np.OrgPatterns))
	} else if !np.hasOrg(myorg1) {
		t.Errorf("Error: PM should have org %v but doesnt, has %v", myorg1, np)
	} else if !np.hasOrg(myorg2) {
		t.Errorf("Error: PM should have org %v but doesnt, has %v", myorg2, np)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg1][pattern1].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg1, pattern1, err)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg1][pattern2].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg1, pattern2, err)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg2][pattern2].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg2, pattern2, err)
	} else if err := np.SetCurrentPatterns(servedPatterns2, policyPath); err != nil {
		t.Errorf("Error %v consuming served patterns %v", err, servedPatterns2)
	} else if err := np.UpdatePatternPolicies(myorg1, definedPatterns1, policyPath); err != nil {
		t.Errorf("Error: error updating pattern policies, %v", err)
	} else if err := np.UpdatePatternPolicies(myorg2, definedPatterns2, policyPath); err != nil {
		t.Errorf("Error: error updating pattern policies, %v", err)
	} else if len(np.OrgPatterns) != 2 {
		t.Errorf("Error: should have 2 org in the PatternManager, have %v", len(np.OrgPatterns))
	} else if !np.hasOrg(myorg2) {
		t.Errorf("Error: PM should have org %v but doesnt, has %v", myorg2, np)
	} else if !np.hasOrg(myorg1) {
		t.Errorf("Error: PM should have org %v but doesnt, has %v", myorg1, np)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg1][pattern1].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg1, pattern1, err)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg2][pattern1].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg2, pattern1, err)
	} else if err := getPatternEntryFiles(np.OrgPatterns[myorg2][pattern2].PolicyFileNames); err != nil {
		t.Errorf("Error getting pattern entry files for %v %v, %v", myorg2, pattern2, err)
	} else {
		t.Log(np)
	}

}

// Utility functions
// Clean up the test directory
func cleanTestDir(policyPath string) error {
	if _, err := os.Stat(policyPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(policyPath); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(policyPath, 0764); err != nil {
		return err
	}
	return nil
}

// Check for policy files referenced by the pattern manager entries
func getPatternEntryFiles(files []string) error {
	for _, filename := range files {
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return errors.New(fmt.Sprintf("File %v does not exist", filename))
		}
	}
	return nil
}

// Check for policy files that shouldnt have been left behind
func getPolicyFiles(homePath string) ([]os.FileInfo, error) {
	res := make([]os.FileInfo, 0, 10)

	if files, err := ioutil.ReadDir(homePath); err != nil {
		return nil, err
	} else {
		for _, fileInfo := range files {
			if strings.HasSuffix(fileInfo.Name(), ".policy") && !fileInfo.IsDir() {
				res = append(res, fileInfo)
			}
		}
		return res, nil
	}
}
