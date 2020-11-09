package ess

import (
	"github.com/open-horizon/edge-sync-service/common"
	"github.com/open-horizon/edge-sync-service/core/security"
	"net/http"
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

// Authenticate verifies that the incoming identity is a node, service or a user:
// 1) a 3 part '/' delimited node identity.
// 2) everything else is a service or user identity.
//
// Returns authentication result code, the user's org and id.
func (auth *HZNDEVAuthenticate) Authenticate(request *http.Request) (int, string, string) {

	if request == nil {
		return security.AuthFailed, "", ""
	}

	appKey, _, ok := request.BasicAuth()
	if !ok {
		return security.AuthFailed, "", ""
	}

	parts := strings.Split(appKey, "/")
	if len(parts) == 3 {
		return security.AuthEdgeNode, parts[0], parts[1] + "/" + parts[2]
	} else if len(parts) == 2 {
		if common.Configuration.NodeType == common.ESS {
			return security.AuthAdmin, common.Configuration.OrgID, parts[1]
		} else {
			return security.AuthAdmin, parts[0], parts[1]
		}
	} else if parts := strings.Split(appKey, "@"); len(parts) == 2 {
		// legacy compensation, can be removed during beta
		return security.AuthAdmin, parts[1], parts[0]
	}

	return security.AuthFailed, "", ""

}

// KeyandSecretForURL returns an app key and an app secret pair to be
// used by the ESS when communicating with the specified URL.
func (auth *HZNDEVAuthenticate) KeyandSecretForURL(url string) (string, string) {
	if strings.HasPrefix(url, common.HTTPCSSURL) {
		return common.Configuration.OrgID + "/" + common.Configuration.DestinationType + "/" + common.Configuration.DestinationID, ""
	}
	return "", ""
}
