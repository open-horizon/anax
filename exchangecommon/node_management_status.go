package exchangecommon

import (
	"fmt"
	"math/rand"
	"time"
)

type ExchangeNMPStatus struct {
	ManagementStatus map[string]*NodeManagementPolicyStatus `json:"managementStatus"`
}

type NodeManagementPolicyStatus struct {
	AgentUpgrade         *AgentUpgradePolicyStatus   `json:"agentUpgradePolicyStatus"`
	AgentUpgradeInternal *AgentUpgradeInternalStatus `json:"agentUpgradeInternal,omitempty"`
}

func (n NodeManagementPolicyStatus) String() string {
	return fmt.Sprintf("AgentUpgrade: %v, AgentUpgradeInternal: %v", n.AgentUpgrade, n.AgentUpgradeInternal)
}

func (n NodeManagementPolicyStatus) DeepCopy() NodeManagementPolicyStatus {
	return NodeManagementPolicyStatus{AgentUpgrade: n.AgentUpgrade.DeepCopy(), AgentUpgradeInternal: n.AgentUpgradeInternal.DeepCopy()}
}

func (n NodeManagementPolicyStatus) Status() string {
	if n.AgentUpgrade != nil {
		return n.AgentUpgrade.Status
	}
	return ""
}

func (n NodeManagementPolicyStatus) SetStatus(status string) {
	if n.AgentUpgrade != nil {
		n.AgentUpgrade.Status = status
	}
}

func (n NodeManagementPolicyStatus) SetErrorMessage(message string) {
	if n.AgentUpgrade != nil {
		n.AgentUpgrade.ErrorMessage = message
	}
}

func (n NodeManagementPolicyStatus) SetCompletionTime(timeStr string) {
	if n.AgentUpgrade != nil {
		n.AgentUpgrade.CompletionTime = timeStr
	}
}

func (n NodeManagementPolicyStatus) SetActualStartTime(timeStr string) {
	if n.AgentUpgrade != nil {
		n.AgentUpgrade.ActualStartTime = timeStr
	}
}

func (n NodeManagementPolicyStatus) IsAgentUpgradePolicy() bool {
	return n.AgentUpgrade != nil
}

type AgentUpgradePolicyStatus struct {
	ScheduledTime        string               `json:"scheduledTime"`
	ActualStartTime      string               `json:"startTime,omitempty"`
	CompletionTime       string               `json:"endTime,omitempty"`
	UpgradedVersions     AgentUpgradeVersions `json:"upgradedVersions"`
	Status               string               `json:"status"`
	ErrorMessage         string               `json:"errorMessage,omitempty"`
	BaseWorkingDirectory string               `json:"workingDirectory,omitempty"`
}

func (a AgentUpgradePolicyStatus) String() string {
	return fmt.Sprintf("ScheduledTime: %v, ActualStartTime: %v, CompletionTime: %v, UpgradedVersions: %v, Status: %v, ErrorMessage: %v, BaseWorkingDirectory: %v",
		a.ScheduledTime, a.ActualStartTime, a.CompletionTime, a.UpgradedVersions, a.Status, a.ErrorMessage, a.BaseWorkingDirectory)
}

func (a AgentUpgradePolicyStatus) DeepCopy() *AgentUpgradePolicyStatus {
	return &AgentUpgradePolicyStatus{ScheduledTime: a.ScheduledTime, ActualStartTime: a.ActualStartTime, CompletionTime: a.CompletionTime,
		UpgradedVersions: a.UpgradedVersions, Status: a.Status, ErrorMessage: a.ErrorMessage, BaseWorkingDirectory: a.BaseWorkingDirectory}
}

type AgentUpgradeInternalStatus struct {
	AllowDowngrade    bool               `json:"allowDowngrade,omitempty"`
	Manifest          string             `json:"manifest,omitempty"`
	ScheduledUnixTime time.Time          `json:"scheduledUnixTime,omitempty"`
	LatestMap         AgentUpgradeLatest `json:"latestMap"`
}

func (a AgentUpgradeInternalStatus) String() string {
	return fmt.Sprintf("AllowDowngrade: %v, Manifest: %v, ScheduledUnixTime: %v, LatestMap: %v", a.AllowDowngrade, a.Manifest, a.ScheduledUnixTime, a.LatestMap)
}

func (a AgentUpgradeInternalStatus) DeepCopy() *AgentUpgradeInternalStatus {
	return &AgentUpgradeInternalStatus{AllowDowngrade: a.AllowDowngrade, Manifest: a.Manifest, ScheduledUnixTime: a.ScheduledUnixTime, LatestMap: a.LatestMap}
}

type AgentUpgradeLatest struct {
	SoftwareLatest bool `json:"softwareLatest"`
	ConfigLatest   bool `json:"configLatest"`
	CertLatest     bool `json:"certLatest"`
}

func (a AgentUpgradeLatest) String() string {
	return fmt.Sprintf("SoftwareLatest: %v, ConfigLatest: %v, CertLatest: %v", a.SoftwareLatest, a.ConfigLatest, a.CertLatest)
}

type AgentUpgradeVersions struct {
	SoftwareVersion string `json:"softwareVersion"`
	CertVersion     string `json:"certVersion"`
	ConfigVersion   string `json:"configVersion"`
}

func (a AgentUpgradeVersions) String() string {
	return fmt.Sprintf("SoftwareVersion: %v, CertVersion: %v, ConfigVersion: %v", a.SoftwareVersion, a.CertVersion, a.ConfigVersion)
}

const (
	STATUS_NEW                 = "waiting"
	STATUS_UNKNOWN             = "unknown"
	STATUS_DOWNLOAD_STARTED    = "download started"
	STATUS_DOWNLOADED          = "downloaded"
	STATUS_DOWNLOAD_FAILED     = "download failed"
	STATUS_SUCCESSFUL          = "successful"
	STATUS_FAILED_JOB          = "failed"
	STATUS_INITIATED           = "initiated"
	STATUS_ROLLBACK_STARTED    = "rollback started"
	STATUS_ROLLBACK_FAILED     = "rollback failed"
	STATUS_ROLLBACK_SUCCESSFUL = "rollback successful"
)

func StatusFromNewPolicy(policy ExchangeNodeManagementPolicy, workingDir string) NodeManagementPolicyStatus {
	newStatus := NodeManagementPolicyStatus{
		AgentUpgrade: &AgentUpgradePolicyStatus{Status: STATUS_NEW}, AgentUpgradeInternal: &AgentUpgradeInternalStatus{},
	}
	if policy.AgentAutoUpgradePolicy != nil {
		startTime, _ := time.Parse(time.RFC3339, policy.PolicyUpgradeTime)
		realStartTime := startTime.Unix()
		if policy.UpgradeWindowDuration > 0 {
			realStartTime = realStartTime + int64(rand.Intn(policy.UpgradeWindowDuration))
		}
		newStatus.AgentUpgrade.ScheduledTime = time.Unix(realStartTime, 0).Format(time.RFC3339)
		newStatus.AgentUpgradeInternal.ScheduledUnixTime = time.Unix(realStartTime, 0)
		newStatus.AgentUpgrade.BaseWorkingDirectory = workingDir
		newStatus.AgentUpgradeInternal.AllowDowngrade = policy.AgentAutoUpgradePolicy.AllowDowngrade
		newStatus.AgentUpgradeInternal.Manifest = policy.AgentAutoUpgradePolicy.Manifest
	}
	return newStatus
}

func (n NodeManagementPolicyStatus) TimeToStart() bool {
	if n.AgentUpgradeInternal != nil {
		return n.AgentUpgradeInternal.ScheduledUnixTime.Before(time.Now())
	}
	return false
}
