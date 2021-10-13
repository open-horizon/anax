package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/exchangesync"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/persistence"
)

// Return an empty policy object or the object that's in the local database.
func FindNodePolicyForOutput(db *bolt.DB) (*exchangecommon.NodePolicy, error) {

	if extPolicy, err := persistence.FindNodePolicy(db); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read node policy object, error %v", err))
	} else if extPolicy == nil {
		return &exchangecommon.NodePolicy{}, nil
	} else {
		return extPolicy, nil
	}
}

// Update the policy object in the local node database and in the exchange.
func UpdateNodePolicy(nodePolicy *exchangecommon.NodePolicy,
	errorhandler DeviceErrorHandler,
	nodeGetPolicyHandler exchange.NodePolicyHandler,
	nodePutPolicyHandler exchange.PutNodePolicyHandler,
	db *bolt.DB) (bool, *exchangecommon.NodePolicy, []*events.NodePolicyMessage) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(nil, NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(nil, NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil, nil
	}

	if rc_deploy, rc_management, err := exchangesync.UpdateNodePolicy(pDevice, db, nodePolicy, nodeGetPolicyHandler, nodePutPolicyHandler); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Unable to sync the local db with the exchange node policy. %v", err))), nil, nil
	} else {
		LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_NEW_NODE_POL, *nodePolicy), persistence.EC_NODE_POLICY_UPDATED, pDevice)

		nodePolicyUpdated := events.NewNodePolicyMessage(events.UPDATE_POLICY, rc_deploy, rc_management)
		return false, nodePolicy, []*events.NodePolicyMessage{nodePolicyUpdated}
	}
}

// Update a single field of the policy object in the local node db and in the exchange
func PatchNodePolicy(attributeName string, patchObject interface{},
	errorhandler DeviceErrorHandler,
	nodeGetPolicyHandler exchange.NodePolicyHandler,
	nodePatchPolicyHandler exchange.PutNodePolicyHandler,
	db *bolt.DB) (bool, *exchangecommon.NodePolicy, []*events.NodePolicyMessage) {

	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(nil, NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(nil, NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil, nil
	}

	if rc_deploy, rc_management, nodePolicy, err := exchangesync.PatchNodePolicy(pDevice, db, attributeName, patchObject, nodeGetPolicyHandler, nodePatchPolicyHandler); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Unable to sync the local db with the exchange node policy. %v", err))), nil, nil
	} else {
		LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_NEW_NODE_POL, patchObject), persistence.EC_NODE_POLICY_UPDATED, pDevice)

		nodePolicyUpdated := events.NewNodePolicyMessage(events.UPDATE_POLICY, rc_deploy, rc_management)
		return false, nodePolicy, []*events.NodePolicyMessage{nodePolicyUpdated}

	}
}

// Delete the node policy object.
func DeleteNodePolicy(errorhandler DeviceErrorHandler, db *bolt.DB,
	nodeGetPolicyHandler exchange.NodePolicyHandler,
	nodeDeletePolicyHandler exchange.DeleteNodePolicyHandler) (bool, []*events.NodePolicyMessage) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(nil, NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil
	} else if pDevice == nil {
		return errorhandler(nil, NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil
	}

	// delete the node policy from both exchange the local db
	if err := exchangesync.DeleteNodePolicy(pDevice, db, nodeGetPolicyHandler, nodeDeletePolicyHandler); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Node policy could not be deleted. %v", err))), nil
	}

	LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_NODE_POL_DELETED), persistence.EC_NODE_POLICY_DELETED, pDevice)

	nodePolicyDeleted := events.NewNodePolicyMessage(events.DELETED_POLICY, externalpolicy.EP_COMPARE_DELETED, externalpolicy.EP_COMPARE_DELETED)
	return false, []*events.NodePolicyMessage{nodePolicyDeleted}

}
