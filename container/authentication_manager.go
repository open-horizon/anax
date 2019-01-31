package container

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
)

type AuthenticationManager struct {
	AuthPath string
}

func NewAuthenticationManager(authPath string) *AuthenticationManager {
	return &AuthenticationManager{
		AuthPath: authPath,
	}
}

func (a AuthenticationManager) String() string {
	return fmt.Sprintf("Authentication Manager: " +
		"AuthPath: %v", a.AuthPath)
}

func (a *AuthenticationManager) GetCredentialPath(key string) string {
	return path.Join(a.AuthPath, key)
}

// Create a new container authentication credential and write it into the Agent's host file system.
func (a *AuthenticationManager) CreateCredential(key string, id string) error {
	cred, err := GenerateNewCredential(id)
	if err != nil {
		return errors.New("unable to generate new authentication token")
	}

	fileName := a.GetCredentialPath(key)

	if credBytes, err := json.Marshal(cred); err != nil {
		return errors.New(fmt.Sprintf("unable to marshal new authentication credential, error: %v", err))
	} else if err := ioutil.WriteFile(fileName, credBytes, 0600); err != nil {
		return errors.New(fmt.Sprintf("unable to write authentication credential file %v, error: %v", fileName, err))		
	}

	return nil
}

func (a *AuthenticationManager) RemoveCredential(key string) error {
	return nil
}

func (a *AuthenticationManager) RemoveAll(key string) error {
	return nil
}