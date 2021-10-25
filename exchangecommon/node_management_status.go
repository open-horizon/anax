package exchangecommon

import (
	"fmt"
)

type NodeManagementPolicyStatus struct {
	ScheduledTime   string `json:"scheduledTime"`
	ActualStartTime string `json:"startTime,omitempty"`
	CompletionTime  string `json:"endTime,omitempty"`
	UpgradedVersion string `json:"upgradedVersion"`
	Status          string `json:"status"`
	ErrorMessage    string `json:"errorMessage,omitempty"`
}

func (n NodeManagementPolicyStatus) String() string {
	return fmt.Sprintf("ScheduledTime: %v, ActualStartTime: %v, CompletionTime: %v, UpgradedVersion: %v, Status: %v, ErrorMessage: %v",
		n.ScheduledTime, n.ActualStartTime, n.CompletionTime, n.UpgradedVersion, n.Status, n.ErrorMessage)
}
