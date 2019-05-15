package nodepolicy

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/persistence"
	"io/ioutil"
	"os"
	"sync"
)

var nodePolicyUpdateLock sync.Mutex //The lock that protects the nodePolicyLastUpdated value

// Check the node policy changes on the exchange and update the local copy with the changes.
func SyncNodePolicyWithExchange(db *bolt.DB, pDevice *persistence.ExchangeDevice, getExchangeNodePolicy exchange.NodePolicyHandler) (bool, *externalpolicy.ExternalPolicy, error) {

	glog.V(4).Infof("Checking the node policy changes.")

	nodePolicyUpdateLock.Lock()
	defer nodePolicyUpdateLock.Unlock()

	// get the node policy from the exchange
	exchangeNodePolicy, err := getExchangeNodePolicy(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id))
	if err != nil {
		return false, nil, fmt.Errorf("Unable to retrieve the node policy from the exchange. Error: %v", err)
	}

	// get the locally saved exchange node policy last updated string
	nodePolicyLastUpdated, err := persistence.GetNodePolicyLatUpdated_Exch(db)
	if err != nil {
		return false, nil, fmt.Errorf("Unable to retrieve the locally saved exchange node policy last updated string. Error: %v", err)
	}

	// if there is a change, update the local copy
	if exchangeNodePolicy != nil {
		if exchangeNodePolicy.GetLastUpdated() != nodePolicyLastUpdated {
			// update the local node policy
			newNodePolicy := exchangeNodePolicy.GetExternalPolicy()
			if err := persistence.SaveNodePolicy(db, &newNodePolicy); err != nil {
				return false, nil, fmt.Errorf("unable to save node policy %v to local database. %v", newNodePolicy, err)
			} else if err := persistence.SaveNodePolicyLatUpdated_Exch(db, exchangeNodePolicy.GetLastUpdated()); err != nil {
				return false, nil, fmt.Errorf("unable to save the exchange node policy last update string %v to local database. %v", nodePolicyLastUpdated, err)
			} else {
				glog.V(3).Infof("Updated the local node policy with the exchange copy: %v", newNodePolicy)
				return true, &newNodePolicy, nil
			}
		}
	} else {
		// get the local node policy
		localNodePolicy, err := persistence.FindNodePolicy(db)
		if err != nil {
			return false, nil, fmt.Errorf("Unable to read local node policy object. %v", err)
		}

		updated := false
		// delete the local node policy
		if localNodePolicy != nil {
			if err := persistence.DeleteNodePolicy(db); err != nil {
				return false, nil, fmt.Errorf("Node policy could not be deleted, error %v", err)
			}
			updated = true
		}

		if nodePolicyLastUpdated != "" {
			if err := persistence.DeleteNodePolicyLastUpdated_Exch(db); err != nil {
				return updated, nil, fmt.Errorf("Exchange node policy last update string could not be deleted from the local database, error %v", err)
			}
		}
		return updated, nil, nil
	}

	return false, nil, nil
}

// Sets the default node policy on local db and the exchange
func SetDefaultNodePolicy(policyFile string, pDevice *persistence.ExchangeDevice, db *bolt.DB,
	getExchangeNodePolicy exchange.NodePolicyHandler,
	putExchangeNodePolicy exchange.PutNodePolicyHandler) (*externalpolicy.ExternalPolicy, error) {

	glog.V(3).Infof("Setting up the default node cpolicy.")

	// check file exists
	if _, err := os.Stat(policyFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("Default node policy file does not exist: %v.", policyFile)
	}

	// read the file
	fileBytes, err := ioutil.ReadFile(policyFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to read the default policy file: %v. %v", policyFile, err)
	}

	// convert the json file to node policy
	var nodePolicy externalpolicy.ExternalPolicy
	err = json.Unmarshal(fileBytes, &nodePolicy)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal bytes to node policy: %v", err)
	}

	// upload the node policy on the exchange
	_, err = putExchangeNodePolicy(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), &exchange.ExchangePolicy{ExternalPolicy: nodePolicy})
	if err != nil {
		return nil, fmt.Errorf("Unable to save node policy in exchange. %v", err)
	}

	// sync the local policy with the one from exchange
	_, _, err = SyncNodePolicyWithExchange(db, pDevice, getExchangeNodePolicy)
	if err != nil {
		return nil, fmt.Errorf("Failed to sync the local node policy with the exchange copy. %v", err)
	}

	glog.V(3).Infof("Default node policy is set. %v", nodePolicy)

	return &nodePolicy, nil
}

// If the both local and exchange node policy are not created, use the default.
// Otherwise, update the local node policy with the one from the exchange.
func NodePolicyInitalSetup(db *bolt.DB, config *config.HorizonConfig,
	getExchangeNodePolicy exchange.NodePolicyHandler,
	putExchangeNodePolicy exchange.PutNodePolicyHandler) (*externalpolicy.ExternalPolicy, error) {

	glog.V(3).Infof("Node policy initial setup.")

	// get the node
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return nil, fmt.Errorf("Unable to read node object from the local database. %v", err)
	} else if pDevice == nil {
		return nil, fmt.Errorf("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.")
	}

	// get the local node policy
	localNodePolicy, err := persistence.FindNodePolicy(db)
	if err != nil {
		return nil, fmt.Errorf("Unable to read local node policy object. %v", err)
	}

	// get the exchage node policy
	var exchangeNodePolicy *exchange.ExchangePolicy
	exchangeNodePolicy = nil
	if pDevice != nil {
		exchangeNodePolicy, err = getExchangeNodePolicy(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id))
		if err != nil {
			return nil, fmt.Errorf("Unable to retrieve the node policy from the exchange. Error: %v", err)
		}
	}

	if localNodePolicy == nil && exchangeNodePolicy == nil {
		// get the default node policy file name from the config and set it up in local and exchange
		policyFile := config.Edge.DefaultNodePolicyFile
		if policyFile == "" {
			// delete the exchange last updated string in the local db
			if err := persistence.DeleteNodePolicyLastUpdated_Exch(db); err != nil {
				return nil, fmt.Errorf("Exchange node policy last update string could not be deleted from the local database, error %v", err)
			}
			glog.Infof("No default node policy file.")
			return nil, nil
		} else {
			return SetDefaultNodePolicy(policyFile, pDevice, db, getExchangeNodePolicy, putExchangeNodePolicy)
		}
	} else {
		// exchange is the master
		if _, nodePolicy, err := SyncNodePolicyWithExchange(db, pDevice, getExchangeNodePolicy); err != nil {
			return nodePolicy, fmt.Errorf("Failed to sync the local node policy with the exchange copy. %v", err)
		} else {
			return nodePolicy, nil
		}
	}
}

// check if the node policy has been changed from last sync.
func ExchangeNodePolicyChanged(pDevice *persistence.ExchangeDevice, db *bolt.DB, getExchangeNodePolicy exchange.NodePolicyHandler) (bool, *externalpolicy.ExternalPolicy, error) {

	// get the node policy from the exchange
	exchangeNodePolicy, err := getExchangeNodePolicy(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id))
	if err != nil {
		return false, nil, fmt.Errorf("Unable to retrieve the node policy from the exchange. Error: %v", err)
	}

	// get the locally saved exchange node policy last updated string
	nodePolicyLastUpdated, err := persistence.GetNodePolicyLatUpdated_Exch(db)
	if err != nil {
		return false, nil, fmt.Errorf("Unable to retrieve the locally saved exchange node policy last updated string. Error: %v", err)
	}

	// if there is a change, update the local copy
	if exchangeNodePolicy != nil {
		nodePolicy := exchangeNodePolicy.GetExternalPolicy()
		if exchangeNodePolicy.GetLastUpdated() != nodePolicyLastUpdated {
			return true, &nodePolicy, nil
		} else {
			return false, &nodePolicy, nil
		}
	} else {
		if nodePolicyLastUpdated != "" {
			return true, nil, nil
		} else {
			return false, nil, nil
		}
	}
}

// Delete the node policy from local db and the exchange
func DeleteNodePolicy(pDevice *persistence.ExchangeDevice, db *bolt.DB,
	getExchangeNodePolicy exchange.NodePolicyHandler,
	deleteExchangeNodePolicy exchange.DeleteNodePolicyHandler) error {

	nodePolicyUpdateLock.Lock()
	defer nodePolicyUpdateLock.Unlock()

	// check if the policy got changed on the exchange since last observation
	// if it is changed, then reject the deletion.
	changed, nodePolicy, err := ExchangeNodePolicyChanged(pDevice, db, getExchangeNodePolicy)
	if err != nil {
		return fmt.Errorf("Failed to check the exchange for the node policy: %v.", err)
	} else if changed {
		return fmt.Errorf("Cannot delete this node policy because the local node policy is out of sync with the exchange copy. Please wait a minute and try again.")
	}

	// delete the node policy from the exchange if it exists
	if nodePolicy != nil {
		if err := deleteExchangeNodePolicy(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id)); err != nil {
			return fmt.Errorf("Node policy could not be deleted from the exchange. %v", err)
		}
	}

	// delete local node policy
	if err := persistence.DeleteNodePolicy(db); err != nil {
		return fmt.Errorf("Node policy could not be deleted, error %v", err)
	}

	if err := persistence.DeleteNodePolicyLastUpdated_Exch(db); err != nil {
		return fmt.Errorf("Exchange node policy last update string could not be deleted from the local database, error %v", err)
	}
	return nil
}

// Update the node policy on local db and the exchange
func UpdateNodePolicy(pDevice *persistence.ExchangeDevice, db *bolt.DB, nodePolicy *externalpolicy.ExternalPolicy,
	nodeGetPolicyHandler exchange.NodePolicyHandler,
	nodePutPolicyHandler exchange.PutNodePolicyHandler) error {

	// verify the policy
	if err := nodePolicy.Validate(); err != nil {
		return fmt.Errorf("Node policy does not validate. %v", err)
	}

	// check if the policy is changed or not on the exchange since last observation,
	// if it is changed, then reject the updates.
	if changed, _, err := ExchangeNodePolicyChanged(pDevice, db, nodeGetPolicyHandler); err != nil {
		return fmt.Errorf("Failed to check the exchange for the node policy: %v.", err)
	} else if changed {
		return fmt.Errorf("Cannot accept this node policy because the local node policy is out of sync with the exchange copy. Please wait a minute and try again.")
	}

	// save it into the exchange and sync the local db with it.
	if _, err := nodePutPolicyHandler(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), &exchange.ExchangePolicy{ExternalPolicy: *nodePolicy}); err != nil {
		return fmt.Errorf("Unable to save node policy in exchange, error %v", err)
	} else if _, _, err := SyncNodePolicyWithExchange(db, pDevice, nodeGetPolicyHandler); err != nil {
		return fmt.Errorf("Unable to sync the local db with the exchange node policy. %v", err)
	}

	return nil
}
