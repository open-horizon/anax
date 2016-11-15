package api

import (
	"github.com/golang/glog"
	"net"
	"time"
)

var HORIZON_SERVERS = [...]string{"firmware.bluehorizon.network", "images.bluehorizon.network"}

// Check if the device has internect connection to the given host or not.
func checkConnectivity(host string) error {
	var err error
	for i := 0; i < 3; i++ {
		_, err = net.LookupHost(host)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}

// Writes the server connectivity info int the Info strucure.
// It is used for /info api
func WriteConnectionStatus(info *Info) error {
	connect := make(map[string]bool, 0)
	for _, host := range HORIZON_SERVERS {
		if err := checkConnectivity(host); err != nil {
			glog.Infof("Error checking connectivity for %s: %v", host, err)
			connect[host] = false
		} else {
			connect[host] = true
		}
	}

	info.Connectivity = connect
	return nil
}
