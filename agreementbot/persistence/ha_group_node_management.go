package persistence

import (
	"fmt"
)

type UpgradingHAGroupNode struct {
	GroupName string `json:"groupName"`
	OrgId     string `json:"orgId"`
	NodeId    string `json:"nodeId"`  // does not contain the org
	NMPName   string `json:"nmpName"` // does not contain the org
}

func (u UpgradingHAGroupNode) String() string {
	return fmt.Sprintf("GroupName: %s, OrgId: %s, NodeId: %s, NMPName: %s", u.GroupName, u.OrgId, u.NodeId, u.NMPName)
}

func (u UpgradingHAGroupNode) ShortString() string {
	return u.String()
}

func (u UpgradingHAGroupNode) DeepEqual(v UpgradingHAGroupNode) bool {
	return u.GroupName == v.GroupName && u.OrgId == v.OrgId && u.NodeId == v.NodeId && u.NMPName == v.NMPName
}

// This function will check if a node in the given HAGroup and org is currently in the upgrading table
// If none is present, the querying node will be added
// The function returns the node in the table for that group after the query
// So if the returned node is the same as the requesting one, give permission to upgrade
// Otherwise, do not allow the requestin node to upgrade
func NodeManagementUpgradeQuery(db AgbotDatabase, requestingNode UpgradingHAGroupNode) (*UpgradingHAGroupNode, error) {
	if dbNode, err := db.CheckIfGroupPresentAndUpdateHATable(requestingNode); err != nil {
		return nil, err
	} else {
		return dbNode, nil
	}
}

func DeleteHAUpgradingNode(db AgbotDatabase, orgId string, groupName string, nodeId string, nmpName string) error {
	return db.DeleteHAUpgradeNode(UpgradingHAGroupNode{GroupName: groupName, OrgId: orgId, NodeId: nodeId, NMPName: nmpName})
}

func GetUpgradingNodeInGroup(db AgbotDatabase, orgId string, groupName string) (*UpgradingHAGroupNode, error) {
	return db.ListUpgradingNodeInGroup(orgId, groupName)
}

type HANodeUpgradeFilter func(UpgradingHAGroupNode) bool

func OrgHANodeUpgradeFilter(orgId string) HANodeUpgradeFilter {
	return func(u UpgradingHAGroupNode) bool { return u.OrgId == orgId }
}

func GroupHANodeUpgradeFilter(groupName string) HANodeUpgradeFilter {
	return func(u UpgradingHAGroupNode) bool { return u.GroupName == groupName }
}
