package css

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/edge-sync-service/core/security"
	"strings"
)

// HorizonAuthenticate is the Horizon plugin for authentication used by the Cloud sync service. This plugin
// utilizes the exchange to authenticate users.
type HorizonAuthenticate struct {
}

// Start initializes the HorizonAuthenticate plugin.
func (auth *HorizonAuthenticate) Start() {
	glog.V(3).Infof(cssALS("Starting"))
	return
}

// Authenticate authenticates a particular appKey/appSecret pair and indicates
// whether it is an edge node, org admin, or plain user. Also returned is the
// user's org and identitity. An edge node's identity is orgID/destType/destID.
//
// Note: This Authenticate implementation is for production use with the Horizon
//      Agent. App keys for APIs are of the form, userID@orgID or
//      email@emailDomain@orgID. If the userID does not
//      appear there, it is assumed to be an admin for the specified org.
//      Edge node app keys are of the form orgID/destType/destID

// Returns authentication result code, the user's org and id.
func (auth *HorizonAuthenticate) Authenticate(appKey, appSecret string) (int, string, string) {

	glog.V(3).Infof(cssALS(fmt.Sprintf("Received authentication request for user %v", appKey)))
	glog.V(5).Infof(cssALS(fmt.Sprintf("Received authentication request for user %v with secret %v", appKey, appSecret)))

	// appKey will be either <org>/<pattern>/<id> - for a node identity,
	// or <id>@<org> for a real person user.

	authCode := security.AuthFailed
	authOrg := ""
	authId := ""

	// If the appKey is shaped like a node identity, let it through as a node.
	parts := strings.Split(appKey, "/")
	if len(parts) == 3 {
		authCode = security.AuthEdgeNode
		authOrg = parts[0]
		authId = parts[1] + "/" + parts[2]
	}

	// If the appKey is shaped like a user identity, let it through as an admin.
	parts = strings.Split(appKey, "@")
	if len(parts) == 2 {
		authCode = security.AuthAdmin
		authOrg = parts[1]
		authId = parts[0]
	}

	glog.V(3).Infof(cssALS(fmt.Sprintf("Returned authentication result code %v org %v id %v", authCode, authOrg, authId)))

	// Everything else gets rejected.
	return authCode, authOrg, authId
}

// KeyandSecretForURL returns an app key and an app secret pair to be
// used by the ESS when communicating with the specified URL.
func (auth *HorizonAuthenticate) KeyandSecretForURL(url string) (string, string) {
	// if strings.HasPrefix(url, common.HTTPCSSURL) {
	// 	return common.Configuration.OrgID + "/" + common.Configuration.DestinationType + "/" +
	// 		common.Configuration.DestinationID, ""
	// }
	return "", ""
}

// Logging function
var cssALS = func(v interface{}) string {
	return fmt.Sprintf("CSS: Authenticator %v", v)
}
