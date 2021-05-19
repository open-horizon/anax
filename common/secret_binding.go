package common

import (
	"fmt"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/semanticversion"
	"golang.org/x/text/message"
)

// Given a top level service and an array of secret bindings, verify that
// all the secrets for itself and dependent services have vault bindings.
func ValidateSecretBinding(secretBinding []exchange.SecretBinding, serviceOrg, serviceName, serviceVersion, serviceArch string,
	     serviceDefResolverHandler exchange.ServiceDefResolverHandler, msgPrinter *message.Printer) error {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	svc_map, sDef, sId, err := serviceDefResolverHandler(serviceName, serviceOrg, serviceVersion, serviceArch)
	if err != nil {
		return fmt.Errorf(msgPrinter.Sprintf("Error retrieving service %v/%v version %v from the Exchange. %v", serviceOrg, serviceName, serviceVersion, err))
	}

	// check top level service
	if err := ValidateSecretBindingForSingleService(secretBinding, serviceOrg, sDef, msgPrinter); err != nil {
		return fmt.Errorf(msgPrinter.Sprintf("Error validating secret bindings for service %v. %v", sId, err))
	}

	// check the dependent services
	for id, s := range svc_map {
		if err := ValidateSecretBindingForSingleService(secretBinding, exchange.GetOrg(id), &s, msgPrinter); err != nil {
			return fmt.Errorf(msgPrinter.Sprintf("Error validating secret bindings for dependent service %v. %v", id, err))
		}
	}

	return nil
}

// Verify that the given secretBinding covers all the secrets defined in the given service.
// It also gives error if the secretBinding has bindings defined for the service but
// the service has no secret defined.
func ValidateSecretBindingForSingleService(secretBinding []exchange.SecretBinding,
	      svcOrg string, sdef *exchange.ServiceDefinition, msgPrinter *message.Printer) error {
	if sdef == nil {
		return nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// get the secret bindings for this service
	p_sb, err := getSecretBindingForService(secretBinding, svcOrg, sdef.URL, sdef.Version, sdef.Arch, msgPrinter)
	if err != nil {
		return err
	}

	// cluster type does not have secrets
	if sdef.GetServiceType() == exchange.SERVICE_TYPE_CLUSTER {
		if p_sb == nil {
			return nil
		} else {
			return fmt.Errorf(msgPrinter.Sprintf("Secret binding for a cluster service is not supported."))
		}
	}

	// convert the deployment string into object
	dConfig, err := ConvertToDeploymentConfig(sdef.Deployment, msgPrinter)
	if err != nil {
		return err
	}

	// create a map of all the secrets in the SecretBinding
	// for this service, it will be used to check if all the 
	// bindings are used or not
	sbNeeded := map[string]bool{}
	if p_sb != nil {
		for _, vbind := range p_sb.Secrets {
			sbNeeded[vbind.Value] = false
		}
	}

	// make sure each service secret has a binding
	noBinding := map[string]bool{}
	if dConfig != nil {
		for _, svcConf := range dConfig.Services {
			for sn, _ := range svcConf.Secrets {
				found := false
				if p_sb != nil {
					for _, vbind := range p_sb.Secrets {
						if sn == vbind.Value {
							found = true
							sbNeeded[sn] = true
							break
						}
					}
				}
				if (!found) {
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
		return fmt.Errorf(msgPrinter.Sprintf("No secret binding found for the following service secrets: %v.", nbArray))				
	}

	// make sure each binding has service secrets defined in the service
	extraSBs := []string{}
	for sn, v := range sbNeeded {
		if v == false {
			extraSBs = append(extraSBs, sn)
		}
	}   
	if len(extraSBs) > 0 {
		return fmt.Errorf(msgPrinter.Sprintf("The service does not have the following secrets defined: %v. Please check the secret bindings.", extraSBs))				
	}

	return nil
}

// Given a list of SecretBinding's for multiples services, return the one for the given service.
func getSecretBindingForService(secretBinding []exchange.SecretBinding, svcOrg, svcName, svcVersion, svcArch string,
	        msgPrinter *message.Printer) (*exchange.SecretBinding, error) {

	if secretBinding == nil {
		return nil, nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}


	for _, sb := range secretBinding {
		if sb.ServiceUrl != svcName || sb.ServiceOrgid != svcOrg {
			continue
		}
		if sb.ServiceArch != "" && sb.ServiceArch != svcArch {
			continue
		}
		if sb.ServiceVersionRange != "" && sb.ServiceVersionRange != svcVersion {
			if vExp, err := semanticversion.Version_Expression_Factory(sb.ServiceVersionRange); err != nil {
				return nil, fmt.Errorf(msgPrinter.Sprintf("Wrong version string %v specified in secret binding for service %v/%v %v %v, error %v", sb.ServiceVersionRange, svcOrg, svcName, svcVersion, svcArch, err))
			} else if inRange, err := vExp.Is_within_range(svcVersion); err != nil {
				return nil, fmt.Errorf(msgPrinter.Sprintf("Error checking version %v in range %v. %v", svcVersion, vExp, err))
			} else if !inRange {
				continue
			}
		}

		return &sb, nil
	}

	return nil, nil
}
