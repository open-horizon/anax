package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/nodepolicy"
	"github.com/open-horizon/anax/persistence"
)

// Return an empty policy object or the object that's in the local database.
func FindNodePolicyForOutput(db *bolt.DB) (*externalpolicy.ExternalPolicy, error) {

	if extPolicy, err := persistence.FindNodePolicy(db); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read node policy object, error %v", err))
	} else if extPolicy == nil {
		return &externalpolicy.ExternalPolicy{
			// Properties: []externalpolicy.Property{},
			// Constraints: "",
		}, nil
	} else {
		return extPolicy, nil
	}
}

// Update the policy object in the local node database and in the exchange.
func UpdateNodePolicy(nodePolicy *externalpolicy.ExternalPolicy,
	errorhandler DeviceErrorHandler,
	nodeGetPolicyHandler exchange.NodePolicyHandler,
	nodePutPolicyHandler exchange.PutNodePolicyHandler,
	db *bolt.DB) (bool, *externalpolicy.ExternalPolicy, []*events.NodePolicyMessage) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(nil, NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(nil, NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil, nil
	}

	if err := nodepolicy.UpdateNodePolicy(pDevice, db, nodePolicy, nodeGetPolicyHandler, nodePutPolicyHandler); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Unable to sync the local db with the exchange node policy. %v", err))), nil, nil
	} else {
		LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("New node policy: %v", *nodePolicy), persistence.EC_NODE_POLICY_UPDATED, pDevice)

		nodePolicyUpdated := events.NewNodePolicyMessage(events.UPDATE_POLICY)
		return false, nodePolicy, []*events.NodePolicyMessage{nodePolicyUpdated}
	}
}

func PatchNodePolicy(patchObject interface{},
	errorhandler DeviceErrorHandler,
	nodeGetPolicyHandler exchange.NodePolicyHandler,
	nodePatchPolicyHandler exchange.PutNodePolicyHandler,
	db *bolt.DB) (bool, *externalpolicy.ExternalPolicy, []*events.NodePolicyMessage) {

	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(nil, NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(nil, NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil, nil
	}

	if nodePolicy, err := nodepolicy.PatchNodePolicy(pDevice, db, patchObject, nodeGetPolicyHandler, nodePatchPolicyHandler); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Unable to sync the local db with the exchange node policy. %v", err))), nil, nil
	} else {
		LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("New node policy: %v", patchObject), persistence.EC_NODE_POLICY_UPDATED, pDevice)

		nodePolicyUpdated := events.NewNodePolicyMessage(events.UPDATE_POLICY)
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
	if err := nodepolicy.DeleteNodePolicy(pDevice, db, nodeGetPolicyHandler, nodeDeletePolicyHandler); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Node policy could not be deleted. %v", err))), nil
	}

	LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("Deleted node policy"), persistence.EC_NODE_POLICY_DELETED, pDevice)

	nodePolicyDeleted := events.NewNodePolicyMessage(events.DELETED_POLICY)
	return false, []*events.NodePolicyMessage{nodePolicyDeleted}
}
