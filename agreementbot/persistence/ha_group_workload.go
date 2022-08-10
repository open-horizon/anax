package persistence

import (
	"errors"
	"fmt"
	"strings"
)

type UpgradingHAGroupWorkload struct {
	GroupName  string `json:"groupName"`
	OrgId      string `json:"orgId"`
	PolicyName string `json:"policyName"`
	NodeId     string `json:"nodeId"`
}

func (u UpgradingHAGroupWorkload) String() string {
	return fmt.Sprintf("GroupName: %s, OrgId: %s, PolicyName: %s, NodeId: %s", u.GroupName, u.OrgId, u.PolicyName, u.NodeId)
}

func (u UpgradingHAGroupWorkload) ShortString() string {
	return u.String()
}

// TODO: need to check this function used in nmp
func (u UpgradingHAGroupWorkload) DeepEqual(v UpgradingHAGroupWorkload) bool {
	return u.GroupName == v.GroupName && u.OrgId == v.OrgId && u.PolicyName == v.PolicyName && u.NodeId == v.NodeId
}

func NewUpgradingHAGroupWorkload(groupName string, orgId string, policyName string, nodeId string) (*UpgradingHAGroupWorkload, error) {
	if groupName == "" || orgId == "" || policyName == "" || nodeId == "" {
		return nil, errors.New("Illegal input: one of groupName, orgId, policyName or nodeId is empty")
	} else if !deviceIDContainsOrg(nodeId) {
		nodeId = fmt.Sprintf("%v/%v", orgId, nodeId)
	}

	return &UpgradingHAGroupWorkload{
		GroupName:  groupName,
		OrgId:      orgId,
		PolicyName: policyName,
		NodeId:     nodeId,
	}, nil
}

type HAWorkloadUpgradeFilter func(UpgradingHAGroupWorkload) bool

// func OrgHANodeUpgradeFilter(orgId string) HANodeUpgradeFilter {
// 	return func(u UpgradingHAGroupNode) bool { return u.OrgId == orgId }
// }

func HAWorkloadUpgradeGroupFilter(org string, groupName string) HAWorkloadUpgradeFilter {
	return func(u UpgradingHAGroupWorkload) bool { return u.GroupName == groupName && u.OrgId == org }
}

func HAWorkloadUpgradeGroupAndNodeFilter(org string, groupName string, nodeId string) HAWorkloadUpgradeFilter {
	return func(u UpgradingHAGroupWorkload) bool {
		return u.GroupName == groupName && u.OrgId == org && u.NodeId == nodeId
	}
}

func deviceIDContainsOrg(deviceID string) bool {
	parts := strings.Split(deviceID, "/")
	return len(parts) == 2
}
