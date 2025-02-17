package persistence

import "github.com/open-horizon/anax/policy"

const SERVICE_UPGRADE_STATUS = "service_upgrade_status"

// this table is to store the top level service upgrade status
// Delete record from service_upgrade_status table once the service upgrade is done
// Key is AgreementId. So for the same agreement Id, there can only have 1 upgrade at one time.
// If service upgrading (-> 2.0.0) is in progress, then agent receives agreement update to upgrade to 3.0.0, what should we do? (like nmp)

type AgreementWorkload struct {
	Workload WorkloadInfo `json:"workload"`
	//Workload          policy.Workload `json:"workload"`
	WorkloadStartTime uint64 `json:"workload_start_time"`
}

type ServiceUpgradeStatus struct {
	AgreementId              string               `json:"agreement_id"`
	CurrentMicroserviceDefId string               `json:"current_microservice_def_id"`
	CurrentRunningWorkload   AgreementWorkload    `json:"current_running_workload"`
	UpgradeMicroserviceDefId string               `json:"upgrade_microservice_def_id"`
	UpgradeWorkload          AgreementWorkload    `json:"upgrade_workload"`
	UpgradePolicy            policy.UpgradePolicy `json:"upgrade_policy"`
}

// ======== need to decide use AgreementWorkload v1 or v2 =========

type AgreementWorkload2 struct {
	//Workload          WorkloadInfo `json:"workload"`
	Workload          policy.Workload `json:"workload"`
	WorkloadStartTime uint64          `json:"workload_start_time"`
}

type ServiceUpgradeStatus2 struct {
	AgreementId              string               `json:"agreement_id"`
	CurrentMicroserviceDefId string               `json:"current_microservice_def_id"`
	CurrentRunningWorkload   AgreementWorkload2   `json:"current_running_workload"`
	UpgradeMicroserviceDefId string               `json:"upgrade_microservice_def_id"`
	UpgradeWorkload          AgreementWorkload2   `json:"upgrade_workload"`
	WorkloadUpgradePolicy    policy.UpgradePolicy `json:"workload_upgrade_policy"`
	UpgradeStatus            string               `json:"workload_upgrade_status"` // do we need this?
}
