// +build unit

package governance

import (
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

func Test_ConvertAttributeToUserInput(t *testing.T) {

	hostonly := false
	publishable := true
	meta := persistence.AttributeMeta{
		Type:        "UserInputAttributes",
		Label:       "UserInputAttributes",
		HostOnly:    &hostonly,
		Publishable: &publishable,
	}

	sp := persistence.ServiceSpec{Url: "service1", Org: "myorg1"}

	var svcSpecs persistence.ServiceSpecs
	svcSpecs = []persistence.ServiceSpec{sp}

	mappings := make(map[string]interface{}, 5)
	mappings["var1"] = "a string"
	mappings["var2"] = 22
	mappings["var3"] = 33.3

	uiAttrib := persistence.UserInputAttributes{
		Meta:         &meta,
		ServiceSpecs: &svcSpecs,
		Mappings:     mappings,
	}

	// nil input returns nil
	ui := ConvertAttributeToUserInput("", "", "amd64", nil)
	if ui != nil {
		t.Errorf("ConvertAttributeToUserInput should have returned nil but not")
	}

	// regular input
	ui = ConvertAttributeToUserInput("service2", "myorg2", "amd64", &uiAttrib)
	if ui == nil {
		t.Errorf("ConvertAttributeToUserInput should not have returned nil")
	} else if ui.ServiceOrgid != "myorg1" || ui.ServiceUrl != "service1" || ui.ServiceArch != "amd64" || ui.ServiceVersionRange != "[0.0.0,INFINITY)" {
		t.Errorf("Wrong service spec got converted: %v", ui)
	} else if len(ui.Inputs) != 3 {
		t.Errorf("There should be 3 elements in the Inputs but got %v", len(ui.Inputs))
	} else {
		var1Found := false
		var2Found := false
		var3Found := false
		for _, input := range ui.Inputs {
			if input.Name == "var1" && input.Value == "a string" {
				var1Found = true
			} else if input.Name == "var2" && input.Value == 22 {
				var2Found = true
			} else if input.Name == "var3" && input.Value == 33.3 {
				var3Found = true
			}
		}

		if !var1Found {
			t.Errorf("Failed to convert var1. %v", ui.Inputs)
		}
		if !var2Found {
			t.Errorf("Failed to convert var2. %v", ui.Inputs)
		}
		if !var3Found {
			t.Errorf("Failed to convert var3. %v", ui.Inputs)
		}
	}

	// this is case where the input is for all the services
	svcSpecs2 := persistence.ServiceSpecs{}
	uiAttrib = persistence.UserInputAttributes{
		Meta:         &meta,
		ServiceSpecs: &svcSpecs2,
		Mappings:     mappings,
	}
	ui = ConvertAttributeToUserInput("service2", "myorg2", "amd64", &uiAttrib)
	if ui == nil {
		t.Errorf("ConvertAttributeToUserInput should not have returned nil")
	} else if ui.ServiceOrgid != "myorg2" || ui.ServiceUrl != "service2" || ui.ServiceArch != "amd64" || ui.ServiceVersionRange != "[0.0.0,INFINITY)" {
		t.Errorf("Wrong service spec got converted: %v", ui)
	} else if len(ui.Inputs) != 3 {
		t.Errorf("There should be 3 elements in the Inputs but got %v", len(ui.Inputs))
	} else {
		var1Found := false
		var2Found := false
		var3Found := false
		for _, input := range ui.Inputs {
			if input.Name == "var1" && input.Value == "a string" {
				var1Found = true
			} else if input.Name == "var2" && input.Value == 22 {
				var2Found = true
			} else if input.Name == "var3" && input.Value == 33.3 {
				var3Found = true
			}
		}

		if !var1Found {
			t.Errorf("Failed to convert var1. %v", ui.Inputs)
		}
		if !var2Found {
			t.Errorf("Failed to convert var2. %v", ui.Inputs)
		}
		if !var3Found {
			t.Errorf("Failed to convert var3. %v", ui.Inputs)
		}
	}

	// input has zero length mappings, it should return nil
	uiAttrib = persistence.UserInputAttributes{
		Meta:         &meta,
		ServiceSpecs: &svcSpecs,
		Mappings:     map[string]interface{}{},
	}

	ui = ConvertAttributeToUserInput("service2", "myorg2", "amd64", &uiAttrib)
	if ui != nil {
		t.Errorf("ConvertAttributeToUserInput should have returned nil but not")
	}
}

func Test_ValidateUserInput(t *testing.T) {
	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	ui1 := policy.UserInput{
		ServiceOrgid:        "myorg1",
		ServiceUrl:          "service1",
		ServiceArch:         "amd64",
		ServiceVersionRange: "[0.0.0,INFINITY)",
		Inputs: []policy.Input{policy.Input{Name: "var1", Value: "a string"},
			policy.Input{Name: "var2", Value: 22},
			policy.Input{Name: "var3", Value: 33.3}},
	}

	ui2 := policy.UserInput{
		ServiceOrgid:        "myorg2",
		ServiceUrl:          "service2",
		ServiceArch:         "amd64",
		ServiceVersionRange: "[0.0.0,INFINITY)",
		Inputs: []policy.Input{policy.Input{Name: "var1", Value: "b string"},
			policy.Input{Name: "var2", Value: 222},
			policy.Input{Name: "var3", Value: 333.333},
			policy.Input{Name: "var4", Value: false}},
	}

	mergedUserInput := []policy.UserInput{ui1, ui2}

	requi1 := exchangecommon.UserInput{Name: "var1", Type: "string", DefaultValue: "This is a string"}
	requi2 := exchangecommon.UserInput{Name: "var2", Type: "int", DefaultValue: ""}
	requi3 := exchangecommon.UserInput{Name: "var3", Type: "float", DefaultValue: ""}
	requi4 := exchangecommon.UserInput{Name: "var4", Type: "boolean", DefaultValue: ""}
	requi5 := exchangecommon.UserInput{Name: "var5", Type: "list of strings", DefaultValue: "a, b, c"}

	sdef1 := exchange.ServiceDefinition{
		URL:        "service1",
		Version:    "1.2.3",
		Arch:       "amd64",
		UserInputs: []exchangecommon.UserInput{requi1, requi2, requi3, requi4, requi5},
	}

	sdef2 := exchange.ServiceDefinition{
		URL:        "service2",
		Version:    "1.2.3",
		Arch:       "amd64",
		UserInputs: []exchangecommon.UserInput{requi1, requi2, requi3, requi4, requi5},
	}

	// userinput has part of the required inputs
	if err := ValidateUserInput(&sdef1, "myorg1", mergedUserInput, db); err == nil {
		t.Errorf("ValidateUserInput should have returned error but got nil")
	} else if !strings.Contains(err.Error(), "var4 is required for service myorg1/service1") {
		t.Errorf("Error should have contained 'var4 is required for service myorg1/service1' but not. mergedUserInput= %v, Error: %v", mergedUserInput, err)
	}

	// userinput has all of the required inputs
	if err := ValidateUserInput(&sdef2, "myorg2", mergedUserInput, db); err != nil {
		t.Errorf("ValidateUserInput should have returned nil but got %v", err)
	}

	// userinput has zero required inputs
	if err := ValidateUserInput(&sdef1, "myorg1", nil, db); err == nil {
		t.Errorf("ValidateUserInput should have returned error but got nil")
	} else if !strings.Contains(err.Error(), "var2 is required for service myorg1/service1") {
		t.Errorf("Error should have contained 'var2 is required for service myorg1/service1' but not. mergedUserInput= %v, Error: %v", mergedUserInput, err)
	}
	if err := ValidateUserInput(&sdef1, "myorg1", []policy.UserInput{}, db); err == nil {
		t.Errorf("ValidateUserInput should have returned error but got nil")
	} else if !strings.Contains(err.Error(), "var2 is required for service myorg1/service1") {
		t.Errorf("Error should have contained 'var2 is required for service myorg1/service1' but not. mergedUserInput= %v, Error: %v", mergedUserInput, err)
	}
	ui3 := policy.UserInput{
		ServiceOrgid:        "myorg1",
		ServiceUrl:          "service1",
		ServiceArch:         "amd64",
		ServiceVersionRange: "[0.0.0,INFINITY)",
		Inputs:              []policy.Input{},
	}
	mergedUserInput3 := []policy.UserInput{ui3, ui2}
	if err := ValidateUserInput(&sdef1, "myorg1", mergedUserInput3, db); err == nil {
		t.Errorf("ValidateUserInput should have returned error but got nil")
	} else if !strings.Contains(err.Error(), "var2 is required for service myorg1/service1") {
		t.Errorf("Error should have contained 'var2 is required for service myorg1/service1' but not. mergedUserInput= %v, Error: %v", mergedUserInput, err)
	}

	// wrong org name
	if err := ValidateUserInput(&sdef1, "myorg3", mergedUserInput3, db); err == nil {
		t.Errorf("ValidateUserInput should have returned error but got nil")
	} else if !strings.Contains(err.Error(), "var2 is required for service myorg3/service1") {
		t.Errorf("Error should have contained 'var2 is required for service myorg3/service1' but not. mergedUserInput= %v, Error: %v", mergedUserInput, err)
	}
}

func Test_ValidateUserInput_with_Attributes(t *testing.T) {
	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	ui1 := policy.UserInput{
		ServiceOrgid:        "myorg1",
		ServiceUrl:          "service1",
		ServiceArch:         "amd64",
		ServiceVersionRange: "[0.0.0,INFINITY)",
		Inputs: []policy.Input{policy.Input{Name: "var1", Value: "a string"},
			policy.Input{Name: "var2", Value: 22},
			policy.Input{Name: "var3", Value: 33.3}},
	}

	ui2 := policy.UserInput{
		ServiceOrgid:        "myorg2",
		ServiceUrl:          "service2",
		ServiceArch:         "amd64",
		ServiceVersionRange: "[0.0.0,INFINITY)",
		Inputs: []policy.Input{policy.Input{Name: "var1", Value: "b string"},
			policy.Input{Name: "var2", Value: 222},
			policy.Input{Name: "var3", Value: 333.333},
			policy.Input{Name: "var4", Value: false}},
	}

	mergedUserInput := []policy.UserInput{ui1, ui2}

	requi1 := exchangecommon.UserInput{Name: "var1", Type: "string", DefaultValue: "This is a string"}
	requi2 := exchangecommon.UserInput{Name: "var2", Type: "int", DefaultValue: ""}
	requi3 := exchangecommon.UserInput{Name: "var3", Type: "float", DefaultValue: ""}
	requi4 := exchangecommon.UserInput{Name: "var4", Type: "boolean", DefaultValue: ""}
	requi5 := exchangecommon.UserInput{Name: "var5", Type: "list of strings", DefaultValue: "a, b, c"}

	sdef1 := exchange.ServiceDefinition{
		URL:        "service1",
		Version:    "1.2.3",
		Arch:       "amd64",
		UserInputs: []exchangecommon.UserInput{requi1, requi2, requi3, requi4, requi5},
	}

	hostonly := false
	publishable := true
	meta := persistence.AttributeMeta{
		Type:        "UserInputAttributes",
		Label:       "UserInputAttributes",
		HostOnly:    &hostonly,
		Publishable: &publishable,
	}

	sp := persistence.ServiceSpec{Url: "service1", Org: "myorg1"}

	var svcSpecs persistence.ServiceSpecs
	svcSpecs = []persistence.ServiceSpec{sp}

	mappings := make(map[string]interface{}, 5)
	mappings["var4"] = false
	mappings["var2"] = 222

	uiAttrib := persistence.UserInputAttributes{
		Meta:         &meta,
		ServiceSpecs: &svcSpecs,
		Mappings:     mappings,
	}

	persistence.SaveOrUpdateAttribute(db, uiAttrib, "", false)

	// attribute has required inputs
	if err := ValidateUserInput(&sdef1, "myorg1", mergedUserInput, db); err != nil {
		t.Errorf("ValidateUserInput should have returned nil but got: %v", err)
	}

	// attribute has partial required inputs
	if err := ValidateUserInput(&sdef1, "myorg1", []policy.UserInput{}, db); err == nil {
		t.Errorf("ValidateUserInput should have returned error but got nil")
	} else if !strings.Contains(err.Error(), "var3 is required for service myorg1/service1") {
		t.Errorf("Error should have contained 'var3 is required for service myorg1/service1' but not. mergedUserInput= %v, Error: %v", mergedUserInput, err)
	}
	if err := ValidateUserInput(&sdef1, "myorg1", nil, db); err == nil {
		t.Errorf("ValidateUserInput should have returned error but got nil")
	} else if !strings.Contains(err.Error(), "var3 is required for service myorg1/service1") {
		t.Errorf("Error should have contained 'var3 is required for service myorg1/service1' but not. mergedUserInput= %v, Error: %v", mergedUserInput, err)
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

func cleanTestDir(dirPath string) error {
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(dirPath); err != nil {
			return err
		}
	}
	return nil
}
