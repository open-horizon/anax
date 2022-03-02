package exchangecommon

import (
	"fmt"
	"math/rand"
	"time"
)

type NodeManagementPolicyStatus struct {
	AgentUpgrade *AgentUpgradePolicyStatus `json:"agentUpgradePolicyStatus"`
}

func (n NodeManagementPolicyStatus) String() string {
	return fmt.Sprintf("AgentUpgrade: %v", n.AgentUpgrade)
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
	ScheduledTime        string `json:"scheduledTime"`
	scheduledUnixTime    time.Time
	ActualStartTime      string               `json:"startTime,omitempty"`
	CompletionTime       string               `json:"endTime,omitempty"`
	UpgradedVersions     AgentUpgradeVersions `json:"upgradedVersions"`
	Status               string               `json:"status"`
	ErrorMessage         string               `json:"errorMessage,omitempty"`
	BaseWorkingDirectory string               `json:"workingDirectory,omitempty"`
	allowDowngrade       bool
	manifest             string
}

func (a AgentUpgradePolicyStatus) String() string {
	return fmt.Sprintf("ScheduledTime: %v, ActualStartTime: %v, CompletionTime: %v, UpgradedVersions: %v, Status: %v, ErrorMessage: %v, BaseWorkingDirectory: %v, AllowDowngrade: %v, Manifest: %v",
		a.ScheduledTime, a.ActualStartTime, a.CompletionTime, a.UpgradedVersions, a.Status, a.ErrorMessage, a.BaseWorkingDirectory, a.allowDowngrade, a.manifest)
}

func (a AgentUpgradePolicyStatus) GetManifest() string {
	return a.manifest
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
		AgentUpgrade: &AgentUpgradePolicyStatus{Status: STATUS_NEW},
	}
	if policy.AgentAutoUpgradePolicy != nil {
		startTime, _ := time.Parse(time.RFC3339, policy.PolicyUpgradeTime)
		realStartTime := startTime.Unix()
		if policy.UpgradeWindowDuration > 0 {
			realStartTime = realStartTime + int64(rand.Intn(policy.UpgradeWindowDuration))
		}
		newStatus.AgentUpgrade.ScheduledTime = time.Unix(realStartTime, 0).Format(time.RFC3339)
		newStatus.AgentUpgrade.scheduledUnixTime = time.Unix(realStartTime, 0)
		newStatus.AgentUpgrade.BaseWorkingDirectory = workingDir
		newStatus.AgentUpgrade.allowDowngrade = policy.AgentAutoUpgradePolicy.AllowDowngrade
		newStatus.AgentUpgrade.manifest = policy.AgentAutoUpgradePolicy.Manifest
	}
	return newStatus
}

func (n NodeManagementPolicyStatus) TimeToStart() bool {
	if n.AgentUpgrade != nil {
		return n.AgentUpgrade.scheduledUnixTime.Before(time.Now())
	}
	return false
}
