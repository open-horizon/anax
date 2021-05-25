package common

import (
	"fmt"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/semanticversion"
	"golang.org/x/text/message"
)

// Validate that each secret in the given services has a vault secret from the given secret binding array.
// And there are no redundant bindings.
// It does not verify that the vault secret actually exists or not.
func ValidateSecretBindingForServices(secretBinding []exchangecommon.SecretBinding, sRef []exchange.ServiceReference,
	serviceDefResolverHandler exchange.ServiceDefResolverHandler, msgPrinter *message.Printer) error {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if sRef == nil || len(sRef) == 0 {
		if secretBinding == nil || len(secretBinding) == 0 {
			return nil
		} else {
			return fmt.Errorf(msgPrinter.Sprintf("No secret is defined for any of the services. The secret binding is not needed: %v.", secretBinding))
		}
	}

	// keep track of which indexes in the secretBinding array were used
	// and the service ids that used it for secret binding check
	all_idx := map[int][]string{}

	// go through each top level services and do the validation for
	// it and its dependent services
	for _, svc := range sRef {
		if svc.ServiceVersions != nil {
			for _, v := range svc.ServiceVersions {
				if index_map, err := ValidateSecretBindingForSvcAndDep(secretBinding, svc.ServiceOrg, svc.ServiceURL, v.Version, svc.ServiceArch, serviceDefResolverHandler, msgPrinter); err != nil {
					return err
				} else {
					for index, ids := range index_map {
						if a, ok := all_idx[index]; ok {
							all_idx[index] = append(a, ids...)
						} else {
							all_idx[index] = ids
						}
					}
				}
			}
		}
	}

	// find redundant secret bindings
	for index, sb := range secretBinding {
		if _, ok := all_idx[index]; !ok {
			return fmt.Errorf(msgPrinter.Sprintf("The following secret binding is redundant: %v.", sb))
		}
	}

	return nil
}

// Given a top level service and an array of vault secret bindings, validate that
// all the secrets for the service and dependent services have vault bindings.
// It returns a map keyed by index of the secretBinding array,
// the value is an array of service ids that use the object for validation.
// It does not verify that the vault secret actually exists or not.
func ValidateSecretBindingForSvcAndDep(secretBinding []exchangecommon.SecretBinding, serviceOrg, serviceName, serviceVersion, serviceArch string,
	serviceDefResolverHandler exchange.ServiceDefResolverHandler, msgPrinter *message.Printer) (map[int][]string, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// map: keyed by index of the secretBinding array, the value is an array of service ids that use the object for
	// validation
	ret := map[int][]string{}

	svc_map, sDef, sId, err := serviceDefResolverHandler(serviceName, serviceOrg, serviceVersion, serviceArch)
	if err != nil {
		return ret, fmt.Errorf(msgPrinter.Sprintf("Error retrieving service %v/%v version %v from the Exchange. %v", serviceOrg, serviceName, serviceVersion, err))
	}

	// check top level service
	if index, err := ValidateSecretBindingForSingleService(secretBinding, serviceOrg, sDef, msgPrinter); err != nil {
		return ret, fmt.Errorf(msgPrinter.Sprintf("Error validating secret bindings for service %v. %v", sId, err))
	} else if index != -1 {
		if a, ok := ret[index]; ok {
			ret[index] = append(a, sId)
		} else {
			ret[index] = []string{sId}
		}
	}

	// check the dependent services
	for id, s := range svc_map {
		if index, err := ValidateSecretBindingForSingleService(secretBinding, exchange.GetOrg(id), &s, msgPrinter); err != nil {
			return ret, fmt.Errorf(msgPrinter.Sprintf("Error validating secret bindings for dependent service %v. %v", id, err))
		} else if index != -1 {
			if a, ok := ret[index]; ok {
				ret[index] = append(a, id)
			} else {
				ret[index] = []string{id}
			}
		}
	}

	return ret, nil
}

// Validate that the given secretBinding covers all the secrets defined in the given service.
// It also gives error if the secretBinding has bindings defined for the service but
// the service has no secrets.
// It returns the index of the SecretBinding object in the given that it is used for validation.
func ValidateSecretBindingForSingleService(secretBinding []exchangecommon.SecretBinding,
	svcOrg string, sdef *exchange.ServiceDefinition, msgPrinter *message.Printer) (int, error) {
	if sdef == nil {
		return -1, nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// get the secret bindings for this service
	index, err := GetSecretBindingForService(secretBinding, svcOrg, sdef.URL, sdef.Version, sdef.Arch, msgPrinter)
	if err != nil {
		return index, err
	}

	// cluster type does not have secrets
	if sdef.GetServiceType() == exchange.SERVICE_TYPE_CLUSTER {
		if index == -1 {
			return index, nil
		} else {
			return index, fmt.Errorf(msgPrinter.Sprintf("Secret binding for a cluster service is not supported."))
		}
	}

	// convert the deployment string into object
	dConfig, err := ConvertToDeploymentConfig(sdef.Deployment, msgPrinter)
	if err != nil {
		return index, err
	}

	// create a map of all the secrets in the SecretBinding
	// for this service, it will be used to check if all the
	// bindings are used or not
	sbNeeded := map[string]bool{}
	if index != -1 {
		for _, vbind := range secretBinding[index].Secrets {
			sbNeeded[vbind.Value] = false
		}
	}

	// make sure each service secret has a binding
	noBinding := map[string]bool{}
	if dConfig != nil {
		for _, svcConf := range dConfig.Services {
			for sn, _ := range svcConf.Secrets {
				found := false
				if index != -1 {
					for _, vbind := range secretBinding[index].Secrets {
						if sn == vbind.Value {
							found = true
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
		return index, fmt.Errorf(msgPrinter.Sprintf("No secret binding found for the following service secrets: %v.", nbArray))
	}

	// make sure each binding has service secrets defined in the service
	extraSBs := []string{}
	for sn, v := range sbNeeded {
		if v == false {
			extraSBs = append(extraSBs, sn)
		}
	}
	if len(extraSBs) == 1 {
		return index, fmt.Errorf(msgPrinter.Sprintf("The secret binding for secret %v is redundant. It is not required by any service.", extraSBs[0]))
	} else if len(extraSBs) > 1 {
		return index, fmt.Errorf(msgPrinter.Sprintf("The secret bindings for secrets %v are redundant. They are not required by any service.", extraSBs))
	}

	return index, nil
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
		if sb.ServiceArch != "" && sb.ServiceArch != svcArch {
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

// Call the agbot API to verify the vault secrets exists
//func VerifySecrets(secretBinding []exchangecommon.SecretBinding, msgPrinter *message.Printer) error {

//}
