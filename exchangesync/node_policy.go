package exchangesync

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
func SyncNodePolicyWithExchange(db *bolt.DB, pDevice *persistence.ExchangeDevice, getExchangeNodePolicy exchange.NodePolicyHandler, putExchangeNodePolicy exchange.PutNodePolicyHandler) (bool, *externalpolicy.ExternalPolicy, error) {

	glog.V(4).Infof("Checking the node policy changes.")

	nodePolicyUpdateLock.Lock()
	defer nodePolicyUpdateLock.Unlock()

	// get the node policy from the exchange
	exchangeNodePolicy, err := GetProcessedExchangeNodePolicy(pDevice, getExchangeNodePolicy, putExchangeNodePolicy)
	if err != nil {
		return false, nil, err
	}

	// get the locally saved exchange node policy last updated string
	nodePolicyLastUpdated, err := persistence.GetNodePolicyLastUpdated_Exch(db)
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
			} else if err := persistence.SaveNodePolicyLastUpdated_Exch(db, exchangeNodePolicy.GetLastUpdated()); err != nil {
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

// This function retrievs the node's policy from the exchange, adds the node built-in properties if needed. Then it saves the new
// node policy to the exchange again and then returns the new node policy. If the exchange node policy already has the built-in properties,
// it just returns the one from the exchange.
func GetProcessedExchangeNodePolicy(pDevice *persistence.ExchangeDevice, getExchangeNodePolicy exchange.NodePolicyHandler, putExchangeNodePolicy exchange.PutNodePolicyHandler) (*exchange.ExchangePolicy, error) {
	exchangeNodePolicy, err := getExchangeNodePolicy(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id))
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve the node policy from the exchange. Error: %v", err)
	}

	hasHwId := exchangeNodePolicy.Properties.HasProperty(externalpolicy.PROP_NODE_HARDWAREID)

	builtinPolicy := externalpolicy.CreateNodeBuiltInPolicy(false, !hasHwId)

	var mergedPol *externalpolicy.ExternalPolicy
	if exchangeNodePolicy == nil {
		mergedPol = builtinPolicy
	} else {
		// check if it contains node's built-in properties and they are the same
		needsBuiltIns := false
		//builtinNodePropNames := []string{externalpolicy.PROP_NODE_CPU, externalpolicy.PROP_NODE_MEMORY, externalpolicy.PROP_NODE_ARCH}
		//for _, propName := range builtinNodePropNames {
		//	if !exchangeNodePolicy.GetExternalPolicy().Properties.HasProperty(propName) {
		//		needsBuiltIns = true
		//		break
		//	}
		//}
		for _, bltinProp := range builtinPolicy.Properties {
			found := false
			for _, exchProp := range exchangeNodePolicy.Properties {
				if exchProp.Name == bltinProp.Name && exchProp.Value == bltinProp.Value {
					found = true
				}
			}
			if !found {
				needsBuiltIns = true
				break
			}
		}

		if needsBuiltIns {
			polTemp := exchangeNodePolicy.GetExternalPolicy()
			mergedPol = &polTemp
			mergedPol.MergeWith(builtinPolicy, true)
		}
	}

	// save the merged policy to the exchange
	if mergedPol != nil {
		_, err := putExchangeNodePolicy(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), &exchange.ExchangePolicy{ExternalPolicy: *mergedPol})
		if err != nil {
			return nil, fmt.Errorf("Unable to save node policy in exchange. %v", err)
		}

		// retrieve the node policy from the exchange again so that we get the last updated time stamp
		newExchangeNodePolicy, err := getExchangeNodePolicy(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id))
		if err != nil {
			return nil, fmt.Errorf("Unable to retrieve the node policy from the exchange. Error: %v", err)
		}
		return newExchangeNodePolicy, nil
	} else {
		return exchangeNodePolicy, nil
	}
}

// Sets the default node policy on local db and the exchange
func SetDefaultNodePolicy(config *config.HorizonConfig, pDevice *persistence.ExchangeDevice, db *bolt.DB,
	getExchangeNodePolicy exchange.NodePolicyHandler,
	putExchangeNodePolicy exchange.PutNodePolicyHandler) (*externalpolicy.ExternalPolicy, error) {

	glog.V(3).Infof("Setting up the default node policy.")

	nodePolicy := new(externalpolicy.ExternalPolicy)
	builtinNodePol := externalpolicy.CreateNodeBuiltInPolicy(false, true)

	// get the default node policy file name from the config and set it up in local and exchange
	policyFile := config.Edge.DefaultNodePolicyFile
	if policyFile == "" {
		// delete the exchange last updated string in the local db
		if err := persistence.DeleteNodePolicyLastUpdated_Exch(db); err != nil {
			return nil, fmt.Errorf("Exchange node policy last update string could not be deleted from the local database, error %v", err)
		}
		glog.Infof("No default node policy file in the anac configuration file. Use node's default built-in properties.")
		if builtinNodePol != nil {
			nodePolicy = builtinNodePol
		}
	} else {
		// check file exists
		if _, err := os.Stat(policyFile); os.IsNotExist(err) {
			glog.Errorf("Default node policy file does not exist: %v.", policyFile)
			if builtinNodePol != nil {
				nodePolicy = builtinNodePol
			}
		} else {
			// read the file
			fileBytes, err := ioutil.ReadFile(policyFile)
			if err != nil {
				return nil, fmt.Errorf("Failed to read the default policy file: %v. %v", policyFile, err)
			}

			// convert the json file to node policy
			err = json.Unmarshal(fileBytes, nodePolicy)
			if err != nil {
				return nil, fmt.Errorf("Failed to unmarshal bytes to node policy: %v", err)
			}

			// add the built-in properties if they are not in the default policy file
			if builtinNodePol != nil {
				nodePolicy.MergeWith(builtinNodePol, true)
			}
		}
	}

	// verify the policy before saving it
	if err := nodePolicy.Validate(); err != nil {
		return nil, fmt.Errorf("Node policy with built-in properties does not validate. %v", err)
	}

	// upload the node policy on the exchange
	_, err := putExchangeNodePolicy(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), &exchange.ExchangePolicy{ExternalPolicy: *nodePolicy})
	if err != nil {
		return nil, fmt.Errorf("Unable to save node policy in exchange. %v", err)
	}

	// sync the local policy with the one from exchange
	_, _, err = SyncNodePolicyWithExchange(db, pDevice, getExchangeNodePolicy, putExchangeNodePolicy)
	if err != nil {
		return nil, fmt.Errorf("Failed to sync the local node policy with the exchange copy. %v", err)
	}

	glog.V(3).Infof("Default node policy is set. %v", nodePolicy)

	return nodePolicy, nil
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
		return SetDefaultNodePolicy(config, pDevice, db, getExchangeNodePolicy, putExchangeNodePolicy)
	} else {
		// exchange is the master
		if _, nodePolicy, err := SyncNodePolicyWithExchange(db, pDevice, getExchangeNodePolicy, putExchangeNodePolicy); err != nil {
			return nodePolicy, fmt.Errorf("Failed to sync the local node policy with the exchange copy. %v", err)
		} else {
			return nodePolicy, nil
		}
	}
}

// check if the node policy has been changed from last sync.
// It returns the latest node policy on the exchange.
func ExchangeNodePolicyChanged(pDevice *persistence.ExchangeDevice, db *bolt.DB, getExchangeNodePolicy exchange.NodePolicyHandler) (bool, *externalpolicy.ExternalPolicy, error) {

	// get the node policy from the exchange
	exchangeNodePolicy, err := getExchangeNodePolicy(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id))
	if err != nil {
		return false, nil, fmt.Errorf("Unable to retrieve the node policy from the exchange. Error: %v", err)
	}

	// get the locally saved exchange node policy last updated string
	nodePolicyLastUpdated, err := persistence.GetNodePolicyLastUpdated_Exch(db)
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

	// check if the policy got changed on the exchange since last observation,
	// the returned the nodePolicy is the current exchange node policy
	_, nodePolicy, err := ExchangeNodePolicyChanged(pDevice, db, getExchangeNodePolicy)
	if err != nil {
		return fmt.Errorf("Failed to check the exchange for the node policy: %v.", err)
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

// Update (create new or replace old) node policy on local db and the exchange
func UpdateNodePolicy(pDevice *persistence.ExchangeDevice, db *bolt.DB, nodePolicy *externalpolicy.ExternalPolicy,
	nodeGetPolicyHandler exchange.NodePolicyHandler,
	nodePutPolicyHandler exchange.PutNodePolicyHandler) error {

	// verify the policy
	if err := nodePolicy.Validate(); err != nil {
		return fmt.Errorf("Node policy does not validate. %v", err)
	}

	hasHwId := nodePolicy.Properties.HasProperty(externalpolicy.PROP_NODE_HARDWAREID)

	// add node's built-in properties
	builtinNodePol := externalpolicy.CreateNodeBuiltInPolicy(false, hasHwId)
	if builtinNodePol != nil {
		nodePolicy.MergeWith(builtinNodePol, true)
	}

	// verify the policy again
	if err := nodePolicy.Validate(); err != nil {
		return fmt.Errorf("Node policy with built-in properties does not validate. %v", err)
	}

	// save it into the exchange and sync the local db with it.
	if _, err := nodePutPolicyHandler(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), &exchange.ExchangePolicy{ExternalPolicy: *nodePolicy}); err != nil {
		return fmt.Errorf("Unable to save node policy in exchange, error %v", err)
	} else if _, _, err := SyncNodePolicyWithExchange(db, pDevice, nodeGetPolicyHandler, nodePutPolicyHandler); err != nil {
		return fmt.Errorf("Unable to sync the local db with the exchange node policy. %v", err)
	}

	return nil
}

func PatchNodePolicy(pDevice *persistence.ExchangeDevice, db *bolt.DB, patchObject interface{},
	nodeGetPolicyHandler exchange.NodePolicyHandler,
	nodePutPolicyHandler exchange.PutNodePolicyHandler) (*externalpolicy.ExternalPolicy, error) {

	if changed, _, err := ExchangeNodePolicyChanged(pDevice, db, nodeGetPolicyHandler); err != nil {
		return nil, fmt.Errorf("Failed to check the exchange for the node policy: %v.", err)
	} else if changed {
		_, _, err = SyncNodePolicyWithExchange(db, pDevice, nodeGetPolicyHandler, nodePutPolicyHandler)
		if err != nil {
			return nil, fmt.Errorf("Failed to sync the local node policy with the exchange copy. %v", err)
		}
	}

	// get the local node policy
	localNodePolicy, err := persistence.FindNodePolicy(db)
	if err != nil {
		return nil, fmt.Errorf("Unable to read local node policy object. %v", err)
	}

	if propertyPatch, ok := patchObject.(externalpolicy.PropertyList); ok {
		localNodePolicy.Properties.MergeWith(&propertyPatch, true)
	} else if conastraintPatch, ok := patchObject.(externalpolicy.ConstraintExpression); ok {
		localNodePolicy.Constraints = conastraintPatch
	} else {
		return nil, fmt.Errorf("Unable to determine type of patch. %T %v", patchObject, patchObject)
	}

	// save it into the exchange and sync the local db with it.
	if _, err := nodePutPolicyHandler(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), &exchange.ExchangePolicy{ExternalPolicy: *localNodePolicy}); err != nil {
		return nil, fmt.Errorf("Unable to save node policy in exchange, error %v", err)
	} else if _, _, err := SyncNodePolicyWithExchange(db, pDevice, nodeGetPolicyHandler, nodePutPolicyHandler); err != nil {
		return nil, fmt.Errorf("Unable to sync the local db with the exchange node policy. %v", err)
	}

	return localNodePolicy, nil
}
