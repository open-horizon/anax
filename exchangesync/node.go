package exchangesync

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"sync"
)

var nodeUpdateLock sync.Mutex //The lock that protects the hash value

var exchNode *exchange.Device
var exchError error

// Return the currently saved exchange node
func GetExchangeNode() (*exchange.Device, error) {
	return exchNode, exchError
}

// Return the currently saved exchange node
func SetExchangeNode(device *exchange.Device) {
	nodeUpdateLock.Lock()
	defer nodeUpdateLock.Unlock()

	exchNode = device
	exchError = nil
}

// Get the node from the exchange and save it
func SyncNodeWithExchange(db *bolt.DB, pDevice *persistence.ExchangeDevice, getDevice exchange.DeviceHandler) (*exchange.Device, error) {

	glog.V(4).Infof("Checking the node changes.")

	// get the node user input from the exchange
	exchDevice, err := getDevice(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token)
	if err != nil {
		nodeUpdateLock.Lock()
		exchNode = nil
		exchError = err
		nodeUpdateLock.Unlock()
		return nil, fmt.Errorf("Failed to get the node %v/%v from the exchange. %v", pDevice.Org, pDevice.Id, err)
	} else {
		nodeUpdateLock.Lock()
		exchNode = exchDevice
		exchError = nil
		nodeUpdateLock.Unlock()
	}

	glog.V(4).Infof("Latest node on exchange is: %v", exchNode)
	return exchNode, nil
}

// Used one time when the local node is first registered
func NodeInitalSetup(db *bolt.DB, getDevice exchange.DeviceHandler) error {

	// get the node
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return fmt.Errorf("Unable to read node object from the local database. %v", err)
	} else if pDevice == nil {
		return fmt.Errorf("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.")
	}

	// get exchange node user input
	if _, err = SyncNodeWithExchange(db, pDevice, getDevice); err == nil {

		// set agent version in exchange
		if exchNode.SoftwareVersions["horizon"] != version.HORIZON_VERSION {
			versions := exchNode.SoftwareVersions
			versions["horizon"] = version.HORIZON_VERSION
			if err = patchDevice(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token, &exchange.PatchDeviceRequest{SoftwareVersions: versions}); err != nil {
				return fmt.Errorf("Unable to update the Exchange with correct node version. %v", err)
			}
		}
	}

	return err
}
