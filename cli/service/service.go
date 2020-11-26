package service

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"net/http"
	"runtime"
	"strings"
)

type APIServices struct {
	Config []api.APIMicroserviceConfig `json:"config"` // the service configurations
	//Instances   map[string][]interface{}    `json:"instances"`   // the service instances that are running
	//Definitions map[string][]interface{}    `json:"definitions"` // the definitions of services from the exchange
}

type OurService struct {
	Url       string                 `json:"url"`     // A URL pointing to the definition of the service
	Org       string                 `json:"org"`     // The organization where the service is defined
	Version   string                 `json:"version"` // The version of the service in OSGI version format
	Arch      string                 `json:"arch"`    // The hardware architecture of the service impl
	Variables map[string]interface{} `json:"variables"`
}

func List() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Get the services
	var apiOutput APIServices
	// Note: intentionally querying /service, because in the future we will probably want to mix in some key runtime info
	httpCode, _ := cliutils.HorizonGet("service/config", []int{200, cliutils.ANAX_NOT_CONFIGURED_YET}, &apiOutput, false)
	//todo: i think config can be queried even before the node is registered?
	if httpCode == cliutils.ANAX_NOT_CONFIGURED_YET {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf(cliutils.MUST_REGISTER_FIRST))
	}

	statusInfo := apicommon.Info{}
	cliutils.HorizonGet("status", []int{200}, &statusInfo, false)
	anaxArch := (*statusInfo.Configuration).Arch

	// Go thru the services and pull out interesting fields
	services := make([]OurService, 0)
	for _, s := range apiOutput.Config {
		serv := OurService{Url: s.SensorUrl, Org: s.SensorOrg, Version: s.SensorVersion, Arch: anaxArch, Variables: make(map[string]interface{})}

		for _, attr := range s.Attributes {
			if b_attr, err := json.Marshal(attr); err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal '/service/config' output attribute %v. %v", attr, err))
				return
			} else if a, err := persistence.HydrateConcreteAttribute(b_attr); err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to convert '/service/config' output attribute %v to its original type. %v", attr, err))
				return
			} else {
				switch a.(type) {
				case persistence.UserInputAttributes:
					// get user input
					serv.Variables = a.GetGenericMappings()
				}
			}
		}

		services = append(services, serv)
	}

	// Convert to json and output
	jsonBytes, err := json.MarshalIndent(services, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn service list' output: %v", err))
	}
	fmt.Printf("%s\n", jsonBytes)
}

func Log(serviceName string, tailing bool) {
	msgPrinter := i18n.GetMessagePrinter()

	// if node is not registered
	horDevice := api.HorizonDevice{}
	cliutils.HorizonGet("node", []int{200}, &horDevice, false)
	if horDevice.Org == nil || *horDevice.Org == "" {
		msgPrinter.Printf("The node is not registered.")
		msgPrinter.Println()
		return
	}

	refUrl := serviceName
	// Get the list of running services from the agent.
	runningServices := api.AllServices{}

	cliutils.HorizonGet("service", []int{200}, &runningServices, false)
	// Search the list of services to find one that matches the input service name. The service's instance Id
	// is what appears in the syslog, so we need to save that.
	serviceFound := false
	var instanceId string
	org, name := cutil.SplitOrgSpecUrl(refUrl)
	for _, serviceInstance := range runningServices.Instances["active"] {
		if (serviceInstance.SpecRef == name && serviceInstance.Org == org) || strings.Contains(serviceInstance.SpecRef, refUrl) {
			instanceId = serviceInstance.InstanceId
			serviceFound = true
			msgPrinter.Printf("Displaying log messages for service %v with service id %v.", serviceInstance.SpecRef, instanceId)
			msgPrinter.Println()
			if tailing {
				msgPrinter.Printf("Use ctrl-C to terminate this command.")
				msgPrinter.Println()
			}
			break
		}
	}
	if !serviceFound {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Service %v is not running on the node.", refUrl))
	}

	// Check service's log-driver to read logs from correct place
	var nonDefaultLogDriverUsed bool
	for _, v := range runningServices.Definitions["active"] {
		def := &persistence.MicroserviceDefinition{}
		defBytes, _ := json.Marshal(v)
		if err := json.Unmarshal(defBytes, def); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Service definition unmarshalling error: %v", err))
		}

		if (def.SpecRef == name && def.Org == org) || strings.Contains(def.SpecRef, refUrl) {
			if def.Deployment != "" {
				deployment := &containermessage.DeploymentDescription{}
				if err := json.Unmarshal([]byte(def.Deployment), deployment); err != nil {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Deployment unmarshalling error: %v", err))
				}

				for _, service := range deployment.Services {
					var err error
					if nonDefaultLogDriverUsed, err = cliutils.ChekServiceLogPossibility(service.LogDriver); err != nil {
						cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Service logs are unavailable: %v", err))
					}
					break
				}
			}
			break
		}
	}

	if runtime.GOOS == "darwin" || nonDefaultLogDriverUsed {
		cliutils.LogMac(instanceId, tailing)
	} else {
		cliutils.LogLinux(instanceId, tailing)
	}
}

func Registered() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// The registered services are listed as policies
	apiOutput := make(map[string]policy.Policy)
	httpCode, _ := cliutils.HorizonGet("service/policy", []int{200, cliutils.ANAX_NOT_CONFIGURED_YET}, &apiOutput, false)
	if httpCode == cliutils.ANAX_NOT_CONFIGURED_YET {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf(cliutils.MUST_REGISTER_FIRST))
	}

	// Convert to json and output
	jsonBytes, err := json.MarshalIndent(apiOutput, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn service registered' output: %v", err))
	}
	fmt.Printf("%s\n", jsonBytes)
}

func ListConfigState() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	apiOutput := make(map[string][]exchange.ServiceConfigState)
	httpCode, _ := cliutils.HorizonGet("service/configstate", []int{200, cliutils.ANAX_NOT_CONFIGURED_YET}, &apiOutput, false)
	if httpCode == cliutils.ANAX_NOT_CONFIGURED_YET {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf(cliutils.MUST_REGISTER_FIRST))
	}

	// Convert to json and output
	jsonBytes, err := json.MarshalIndent(apiOutput, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn service configstate' output: %v", err))
	}
	fmt.Printf("%s\n", jsonBytes)
}

func Suspend(forceSuspend bool, applyAll bool, serviceOrg string, serviceUrl string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	msg_part := msgPrinter.Sprintf("all the registered services")
	if !applyAll {
		if serviceOrg != "" {
			if serviceUrl != "" {
				msg_part = msgPrinter.Sprintf("service %v/%v", serviceOrg, serviceUrl)
			} else {
				msg_part = msgPrinter.Sprintf("all the registered services from organization %v", serviceOrg)
			}
		} else if serviceUrl != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the organization for service %v.", serviceUrl))
		}
	}

	if !forceSuspend {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to suspend %v for this Horizon node?", msg_part))
	}

	msgPrinter.Printf("Suspending %v, cancelling related agreements, stopping related service containers...", msg_part)
	msgPrinter.Println()

	if applyAll {
		serviceOrg = ""
		serviceUrl = ""
	}
	apiInput := exchange.ServiceConfigState{
		Url:         serviceUrl,
		Org:         serviceOrg,
		ConfigState: exchange.SERVICE_CONFIGSTATE_SUSPENDED,
	}

	cliutils.HorizonPutPost(http.MethodPost, "service/configstate", []int{201, 200}, apiInput, true)

	msgPrinter.Printf("Service suspending request successfully sent, please use 'hzn agreement' and 'docker ps' to make sure the related agreements and service containers are removed. It may take a couple of minutes.")
	msgPrinter.Println()
}

func Resume(applyAll bool, serviceOrg string, serviceUrl string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	msg_part := msgPrinter.Sprintf("all the registered services")
	if !applyAll {
		if serviceOrg != "" {
			if serviceUrl != "" {
				msg_part = msgPrinter.Sprintf("service %v/%v", serviceOrg, serviceUrl)
			} else {
				msg_part = msgPrinter.Sprintf("all the registered services from organization %v", serviceOrg)
			}
		} else if serviceUrl != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the organization for service %v.", serviceUrl))
		}
	}

	msgPrinter.Printf("Resuming %v ...", msg_part)
	msgPrinter.Println()

	if applyAll {
		serviceOrg = ""
		serviceUrl = ""
	}
	apiInput := exchange.ServiceConfigState{
		Url:         serviceUrl,
		Org:         serviceOrg,
		ConfigState: exchange.SERVICE_CONFIGSTATE_ACTIVE,
	}

	cliutils.HorizonPutPost(http.MethodPost, "service/configstate", []int{201, 200}, apiInput, true)

	msgPrinter.Printf("Service resuming request successfully sent, please use 'hzn agreement' and 'docker ps' to make sure the related agreements and service containers are started. It may take a couple of minutes.")
	msgPrinter.Println()
}
