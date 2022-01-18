package exchangecommon

import (
	"fmt"
	"math/rand"
	"time"
)

type NodeManagementPolicyStatus struct {
	AgentUpgrade *AgentUpgradePolicyStatus `json:"agentUpgrade"`
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

type AgentUpgradePolicyStatus struct {
	ScheduledTime        string `json:"scheduledTime"`
	scheduledUnixTime    time.Time
	ActualStartTime      string `json:"startTime,omitempty"`
	CompletionTime       string `json:"endTime,omitempty"`
	UpgradedVersion      string `json:"upgradedVersion"`
	Status               string `json:"status"`
	ErrorMessage         string `json:"errorMessage,omitempty"`
	BaseWorkingDirectory string `json:"workingDirectory"`
}

const (
	STATUS_NEW             = "waiting"
	STATUS_UNKNOWN         = "unknown"
	STATUS_DOWNLOADED      = "downloaded"
	STATUS_DOWNLOAD_FAILED = "failed download"
	STATUS_SUCCESSFUL      = "successful"
	STATUS_FAILED_JOB      = "failed"
	STATUS_INITIATED       = "initiated"
)

func (a AgentUpgradePolicyStatus) String() string {
	return fmt.Sprintf("ScheduledTime: %v, ActualStartTime: %v, CompletionTime: %v, UpgradedVersion: %v, Status: %v, ErrorMessage: %v, BaseWorkingDirectory: %v",
		a.ScheduledTime, a.ActualStartTime, a.CompletionTime, a.UpgradedVersion, a.Status, a.ErrorMessage, a.BaseWorkingDirectory)
}

func StatusFromNewPolicy(policy ExchangeNodeManagementPolicy, workingDir string) NodeManagementPolicyStatus {
	newStatus := NodeManagementPolicyStatus{
		AgentUpgrade: &AgentUpgradePolicyStatus{Status: STATUS_NEW},
	}
	if policy.AgentAutoUpgradePolicy != nil {
		startTime, _ := time.Parse(time.RFC3339, policy.AgentAutoUpgradePolicy.PolicyUpgradeTime)
		realStartTime := startTime.Unix()
		if policy.AgentAutoUpgradePolicy.UpgradeWindowDuration > 0 {
			realStartTime = realStartTime + int64(rand.Intn(policy.AgentAutoUpgradePolicy.UpgradeWindowDuration))
		}
		newStatus.AgentUpgrade.ScheduledTime = time.Unix(realStartTime, 0).Format(time.RFC3339)
		newStatus.AgentUpgrade.scheduledUnixTime = time.Unix(realStartTime, 0)
		newStatus.AgentUpgrade.BaseWorkingDirectory = workingDir
	}
	return newStatus
}

func (n NodeManagementPolicyStatus) TimeToStart() bool {
	if n.AgentUpgrade != nil {
		return n.AgentUpgrade.scheduledUnixTime.Before(time.Now())
	}
	return false
}
