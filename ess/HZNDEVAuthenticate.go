package ess

import (
	"github.com/open-horizon/edge-sync-service/common"
	"github.com/open-horizon/edge-sync-service/core/security"
	"strings"
)

// HZNDEVAuthenticate is the Horizon hzn dev plugin for authentication used by the ESS and CSS sync service. This plugin
// only verifies identity format, there is no actual authentication performed.
type HZNDEVAuthenticate struct {
}

// Start initializes the HorizonAuthenticate plugin.
func (auth *HZNDEVAuthenticate) Start() {
	return
}

// Authenticate verifies that the incoming identity has one of the valid formats:
// 1) a 3 part '/' delimited node identity.
// 2) a 2 part '@' delimited user identity.
// 3) a 2 part '/' delimited service identity.
//
// Returns authentication result code, the user's org and id.
func (auth *HZNDEVAuthenticate) Authenticate(appKey, appSecret string) (int, string, string) {

	parts := strings.Split(appKey, "/")
	if len(parts) == 3 {
		return security.AuthEdgeNode, parts[0], parts[1] + "/" + parts[2]
	} else if len(parts) == 2 {
		return security.AuthAdmin, common.Configuration.OrgID, ""
	}

	parts = strings.Split(appKey, "@")
	if len(parts) == 2 {
		return security.AuthAdmin, parts[1], parts[0]
	}

	return security.AuthFailed, "", ""

}

// KeyandSecretForURL returns an app key and an app secret pair to be
// used by the ESS when communicating with the specified URL.
func (auth *HZNDEVAuthenticate) KeyandSecretForURL(url string) (string, string) {
	if strings.HasPrefix(url, common.HTTPCSSURL) {
		return common.Configuration.OrgID + "/" + common.Configuration.DestinationType + "/" +
			common.Configuration.DestinationID, ""
	}
	return "", ""
}