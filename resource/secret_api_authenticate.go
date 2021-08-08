package resource

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/i18n"
	"net/http"
)

type SecretsAPIAuthenticate struct {
	AuthMgr *AuthenticationManager
}

// return bool, token error
func (auth *SecretsAPIAuthenticate) Authenticate(request *http.Request) (bool, string, error) {
	// get message printer, this function is called by CLI
	msgPrinter := i18n.GetMessagePrinter()

	if request == nil {
		glog.Errorf(secretsALS(fmt.Sprintf("called with a nil HTTP request")))
		return false, "", errors.New(msgPrinter.Sprintf("Unable to authenticate nil request for secrets API"))
	}

	// appKey: e2edev@somecomp.com/my.company.com.services.usehello2
	// appSecret: 5c7319c8aaa03362347ed04b559d54d1938815958d445a4bcc211adce89df2e9
	if appKey, appSecret, ok := request.BasicAuth(); !ok {
		glog.Errorf(secretsALS(fmt.Sprintf("failed to extract basic auth for request")))
		return false, "", errors.New(msgPrinter.Sprintf("Unable to extract basic auth for request"))
	} else if ok, _, err := auth.AuthMgr.Authenticate(appKey, appSecret); err != nil {
		glog.Errorf(secretsALS(fmt.Sprintf("unable to verify %v, error %v", appKey, err)))
		return false, "", errors.New(msgPrinter.Sprintf("unable to verify %v, error %v", appKey, err))
	} else if !ok {
		glog.Errorf(secretsALS(fmt.Sprintf("credentials for %v are not valid", appKey)))
		return false, "", errors.New(msgPrinter.Sprintf("credentials for %v are not valid", appKey))
	} else {
		return true, appSecret, nil
	}
}

// Logging function
var secretsALS = func(v interface{}) string {
	return fmt.Sprintf("Secrets API : Authenticator %v", v)
}
