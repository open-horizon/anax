package resource

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/semanticversion"
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

func (s SecretsManager) ProcessServiceSecretsWithInstanceId(agId string, msInstKey string) error {
	if s.db == nil {
		return nil
	}
	if msIntf, err := persistence.GetMicroserviceInstIWithKey(s.db, msInstKey); err != nil {
		return fmt.Errorf(secLogString(fmt.Sprintf("Failed to get microservice instance interface for: %v. Error was: %v", msInstKey, err)))
	} else if err = s.SaveMicroserviceInstanceSecretsFromAgreementSecrets(agId, msIntf); err != nil {
		return fmt.Errorf(secLogString(fmt.Sprintf("Failed to convert secrets for agreement %v to persistent microservice form: %v", agId, err)))
	} else if err = s.WriteNewServiceSecretsToFile(msInstKey); err != nil {
		return fmt.Errorf(secLogString(fmt.Sprintf("Failed to write all secrets for agreement %v to file: %v", agId, err)))
	}
	return nil
}

func (s SecretsManager) ProcessServiceSecretUpdates(agId string, updatedSecList []persistence.PersistedServiceSecret) error {
	for _, updatedSec := range updatedSecList {
		existingSvcSecList, err := persistence.FindAllServiceSecretsWithSpecs(s.db, updatedSec.SvcUrl, updatedSec.SvcOrgid)
		if err != nil {
			return err
		}
		for _, existingSvcSec := range existingSvcSecList {
			if existingSec, ok := existingSvcSec.SecretsMap[updatedSec.SvcSecretName]; ok {
				if cutil.SliceContains(existingSec.AgreementIds, agId) {
					if err := persistence.SaveSecret(s.db, updatedSec.SvcSecretName, existingSvcSec.MsInstKey, existingSvcSec.MsInstVers, &updatedSec); err != nil {
						return err
					} else if err := s.WriteExistingServiceSecretsToFile(existingSvcSec.MsInstKey, updatedSec); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (s SecretsManager) FindSecretsMatchingMsInst(allSecrets *[]persistence.PersistedServiceSecret, msInst persistence.MicroserviceInstInterface) (*[]persistence.PersistedServiceSecret, error) {
	matchingSecrets := []persistence.PersistedServiceSecret{}
	instOrg := msInst.GetOrg()
	instUrl := msInst.GetURL()
	instVers := msInst.GetVersion()

	for _, sec := range *allSecrets {
		if semVersRange, err := semanticversion.Version_Expression_Factory(sec.SvcVersionRange); err != nil {
			return nil, err
		} else if match, err := semVersRange.Is_within_range(instVers); err != nil {
			return nil, err
		} else if !match {
			continue
		}

		if sec.SvcOrgid == instOrg && sec.SvcUrl == instUrl {
			matchingSecrets = append(matchingSecrets, sec)
		}
	}
	return &matchingSecrets, nil
}

// This will save the secrets from an agreement with their associated microservice instance secrets
func (s SecretsManager) SaveMicroserviceInstanceSecretsFromAgreementSecrets(agId string, msInst persistence.MicroserviceInstInterface) error {
	if agAllSecretsList, err := persistence.FindAgreementSecrets(s.db, agId); err != nil {
		return err
	} else if agSecretsList, err := s.FindSecretsMatchingMsInst(agAllSecretsList, msInst); err != nil {
		return err
	} else if existingInstSvcSec, err := persistence.FindAllSecretsForMS(s.db, msInst.GetKey()); err != nil {
		return err
	} else if existingInstSvcSec != nil {
		// This ms inst already exists. This must be a singleton service.
		// If there are any conflicting secret details, log an error and ignore the new details.
		// Otherwise copy the agreement secrets to the existing record.
		for _, agSecret := range *agSecretsList {
			if existingSvcSec, ok := existingInstSvcSec.SecretsMap[agSecret.SvcSecretName]; ok && existingSvcSec.SvcSecretValue != agSecret.SvcSecretValue {
				// New secret info and existing info do not match. This is an error. Log and ignore new secret details.
				glog.Errorf(secLogString(fmt.Sprintf("Conflicting service secret details for secret %v for service instance %v. Proceding with original secret detail.", agSecret.SvcSecretName, msInst.GetKey())))
				existingSvcSec.AgreementIds = append(existingSvcSec.AgreementIds, agId)
				existingInstSvcSec.SecretsMap[agSecret.SvcSecretName] = existingSvcSec
				err = persistence.SaveAllSecretsForService(s.db, msInst.GetKey(), existingInstSvcSec)
			} else if err = persistence.SaveSecret(s.db, agSecret.SvcSecretName, msInst.GetKey(), msInst.GetVersion(), &agSecret); err != nil {
				return err
			}
		}
	} else {
		for _, agSecret := range *agSecretsList {
			if err = persistence.SaveSecret(s.db, agSecret.SvcSecretName, msInst.GetKey(), msInst.GetVersion(), &agSecret); err != nil {
				return err
			}
		}
	}
	return nil
}

// This is for updating service secrets. This assumes that the updated secrets are already updated in the agent db.
func (s SecretsManager) WriteExistingServiceSecretsToFile(msInstKey string, updatedSec persistence.PersistedServiceSecret) error {
	if contentBytes, err := base64.StdEncoding.DecodeString(updatedSec.SvcSecretValue); err != nil {
		return err
	} else {
		err = WriteToFile(contentBytes, path.Join(s.SecretsStorePath, msInstKey, updatedSec.SvcSecretName), path.Join(s.SecretsStorePath, msInstKey))
	}
	return nil
}

// Remove this agId from all the secrets it is in. If it is the only agId for that secret, remove the secret
func (s SecretsManager) DeleteAllSecForAgreement(db *bolt.DB, agreementId string) error {
	if allSec, err := persistence.FindAllServiceSecretsWithFilters(db, []persistence.SecFilter{}); err != nil {
		return err
	} else {
		for _, svcAllSec := range allSec {
			for secName, svcSec := range svcAllSec.SecretsMap {
				if cutil.SliceContains(svcSec.AgreementIds, agreementId) {
					if len(svcSec.AgreementIds) == 1 {
						if _, err := persistence.DeleteSecrets(db, secName, svcAllSec.MsInstKey); err != nil {
							glog.Errorf("Error deleting secret %v for agreement %v from db: %v", secName, agreementId, err)
						} else if len(svcAllSec.SecretsMap) == 1 {
							// Last secret for this microservice. Remove the whole file.
							if err = s.RemoveFile(svcAllSec.MsInstKey); err != nil {
								glog.Errorf("Error deleting secret folder %v for agreement %v: %v", svcAllSec.MsInstKey, agreementId, err)
							}
						} else {
							if err = s.RemoveSecretFile(svcAllSec.MsInstKey, secName); err != nil {
								glog.Errorf("Error deleting secret file %v for agreement %v: %v", secName, agreementId, err)
							}
						}
					} else {
						for ix, agId := range svcSec.AgreementIds {
							if agId == agreementId {
								svcSec.AgreementIds = append(svcSec.AgreementIds[:ix], svcSec.AgreementIds[ix+1:]...)
								break
							}
						}
						if err := persistence.SaveAllSecretsForService(db, svcAllSec.MsInstKey, &svcAllSec); err != nil {
							glog.Errorf("Error saving updated secret object after deleting secret %v for agreement %v: %v", secName, agreementId, err)
						}
					}
				}
			}
		}
	}
	return nil
}

// Remove the file containing the secret given
func (s SecretsManager) RemoveSecretFile(msInstKey string, secretName string) error {
	return os.RemoveAll(path.Join(s.SecretsStorePath, msInstKey))
}

// This is for writing new service secrets to a file for the service container. This assumes that the secrets are already in the agent db.
func (s SecretsManager) WriteNewServiceSecretsToFile(msInstKey string) error {
	if secretsForService, err := persistence.FindAllSecretsForMS(s.db, msInstKey); err != nil {
		return err
	} else if secretsForService != nil {
		for singleSecName, singleSecValue := range secretsForService.SecretsMap {
			if contentBytes, err := base64.StdEncoding.DecodeString(singleSecValue.SvcSecretValue); err != nil {
				return fmt.Errorf("Error decoding base64 encoded secret string: %v", err)
			} else if err = CreateAndWriteToFile(contentBytes, msInstKey, path.Join(s.SecretsStorePath, msInstKey, singleSecName), path.Join(s.SecretsStorePath, msInstKey)); err != nil {
				return err
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
		return errors.New(fmt.Sprintf("unable to create directory path %v for service secret, error: %v", filePath, err))
	} else if err := ioutil.WriteFile(fileName, contents, fileMode); err != nil {
		return errors.New(fmt.Sprintf("unable to write service secret file %v, error: %v", fileName, err))
	}

	glog.V(5).Infof(secLogString(fmt.Sprintf("Wrote secret contents to file %v ", filePath)))

	// change group owner of secrets foler and file, and set the group in groupAdd for service docker container in container.go
	if err := os.Chown(filePath, currUserUidInt, groupIdInt); err != nil {
		return errors.New(fmt.Sprintf("unable to change group to (group id: %v, group name:%v) for the service secret folder %v, error: %v", group.Gid, groupName, filePath, err))
	}

	if err := os.Chown(fileName, currUserUidInt, groupIdInt); err != nil {
		return errors.New(fmt.Sprintf("unable to change group to (group id: %v, group name:%v) for the service secret file %v, error: %v", group.Gid, groupName, fileName, err))
	}

	return nil
}

// This is for updating a service secrets file that already exists
func WriteToFile(contents []byte, fileName string, filePath string) error {
	var fileMode os.FileMode
	fileMode = 0750

	if err := ioutil.WriteFile(fileName, contents, fileMode); err != nil {
		return errors.New(fmt.Sprintf("unable to write service secret file %v, error: %v", fileName, err))
	}

	glog.V(5).Infof(secLogString(fmt.Sprintf("Wrote secret contents to file %v ", filePath)))

	return nil
}

func (s *SecretsManager) RemoveFile(key string) error {
	if err := os.RemoveAll(s.GetSecretsPath(key)); err != nil {
		return errors.New(fmt.Sprintf("unable to remove service secret file %v, error: %v", s.GetSecretsPath(key), err))
	}
	glog.V(5).Infof(secLogString(fmt.Sprintf("Removed service secret for service %v.", key)))

	groupName := cutil.GetHashFromString(key)
	if _, err := user.LookupGroup(groupName); err != nil {
		switch err.(type) {
		default:
			return errors.New(fmt.Sprintf("failed to look up group by group name: %v for file %v, error: %v", groupName, s.GetSecretsPath(key), err))
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

func (s SecretsManager) GetSecretsPath(msInstKey string) string {
	return path.Join(s.SecretsStorePath, msInstKey)
}

var secLogString = func(v interface{}) string {
	return fmt.Sprintf("Secrets Manager: %v", v)
}
