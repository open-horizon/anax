package bolt

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/agreementbot/persistence"
)

const HABUCKET = "ha_updates"

func (db *AgbotBoltDB) CheckIfGroupPresentAndUpdateHATable(requestingNode persistence.UpgradingHAGroupNode) (*persistence.UpgradingHAGroupNode, error) {
	var updatedDBNode persistence.UpgradingHAGroupNode

	dbErr := db.db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(HABUCKET)); err != nil {
			return err
		} else {
			dbNodeJson := b.Get([]byte(groupId(requestingNode.OrgId, requestingNode.GroupName)))
			var dbNode persistence.UpgradingHAGroupNode

			// there is no node in this group updating. put the requesting node into the table
			if dbNodeJson == nil {
				if serialized, err := json.Marshal(dbNodeJson); err != nil {
					return err
				} else if err = b.Put([]byte(HABUCKET), serialized); err != nil {
					return err
				}
				updatedDBNode = requestingNode
				return nil
			}

			if err := json.Unmarshal(dbNodeJson, &dbNode); err != nil {
				return err
			}
			updatedDBNode = dbNode
			return nil
		}
	})

	return &updatedDBNode, dbErr
}

func (db *AgbotBoltDB) DeleteHAUpgradeNode(nodeToDelete persistence.UpgradingHAGroupNode) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(HABUCKET)); b == nil {
			return fmt.Errorf("Unknown bucket %v", HABUCKET)
		} else {
			b.Get([]byte(groupId(nodeToDelete.OrgId, nodeToDelete.GroupName)))
			return b.Delete([]byte(groupId(nodeToDelete.OrgId, nodeToDelete.GroupName)))
		}
	})
}

func (db *AgbotBoltDB) DeleteHAUpgradeNodeByGroup(orgId string, groupName string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(HABUCKET)); b == nil {
			return fmt.Errorf("Unknown bucket %v", HABUCKET)
		} else {
			return b.Delete([]byte(groupId(orgId, groupName)))
		}
	})
}

func (db *AgbotBoltDB) FindHAUpgradeNodesWithFilters(filterSlice []persistence.HANodeUpgradeFilter) ([]persistence.UpgradingHAGroupNode, error) {
	upgradingHANodes := make([]persistence.UpgradingHAGroupNode, 0)

	readErr := db.db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(HABUCKET)); b != nil {
			b.ForEach(func(k, v []byte) error {
				var haNode persistence.UpgradingHAGroupNode

				if err := json.Unmarshal(v, &haNode); err != nil {
					return err
				} else {
					include := true
					for _, filter := range filterSlice {
						if !filter(haNode) {
							include = false
						}
					}

					if include {
						upgradingHANodes = append(upgradingHANodes, haNode)
					}
				}
				return nil
			})
		}
		return nil
	})

	if readErr == nil {
		return upgradingHANodes, nil
	}
	return nil, readErr
}

func (db *AgbotBoltDB) ListUpgradingNodeInGroup(orgId string, groupName string) (*persistence.UpgradingHAGroupNode, error) {
	if nodesInGroup, err := db.FindHAUpgradeNodesWithFilters([]persistence.HANodeUpgradeFilter{persistence.OrgHANodeUpgradeFilter(orgId), persistence.GroupHANodeUpgradeFilter(groupName)}); err != nil {
		return nil, err
	} else if len(nodesInGroup) == 0 {
		return nil, nil
	} else if len(nodesInGroup) > 1 {
		return nil, fmt.Errorf("Error: multiple nodes in group %v/%v are present in upgrading table.", orgId, groupName)
	} else {
		return &(nodesInGroup)[0], nil
	}
}

func (db *AgbotBoltDB) ListAllUpgradingHANode() ([]persistence.UpgradingHAGroupNode, error) {
	return db.FindHAUpgradeNodesWithFilters([]persistence.HANodeUpgradeFilter{})
}

func groupId(orgId string, groupName string) string {
	return fmt.Sprintf("%s/%s", orgId, groupName)
}
