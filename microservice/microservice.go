package microservice

import (
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
)

// Currently the MicroserviceDefiniton structures in exchange and persistence packages are identical.
// But we need to make them separate structures in case we need to put more in persistence structure.
// This function converts the structure from exchange to persistence.
func ConvertToPersistent(ems *exchange.MicroserviceDefinition) *persistence.MicroserviceDefinition {
	pms := new(persistence.MicroserviceDefinition)

	pms.Owner = ems.Owner
	pms.Label = ems.Label
	pms.Description = ems.Description
	pms.SpecRef = ems.SpecRef
	pms.Version = ems.Version
	pms.Arch = ems.Arch
	pms.Sharable = ems.Sharable
	pms.DownloadURL = ems.DownloadURL

	hwmatch := persistence.NewHardwareMatch(ems.MatchHardware.USBDeviceIds, ems.MatchHardware.Devfiles)
	pms.MatchHardware = *hwmatch

	user_inputs := make([]persistence.UserInput, 0)
	for _, ui := range ems.UserInputs {
		new_ui := persistence.NewUserInput(ui.Name, ui.Label, ui.Type, ui.DefaultValue)
		user_inputs = append(user_inputs, *new_ui)
	}
	pms.UserInputs = user_inputs

	workloads := make([]persistence.WorkloadDeployment, 0)
	for _, wl := range ems.Workloads {
		new_wl := persistence.NewWorkloadDeployment(wl.Deployment, wl.DeploymentSignature, wl.Torrent)
		workloads = append(workloads, *new_wl)
	}
	pms.Workloads = workloads

	pms.LastUpdated = ems.LastUpdated

	return pms
}
