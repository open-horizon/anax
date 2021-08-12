package exchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"golang.org/x/text/message"
	"net/http"
	"runtime"
)

//BusinessListPolicy lists all the policies in the org or only the specified policy if one is given
func BusinessListPolicy(org string, credToUse string, policy string, namesOnly bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)

	var polOrg string
	polOrg, policy = cliutils.TrimOrg(org, policy)

	if policy == "*" {
		policy = ""
	}

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//get policy list from Horizon Exchange
	var policyList exchange.GetBusinessPolicyResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &policyList)
	if httpCode == 404 && policy != "" {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Policy %s not found in org %s", policy, polOrg))
	} else if httpCode == 404 {
		policyNameList := []string{}
		fmt.Println(policyNameList)
	} else if namesOnly && policy == "" {
		policyNameList := []string{}
		for bPolicy := range policyList.BusinessPolicy {
			policyNameList = append(policyNameList, bPolicy)
		}
		jsonBytes, err := json.MarshalIndent(policyNameList, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange deployment listpolicy' output: %v", err))
		}
		fmt.Println(string(jsonBytes))
	} else {
		buf := new(bytes.Buffer)
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", cliutils.JSON_INDENT)
		err := enc.Encode(policyList.BusinessPolicy)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange deployment listpolicy' output: %v", err))
		}
		fmt.Println(string(buf.String()))
	}
}

//BusinessAddPolicy will add a new policy or overwrite an existing policy byt he same name in the Horizon Exchange
func BusinessAddPolicy(org string, credToUse string, policy string, jsonFilePath string, noConstraints bool) {

	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var polOrg string
	polOrg, policy = cliutils.TrimOrg(org, policy)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//read in the new business policy from file
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(jsonFilePath)
	var policyFile businesspolicy.BusinessPolicy
	err := json.Unmarshal(newBytes, &policyFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", jsonFilePath, err))
	}

	//validate the format of the business policy
	err = policyFile.Validate()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect deployment policy format in file %s: %v", jsonFilePath, err))
	}

	// validate and verify the secret bindings
	ec := cliutils.GetUserExchangeContext(org, credToUse)
	verifySecretBindingForPolicy(&policyFile, polOrg, ec)

	// if the --no-constraints flag is not specified and the given policy has no constraints, alert the user.
	if (!noConstraints) && policyFile.HasNoConstraints() {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The deployment policy has no constraints which might result in the service being deployed to all nodes. Please specify --no-constraints to confirm that this is acceptable."))
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	//add/overwrite business policy file
	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201, 403}, policyFile, &resp)
	if httpCode == 403 {
		//try to update the existing policy
		httpCode = cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201, 404}, policyFile, nil)
		if httpCode == 201 {
			msgPrinter.Printf("Deployment policy: %v/%v updated in the Horizon Exchange", polOrg, policy)
			msgPrinter.Println()
		} else if httpCode == 404 {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot create deployment policy %v/%v: %v", polOrg, policy, resp.Msg))
		}
	} else {
		msgPrinter.Printf("Deployment policy: %v/%v added in the Horizon Exchange", polOrg, policy)
		msgPrinter.Println()
	}
}

//BusinessUpdatePolicy will replace a single attribute of a business policy in the Horizon Exchange
func BusinessUpdatePolicy(org string, credToUse string, policyName string, filePath string) {

	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var polOrg string
	polOrg, policyName = cliutils.TrimOrg(org, policyName)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//Read in the file
	attribute := cliconfig.ReadJsonFileWithLocalConfig(filePath)

	//verify that the policy exists
	var exchangePolicy exchange.GetBusinessPolicyResponse
	httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &exchangePolicy)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Policy %s not found in org %s", policyName, polOrg))
	}

	findPatchType := make(map[string]interface{})

	if err := json.Unmarshal([]byte(attribute), &findPatchType); err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", filePath, err))
	}

	var patch interface{}
	var err error
	if _, ok := findPatchType["service"]; ok {
		patch = make(map[string]businesspolicy.ServiceRef)
		err = json.Unmarshal([]byte(attribute), &patch)
	} else if _, ok := findPatchType["properties"]; ok {
		props := make(map[string]externalpolicy.PropertyList)
		err = json.Unmarshal([]byte(attribute), &props)
		patch = props
		if err == nil {
			newValue := props["properties"]
			err1 := newValue.Validate()
			if err1 != nil {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid format for properties: %v", err1))
			}
		}
	} else if _, ok := findPatchType["constraints"]; ok {
		constraints := make(map[string]externalpolicy.ConstraintExpression)
		err = json.Unmarshal([]byte(attribute), &constraints)
		patch = constraints
		if err == nil {
			newValue := constraints["constraints"]
			_, err1 := newValue.Validate()
			if err1 != nil {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid format for constraints: %v", err1))
			}
		}
	} else if _, ok := findPatchType["userInput"]; ok {
		patch = make(map[string][]policy.UserInput)
		err = json.Unmarshal([]byte(attribute), &patch)
	} else if _, ok := findPatchType["secretBinding"]; ok {
		sb := make(map[string][]exchangecommon.SecretBinding)
		err = json.Unmarshal([]byte(attribute), &sb)
		patch = sb
		if err == nil {
			// varify the secret bindings
			for _, exchPol := range exchangePolicy.BusinessPolicy {
				pol := exchPol.GetBusinessPolicy()
				pol.SecretBinding = sb["secretBinding"]
				ec := cliutils.GetUserExchangeContext(org, credToUse)
				verifySecretBindingForPolicy(&pol, polOrg, ec)

				break
			}
		}
	} else {
		_, ok := findPatchType["label"]
		_, ok2 := findPatchType["description"]
		if ok || ok2 {
			patch = make(map[string]string)
			err = json.Unmarshal([]byte(attribute), &patch)
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Deployment policy attribute to be updated is not found in the input file. Supported attributes are: label, description, service, properties, constraints, userInput and secretBinding."))
		}
	}

	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
	}
	msgPrinter.Printf("Updating Policy %v/%v in the Horizon Exchange and re-evaluating all agreements based on this deployment policy. Existing agreements might be cancelled and re-negotiated.", polOrg, policyName)
	msgPrinter.Println()
	cliutils.ExchangePutPost("Exchange", http.MethodPatch, exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch, nil)
	msgPrinter.Printf("Policy %v/%v updated in the Horizon Exchange", polOrg, policyName)
	msgPrinter.Println()
}

//BusinessRemovePolicy will remove an existing business policy in the Horizon Exchange
func BusinessRemovePolicy(org string, credToUse string, policy string, force bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	var polOrg string
	polOrg, policy = cliutils.TrimOrg(org, policy)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove deployment policy %v for org %v from the Horizon Exchange?", policy, polOrg))
	}

	//remove policy
	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	if httpCode == 404 {
		msgPrinter.Printf("Policy %v/%v not found in the Horizon Exchange", polOrg, policy)
		msgPrinter.Println()
	} else {
		msgPrinter.Printf("Removing deployment policy %v/%v and re-evaluating all agreements. Existing agreements might be cancelled and re-negotiated", polOrg, policy)
		msgPrinter.Println()
		msgPrinter.Printf("Deployment policy %v/%v removed", polOrg, policy)
		msgPrinter.Println()
	}
}

// Validate and verify the secret binding defined in the given deployment policy.
// It will output warning messages if the vault secret does not exist or error
// accessing vault.
func verifySecretBindingForPolicy(policy *businesspolicy.BusinessPolicy, polOrg string, ec exchange.ExchangeContext) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// make sure the all the service secrets have bindings.
	neededSB, extraneousSB, err := ValidateSecretBindingForDeplPolicy(policy, ec, true, msgPrinter)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Failed to validate the secret binding. %v", err))
	} else if extraneousSB != nil && len(extraneousSB) > 0 {
		msgPrinter.Printf("Note: The following secret bindings are not required by any services for this deployment policy:")
		msgPrinter.Println()
		for _, sb := range extraneousSB {
			fmt.Printf("  %v", sb)
			msgPrinter.Println()
		}
	}

	if neededSB == nil || len(neededSB) == 0 {
		return
	}

	// make sure the vault secret exists
	agbotUrl := cliutils.GetAgbotSecureAPIUrlBase()
	vaultSecretExists := exchange.GetHTTPVaultSecretExistsHandler(ec)
	msgMap, err := compcheck.VerifyVaultSecrets(neededSB, polOrg, agbotUrl, vaultSecretExists, msgPrinter)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Failed to verify the binding secret in the secret manager. %v", err))
	} else if msgMap != nil && len(msgMap) > 0 {
		msgPrinter.Printf("Warning: The following binding secrets cannot be verified in the secret manager:")
		msgPrinter.Println()
		for vsn, msg := range msgMap {
			fmt.Printf("  %v: %v", vsn, msg)
			msgPrinter.Println()
		}
	}
}

// Validate that each service secret has a vault binding in the given deployment policy.
// checkAllArches -- if the arch for the service is '*' or an empty string,
//   validate the secret bindings for all the arches that have this service.
// It does not verify if the vault secret exist in vault.
// It returns 2 array of SecretBinding objects. One for needed and one for extraneous.
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
			compcheck.CombineIndexMap(index_map, new_index_map)
		}
	}

	// group needed and extraneous secret bindings
	neededSB, extraneousSB := compcheck.GroupSecretBindings(secretBinding, index_map)

	return neededSB, extraneousSB, nil
}

// Given a top level service and an array of vault secret bindings, validate that
// all the secrets for the service and dependent services have vault bindings.
// It returns an index map keyed by index of the secretBinding array,
//    the value is a map of service secret names in the binding
//    that are needed. Using map here instead of array to make it easy to remove the
//    duplicates.
//    It also returns a map of dependent services keyed by the service id, the top
//    level service definition and id.
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
	// service secret names in the binding that are needed, i.e. not extraneous.
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
		_, svc_map, sDef, sId, err := getServiceResolvedDef(serviceName, serviceOrg, serviceVersion, arch)
		if err != nil {
			return nil, fmt.Errorf(msgPrinter.Sprintf("Error retrieving service %v/%v version %v from the Exchange. %v", serviceOrg, serviceName, serviceVersion, err))
		} else {
			// check top level service
			if index, neededSb, err := compcheck.ValidateSecretBindingForSingleService(secretBinding, &compcheck.ServiceDefinition{Org: serviceOrg, ServiceDefinition: *sDef}, msgPrinter); err != nil {
				return ret, fmt.Errorf(msgPrinter.Sprintf("Error validating secret bindings for service %v. %v", sId, err))
			} else {
				compcheck.UpdateIndexMap(ret, index, neededSb)
			}

			// check the dependent services
			for id, s := range svc_map {
				sOrg := exchange.GetOrg(id)
				if index, neededSb, err := compcheck.ValidateSecretBindingForSingleService(secretBinding, &compcheck.ServiceDefinition{Org: sOrg, ServiceDefinition: s}, msgPrinter); err != nil {
					return ret, fmt.Errorf(msgPrinter.Sprintf("Error validating secret bindings for dependent service %v. %v", id, err))
				} else {
					compcheck.UpdateIndexMap(ret, index, neededSb)
				}
			}
		}
	}

	return ret, nil
}

// Display an empty business policy template as an object.
func BusinessNewPolicy() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var business_policy_template = []string{
		`{`,
		`  "label": "",       /* ` + msgPrinter.Sprintf("Deployment policy label.") + ` */`,
		`  "description": "", /* ` + msgPrinter.Sprintf("Deployment policy description.") + ` */`,
		`  "service": {       `,
		`    "name": "",      /* ` + msgPrinter.Sprintf("The name of the service.") + ` */`,
		`    "org": "",       /* ` + msgPrinter.Sprintf("The org of the service.") + ` */`,
		`    "arch": "",      /* ` + msgPrinter.Sprintf("Set to '*' to use services of any hardware architecture.") + ` */`,
		`    "serviceVersions": [  /* ` + msgPrinter.Sprintf("A list of service versions.") + ` */`,
		`      {`,
		`        "version": "",`,
		`        "priority":{}`,
		`      }`,
		`    ]`,
		`  },`,
		`  "properties": [   /* ` + msgPrinter.Sprintf("A list of policy properties that describe the service being deployed.") + ` */`,
		`    {`,
		`       "name": "",`,
		`       "value": null`,
		`      }`,
		`  ],`,
		`  "constraints": [  /* ` + msgPrinter.Sprintf("A list of constraint expressions of the form <property name> <operator> <property value>,") + ` */`,
		`                    /* ` + msgPrinter.Sprintf("separated by boolean operators AND (&&) or OR (||).") + `*/`,
		`       "myproperty == myvalue" `,
		`  ], `,
		`  "userInput": [    /* ` + msgPrinter.Sprintf("A list of userInput variables to set when the service runs, listed by service.") + ` */`,
		`    {            `,
		`      "serviceOrgid": "",         /* ` + msgPrinter.Sprintf("The org of the service.") + ` */`,
		`      "serviceUrl": "",           /* ` + msgPrinter.Sprintf("The name of the service.") + ` */`,
		`      "serviceVersionRange": "",  /* ` + msgPrinter.Sprintf("The service version range to which these variables should be applied.") + ` */`,
		`      "inputs": [                 /* ` + msgPrinter.Sprintf("The input variables to be set.") + `*/`,
		`        {`,
		`          "name": "",`,
		`          "value": null`,
		`        }`,
		`      ]`,
		`    }`,
		`  ],`,
		`  "secretBinding": [ /* ` + msgPrinter.Sprintf("A list of secret bindings for the secret names defined in the services, listed by service.") + ` */`,
		`    {           `,
		`      "serviceOrgid": "",         /* ` + msgPrinter.Sprintf("The org of the service.") + ` */`,
		`      "serviceUrl": "",           /* ` + msgPrinter.Sprintf("The name of the service.") + ` */`,
		`      "serviceVersionRange": "",  /* ` + msgPrinter.Sprintf("The service version range.") + ` */`,
		`      "secrets": [                /* ` + msgPrinter.Sprintf("The secret bindings.") + ` */`,
		`        {      `,
		`          "<service-secret-name>": "<secret-provider-secret-name>"  /* ` + msgPrinter.Sprintf("The valid formats for secret provider secret names are:") + ` */`,
		`                                                          /* ` + msgPrinter.Sprintf("  '<secretname>' for organization level secret.") + ` */`,
		`                                                          /* ` + msgPrinter.Sprintf("  'user/<username>/<secretname>' for user level secret.") + ` */`,
		`        }`,
		`      ]`,
		`    }`,
		`  ]`,
		`}`,
	}

	for _, s := range business_policy_template {
		fmt.Println(s)
	}
}
