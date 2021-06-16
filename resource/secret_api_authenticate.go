package resource

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"net/http"
	"strings"
)

type SecretsAPIAuthenticate struct {
	AuthMgr *AuthenticationManager
}

// return bool, serviceURL, error
func (auth *SecretsAPIAuthenticate) Authenticate(request *http.Request) (bool, string, error) {
	if request == nil {
		glog.Errorf(secretsALS(fmt.Sprintf("called with a nil HTTP request")))
		return false, "", errors.New(fmt.Sprintf("Unable to authenticate nil request for secrets API"))
	}

	if appKey, appSecret, ok := request.BasicAuth(); !ok {
		glog.Errorf(secretsALS(fmt.Sprintf("failed to extract basic auth for request")))
		return false, "", errors.New(fmt.Sprintf("Unable to extract basic auth for request"))
	} else if ok, _, err := auth.AuthMgr.Authenticate(appKey, appSecret); err != nil {
		glog.Errorf(secretsALS(fmt.Sprintf("unable to verify %v, error %v", appKey, err)))
		return false, "", errors.New(fmt.Sprintf("unable to verify %v, error %v", appKey, err))
	} else if !ok {
		glog.Errorf(secretsALS(fmt.Sprintf("credentials for %v are not valid", appKey)))
		return false, "", errors.New(fmt.Sprintf("credentials for %v are not valid", appKey))
	} else if _, serviceURL, err := extractAppKey(appKey); err != nil {
		glog.Errorf(secretsALS(fmt.Sprintf("failed to extract serviceOrg and serviceURL from appKey Error: %v", err)))
		return false, "", errors.New(fmt.Sprintf(err.Error()))
	} else {
		return true, serviceURL, nil
	}

}

// returns serviceOrg, serviceURL, error from the appKey
// appKey format: {serviceOrg}/{serviceURL}
func extractAppKey(appKey string) (string, string, error) {
	parts := strings.Split(appKey, "/")
	if len(parts) != 2 {
		return "", "", errors.New(fmt.Sprintf("appKey %s is not in the correct format {serviceOrg}/{serviceURL}", appKey))
	}

	return parts[0], parts[1], nil
}

// Logging function
var secretsALS = func(v interface{}) string {
	return fmt.Sprintf("Secrets API : Authenticator %v", v)
}

