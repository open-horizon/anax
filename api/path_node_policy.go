package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
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

// Update the policy object in the local node database and in the exchange.
func UpdateNodePolicy(nodePolicy *externalpolicy.ExternalPolicy,
	errorhandler ErrorHandler,
	exchangeHandler exchange.PutNodePolicyHandler,
	db *bolt.DB,
	config *config.HorizonConfig) (bool, *externalpolicy.ExternalPolicy, []*events.NodePolicyMessage) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		eventlog.LogDatabaseEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Unable to read node object from database, error %v", err), persistence.EC_DATABASE_ERROR)
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Error in node configuration. The node is not found from the database."), persistence.EC_ERROR_NODE_CONFIG_REG, nil)
		return errorhandler(NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil, nil
	}

	// Validate that the input policy is valid, and if so, save it into database.
	if err := nodePolicy.Validate(); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Node policy does not validate, error %v", err))), nil, nil
	} else if err := persistence.SaveNodePolicy(db, nodePolicy); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to save node policy, error %v", err))), nil, nil
	} else if _, err := exchangeHandler(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), &exchange.ExchangePolicy{ExternalPolicy: *nodePolicy}); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to save node policy in exchange, error %v", err))), nil, nil
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
