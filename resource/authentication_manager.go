package resource

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"io/ioutil"
	"os"
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

// Create a new container authentication credential and write it into the Agent's host file system. The credential file
// live in a directory named by the key input parameter.
func (a *AuthenticationManager) CreateCredential(key string, id string) error {
	cred, err := GenerateNewCredential(id)
	if err != nil {
		return errors.New("unable to generate new authentication token")
	}

	fileName := path.Join(a.GetCredentialPath(key), config.HZN_FSS_AUTH_FILE)

	if credBytes, err := json.Marshal(cred); err != nil {
		return errors.New(fmt.Sprintf("unable to marshal new authentication credential, error: %v", err))
	} else if err := os.MkdirAll(a.GetCredentialPath(key), 0700); err != nil {
		return errors.New(fmt.Sprintf("unable to create directory path %v for authentication credential, error: %v", a.GetCredentialPath(key), err))
	} else if err := ioutil.WriteFile(fileName, credBytes, 0700); err != nil {
		return errors.New(fmt.Sprintf("unable to write authentication credential file %v, error: %v", fileName, err))		
	}

	glog.V(5).Infof(authLogString(fmt.Sprintf("Created credential for service %v, assigned id %v.", key, id)))

	return nil
}

// Verify that the input credentials are in the auth manager.
func (a *AuthenticationManager) Authenticate(authId string, appSecret string) (bool, error) {

	// Iterate through the list of all directories in the auth manager. Each directory represents a running service
	// that has been assigned FSS (ESS) API credentials.
	if dirs, err := ioutil.ReadDir(a.AuthPath); err != nil {
		return false, errors.New(fmt.Sprintf("unable to read authentication credential file directories in %v, error: %v", a.AuthPath, err))
	} else {
		for _, d := range dirs {

			// Skip the SSL cert and key path in the auth filesystem.
			if d.Name() == "SSL" {
				continue
			}

			// Demarshal the auth.json file and check to see if the id and pw contained within it matches the input authId and appSecret.
			authFileName := path.Join(a.GetCredentialPath(d.Name()), config.HZN_FSS_AUTH_FILE)
			if authFile, err := os.Open(authFileName); err != nil {
				return false, errors.New(fmt.Sprintf("unable to open auth file %v, error: %v", authFileName, err))
			} else if bytes, err := ioutil.ReadAll(authFile); err != nil {
				return false, errors.New(fmt.Sprintf("unable to read auth file %v, error: %v", authFileName, err))
			} else {
				authObj := new(AuthenticationCredential)
				if err := json.Unmarshal(bytes, authObj); err != nil {
					return false, errors.New(fmt.Sprintf("unable to demarshal auth file %v, error: %v", authFileName, err))
				} else if authObj.Id == authId && authObj.Token == appSecret {
					glog.V(5).Infof(authLogString(fmt.Sprintf("Found valid credential for %v.", authId)))
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// Remove a container authentication credential from the Agent's host file system.
func (a *AuthenticationManager) RemoveCredential(key string) error {
	if err := os.RemoveAll(a.GetCredentialPath(key)); err != nil {
		return errors.New(fmt.Sprintf("unable to remove authentication credential file %v, error: %v", path.Join(a.GetCredentialPath(key), config.HZN_FSS_AUTH_FILE), err))
	}
	glog.V(5).Infof(authLogString(fmt.Sprintf("Removed credential for service %v.", key)))
	return nil
}

// Remove all container authentication credentials from the Agent's host file system.
func (a *AuthenticationManager) RemoveAll() error {
	if dirs, err := ioutil.ReadDir(a.AuthPath); err != nil {
		return errors.New(fmt.Sprintf("unable to remove all authentication credential files %v, error: %v", a.AuthPath, err))
	} else {
		for _, d := range dirs {
			if err := a.RemoveCredential(d.Name()); err != nil {
				return err
			}
		}
	}

	return nil
}

// Logging function
var authLogString = func(v interface{}) string {
	return fmt.Sprintf("Container Authentication Manager: %v", v)
}