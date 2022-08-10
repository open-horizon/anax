package persistence

import (
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/policy"
)

// An agbot can be configured to run with several different databases. When running in a node agent, then the
// bolt DB is used. When running standalone in a cloud deployment, it will use postgresql so that there can
// be multiple instances of the agbot working together. This file contains the abstract interface representing
// the database handle used by the runtime to access the real database.

type AgbotDatabase interface {

	// Database related functions
	Initialize(cfg *config.HorizonConfig) error
	Close()

	// Database partition related functions.
	FindPartitions() ([]string, error)
	ClaimPartition(timeout uint64) (string, error)
	HeartbeatPartition() error
	GetHeartbeat() (uint64, error)
	QuiescePartition() error
	GetPartitionOwner(id string) (string, error)
	MovePartition(timeout uint64) (bool, error)

	// Persistent agreement related functions
	FindAgreements(filters []AFilter, protocol string) ([]Agreement, error)
	FindSingleAgreementByAgreementId(agreementid string, protocol string, filters []AFilter) (*Agreement, error)
	FindSingleAgreementByAgreementIdAllProtocols(agreementid string, protocols []string, filters []AFilter) (*Agreement, error)

	GetAgreementCount(partition string) (int64, int64, error)

	SingleAgreementUpdate(agreementid string, protocol string, fn func(Agreement) *Agreement) (*Agreement, error)

	AgreementAttempt(agreementid string, org string, deviceid string, deviceType string, policyName string, bcType string, bcName string, bcOrg string, agreementProto string, pattern string, serviceId []string, nhPolicy policy.NodeHealth, protocolTimeout uint64, agreementTimeout uint64) error
	AgreementFinalized(agreementid string, protocol string) (*Agreement, error)
	AgreementUpdate(agreementid string, proposal string, policy string, dvPolicy policy.DataVerification, defaultCheckRate uint64, hash string, sig string, protocol string, agreementProtoVersion int) (*Agreement, error)
	AgreementMade(agreementId string, counterParty string, signature string, protocol string, hapartners []string, bcType string, bcName string, bcOrg string) (*Agreement, error)
	AgreementBlockchainUpdate(agreementId string, consumerSig string, hash string, counterParty string, signature string, protocol string) (*Agreement, error)
	AgreementBlockchainUpdateAck(agreementId string, protocol string) (*Agreement, error)
	AgreementTimedout(agreementid string, protocol string) (*Agreement, error)
	AgreementSecretUpdateTime(agreementid string, protocol string, secretUpdateTime uint64) (*Agreement, error)
	AgreementSecretUpdateAckTime(agreementid string, protocol string, secretUpdateAckTime uint64) (*Agreement, error)

	DataNotification(agreementid string, protocol string) (*Agreement, error)
	DataVerified(agreementid string, protocol string) (*Agreement, error)
	DataNotVerified(agreementid string, protocol string) (*Agreement, error)
	MeteringNotification(agreementid string, protocol string, mn string) (*Agreement, error)

	DeleteAgreement(pk string, protocol string) error
	ArchiveAgreement(agreementid string, protocol string, reason uint, desc string) (*Agreement, error)

	// Workoad usage related functions
	NewWorkloadUsage(deviceId string, hapartners []string, policy string, policyName string, priority int, retryDurationS int, verifiedDurationS int, reqsNotMet bool, agid string) error
	FindSingleWorkloadUsageByDeviceAndPolicyName(deviceid string, policyName string) (*WorkloadUsage, error)
	FindWorkloadUsages(filters []WUFilter) ([]WorkloadUsage, error)

	GetWorkloadUsagesCount(partition string) (int64, error)

	SingleWorkloadUsageUpdate(deviceid string, policyName string, fn func(WorkloadUsage) *WorkloadUsage) (*WorkloadUsage, error)

	UpdatePendingUpgrade(deviceid string, policyName string) (*WorkloadUsage, error)
	UpdatePriority(deviceid string, policyName string, priority int, retryDurationS int, verifiedDurationS int, agid string) (*WorkloadUsage, error)
	UpdateRetryCount(deviceid string, policyName string, retryCount int, agid string) (*WorkloadUsage, error)
	UpdatePolicy(deviceid string, policyName string, pol string) (*WorkloadUsage, error)
	UpdateWUAgreementId(deviceid string, policyName string, agid string, protocol string) (*WorkloadUsage, error)
	DisableRollbackChecking(deviceid string, policyName string) (*WorkloadUsage, error)

	DeleteWorkloadUsage(deviceid string, policyName string) error

	// Functions related to persistence of search sessions with the Exchange.
	ObtainSearchSession(policyName string) (string, uint64, error)
	UpdateSearchSessionChangedSince(currentChangedSince uint64, newChangedSince uint64, policyName string) (bool, error)
	ResetAllChangedSince(newChangedSince uint64) error
	ResetPolicyChangedSince(policy string, newChangedSince uint64) error
	DumpSearchSessions() error

	// Functions related to persistence of secrets in use by active agreements.
	AddManagedPolicySecret(secretOrg, secretName, policyOrg, policyName string, updateTime int64) error
	AddManagedPatternSecret(secretOrg, secretName, patternOrg, patternName string, updateTime int64) error
	GetManagedPolicySecretNames(policyOrg, policyName string) ([]string, error)
	GetManagedPatternSecretNames(patternOrg, patternName string) ([]string, error)
	GetPoliciesWithUpdatedSecrets(secretOrg, secretName string, lastUpdate int64) ([]string, error)
	GetPatternsWithUpdatedSecrets(secretOrg, secretName string, lastUpdate int64) ([]string, error)
	SetSecretUpdate(secretOrg, secretName string, secretUpdateTime int64) error
	GetPoliciesInOrg(org string) ([]string, error)
	GetPatternsInOrg(org string) ([]string, error)
	DeleteSecretsForPolicy(polOrg, polName string) error
	DeleteSecretsForPattern(polOrg, patternName string) error
	DeletePolicySecret(secretOrg, secretName, policyOrg, policyName string) error
	DeletePatternSecret(secretOrg, secretName, patternOrg, patternName string) error

	// Functions related to persistence of the state of nodes in ha groups executing node management upgrades.
	CheckIfGroupPresentAndUpdateHATable(requestingNode UpgradingHAGroupNode) (*UpgradingHAGroupNode, error)
	DeleteHAUpgradeNode(nodeToDelete UpgradingHAGroupNode) error
	ListAllUpgradingNodesInOrg(orgId string) (*[]UpgradingHAGroupNode, error)

	// Functions related to persistence of the state of workload in ha groups executing service upgrades.
	//CheckIfWorkloadPresentAndUpdateHATable(requestingWorkload UpgradingHAGroupWorkload) (*UpgradingHAGroupWorkload, error)
	DeleteHAUpgradingWorkload(workloadToDelete UpgradingHAGroupWorkload) error
	DeleteHAUpgradingWorkloadsByGroupName(org string, haGroupName string) error
	DeleteHAUpgradingWorkloadsByGroupNameAndDeviceId(org string, haGroupName string, deviceId string) error
	ListHAUpgradingWorkloadsByGroupName(org string, haGroupName string) ([]UpgradingHAGroupWorkload, error)
	GetHAUpgradingWorkload(org string, haGroupName string, policyName string) (*UpgradingHAGroupWorkload, error)
	UpdateHAUpgradingWorkloadForGroupAndPolicy(org string, haGroupName string, policyName string, deviceId string) (bool, error)
	InsertHAUpgradingWorkloadForGroupAndPolicy(org string, haGroupName string, policyName string, deviceId string) error
}
