// +build unit

package policy

import (
	"reflect"
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

func Test_MergeUserInputArrays(t *testing.T) {
	svcUserInput1 := UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu1",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val1"}, Input{Name: "var2", Value: "val2"}},
	}

	svcUserInput2 := UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu2",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val11"}, Input{Name: "var2", Value: "val21"}, Input{Name: "var3", Value: "val3"}},
	}

	svcUserInput3 := UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu3",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var31", Value: "val31"}, Input{Name: "var32", Value: "val31"}, Input{Name: "var33", Value: "val33"}},
	}

	ui1 := []UserInput{svcUserInput1, svcUserInput2}
	ui2 := []UserInput{svcUserInput2, svcUserInput3}

	ui_new := MergeUserInputArrays(ui1, ui2, false)

	if len(ui_new) != 3 {
		t.Errorf("The lengh of ui should be 3 but got %v.", len(ui_new))
	} else if !reflect.DeepEqual(ui_new[0], svcUserInput1) {
		t.Errorf("The first element should be %v, but got %v.", svcUserInput1, ui_new[0])
	} else if !reflect.DeepEqual(ui_new[1], svcUserInput2) {
		t.Errorf("The first element should be %v, but got %v.", svcUserInput2, ui_new[1])
	} else if !reflect.DeepEqual(ui_new[2], svcUserInput3) {
		t.Errorf("The first element should be %v, but got %v.", svcUserInput3, ui_new[2])
	}

	svcUserInput4 := UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu2",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val41"}, Input{Name: "var2", Value: "val42"}, Input{Name: "var5", Value: "val45"}},
	}

	svcUserInput5 := UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu2",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val41"}, Input{Name: "var2", Value: "val42"}, Input{Name: "var3", Value: "val3"}, Input{Name: "var5", Value: "val45"}},
	}

	ui2 = []UserInput{svcUserInput1, svcUserInput4, svcUserInput3}
	ui_new = MergeUserInputArrays(ui1, ui2, true)

	if len(ui_new) != 3 {
		t.Errorf("The lengh of ui should be 3 but got %v.", len(ui_new))
	} else if !reflect.DeepEqual(ui_new[0], svcUserInput1) {
		t.Errorf("The first element should be %v, but got %v.", svcUserInput1, ui_new[0])
	} else if !reflect.DeepEqual(ui_new[1], svcUserInput5) {
		t.Errorf("The second element should be %v, but got %v.", svcUserInput5, ui_new[1])
	} else if !reflect.DeepEqual(ui_new[2], svcUserInput3) {
		t.Errorf("The third element should be %v, but got %v.", svcUserInput3, ui_new[2])
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

	u1, _, err := FindUserInput("cpu", "mycomp2", "1.0.0", "amd64", userInput)
	if err != nil {
		t.Errorf("FindUserInput should not return errror but got %v", err)
	} else if u1 == nil {
		t.Errorf("FindUserInput should not return nil but it did.")
	} else if len(u1.Inputs) != 3 {
		t.Errorf("FindUserInput should return the user input with 3 variables but got %v", len(u1.Inputs))
	} else if u1.ServiceOrgid != "mycomp2" {
		t.Errorf("FindUserInput should return the user input with org id mycomp2 but got %v", u1.ServiceOrgid)
	}

	u2, _, err := FindUserInput("cpu", "mycomp1", "1.0.0", "amd64", userInput)
	if err != nil {
		t.Errorf("FindUserInput should not return errror but got %v", err)
	} else if u2 != nil {
		t.Errorf("FindUserInput should return nil but it did not. %v", u2)
	}

	u3, _, err := FindUserInput("cpu", "mycomp1", "3.0.0", "amd64", userInput)
	if err != nil {
		t.Errorf("FindUserInput should not return errror but got %v", err)
	} else if u3 == nil {
		t.Errorf("FindUserInput should not return nil but it did.")
	} else if len(u3.Inputs) != 2 {
		t.Errorf("FindUserInput should return the user input with 3 variables but got %v", len(u3.Inputs))
	} else if u3.ServiceOrgid != "mycomp1" {
		t.Errorf("FindUserInput should return the user input with org id mycomp2 but got %v", u3.ServiceOrgid)
	}

	_, index, err := FindUserInput("cpu", "mycomp1", "x.0.0", "amd64", userInput)
	if err == nil {
		t.Errorf("FindUserInput should return errror but did not.")
	} else if index != -1 {
		t.Errorf("FindUserInput should return index as -1 but did not.")
	}

	u5, _, err := FindUserInput("cpu2", "mycomp1", "1.0.0", "amd64", userInput)
	if err != nil {
		t.Errorf("FindUserInput should not return errror but got %v", err)
	} else if u5 != nil {
		t.Errorf("FindUserInput should return nil but it did not. %v", u5)
	}

	u6, index, err := FindUserInput("cpu", "mycomp2", "", "", userInput)
	if err != nil {
		t.Errorf("FindUserInput should not return errror but got %v", err)
	} else if u6 == nil {
		t.Errorf("FindUserInput should not return nil but it did.")
	} else if len(u6.Inputs) != 3 {
		t.Errorf("FindUserInput should return the user input with 3 variables but got %v", len(u6.Inputs))
	} else if u6.ServiceOrgid != "mycomp2" {
		t.Errorf("FindUserInput should return the user input with org id mycomp2 but got %v", u6.ServiceOrgid)
	} else if index != 1 {
		t.Errorf("FindUserInput should return index as 1 but did not.")
	}
}

func Test_UpdateSettingsWithUserInputs(t *testing.T) {
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

	policy1 := new(Policy)
	policy1.UserInput = userInput

	existingUserSettings := map[string]string{"var1": "default value1", "var3": "default value3", "var4": "default value4", "var5": "15"}

	newUI1, err := UpdateSettingsWithUserInputs(policy1.UserInput, existingUserSettings, "cpu", "mycomp2")
	if err != nil {
		t.Errorf("UpdateSettingsWithUserInputs should not return errror but got %v", err)
	} else if newUI1 == nil || len(newUI1) == 0 {
		t.Errorf("UpdateSettingsWithUserInputs should not return nil but it did.")
	} else if len(newUI1) != 5 {
		t.Errorf("UpdateSettingsWithUserInputs should return the user input with 5 variables but got %v", len(newUI1))
	} else if newUI1["var3"] != "default value3" {
		t.Errorf("UpdateSettingsWithUserInputs should return var3=default value3 but got val3=%v", newUI1["var3"])
	} else if newUI1["var4"] != "default value4" {
		t.Errorf("UpdateSettingsWithUserInputs should return var4=default value4 %v", newUI1["var4"])
	}

	newUI2, err := UpdateSettingsWithUserInputs(policy1.UserInput, map[string]string{}, "cpu", "mycomp2")
	if err != nil {
		t.Errorf("UpdateSettingsWithUserInputs should not return errror but got %v", err)
	} else if newUI2 == nil || len(newUI2) == 0 {
		t.Errorf("UpdateSettingsWithUserInputs should not return nil but it did.")
	} else if len(newUI2) != 3 {
		t.Errorf("UpdateSettingsWithUserInputs should return the user input with 3 variables but got %v", len(newUI2))
	} else if newUI2["var3"] != "val3" {
		t.Errorf("UpdateSettingsWithUserInputs should return var3=val3 %v", newUI1["var3"])
	}
}

func Test_UserInputArrayIsSame(t *testing.T) {
	svcUserInput1 := UserInput{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "[2.0.1,INFINITY)",
		Inputs:              []Input{Input{Name: "var1", Value: "val1"}, Input{Name: "var2", Value: 10}, Input{Name: "var2", Value: 10.22}},
	}

	svcUserInput2 := UserInput{
		ServiceOrgid:        "mycomp2",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val11"}, Input{Name: "var2", Value: []string{"a", "b", "c"}}, Input{Name: "var3", Value: false}},
	}

	userInput1 := []UserInput{svcUserInput1, svcUserInput2}
	userInput2 := []UserInput{svcUserInput2, svcUserInput1}
	same := UserInputArrayIsSame(userInput1, userInput2)
	if !same {
		t.Errorf("UserInputArrayIsSame should have returned true but got false.")
	}

	userInput1 = []UserInput{svcUserInput1}
	userInput2 = []UserInput{svcUserInput2}
	same = UserInputArrayIsSame(userInput1, userInput2)
	if same {
		t.Errorf("UserInputArrayIsSame should have returned false but got true.")
	}

	userInput1 = []UserInput{}
	userInput2 = []UserInput{svcUserInput2}
	same = UserInputArrayIsSame(userInput1, userInput2)
	if same {
		t.Errorf("UserInputArrayIsSame should have returned false but got true.")
	}

	userInput1 = []UserInput{}
	userInput2 = []UserInput{}
	same = UserInputArrayIsSame(userInput1, userInput2)
	if !same {
		t.Errorf("UserInputArrayIsSame should have returned true but got false.")
	}

	userInput1 = nil
	userInput2 = []UserInput{}
	same = UserInputArrayIsSame(userInput1, userInput2)
	if !same {
		t.Errorf("UserInputArrayIsSame should have returned true but got false.")
	}

	userInput1 = []UserInput{svcUserInput2}
	userInput2 = nil
	same = UserInputArrayIsSame(userInput1, userInput2)
	if same {
		t.Errorf("UserInputArrayIsSame should have returned false but got true.")
	}

	userInput1 = nil
	userInput2 = nil
	same = UserInputArrayIsSame(userInput1, userInput2)
	if !same {
		t.Errorf("UserInputArrayIsSame should have returned true but got false.")
	}

	// changed the service ServiceOrgid
	svcUserInput3 := UserInput{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{Input{Name: "var1", Value: "val11"}, Input{Name: "var2", Value: []string{"a", "b", "c"}}, Input{Name: "var3", Value: false}},
	}
	userInput1 = []UserInput{svcUserInput1, svcUserInput2}
	userInput2 = []UserInput{svcUserInput1, svcUserInput3}
	same = UserInputArrayIsSame(userInput1, userInput2)
	if same {
		t.Errorf("UserInputArrayIsSame should have returned false but got true.")
	}

	svcUserInput3 = UserInput{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []Input{},
	}
	userInput1 = []UserInput{svcUserInput2}
	userInput2 = []UserInput{svcUserInput3}
	same = UserInputArrayIsSame(userInput1, userInput2)
	if same {
		t.Errorf("UserInputArrayIsSame should have returned false but got true.")
	}

	svcUserInput2 = UserInput{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              nil,
	}
	userInput1 = []UserInput{svcUserInput2}
	userInput2 = []UserInput{svcUserInput3}
	same = UserInputArrayIsSame(userInput1, userInput2)
	if !same {
		t.Errorf("UserInputArrayIsSame should have returned true but got false.")
	}

	userInput1 = []UserInput{svcUserInput2}
	userInput2 = []UserInput{svcUserInput2}
	same = UserInputArrayIsSame(userInput1, userInput2)
	if !same {
		t.Errorf("UserInputArrayIsSame should have returned true but got false.")
	}
}
