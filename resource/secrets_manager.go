package resource

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/persistence"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
)

type SecretsManager struct {
	SecretsStorePath string
	db               *bolt.DB
}

func NewSecretsManager(secFilePath string, database *bolt.DB) *SecretsManager {
	return &SecretsManager{SecretsStorePath: secFilePath, db: database}
}

// This is for updating service secrets. This assumes that the updated secrets are already updated in the agent db.
func (s SecretsManager) UpdateServiceSecrets(depDesc *containermessage.DeploymentDescription, agId string) error {
	if depDesc == nil || depDesc.Services == nil {
		return nil
	}
	for svcName, svc := range depDesc.Services {
		if svc != nil && svc.Secrets != nil {
			for secName, sec := range svc.Secrets {
				if s.db != nil {
					secretsFromDB, err := persistence.FindAllServiceSecretsWithSpecs(s.db, sec.SvcUrl, sec.SvcOrg, sec.SvcVersionRange)
					if err != nil {
						return err
					}
					for _, allSecretsForService := range secretsFromDB {
						if secret, ok := allSecretsForService.SecretsMap[secName]; ok {
							secretBytes, err := json.Marshal(secret)
							if err != nil {
								return err
							}
							err = WriteToFile(secretBytes, getSecretsKey(svcName, secName), path.Join(s.SecretsStorePath, agId, svcName, secName), path.Join(s.SecretsStorePath, agId, svcName))
							if err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// Remove the whole folder containing the secrets for the given agreement
func (s SecretsManager) RemoveSecretsFolderForAgreement(agId string) error {
	return os.RemoveAll(path.Join(s.SecretsStorePath, agId))
}

// This is for writing new service secrets to a file for the service container. This assumes that the secrets are already in the agent db.
func (s SecretsManager) WriteServiceSecretsToFile(svcOrgAndName string, agId string, containerId string) error {
	svcOrg, svcUrl := cutil.SplitOrgSpecUrl(svcOrgAndName)
	if allSec, err := persistence.FindAllServiceSecretsWithFilters(s.db, []persistence.SecFilter{persistence.UrlSecFilter(svcUrl), persistence.OrgSecFilter(svcOrg)}); err != nil {
		return err
	} else {
		for _, svcSecret := range allSec {
			for secName, sec := range svcSecret.SecretsMap {
				if cutil.SliceContains(sec.AgreementIds, agId) {
					if contentBytes, err := base64.StdEncoding.DecodeString(sec.SvcSecretValue); err != nil {
						return fmt.Errorf("Error decoding base64 encoded secret string: %v", err)
					} else if err = CreateAndWriteToFile(contentBytes, containerId, path.Join(s.SecretsStorePath, containerId, secName), path.Join(s.SecretsStorePath, containerId)); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func CreateAndWriteToFile(contents []byte, key string, fileName string, filePath string) error {
	// This way of creating a secured file is borrowed from the ess authentication manager. This will create a file that is accessible to the service container it belongs to but not other containers.
	// This is achieved by:
	// 1. agent creates a group using the hash value of agreement id as group name
	// 2. agent sets the group created above as the group owner of ess auth folder/file on the host
	// 3. service container is started with the same group (passing in group id in docker HostConfig). This step is done in container.go
	var currUserUidInt, groupIdInt int
	var groupName string
	var group *user.Group
	var fileMode os.FileMode

	currUser, err := user.Current()
	if err != nil {
		return errors.New("unable to get current OS user")
	}
	currUserUidInt, err = strconv.Atoi(currUser.Uid)
	if err != nil {
		return errors.New("unable to convert current user uid from string to int")
	}

	groupName = cutil.GetHashFromString(key)
	groupAddCmd := exec.Command("groupadd", "-f", groupName)

	var cmdErr bytes.Buffer
	groupAddCmd.Stderr = &cmdErr
	if err := groupAddCmd.Run(); err != nil {
		return errors.New(fmt.Sprintf("failed to create group %v for key(agreementId) %v, error: %v, stderr: %v", groupName, key, err, cmdErr.String()))
	}

	// Verifying group is created
	group, err = user.LookupGroup(groupName)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to find group %v created for auth file %v", groupName, fileName))
	}

	// get group id
	groupIdInt, err = strconv.Atoi(group.Gid)
	if err != nil {
		return errors.New(fmt.Sprintf("failed to get group id %v as string, error: %v", group.Gid, err))
	}

	fileMode = 0750

	if err := os.MkdirAll(filePath, fileMode); err != nil {
		return errors.New(fmt.Sprintf("unable to create directory path %v for authentication credential, error: %v", filePath, err))
	} else if err := ioutil.WriteFile(fileName, contents, fileMode); err != nil {
		return errors.New(fmt.Sprintf("unable to write authentication credential file %v, error: %v", fileName, err))
	}

	glog.V(5).Infof(secLogString(fmt.Sprintf("Wrote secret contents to file %v ", filePath)))

	// change group owner of secrets foler and file, and set the group in groupAdd for service docker container in container.go
	if err := os.Chown(filePath, currUserUidInt, groupIdInt); err != nil {
		return errors.New(fmt.Sprintf("unable to change group to (group id: %v, group name:%v) for the authentication credential folder %v, error: %v", group.Gid, groupName, filePath, err))
	}

	if err := os.Chown(fileName, currUserUidInt, groupIdInt); err != nil {
		return errors.New(fmt.Sprintf("unable to change group to (group id: %v, group name:%v) for the authentication credential file %v, error: %v", group.Gid, groupName, fileName, err))
	}

	return nil
}

// This is for updating a service secrets file that already exists
func WriteToFile(contents []byte, key string, fileName string, filePath string) error {
	var fileMode os.FileMode
	fileMode = 0750

	if err := os.MkdirAll(filePath, fileMode); err != nil {
		return errors.New(fmt.Sprintf("unable to create directory path %v for authentication credential, error: %v", filePath, err))
	} else if err := ioutil.WriteFile(fileName, contents, fileMode); err != nil {
		return errors.New(fmt.Sprintf("unable to write authentication credential file %v, error: %v", fileName, err))
	}

	glog.V(5).Infof(secLogString(fmt.Sprintf("Wrote secret contents to file %v ", filePath)))

	return nil
}

func (s SecretsManager) CleanUpServiceSecrets(svcOrg string, svcName string, svcVersionRange string, secName string) error {
	err := persistence.DeleteSecretsSpec(s.db, secName, svcOrg, svcName, svcVersionRange)
	if err != nil {
		glog.Errorf("Error removing secrets for service %s from the database: %s", getSecretsKey(svcName, secName), err)
	}
	return s.RemoveFile(getSecretsKey(svcName, secName))
}

func (s *SecretsManager) RemoveFile(key string) error {
	if err := os.RemoveAll(s.GetSecretsPath(key)); err != nil {
		return errors.New(fmt.Sprintf("unable to remove authentication credential file %v, error: %v", path.Join(s.GetSecretsPath(key), config.HZN_FSS_AUTH_FILE), err))
	}
	glog.V(5).Infof(secLogString(fmt.Sprintf("Removed credential for service %v.", key)))

	groupName := cutil.GetHashFromString(key)
	if _, err := user.LookupGroup(groupName); err != nil {
		switch err.(type) {
		default:
			return errors.New(fmt.Sprintf("failed to look up group by group name: %v for file %v, error: %v", groupName, path.Join(s.GetSecretsPath(key), config.HZN_FSS_AUTH_FILE), err))
		case user.UnknownGroupIdError:
			glog.V(5).Infof(secLogString(fmt.Sprintf("Group name %v not exist for %v, skip group deletion", groupName, key)))
		}
	}

	var cmdErr bytes.Buffer
	groupDelCmd := exec.Command("groupdel", groupName)
	groupDelCmd.Stderr = &cmdErr
	if err := groupDelCmd.Run(); err != nil {
		return errors.New(fmt.Sprintf("failed to delete group %v for agreementId: %v, error: %v, stderr: %v", groupName, key, err, cmdErr.String()))
	}
	glog.V(5).Infof(secLogString(fmt.Sprintf("Removed group for service %v.", key)))

	return nil
}

func getSecretsKey(svcName string, secName string) string {
	return fmt.Sprintf("%s/%s", svcName, secName)
}

func (s SecretsManager) GetSecretsPath(agId string) string {
	return path.Join(s.SecretsStorePath, agId)
}

var secLogString = func(v interface{}) string {
	return fmt.Sprintf("Secrets Manager: %v", v)
}
