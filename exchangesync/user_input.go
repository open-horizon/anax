package exchangesync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"golang.org/x/crypto/sha3"
	"reflect"
	"sync"
)

var nodeUserInputUpdateLock sync.Mutex //The lock that protects the hash value

// Gets all the UserInputAttriutues from the DB and convert then into
func SyncLocalUserInputWithExchange(db *bolt.DB, pDevice *persistence.ExchangeDevice, getDevice exchange.DeviceHandler) (bool, persistence.ServiceSpecs, error) {

	glog.V(4).Infof("Checking the node user input changes.")

	nodeUserInputUpdateLock.Lock()
	defer nodeUserInputUpdateLock.Unlock()

	// get the node user input from the exchange
	exchDevice, err := getDevice(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token)
	if err != nil {
		return false, nil, fmt.Errorf("Failed to get the device %v/%v from the exchange. %v", pDevice.Org, pDevice.Id, err)
	}

	// safeguard for nil
	if exchDevice.UserInput == nil {
		exchDevice.UserInput = []policy.UserInput{}
	}

	// create a hash for the user input
	exchHash, err := HashUserInput(exchDevice.UserInput)
	if err != nil {
		return false, nil, fmt.Errorf("Failed to hash the UserInput. %v", err)
	}

	// get the saved hash
	savedHash, err := persistence.GetNodeUserInputHash_Exch(db)
	if err != nil {
		return false, nil, fmt.Errorf("Failed to get the saved user input hash from the local db. %v", err)
	}

	// if the 2 hashes are the same, then do nothing.
	// otherwise, replace all UserInputAttributes with the UserInput from the exchange.
	if bytes.Equal(exchHash, savedHash) {
		return false, nil, nil
	} else {
		oldUserInput, err := persistence.FindNodeUserInput(db)
		if err != nil {
			return true, nil, fmt.Errorf("Failed get user input from local db. %v", err)
		}

		// save exchange node user input to local db
		if err := persistence.SaveNodeUserInput(db, exchDevice.UserInput); err != nil {
			return true, nil, fmt.Errorf("Failed save user input %v to local db. %v", exchDevice.UserInput, err)
		}

		// update the hash
		if err := persistence.SaveNodeUserInputHash_Exch(db, exchHash); err != nil {
			return true, nil, fmt.Errorf("Failed to save the user input hash %v to the local db. %v", exchHash, err)
		}

		glog.V(3).Infof("Node synced with exchange. New node user input is: %v", exchDevice.UserInput)

		// Get a list of what services has been changed
		changedServiceSpecs := GetChangedServices(oldUserInput, exchDevice.UserInput)

		return true, changedServiceSpecs, nil
	}
}

func HashUserInput(ui []policy.UserInput) ([]byte, error) {
	if mashled_ui, err := json.Marshal(ui); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to marshal user input %v to a string, error %v", ui, err))
	} else {
		hash := sha3.Sum256([]byte(mashled_ui))
		return hash[:], nil
	}
}

// For backward compatibility. If the exchange node user input is not set and local nodes has UserInputAttributes,
// convert all UserInputAttributes to UserInput format and save it locally and on the exchange.
// If the exchange has node user input for this node, sync it to the local node.
// All UserInputAttributes will be removed.
// Exchange is the master.
func NodeUserInputInitalSetup(db *bolt.DB,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler) error {

	glog.V(3).Infof("Node user input initial setup.")

	// get the node
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return fmt.Errorf("Unable to read node object from the local database. %v", err)
	} else if pDevice == nil {
		return fmt.Errorf("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.")
	}

	// get exchange node user input
	exchDevice, err := getDevice(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token)
	if err != nil {
		return fmt.Errorf("Failed to get the device %v/%v from the exchange. %v", pDevice.Org, pDevice.Id, err)
	}

	// exchange does not have user input, then check if there are UserInputAttributes attributes for backward comaptibility
	if exchDevice.UserInput == nil || len(exchDevice.UserInput) == 0 {
		attributes, err := persistence.GetAllUserInputAttributes(db)
		if err != nil {
			return fmt.Errorf("Error getting all UserInputAttributes from local node. %v", err)
		}
		if attributes != nil && len(attributes) != 0 {
			convertedUI := ConvertAttributesToUserInput(attributes)

			if convertedUI != nil && len(convertedUI) > 0 {
				pdr := exchange.PatchDeviceRequest{}
				pdr.UserInput = &convertedUI

				glog.V(3).Infof("Saving the converted user input to the exchange: %v.", pdr)
				if err := patchDevice(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token, &pdr); err != nil {
					return err
				}
			}
		}
	}

	// now exchange is the master
	if _, _, err := SyncLocalUserInputWithExchange(db, pDevice, getDevice); err != nil {
		return fmt.Errorf("Failed to sync the local user input with the exchange for node %v/%v. %v", pDevice.Org, pDevice.Id, err)
	}

	// remove all UserInputAttributes from local db which was the old way of saving user input.
	if err := persistence.DeleteAllUserInputAttributes(db); err != nil {
		return fmt.Errorf("Error deleting all UserInputAttributes from local node. %v", err)
	}

	return nil
}

// check if the node user input has been changed from last sync.
func ExchangeNodeUserInputChanged(pDevice *persistence.ExchangeDevice, db *bolt.DB, getDevice exchange.DeviceHandler) (bool, []policy.UserInput, error) {

	// get the node user input from the exchange
	exchDevice, err := getDevice(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token)
	if err != nil {
		return false, nil, fmt.Errorf("Failed to get the device %v/%v from the exchange. %v", pDevice.Org, pDevice.Id, err)
	}

	// safeguard for nil
	if exchDevice.UserInput == nil {
		exchDevice.UserInput = []policy.UserInput{}
	}

	// create a hash for the user input
	exchHash, err := HashUserInput(exchDevice.UserInput)
	if err != nil {
		return false, nil, fmt.Errorf("Failed to hash the UserInput. %v", err)
	}

	// get the saved hash
	savedHash, err := persistence.GetNodeUserInputHash_Exch(db)
	if err != nil {
		return false, nil, fmt.Errorf("Failed to get the saved user input hash from the local db. %v", err)
	}

	if bytes.Equal(exchHash, savedHash) {
		return false, exchDevice.UserInput, nil
	} else {
		return true, exchDevice.UserInput, nil
	}
}

func ConvertAttributesToUserInput(attributes []persistence.UserInputAttributes) []policy.UserInput {
	userInput := []policy.UserInput{}

	if attributes == nil || len(attributes) == 0 {
		return userInput
	}

	for _, attr := range attributes {
		var ui policy.UserInput
		//ignore the ones with no mappings
		if len(attr.Mappings) == 0 {
			continue
		}

		// ignore the ones without service url or org specified
		if attr.ServiceSpecs == nil || len(*attr.ServiceSpecs) == 0 {
			continue
		}

		ui.ServiceUrl = (*attr.ServiceSpecs)[0].Url
		ui.ServiceOrgid = (*attr.ServiceSpecs)[0].Url

		// convert mappings
		m := []policy.Input{}
		for k, v := range attr.Mappings {
			m = append(m, policy.Input{Name: k, Value: v})
		}
		ui.Inputs = m

		userInput = append(userInput, ui)
	}

	return userInput
}

func ConvertUserInputToAttributes(userInput []policy.UserInput) []persistence.Attribute {
	attributes := make([]persistence.Attribute, 0)
	if userInput == nil || len(userInput) == 0 {
		return attributes
	}

	for _, ui := range userInput {
		// if the user input does not have variables, skip it.
		if ui.Inputs == nil || len(ui.Inputs) == 0 {
			continue
		}
		publishable := true
		hostonly := false
		a_meta := persistence.AttributeMeta{
			Type:        "UserInputAttributes",
			Label:       "",
			Publishable: &publishable,
			HostOnly:    &hostonly,
		}
		a_svc := new(persistence.ServiceSpecs)
		a_svc.AppendServiceSpec(persistence.ServiceSpec{Url: ui.ServiceUrl, Org: ui.ServiceOrgid})
		a_mapping := map[string]interface{}{}
		for _, item := range ui.Inputs {
			a_mapping[item.Name] = item.Value
		}

		a := persistence.UserInputAttributes{
			Meta:         &a_meta,
			ServiceSpecs: a_svc,
			Mappings:     a_mapping,
		}
		attributes = append(attributes, a)
	}

	return attributes
}

// Compare old and new user inputs and get a list of services that have been updated, added or deleted.
func GetChangedServices(oldUserInput, newUserInput []policy.UserInput) persistence.ServiceSpecs {
	changedSvcs := new(persistence.ServiceSpecs)
	comparedIndexes := make(map[int]int, 0)

	// get the changed and deleted ones
	for _, oldUi := range oldUserInput {
		found := false
		for i_new, newUi := range newUserInput {
			if oldUi.ServiceUrl == newUi.ServiceUrl && oldUi.ServiceOrgid == newUi.ServiceOrgid {
				comparedIndexes[i_new] = 1
				if reflect.DeepEqual(newUi, oldUi) {
					found = true
					break
				}
			}
		}
		if !found {
			changedSvcs.AppendServiceSpec(persistence.ServiceSpec{Url: oldUi.ServiceUrl, Org: oldUi.ServiceOrgid})
		}
	}

	// get the added oned
	for i_new, newUi := range newUserInput {
		if _, ok := comparedIndexes[i_new]; !ok {
			changedSvcs.AppendServiceSpec(persistence.ServiceSpec{Url: newUi.ServiceUrl, Org: newUi.ServiceOrgid})
		}
	}

	return *changedSvcs
}

// Delete the node user input from local db and the exchange
func DeleteNodeUserInput(pDevice *persistence.ExchangeDevice, db *bolt.DB,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler) error {

	// check if the user input got changed on the exchange since last observation
	// if it is changed, then reject the deletion.
	changed, _, err := ExchangeNodeUserInputChanged(pDevice, db, getDevice)
	if err != nil {
		return fmt.Errorf("Failed to check the exchange for the node user input: %v.", err)
	} else if changed {
		return fmt.Errorf("Cannot delete this node user input because the local node user input is out of sync with the exchange copy. Please wait a minute and try again.")
	}

	// delete the node policy from the exchange if it exists
	userInputs := []policy.UserInput{}
	return SaveNodeUserInput(pDevice, db, userInputs, getDevice, patchDevice)
}

// Update the node user input on local db and the exchange
func UpdateNodeUserInput(pDevice *persistence.ExchangeDevice, db *bolt.DB,
	userInputs []policy.UserInput,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler) (persistence.ServiceSpecs, error) {

	// check if the user input is changed or not on the exchange since last observation,
	// if it is changed, then reject the patch.
	changed, exchUserInput, err := ExchangeNodeUserInputChanged(pDevice, db, getDevice)
	if err != nil {
		return nil, fmt.Errorf("Failed to check the exchange for the node user input: %v.", err)
	} else if changed {
		return nil, fmt.Errorf("Cannot accept this node user input because the local node user input is out of sync with the exchange copy. Please wait a minute and try again.")
	}

	if err := SaveNodeUserInput(pDevice, db, userInputs, getDevice, patchDevice); err != nil {
		return nil, fmt.Errorf("Failed to save the user input %v. %v.", userInputs, err)
	}

	// Get a list of what services has been changed
	changedServiceSpecs := GetChangedServices(exchUserInput, userInputs)

	return changedServiceSpecs, nil
}

// Add the given user input to the exchange node user input.
func PatchNodeUserInput(pDevice *persistence.ExchangeDevice, db *bolt.DB,
	userInputs []policy.UserInput,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler) error {

	if userInputs == nil || len(userInputs) == 0 {
		return nil
	}

	// check if the user input is changed or not on the exchange since last observation,
	// if it is changed, then reject the patch.
	changed, exchUserInput, err := ExchangeNodeUserInputChanged(pDevice, db, getDevice)
	if err != nil {
		return fmt.Errorf("Failed to check the exchange for the node user input: %v.", err)
	} else if changed {
		return fmt.Errorf("Cannot accept this node user input because the local node user input is out of sync with the exchange copy. Please wait a minute and try again.")
	}

	// patch the exchange userinput with the newly added one on the node
	new_ui := policy.MergeUserInputArrays(exchUserInput, userInputs, false)

	return SaveNodeUserInput(pDevice, db, new_ui, getDevice, patchDevice)
}

// This fuction saves the given user input into exchange, and local db
func SaveNodeUserInput(pDevice *persistence.ExchangeDevice, db *bolt.DB,
	userInputs []policy.UserInput,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler) error {

	nodeUserInputUpdateLock.Lock()
	defer nodeUserInputUpdateLock.Unlock()

	if userInputs == nil {
		userInputs = []policy.UserInput{}
	}

	pdr := exchange.PatchDeviceRequest{}
	pdr.UserInput = &userInputs

	glog.V(3).Infof("Updating exchange with new user input: %v.", pdr)
	if err := patchDevice(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token, &pdr); err != nil {
		return err
	}

	glog.V(3).Infof("Updating local db with new user input.")
	// save exchange node user input to local db
	if err := persistence.SaveNodeUserInput(db, userInputs); err != nil {
		return fmt.Errorf("Failed save user input %v to local db. %v", userInputs, err)
	}

	// get the node user input from the exchange do that we can get an accurate hash
	newExchDevice, err := getDevice(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token)
	if err != nil {
		return fmt.Errorf("Failed to get the device %v/%v from the exchange. %v", pDevice.Org, pDevice.Id, err)
	}

	// save the hash for later comparison
	if hash, err := HashUserInput(newExchDevice.UserInput); err != nil {
		return fmt.Errorf("Failed to hash the user input. %v", err)
	} else if err := persistence.SaveNodeUserInputHash_Exch(db, hash); err != nil {
		return err
	}

	return nil
}
