package compcheck

import (
	"fmt"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
)

type ServiceSpec struct {
	ServiceOrgid        string `json:"serviceOrgid"`
	ServiceUrl          string `json:"serviceUrl"`
	ServiceArch         string `json:"serviceArch"`
	ServiceVersionRange string `json:"serviceVersionRange"` // version or version range. empty string means it applies to all versions
}

func (s ServiceSpec) String() string {
	return fmt.Sprintf("ServiceOrgid: %v, "+
		"ServiceUrl: %v, "+
		"ServiceArch: %v, "+
		"ServiceVersionRange: %v",
		s.ServiceOrgid, s.ServiceUrl, s.ServiceArch, s.ServiceVersionRange)
}

func NewServiceSpec(svcName, svcOrg, svcVersion, svcArch string) *ServiceSpec {
	return &ServiceSpec{
		ServiceOrgid:        svcOrg,
		ServiceUrl:          svcName,
		ServiceArch:         svcArch,
		ServiceVersionRange: svcVersion,
	}
}

// Verfiy that all userInput variables are correctly typed and that non-defaulted userInput variables are specified.
func VerifyUserInputForService(svcSpec *ServiceSpec,
	getService exchange.ServiceHandler,
	bpUserInput []policy.UserInput,
	deviceUserInput []policy.UserInput) error {

	// nothing to check
	if svcSpec == nil {
		return nil
	}

	// get the service from the exchange
	vExp, _ := semanticversion.Version_Expression_Factory(svcSpec.ServiceVersionRange)
	sdef, _, err := getService(svcSpec.ServiceUrl, svcSpec.ServiceOrgid, vExp.Get_expression(), svcSpec.ServiceArch)
	if err != nil {
		return fmt.Errorf("Failed to get the service %v from the exchange. %v", svcSpec, err)
	} else if sdef == nil {
		return fmt.Errorf("Servcie %v does not exist on the exchange.", svcSpec)
	}

	return VerifyUserInputForServiceDef(sdef, svcSpec.ServiceOrgid, bpUserInput, deviceUserInput)
}

// Verfiy that all userInput variables are correctly typed and that non-defaulted userInput variables are specified.
func VerifyUserInputForServiceDef(sdef *exchange.ServiceDefinition, svcOrg string, bpUserInput []policy.UserInput, deviceUserInput []policy.UserInput) error {
	// service does not need user input
	if !sdef.NeedsUserInput() {
		return nil
	}

	// service needs user input, find the correct elements in the array
	var mergedUI *policy.UserInput
	ui1, err := policy.FindUserInput(sdef.URL, svcOrg, sdef.Version, sdef.Arch, bpUserInput)
	if err != nil {
		return err
	}
	ui2, err := policy.FindUserInput(sdef.URL, svcOrg, sdef.Version, sdef.Arch, deviceUserInput)
	if err != nil {
		return err
	}

	if ui1 == nil && ui2 == nil {
		return fmt.Errorf("Cannot find user input for service %v/%v %v %v.", svcOrg, sdef.URL, sdef.Version, sdef.Arch)
	}

	if ui1 != nil && ui2 != nil {
		mergedUI, _ = policy.MergeUserInput(*ui1, *ui2, false)
	} else if ui1 != nil {
		mergedUI = ui1
	} else {
		mergedUI = ui2
	}

	// Verify that non-default variables are present.
	for _, ui := range sdef.UserInputs {
		found := false
		for _, mui := range mergedUI.Inputs {
			if ui.Name == mui.Name {
				found = true
				if err := cutil.VerifyWorkloadVarTypes(mui.Value, ui.Type); err != nil {
					return fmt.Errorf("Failed to validate the user input type for variable %v for service %v/%v %v %v. %v", ui.Name, svcOrg, sdef.URL, sdef.Version, sdef.Arch, err)
				}
			}
		}

		if !found && ui.DefaultValue == "" {
			return fmt.Errorf("A required user input value is missing for variable %v for service %v/%v %v %v", ui.Name, svcOrg, sdef.URL, sdef.Version, sdef.Arch)
		}
	}

	return nil
}
