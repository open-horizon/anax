package container

import (
	"github.com/open-horizon/anax/cutil"
)

type AuthenticationCredential struct {
	Id string `json:"id"`
	Token string `json:"token"`
}

// When generating an authetication token, we can use the agreement id generation algorithm because it generates
// uniquely long strings which are sufficient to use as authentication tokens.
func GenerateNewCredential(id string) (*AuthenticationCredential, error) {

	generated, err := cutil.GenerateAgreementId()
	if err != nil {
		return nil, err
	}

	return &AuthenticationCredential{
		Id: id,
		Token: generated,
	}, nil
}