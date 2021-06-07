package common

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/semanticversion"
	"golang.org/x/text/message"
)

// Validate that each service secret has a vault binding in the given deployment policy.
// checkAllArches -- if the arch for the service is '*' or an empty string,
//   validate the secret bindings for all the arches that have this service.
// It does not verify if the vault secret exist in vault.
// It returns 2 array of SecretBinding objects. One for needed and one for redundant.
func ValidateSecretBindingForDeplPolicy(policy *businesspolicy.BusinessPolicy,
	ec exchange.ExchangeContext, checkAllArches bool,
	msgPrinter *message.Printer) ([]exchangecommon.SecretBinding, []exchangecommon.SecretBinding, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// validate secret bindings for all service versions defined
	svc := policy.Service
	secretBinding := policy.SecretBinding
	getServiceResolvedDef := exchange.GetHTTPServiceDefResolverHandler(ec)
	getSelectedServices := exchange.GetHTTPSelectedServicesHandler(ec)

	index_map := map[int]map[string]bool{}
	for _, wl := range svc.ServiceVersions {

		if new_index_map, err := ValidateSecretBindingForSvcAndDep(secretBinding, svc.Org, svc.Name, wl.Version, svc.Arch,
			checkAllArches, getServiceResolvedDef, getSelectedServices, msgPrinter); err != nil {
			return nil, nil, err
		} else {
			combineIndexMap(index_map, new_index_map)
		}
	}

	// group needed and redundant secret bindings
	neededSB, redundantSB := GroupSecretBindings(secretBinding, index_map)

	return neededSB, redundantSB, nil
}

// It validates that each secret in the given services has a vault secret from the given secret binding array.
// checkAllArches -- if the arch for the service is '*' or an empty string,
// validate the secret bindings for all the arches that have this service.
// It does not verify the vault secret exists in the vault.
// It returns 2 array of SecretBinding objects. One for needed and one for redundant.
//
func ValidateSecretBinding(secretBinding []exchangecommon.SecretBinding,
	sRef []exchange.ServiceReference, ec exchange.ExchangeContext, checkAllArches bool,
	msgPrinter *message.Printer) ([]exchangecommon.SecretBinding, []exchangecommon.SecretBinding, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if sRef == nil || len(sRef) == 0 {
		if secretBinding == nil || len(secretBinding) == 0 {
			return nil, nil, nil
		} else {
			return nil, nil, fmt.Errorf(msgPrinter.Sprintf("No secret is defined for any of the services. The secret binding is not needed: %v.", secretBinding))
		}
	}

	getServiceResolvedDef := exchange.GetHTTPServiceDefResolverHandler(ec)
	getSelectedServices := exchange.GetHTTPSelectedServicesHandler(ec)

	// keep track of which indexes in the secretBinding array were used
	index_map := map[int]map[string]bool{}

	// go through each top level services and do the validation for
	// it and its dependent services
	for _, svc := range sRef {
		if svc.ServiceVersions != nil {
			for _, v := range svc.ServiceVersions {
				if new_index_map, err := ValidateSecretBindingForSvcAndDep(secretBinding, svc.ServiceOrg, svc.ServiceURL, v.Version, svc.ServiceArch,
					checkAllArches, getServiceResolvedDef, getSelectedServices, msgPrinter); err != nil {
					return nil, nil, err
				} else {
					combineIndexMap(index_map, new_index_map)
				}
			}
		}
	}

	// group needed and redundant secret bindings
	neededSB, redundantSB := GroupSecretBindings(secretBinding, index_map)
	return neededSB, redundantSB, nil
}

// Given a top level service and an array of vault secret bindings, validate that
// all the secrets for the service and dependent services have vault bindings.
// It returns an index map keyed by index of the secretBinding array,
//    the value is a map of service secret names in the binding
//    that are needed. Using map here instead of array to make it easy to remove the
//    duplicates.
//
// checkAllArches -- if the arch for the service is '*' or an empty string,
// validate the secret bindings for all the arches that have this service.
// It does not verify that the vault secret actually exists or not.
func ValidateSecretBindingForSvcAndDep(secretBinding []exchangecommon.SecretBinding,
	serviceOrg, serviceName, serviceVersion, serviceArch string, checkAllArches bool,
	getServiceResolvedDef exchange.ServiceDefResolverHandler,
	getSelectedServices exchange.SelectedServicesHandler,
	msgPrinter *message.Printer) (map[int]map[string]bool, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// map: keyed by index of the secretBinding array, the value is a map of
	// service secret names in the binding that are needed, i.e. not redundant.
	//    map[index]map[service_secret_names]bool
	ret := map[int]map[string]bool{}

	arches := []string{}
	if serviceArch == "" || serviceArch == "*" {
		if checkAllArches {
			// include all the arches
			if svcMeta, err := getSelectedServices(serviceName, serviceOrg, serviceVersion, ""); err != nil {
				return ret, fmt.Errorf(msgPrinter.Sprintf("Failed to get services %v/%v version %v from the exchange for all archetctures. %v", serviceOrg, serviceName, serviceVersion, err))
			} else {
				for _, svc := range svcMeta {
					arches = append(arches, svc.Arch)
				}
			}
		} else {
			// just include the service with current node arch
			arches = append(arches, runtime.GOARCH)
		}
	} else {
		arches = append(arches, serviceArch)
	}

	for _, arch := range arches {
		svc_map, sDef, sId, err := getServiceResolvedDef(serviceName, serviceOrg, serviceVersion, arch)
		if err != nil {
			return nil, fmt.Errorf(msgPrinter.Sprintf("Error retrieving service %v/%v version %v from the Exchange. %v", serviceOrg, serviceName, serviceVersion, err))
		} else {
			// check top level service
			if index, neededSb, err := ValidateSecretBindingForSingleService(secretBinding, serviceOrg, sDef, msgPrinter); err != nil {
				return ret, fmt.Errorf(msgPrinter.Sprintf("Error validating secret bindings for service %v. %v", sId, err))
			} else {
				updateIndexMap(ret, index, neededSb)
			}

			// check the dependent services
			for id, s := range svc_map {
				if index, neededSb, err := ValidateSecretBindingForSingleService(secretBinding, exchange.GetOrg(id), &s, msgPrinter); err != nil {
					return ret, fmt.Errorf(msgPrinter.Sprintf("Error validating secret bindings for dependent service %v. %v", id, err))
				} else {
					updateIndexMap(ret, index, neededSb)
				}
			}
		}
	}

	return ret, nil
}

// Validate that the given secretBinding covers all the secrets defined in the given service.
// It also gives error if the secretBinding has bindings defined for the service but
// the service has no secrets.
// It returns the index of the SecretBinding object in the given that it is used for validation
// and an array of service secret names in the service binding that are needed by this service.
func ValidateSecretBindingForSingleService(secretBinding []exchangecommon.SecretBinding,
	svcOrg string, sdef *exchange.ServiceDefinition, msgPrinter *message.Printer) (int, []string, error) {
	if sdef == nil {
		return -1, nil, nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// get the secret bindings for this service
	index, err := GetSecretBindingForService(secretBinding, svcOrg, sdef.URL, sdef.Version, sdef.Arch, msgPrinter)
	if err != nil {
		return index, nil, err
	}

	// cluster type does not have secrets
	if sdef.GetServiceType() == exchange.SERVICE_TYPE_CLUSTER {
		if index == -1 {
			return index, nil, nil
		} else {
			return index, nil, fmt.Errorf(msgPrinter.Sprintf("Secret binding for a cluster service is not supported."))
		}
	}

	// convert the deployment string into object
	dConfig, err := ConvertToDeploymentConfig(sdef.Deployment, msgPrinter)
	if err != nil {
		return index, nil, err
	}

	// create a map of all the secrets in the SecretBinding
	// for this service, it will be used to check if all the
	// bindings are used or not
	sbNeeded := map[string]bool{}

	// make sure each service secret has a binding
	noBinding := map[string]bool{}
	if dConfig != nil {
		for _, svcConf := range dConfig.Services {
			for sn, _ := range svcConf.Secrets {
				found := false
				if index != -1 {
					for _, vbind := range secretBinding[index].Secrets {
						key, vs := vbind.GetBinding()
						if sn == key {
							found = true
							if _, _, err := ParseVaultSecretName(vs, msgPrinter); err != nil {
								return index, nil, err
							}
							sbNeeded[sn] = true
							break
						}
					}
				}
				if !found {
					noBinding[sn] = true
				}
			}
		}
	}

	if len(noBinding) > 0 {
		// convert to array to display
		nbArray := []string{}
		for sn, _ := range noBinding {
			nbArray = append(nbArray, sn)
		}
		return index, nil, fmt.Errorf(msgPrinter.Sprintf("No secret binding found for the following service secrets: %v.", nbArray))
	}

	// return an array of service secret names in the biniding that are needed.
	used_sb := []string{}
	for k, v := range sbNeeded {
		if v == true {
			used_sb = append(used_sb, k)
		}
	}

	return index, used_sb, nil
}

// Given a list of SecretBinding's for multiples services, return index for
// the secret binding object in the given array that will be used by the given service.
// -1 means no secret binding defined for the given service
func GetSecretBindingForService(secretBinding []exchangecommon.SecretBinding, svcOrg, svcName, svcVersion, svcArch string,
	msgPrinter *message.Printer) (int, error) {

	if secretBinding == nil {
		return -1, nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	for index, sb := range secretBinding {
		if sb.ServiceUrl != svcName || sb.ServiceOrgid != svcOrg {
			continue
		}
		if sb.ServiceArch != "" && sb.ServiceArch != "*" && sb.ServiceArch != svcArch {
			continue
		}
		if sb.ServiceVersionRange != "" && sb.ServiceVersionRange != svcVersion {
			if vExp, err := semanticversion.Version_Expression_Factory(sb.ServiceVersionRange); err != nil {
				return -1, fmt.Errorf(msgPrinter.Sprintf("Wrong version string %v specified in secret binding for service %v/%v %v %v, error %v", sb.ServiceVersionRange, svcOrg, svcName, svcVersion, svcArch, err))
			} else if inRange, err := vExp.Is_within_range(svcVersion); err != nil {
				return -1, fmt.Errorf(msgPrinter.Sprintf("Error checking version %v in range %v. %v", svcVersion, vExp, err))
			} else if !inRange {
				continue
			}
		}

		return index, nil
	}

	return -1, nil
}

// Call the agbot API to verify the vault secrets exists.
// It does not return when the vault secret does not exist or there is an error accessing
// the vault api. Instead it will return a messages for each vault secret name that could
// not be verified.
func VerifyVaultSecrets(secretBinding []exchangecommon.SecretBinding, secretOrg string, agbotURL string,
	vaultSecretExists exchange.VaultSecretExistsHandler, msgPrinter *message.Printer) (map[string]string, error) {

	if secretBinding == nil || len(secretBinding) == 0 {
		return nil, nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if agbotURL == "" {
		return nil, fmt.Errorf(msgPrinter.Sprintf("agbot URL cannot be an empty string when checking secret binding. Please make sure HZN_AGBOT_URL is set."))
	}

	if secretOrg == "" {
		return nil, fmt.Errorf(msgPrinter.Sprintf("The organization for the vault secret must be provided."))
	}

	// go through each secret binding making sure the vault secret exist in vault
	ret := map[string]string{}
	vs_checked := map[string]bool{}
	for _, sn := range secretBinding {
		for _, vbind := range sn.Secrets {

			// make sure each vault get checked only once
			_, vaultSecretName := vbind.GetBinding()
			if _, ok := vs_checked[vaultSecretName]; ok {
				continue
			} else {
				vs_checked[vaultSecretName] = true
			}

			if exists, err := VerifySingleVaultSecret(vaultSecretName, secretOrg, agbotURL, vaultSecretExists, msgPrinter); err != nil {
				ret[vaultSecretName] = err.Error()
			} else if !exists {
				ret[vaultSecretName] = msgPrinter.Sprintf("Vault secret %v does not exist.", vaultSecretName)
			}
		}
	}

	return ret, nil
}

// Call the agbot API to verify the vault secrets exists.
// It returns immediately when a vault secret does not exist or there is an error accessing
// the vault api.
func VerifyVaultSecrets_strict(secretBinding []exchangecommon.SecretBinding, secretOrg string, agbotURL string,
	vaultSecretExists exchange.VaultSecretExistsHandler, msgPrinter *message.Printer) error {
	if secretBinding == nil || len(secretBinding) == 0 {
		return nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if agbotURL == "" {
		return fmt.Errorf(msgPrinter.Sprintf("agbot URL cannot be an empty string when checking secret binding. Please make sure HZN_AGBOT_URL is set."))
	}

	if secretOrg == "" {
		return fmt.Errorf(msgPrinter.Sprintf("The organization for the vault secret must be provided."))
	}

	// go through each secret binding making sure the vault secret exist in vault
	vs_checked := map[string]bool{}
	for _, sn := range secretBinding {
		for _, vbind := range sn.Secrets {
			_, vaultSecretName := vbind.GetBinding()

			// make sure each vault get checked only once
			if _, ok := vs_checked[vaultSecretName]; ok {
				continue
			} else {
				vs_checked[vaultSecretName] = true
			}

			if exists, err := VerifySingleVaultSecret(vaultSecretName, secretOrg, agbotURL, vaultSecretExists, msgPrinter); err != nil {
				return err
			} else if !exists {
				return fmt.Errorf(msgPrinter.Sprintf("Vault secret %v does not exist.", vaultSecretName))
			}
		}
	}

	return nil
}

// It calls the agbot API to verify whether the given secret name exist in vault or not.
func VerifySingleVaultSecret(vaultSecretName string, secretOrg string, agbotURL string,
	vaultSecretExists exchange.VaultSecretExistsHandler, msgPrinter *message.Printer) (bool, error) {

	// parse the name
	userName, sName, err_parse := ParseVaultSecretName(vaultSecretName, msgPrinter)
	if err_parse != nil {
		return false, fmt.Errorf(msgPrinter.Sprintf("Error parsing vault secret name in the secret binding. %v", err_parse))
	}

	// check the existance
	if exists, err := vaultSecretExists(agbotURL, secretOrg, userName, sName); err != nil {
		return false, fmt.Errorf(msgPrinter.Sprintf("Error checking vault secret %v exists. %v", vaultSecretName, err))
	} else {
		return exists, nil
	}
}

// Parse the given vault secret name and return (user_name, secret_name, fully_qualified_name)
// The vault secret name has the following formats:
//     mysecret
//     user/myusername/mysecrte
// The fully qualified name in vault is the name above preceded by "openhorizon/<orgname>".
// However, it is not valid to specify the fully qualified name in the deployment policy or the pattern.
// The <org_name> will always be the node's org name at the deployment time.
// For deployment policy and private pattern, it is actually the org name of the policy or pattrn.
// For public pattern, it is the org name of the node.
func ParseVaultSecretName(secretName string, msgPrinter *message.Printer) (string, string, error) {

	// cannot be empty string
	if secretName == "" {
		return "", "", fmt.Errorf(msgPrinter.Sprintf("The vault secret name cannot be an empty string. The valid formats are: '<secretname>' for the organization level secret and 'user/<username>/<secretname>' for the user level secret."))
	}

	parts := strings.Split(secretName, "/")
	length := len(parts)
	if parts[0] != "openhorizon" {
		if parts[0] != "user" && parts[0] != "" {
			// case: mysecret
			return "", secretName, nil
		} else if parts[0] == "" && parts[1] != "user" {
			// case: /mysecret
			return "", strings.Join(parts[1:], "/"), nil
		} else if parts[0] == "user" && length >= 3 {
			// case: user/myusername/mysecrte
			return parts[1], strings.Join(parts[2:], "/"), nil
		} else if parts[0] == "" && parts[1] == "user" && length >= 4 {
			// case: /usr/myusername/mysecrte
			return parts[2], strings.Join(parts[3:], "/"), nil
		}
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	return "", "", fmt.Errorf(msgPrinter.Sprintf("Invalid format for the vault secret name: %v. The valid formats are: '<secretname>' for the organization level secret and 'user/<username>/<secretname>' for the user level secret.", secretName))
}

// update the index map.
func updateIndexMap(indexMap map[int]map[string]bool, index int, neededSb []string) {
	if index == -1 {
		return
	}

	if neededSb == nil || len(neededSb) == 0 {
		return
	}

	if _, ok := indexMap[index]; !ok {
		indexMap[index] = map[string]bool{}
	}

	for _, sn := range neededSb {
		indexMap[index][sn] = true
	}
}

// update the index map with the new one.
func combineIndexMap(indexMap map[int]map[string]bool, newIndexMap map[int]map[string]bool) {

	if newIndexMap == nil || len(newIndexMap) == 0 {
		return
	}

	for index, sn_map := range newIndexMap {
		if _, ok := indexMap[index]; !ok {
			indexMap[index] = newIndexMap[index]
		} else {
			for sn, _ := range sn_map {
				indexMap[index][sn] = true
			}
		}
	}
}

// given an array of secret bindings and an index map, group the
// secret bindings into 2 groups: needed and redundant.
func GroupSecretBindings(secretBinding []exchangecommon.SecretBinding, indexMap map[int]map[string]bool) ([]exchangecommon.SecretBinding, []exchangecommon.SecretBinding) {
	// group needed and redundant secret bindings
	neededSB := []exchangecommon.SecretBinding{}
	redundantSB := []exchangecommon.SecretBinding{}

	if secretBinding == nil || len(secretBinding) == 0 {
		return neededSB, redundantSB
	}

	if indexMap == nil || len(indexMap) == 0 {
		redundantSB = append(redundantSB, secretBinding...)
		return neededSB, redundantSB
	}

	for index, sb := range secretBinding {
		if _, ok := indexMap[index]; !ok {
			// the whole SecretBinding object is redundant.
			redundantSB = append(redundantSB, secretBinding[index])
		} else {
			if len(sb.Secrets) == len(indexMap[index]) {
				// the whole SecretBinding object is needed.
				neededSB = append(neededSB, secretBinding[index])
			} else {
				// partially redundant. break it into 2
				// copy the structure, reset the Secrets to empty
				sb_needed := secretBinding[index]
				sb_redundant := secretBinding[index]
				sb_needed.Secrets = []exchangecommon.VaultBinding{}
				sb_redundant.Secrets = []exchangecommon.VaultBinding{}
				for _, s := range sb.Secrets {
					k, _ := s.GetBinding()
					if _, ok1 := indexMap[index][k]; !ok1 {
						sb_redundant.Secrets = append(sb_redundant.Secrets, s)
					} else {
						sb_needed.Secrets = append(sb_needed.Secrets, s)
					}
				}

				neededSB = append(neededSB, sb_needed)
				redundantSB = append(redundantSB, sb_redundant)
			}
		}
	}

	return neededSB, redundantSB
}
