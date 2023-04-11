package exchangesync

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/version"
	"os"
	"sync"
)

var nodeUpdateLock sync.Mutex //The lock that protects the hash value

var exchNode *exchange.Device
var exchError error

const HZN_CONFIG_VERSION_ENV = "HZN_CONFIG_VERSION"

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
func NodeInitalSetup(db *bolt.DB, getDevice exchange.DeviceHandler, patchDevice exchange.PatchDeviceHandler) error {

	// get the node
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return fmt.Errorf("Unable to read node object from the local database. %v", err)
	} else if pDevice == nil {
		return fmt.Errorf("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.")
	}

	// get exchange node user input
	if _, err = SyncNodeWithExchange(db, pDevice, getDevice); err == nil {

		// set agent version, cert version and config version in exchange
		cert_version := ""
		config_version := ""

		mhCertPath := os.Getenv(config.ManagementHubCertPath)
		versionInCert := cutil.GetCertificateVersion(mhCertPath)
		if versionInCert != "" {
			cert_version = versionInCert
			pDevice.SetCertVersion(db, pDevice.Id, cert_version)
		}

		versionInConfig := os.Getenv(HZN_CONFIG_VERSION_ENV)
		if versionInConfig != "" {
			config_version = versionInConfig
			pDevice.SetConfigVersion(db, pDevice.Id, config_version)
		}

		if exchNode.SoftwareVersions == nil {
			exchNode.SoftwareVersions = make(map[string]string, 0)
		}

		if exchNode.SoftwareVersions[exchangecommon.HORIZON_VERSION] != version.HORIZON_VERSION ||
			exchNode.SoftwareVersions[exchangecommon.CERT_VERSION] != cert_version ||
			exchNode.SoftwareVersions[exchangecommon.CONFIG_VERSION] != config_version {
			versions := exchNode.SoftwareVersions
			versions[exchangecommon.HORIZON_VERSION] = version.HORIZON_VERSION
			versions[exchangecommon.CERT_VERSION] = cert_version
			versions[exchangecommon.CONFIG_VERSION] = config_version

			if err = patchDevice(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token, &exchange.PatchDeviceRequest{SoftwareVersions: versions}); err != nil {
				return fmt.Errorf("Unable to update the Exchange with correct node version. %v", err)
			}
		}

		if pDevice.Config.State == persistence.CONFIGSTATE_CONFIGURED {
			if pDevice.GetNodeType() == persistence.DEVICE_TYPE_CLUSTER && exchNode.ClusterNamespace != cutil.GetClusterNamespace() {
				// update exchange with correct clusterNamespace
				cluster_namespace := cutil.GetClusterNamespace()
				if err = patchDevice(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token, &exchange.PatchDeviceRequest{ClusterNamespace: &cluster_namespace}); err != nil {
					return fmt.Errorf("Unable to update the Exchange with correct node cluster namespace. %v", err)
				}

			} else if pDevice.GetNodeType() == persistence.DEVICE_TYPE_DEVICE && exchNode.ClusterNamespace != "" {
				// clear the cluster namespace for the exchange node if node type is device
				if err = patchDevice(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token, &exchange.PatchDeviceRequest{ClusterNamespace: nil}); err != nil {
					return fmt.Errorf("Unable to update the Exchange node with nil cluster namespace. %v", err)
				}
			}
		}
	}

	return err
}
