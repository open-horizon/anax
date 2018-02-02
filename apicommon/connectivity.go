package apicommon

import (
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
)

var HORIZON_SERVERS = [...]string{"firmware.bluehorizon.network", "images.bluehorizon.network"}

// Writes the server connectivity info int the Info strucure.
// It is used for /info api
func WriteConnectionStatus(info *Info) error {
	connect := make(map[string]bool, 0)
	for _, host := range HORIZON_SERVERS {
		if err := cutil.CheckConnectivity(host); err != nil {
			glog.Infof("Error checking connectivity for %s: %v", host, err)
			connect[host] = false
		} else {
			connect[host] = true
		}
	}

	info.Connectivity = connect
	return nil
}
