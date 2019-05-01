package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/externalpolicy"
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

// Update the policy object. This function does not require the device object to be created first.
func UpdateNodePolicy(nodePolicy *externalpolicy.ExternalPolicy,
	errorhandler ErrorHandler,
	db *bolt.DB,
	config *config.HorizonConfig) (bool, *externalpolicy.ExternalPolicy, []*events.NodePolicyMessage) {

	// Validate that the input policy is valid, and if so, save it into database.
	if err := nodePolicy.Validate(); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Node policy does not validate, error %v", err))), nil, nil
	} else if err := persistence.SaveNodePolicy(db, nodePolicy); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to save node policy, error %v", err))), nil, nil
	} else {

		LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("New node policy: %v", *nodePolicy), persistence.EC_NODE_POLICY_UPDATED, nil)

		nodePolicyUpdated := events.NewNodePolicyMessage(events.UPDATE_POLICY)
		return false, nodePolicy, []*events.NodePolicyMessage{nodePolicyUpdated}
	}

}

// Delete the node policy object from the local database.
func DeleteNodePolicy(errorhandler ErrorHandler, db *bolt.DB) (bool, []*events.NodePolicyMessage) {

	if err := persistence.DeleteNodePolicy(db); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Node policy could not be deleted, error %v", err))), nil
	}

	LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("Deleted node policy"), persistence.EC_NODE_POLICY_DELETED, nil)
	nodePolicyDeleted := events.NewNodePolicyMessage(events.DELETED_POLICY)
	return false, []*events.NodePolicyMessage{nodePolicyDeleted}
}
