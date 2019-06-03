package resource

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/edge-sync-service/common"
	"github.com/open-horizon/edge-sync-service/core/security"
	"net/http"
	"strings"
)

// FSSAuthenticate is the plugin for authenticating FSS (ESS) API calls from a service to anax.
// It implements the security.Authentication interface. It is also called by the embedded ESS to
// provide credentials for the node to access the CSS (over the internal SPI).
type FSSAuthenticate struct {
	nodeOrg   string
	nodeID    string
	nodeToken string
	AuthMgr   *AuthenticationManager
}

// Start initializes the HorizonAuthenticate plugin.
func (auth *FSSAuthenticate) Start() {
	glog.V(3).Infof(essALS("Starting"))
}

// FSSAuthenticate authenticates a particular appKey/appSecret pair and indicates
// whether it is an edge service or not. Also returned is the user's org and identitity.
// An edge service's identity is <service-org>/<service-name>.
//
// Returns authentication result code, the user's org and id.
func (auth *FSSAuthenticate) Authenticate(request *http.Request) (int, string, string) {

	if request == nil {
		glog.Errorf(essALS(fmt.Sprintf("called with a nil HTTP request")))
		return security.AuthFailed, "", ""
	}

	appKey, appSecret, ok := request.BasicAuth()
	if !ok {
		glog.Errorf(essALS(fmt.Sprintf("unable to extract basic auth information")))
		return security.AuthFailed, "", ""
	}

	glog.V(3).Infof(essALS(fmt.Sprintf("received authentication request for user %v", appKey)))
	glog.V(6).Infof(essALS(fmt.Sprintf("received authentication request for user %v with secret %v", appKey, appSecret)))

	// appKey will be <service-org>/<service-name> indicating a service running on this node.
	authCode := security.AuthFailed
	authId := appKey

	// Verify that this identity is still in use. If there is an error, log it and return not authenticated.
	if ok, err := auth.AuthMgr.Authenticate(authId, appSecret); err != nil {
		glog.Errorf(essALS(fmt.Sprintf("unable to verify %v, error %v", authId, err)))
		return authCode, "", ""
	} else if !ok {
		glog.Errorf(essALS(fmt.Sprintf("credentials for %v are not valid", authId)))
		return authCode, "", ""
	}

	// The service identity is authenticated.
	authCode = security.AuthService
	glog.V(3).Infof(essALS(fmt.Sprintf("returned authentication result code %v org %v id %v", authCode, auth.nodeOrg, authId)))

	return authCode, auth.nodeOrg, authId
}

// KeyandSecretForURL returns an app key and an app secret pair to be used by the ESS when communicating
// with the specified URL. For ESS to CSS SPI communication, the node id and token is used.
func (auth *FSSAuthenticate) KeyandSecretForURL(url string) (string, string) {

	glog.V(6).Infof(essALS(fmt.Sprintf("received request for URL %v credentials", url)))

	if strings.HasPrefix(url, common.HTTPCSSURL) {
		id := common.Configuration.OrgID + "/" + common.Configuration.DestinationType + "/" + common.Configuration.DestinationID
		glog.V(6).Infof(essALS(fmt.Sprintf("returning credentials %v %v", id, auth.nodeToken)))
		return id, auth.nodeToken
	}

	return "", ""
}

// Logging function
var essALS = func(v interface{}) string {
	return fmt.Sprintf("ESS: Authenticator %v", v)
}
