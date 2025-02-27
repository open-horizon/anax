package resource

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
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
	return fmt.Sprintf("Authentication Manager: "+
		"AuthPath: %v", a.AuthPath)
}

func (a *AuthenticationManager) GetCredentialPath(key string) string {
	return path.Join(a.AuthPath, key)
}

// Create a new container authentication credential and write it into the Agent's host file system. The credential file
// live in a directory named by the key input parameter.
func (a *AuthenticationManager) CreateCredential(key string, id string, ver string, secureCreds bool) (*AuthenticationCredential, error) {
	cred, err := GenerateNewCredential(id, ver)
	if err != nil {
		return nil, errors.New("unable to generate new authentication token")
	}

	fileName := path.Join(a.GetCredentialPath(key), config.HZN_FSS_AUTH_FILE)
	// For insecure creds (when secureCreds set to false), the group is not needed because the file permissions will be universal.

	// For secure creds, the auth folder and auth file should be protected and accessed by service account (root or non-root)
	// We configured ess auth and service container with the same group, so that the ess auth can only be accessed by the correct service.
	// This is achieved by:
	// 1. agent creates a group using the hash value of agreement id as group name
	// 2. agent sets the group created above as the group owner of ess auth folder/file on the host
	// 3. service container is started with the same group (passing in group id in docker HostConfig). This step is done in container.go
	var currUserUidInt, groupIdInt int
	var groupName string
	var group *user.Group
	var fileMode os.FileMode

	if secureCreds {
		currUser, err := user.Current()
		if err != nil {
			return nil, errors.New("unable to get current OS user")
		}
		currUserUidInt, err = strconv.Atoi(currUser.Uid)
		if err != nil {
			return nil, errors.New("unable to convert current user uid from string to int")
		}

		groupName = cutil.GetHashFromString(key)
		groupAddCmd := exec.Command("groupadd", "-f", groupName)

		var cmdErr bytes.Buffer
		groupAddCmd.Stderr = &cmdErr
		if err := groupAddCmd.Run(); err != nil {
			return nil, errors.New(fmt.Sprintf("failed to create group %v for key(agreementId) %v, error: %v, stderr: %v", groupName, key, err, cmdErr.String()))
		}

		// Verifying group is created
		group, err = user.LookupGroup(groupName)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("unable to find group %v created for auth file %v", groupName, fileName))
		}

		// get group id
		groupIdInt, err = strconv.Atoi(group.Gid)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to get group id %v as string, error: %v", group.Gid, err))
		}

		fileMode = 0750
	} else {
		// set auth file to 0755 if is "hzn dev"
		fileMode = 0755
	}

	if credBytes, err := json.Marshal(cred); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to marshal new authentication credential, error: %v", err))
	} else if err := os.MkdirAll(a.GetCredentialPath(key), fileMode); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to create directory path %v for authentication credential, error: %v", a.GetCredentialPath(key), err))
	} else if err := os.WriteFile(fileName, credBytes, fileMode); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to write authentication credential file %v, error: %v", fileName, err))
	}

	if secureCreds {
		// change group owner of auth foler and file, and set the group in groupAdd for service docker container in container.go
		if err = os.Chown(a.GetCredentialPath(key), currUserUidInt, groupIdInt); err != nil {
			return nil, errors.New(fmt.Sprintf("unable to change group to (group id: %v, group name:%v) for the authentication credential folder %v, error: %v", group.Gid, groupName, a.GetCredentialPath(key), err))
		}

		if err = os.Chown(fileName, currUserUidInt, groupIdInt); err != nil {
			return nil, errors.New(fmt.Sprintf("unable to change group to (group id: %v, group name:%v) for the authentication credential file %v, error: %v", group.Gid, groupName, fileName, err))
		}
	}

	glog.V(5).Infof(authLogString(fmt.Sprintf("Created credential for service %v, assigned id %v.", key, id)))

	return cred, nil
}

// Verify that the input credentials are in the auth manager.
func (a *AuthenticationManager) Authenticate(authId string, appSecret string) (bool, string, error) {

	// Iterate through the list of all directories in the auth manager. Each directory represents a running service
	// that has been assigned FSS (ESS) API credentials.
	if dirs, err := os.ReadDir(a.AuthPath); err != nil {
		return false, "", errors.New(fmt.Sprintf("unable to read authentication credential file directories in %v, error: %v", a.AuthPath, err))
	} else {
		for _, d := range dirs {

			// Skip the SSL cert and key path in the auth filesystem.
			if d.Name() == "SSL" {
				continue
			}

			// Demarshal the auth.json file and check to see if the id and pw contained within it matches the input authId and appSecret.
			if authObj, err := a.ReadCredFromAuthFile(d.Name()); err != nil {
				return false, "", err
			} else if authObj.Id == authId && authObj.Token == appSecret {
				glog.V(5).Infof(authLogString(fmt.Sprintf("Found valid credential for %v.", authId)))
				return true, authObj.Version, nil
			}
		}
	}

	return false, "", nil
}

// Remove a container authentication credential from the Agent's host file system.
func (a *AuthenticationManager) RemoveCredential(key string, secureCreds bool) (string, error) {
	// Read creds
	authObj, err := a.ReadCredFromAuthFile(key)
	if err != nil {
		return "", err
	}

	if err := os.RemoveAll(a.GetCredentialPath(key)); err != nil {
		return "", errors.New(fmt.Sprintf("unable to remove authentication credential file %v, error: %v", path.Join(a.GetCredentialPath(key), config.HZN_FSS_AUTH_FILE), err))
	}
	glog.V(5).Infof(authLogString(fmt.Sprintf("Removed credential for service %v.", key)))

	if secureCreds {
		groupName := cutil.GetHashFromString(key)
		if _, err := user.LookupGroup(groupName); err != nil {
			switch err.(type) {
			default:
				return "", errors.New(fmt.Sprintf("failed to look up group by group name: %v for file %v, error: %v", groupName, path.Join(a.GetCredentialPath(key), config.HZN_FSS_AUTH_FILE), err))
			case user.UnknownGroupIdError:
				glog.V(5).Infof(authLogString(fmt.Sprintf("Group name %v not exist for %v, skip group deletion", groupName, key)))
			}
		}

		var cmdErr bytes.Buffer
		groupDelCmd := exec.Command("groupdel", groupName)
		groupDelCmd.Stderr = &cmdErr
		if err := groupDelCmd.Run(); err != nil {
			return "", errors.New(fmt.Sprintf("failed to delete group %v for agreementId: %v, error: %v, stderr: %v", groupName, key, err, cmdErr.String()))
		}
		glog.V(5).Infof(authLogString(fmt.Sprintf("Removed group for service %v.", key)))
	}

	return authObj.Token, nil
}

// Remove all container authentication credentials from the Agent's host file system.
func (a *AuthenticationManager) RemoveAll(secureCreds bool) error {
	if dirs, err := os.ReadDir(a.AuthPath); err != nil {
		return errors.New(fmt.Sprintf("unable to remove all authentication credential files %v, error: %v", a.AuthPath, err))
	} else {
		for _, d := range dirs {
			if _, err := a.RemoveCredential(d.Name(), secureCreds); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *AuthenticationManager) ReadCredFromAuthFile(key string) (*AuthenticationCredential, error) {
	authFileName := path.Join(a.GetCredentialPath(key), config.HZN_FSS_AUTH_FILE)
	if authFile, err := os.Open(authFileName); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to open auth file %v, error: %v", authFileName, err))
	} else if bytes, err := io.ReadAll(authFile); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read auth file %v, error: %v", authFileName, err))
	} else {
		authObj := new(AuthenticationCredential)
		if err := json.Unmarshal(bytes, authObj); err != nil {
			return nil, errors.New(fmt.Sprintf("unable to demarshal auth file %v, error: %v", authFileName, err))
		} else {
			return authObj, nil
		}
	}
}

// Logging function
var authLogString = func(v interface{}) string {
	return fmt.Sprintf("Container Authentication Manager: %v", v)
}
