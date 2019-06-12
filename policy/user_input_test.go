// +build unit

package policy

import (
	"testing"
)

func Test_MergeUserInput(t *testing.T) {
	svcUserInput1 := UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val1"}, Input{Name: "var2", Value: "val2"}},
	}

	svcUserInput2 := UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val11"}, Input{Name: "var2", Value: "val21"}, Input{Name: "var3", Value: "val3"}},
	}

	ui, _ := MergeUserInput(svcUserInput1, svcUserInput2, false)
	if ui.ServiceOrgid != "mycomp" {
		t.Errorf("The service org should be mycomp but got %v.", ui.ServiceOrgid)
	} else if ui.ServiceUrl != "cpu" {
		t.Errorf("The service ServiceUrl should be cpu but got %v.", ui.ServiceUrl)
	} else if len(ui.Inputs) != 3 {
		t.Errorf("The lengh of ui should be 3 but got %v.", len(ui.Inputs))
	} else if ui.Inputs[0].Name != "var1" || ui.Inputs[0].Value != "val11" {
		t.Errorf("Wrong first input value %v.", ui.Inputs[0])
	} else if ui.Inputs[1].Name != "var2" || ui.Inputs[1].Value != "val21" {
		t.Errorf("Wrong first input value %v.", ui.Inputs[0])
	} else if ui.Inputs[2].Name != "var3" || ui.Inputs[2].Value != "val3" {
		t.Errorf("Wrong first input value %v.", ui.Inputs[3])
	}

	svcUserInput1 = UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
	}

	svcUserInput2 = UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val11"}, Input{Name: "var2", Value: "val21"}, Input{Name: "var3", Value: "val3"}},
	}
	ui, _ = MergeUserInput(svcUserInput1, svcUserInput2, false)
	if ui.ServiceOrgid != "mycomp" {
		t.Errorf("The service org should be mycomp but got %v.", ui.ServiceOrgid)
	} else if ui.ServiceUrl != "cpu" {
		t.Errorf("The service ServiceUrl should be cpu but got %v.", ui.ServiceUrl)
	} else if len(ui.Inputs) != 3 {
		t.Errorf("The lengh of ui should be 3 but got %v.", len(ui.Inputs))
	} else if ui.Inputs[0].Name != "var1" || ui.Inputs[0].Value != "val11" {
		t.Errorf("Wrong first input value %v.", ui.Inputs[0])
	} else if ui.Inputs[1].Name != "var2" || ui.Inputs[1].Value != "val21" {
		t.Errorf("Wrong first input value %v.", ui.Inputs[0])
	} else if ui.Inputs[2].Name != "var3" || ui.Inputs[2].Value != "val3" {
		t.Errorf("Wrong first input value %v.", ui.Inputs[3])
	}

	svcUserInput1 = UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val1"}, Input{Name: "var2", Value: "val2"}},
	}

	svcUserInput2 = UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
	}

	ui, _ = MergeUserInput(svcUserInput1, svcUserInput2, false)
	if ui.ServiceOrgid != "mycomp" {
		t.Errorf("The service org should be mycomp but got %v.", ui.ServiceOrgid)
	} else if ui.ServiceUrl != "cpu" {
		t.Errorf("The service ServiceUrl should be cpu but got %v.", ui.ServiceUrl)
	} else if len(ui.Inputs) != 2 {
		t.Errorf("The lengh of ui should be 3 but got %v.", len(ui.Inputs))
	} else if ui.Inputs[0].Name != "var1" || ui.Inputs[0].Value != "val1" {
		t.Errorf("Wrong first input value %v.", ui.Inputs[0])
	} else if ui.Inputs[1].Name != "var2" || ui.Inputs[1].Value != "val2" {
		t.Errorf("Wrong first input value %v.", ui.Inputs[0])
	}

	svcUserInput1 = UserInput{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val1"}, Input{Name: "var2", Value: "val2"}},
	}

	svcUserInput2 = UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val11"}, Input{Name: "var2", Value: "val21"}, Input{Name: "var3", Value: "val3"}},
	}

	ui, err := MergeUserInput(svcUserInput1, svcUserInput2, true)
	if err == nil {
		t.Errorf("MergeUserInputs should return error but did not.")
	}
}

func Test_FindUserInput(t *testing.T) {
	svcUserInput1 := UserInput{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "[2.0.1,INFINITY)",
		Inputs:              []Input{Input{Name: "var1", Value: "val1"}, Input{Name: "var2", Value: "val2"}},
	}

	svcUserInput2 := UserInput{
		ServiceOrgid:        "mycomp2",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val11"}, Input{Name: "var2", Value: "val21"}, Input{Name: "var3", Value: "val3"}},
	}

	userInput := []UserInput{svcUserInput1, svcUserInput2} 

	u1, err := FindUserInput("cpu", "mycomp2", "1.0.0", "amd64", userInput)
	if err != nil {
		t.Errorf("FindUserInput should not return errror but got %v", err)
	} else if u1 == nil {
		t.Errorf("FindUserInput should not return nil but it did.")
	} else if len(u1.Inputs) != 3 {
		t.Errorf("FindUserInput should return the user input with 3 variables but got %v", len(u1.Inputs))
	} else if u1.ServiceOrgid != "mycomp2" {
		t.Errorf("FindUserInput should return the user input with org id mycomp2 but got %v", u1.ServiceOrgid)	
	}

	u2, err := FindUserInput("cpu", "mycomp1", "1.0.0", "amd64", userInput)
	if err != nil {
		t.Errorf("FindUserInput should not return errror but got %v", err)
	} else if u2 != nil {
		t.Errorf("FindUserInput should return nil but it did not. %v", u2)
	}

	u3, err := FindUserInput("cpu", "mycomp1", "3.0.0", "amd64", userInput)
	if err != nil {
		t.Errorf("FindUserInput should not return errror but got %v", err)
	} else if u3 == nil {
		t.Errorf("FindUserInput should not return nil but it did.")
	} else if len(u3.Inputs) != 2 {
		t.Errorf("FindUserInput should return the user input with 3 variables but got %v", len(u3.Inputs))
	} else if u3.ServiceOrgid != "mycomp1" {
		t.Errorf("FindUserInput should return the user input with org id mycomp2 but got %v", u3.ServiceOrgid)	
	}

	_, err = FindUserInput("cpu", "mycomp1", "x.0.0", "amd64", userInput)
	if err == nil {
		t.Errorf("FindUserInput should return errror but did not.")
	} 

	u5, err := FindUserInput("cpu2", "mycomp1", "1.0.0", "amd64", userInput)
	if err != nil {
		t.Errorf("FindUserInput should not return errror but got %v", err)
	} else if u5 != nil {
		t.Errorf("FindUserInput should return nil but it did not. %v", u5)
	}

	u6, err := FindUserInput("cpu", "mycomp2", "", "", userInput)
	if err != nil {
		t.Errorf("FindUserInput should not return errror but got %v", err)
	} else if u6 == nil {
		t.Errorf("FindUserInput should not return nil but it did.")
	} else if len(u6.Inputs) != 3 {
		t.Errorf("FindUserInput should return the user input with 3 variables but got %v", len(u6.Inputs))
	} else if u6.ServiceOrgid != "mycomp2" {
		t.Errorf("FindUserInput should return the user input with org id mycomp2 but got %v", u6.ServiceOrgid)	
	}
}

func Test_UpdateSettingsWithPolicyUserInput(t *testing.T) {
	svcUserInput1 := UserInput{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "[2.0.1,INFINITY)",
		Inputs:              []Input{Input{Name: "var1", Value: "val1"}, Input{Name: "var2", Value: "val2"}},
	}

	svcUserInput2 := UserInput{
		ServiceOrgid:        "mycomp2",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val11"}, Input{Name: "var2", Value: "val21"}, Input{Name: "var3", Value: "val3"}},
	}

	userInput := []UserInput{svcUserInput1, svcUserInput2} 

	policy := new(Policy)
	policy.UserInput = userInput

	existingUserSettings := map[string]string{"var1": "default value1", "var3": "default value3", "var4": "default value4", "var5": "15"}

	newUI1, err := UpdateSettingsWithPolicyUserInput(policy, existingUserSettings, "cpu", "mycomp2")
	if err != nil {
		t.Errorf("UpdateSettingsWithPolicyUserInput should not return errror but got %v", err)
	} else if newUI1 == nil || len(newUI1) == 0 {
		t.Errorf("UpdateSettingsWithPolicyUserInput should not return nil but it did.")
	} else if len(newUI1) != 5 {
		t.Errorf("UpdateSettingsWithPolicyUserInput should return the user input with 5 variables but got %v", len(newUI1))
	} else if newUI1["var3"] != "default value3" {
		t.Errorf("UpdateSettingsWithPolicyUserInput should return var3=default value3 but got val3=%v", newUI1["var3"])	
	} else if newUI1["var4"] != "default value4" {
		t.Errorf("UpdateSettingsWithPolicyUserInput should return var4=default value4 %v", newUI1["var4"])	
	}

	newUI2, err := UpdateSettingsWithPolicyUserInput(policy, map[string]string{}, "cpu", "mycomp2")
	if err != nil {
		t.Errorf("UpdateSettingsWithPolicyUserInput should not return errror but got %v", err)
	} else if newUI2 == nil || len(newUI2) == 0 {
		t.Errorf("UpdateSettingsWithPolicyUserInput should not return nil but it did.")
	} else if len(newUI2) != 3 {
		t.Errorf("UpdateSettingsWithPolicyUserInput should return the user input with 3 variables but got %v", len(newUI2))
	} else if newUI2["var3"] != "val3" {
		t.Errorf("UpdateSettingsWithPolicyUserInput should return var3=val3 %v", newUI1["var3"])	
	}

}

