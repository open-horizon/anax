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
