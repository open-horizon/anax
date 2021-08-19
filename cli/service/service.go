package service

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
	"net/http"
	"runtime"
	"strings"
)

type OurService struct {
	Url     string `json:"url"`     // A URL pointing to the definition of the service
	Org     string `json:"org"`     // The organization where the service is defined
	Version string `json:"version"` // The version of the service in OSGI version format
	Arch    string `json:"arch"`    // The hardware architecture of the service impl
}

func List() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Get the services
	var apiOutput api.AllServices
	httpCode, _ := cliutils.HorizonGet("service", []int{200, cliutils.ANAX_NOT_CONFIGURED_YET}, &apiOutput, false)
	if httpCode == cliutils.ANAX_NOT_CONFIGURED_YET {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf(cliutils.MUST_REGISTER_FIRST))
	}

	// Go thru the services and pull out interesting fields
	services := make([]OurService, 0)
	for _, s := range apiOutput.Definitions["active"] {
		serv := OurService{Url: s.SpecRef, Org: s.Org, Version: s.Version, Arch: s.Arch}

		services = append(services, serv)
	}

	// Convert to json and output
	jsonBytes, err := json.MarshalIndent(services, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn service list' output: %v", err))
	}
	fmt.Printf("%s\n", jsonBytes)
}

func Log(serviceName string, serviceVersion, containerName string, tailing bool) {
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
	var serviceInstanceFound *api.MicroserviceInstanceOutput
	var instanceId string
	var msdefId string
	org, name := cutil.SplitOrgSpecUrl(refUrl)
	for _, serviceInstance := range runningServices.Instances["active"] {
		if (serviceVersion == "" || serviceVersion == serviceInstance.Version) {
			if serviceInstance.SpecRef == name && (serviceInstance.Org == org || org == "") {
				serviceInstanceFound = serviceInstance
				serviceFound = true
				break
			} else if !serviceFound && strings.Contains(serviceInstance.SpecRef, name) && (serviceInstance.Org == org || org == "") {
				serviceInstanceFound = serviceInstance
				serviceFound = true
			}
		}
	}
	if serviceFound {
		instanceId = serviceInstanceFound.InstanceId
		msdefId = serviceInstanceFound.MicroserviceDefId
		msgPrinter.Printf("Found service %v with service id %v.", serviceInstanceFound.SpecRef, instanceId)
		msgPrinter.Println()
	} else {
		if serviceVersion == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Service %v is not running on the node.", refUrl))
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Service %v version %v is not running on the node.", refUrl, serviceVersion))
		}
	}

	// Check service's log-driver to read logs from correct place
	containerFound := false
	var nonDefaultLogDriverUsed bool
	for _, def := range runningServices.Definitions["active"] {
		if def.Id == msdefId {
			if def.Deployment != "" {
				deployment := &containermessage.DeploymentDescription{}
				if err := json.Unmarshal([]byte(def.Deployment), deployment); err != nil {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Deployment unmarshalling error: %v", err))
				}

				if len(deployment.Services) > 1 && containerName == "" {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Service definition %v consists of more than one container. Please specify the container name using the -c flag.", serviceName))
				}
				for deployedContainerName, service := range deployment.Services {
					if containerName == deployedContainerName || (containerName == "" && len(deployment.Services) == 1) {
						var err error
						if nonDefaultLogDriverUsed, err = cliutils.ChekServiceLogPossibility(service.LogDriver); err != nil {
							cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Service logs are unavailable: %v", err))
						}
						containerName = deployedContainerName
						containerFound = true
						break
					}
				}
			}
			break
		}
	}
	if !containerFound && containerName != "" {
		if serviceVersion == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Container %v is not running as part of service %v.", containerName, serviceName))
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Container %v is not running as part of service %v version.", containerName, serviceName, serviceVersion))
		}
	} else if !containerFound {
		if serviceVersion == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Could not find service %v running on the node.", serviceName))
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Could not find service %v version %v running on the node.", serviceName, serviceVersion))
		}
	} else {
		msgPrinter.Printf("Displaying log messages of container %v for service %v with service id %v.", containerName, name, instanceId)
		msgPrinter.Println()
		if tailing {
			msgPrinter.Printf("Use ctrl-C to terminate this command.")
			msgPrinter.Println()
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

func Suspend(forceSuspend bool, applyAll bool, serviceOrg string, serviceUrl string, serviceVer string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	msg_part := msgPrinter.Sprintf("all the registered services")
	if !applyAll {
		if serviceOrg != "" {
			if serviceUrl != "" {
				if serviceVer != "" {
					if semanticversion.IsVersionString(serviceVer) {
						msg_part = msgPrinter.Sprintf("service %v/%v version %v", serviceOrg, serviceUrl, serviceVer)
					} else {
						cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid version format: %v.", serviceVer))
					}
				} else {
					msg_part = msgPrinter.Sprintf("all the versions for service %v/%v", serviceOrg, serviceUrl)
				}
			} else {
				if serviceVer == "" {
					msg_part = msgPrinter.Sprintf("all the registered services from organization %v", serviceOrg)
				} else {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the service name for version %v.", serviceVer))
				}
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
		serviceVer = ""
	}
	apiInput := exchange.ServiceConfigState{
		Url:         serviceUrl,
		Org:         serviceOrg,
		Version:     serviceVer,
		ConfigState: exchange.SERVICE_CONFIGSTATE_SUSPENDED,
	}

	httpCode, respBody, err := cliutils.HorizonPutPost(http.MethodPost, "service/configstate", []int{201, 200, 400}, apiInput, false)
	if httpCode == 200 || httpCode == 201 {
		msgPrinter.Printf("Service suspending request successfully sent, please use 'hzn agreement' and 'docker ps' to make sure the related agreements and service containers are removed. It may take a couple of minutes.")
	} else if httpCode == 400 {
		msgPrinter.Printf("Error returned suspending the service: %v", respBody)
	} else {
		msgPrinter.Printf("Error: %v", err)
	}
	msgPrinter.Println()
}

func Resume(applyAll bool, serviceOrg string, serviceUrl string, serviceVer string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	msg_part := msgPrinter.Sprintf("all the registered services")
	if !applyAll {
		if serviceOrg != "" {
			if serviceUrl != "" {
				if serviceVer != "" {
					if semanticversion.IsVersionString(serviceVer) {
						msg_part = msgPrinter.Sprintf("service %v/%v version %v", serviceOrg, serviceUrl, serviceVer)
					} else {
						cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid version format: %v.", serviceVer))
					}
				} else {
					msg_part = msgPrinter.Sprintf("all the versions for service %v/%v", serviceOrg, serviceUrl)
				}
			} else {
				if serviceVer == "" {
					msg_part = msgPrinter.Sprintf("all the registered services from organization %v", serviceOrg)
				} else {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the service name for version %v.", serviceVer))
				}
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
		serviceVer = ""
	}
	apiInput := exchange.ServiceConfigState{
		Url:         serviceUrl,
		Org:         serviceOrg,
		Version:     serviceVer,
		ConfigState: exchange.SERVICE_CONFIGSTATE_ACTIVE,
	}

	httpCode, respBody, err := cliutils.HorizonPutPost(http.MethodPost, "service/configstate", []int{201, 200, 400}, apiInput, false)

	if httpCode == 200 || httpCode == 201 {
		msgPrinter.Printf("Service resuming request successfully sent, please use 'hzn agreement' and 'docker ps' to make sure the related agreements and service containers are started. It may take a couple of minutes.")
	} else if httpCode == 400 {
		msgPrinter.Printf("Error returned resuming the service: %v", respBody)
	} else {
		msgPrinter.Printf("Error: %v", err)
	}
	msgPrinter.Println()
}
