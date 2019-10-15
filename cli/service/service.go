package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
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

func Log(ServiceName string, ServiceNameTail string) {
	msgPrinter := i18n.GetMessagePrinter()
	RefURL := ServiceName
	if ServiceNameTail != "" {
		RefURL = ServiceNameTail
	}
	var inputs api.AllServices
	cliutils.HorizonGet("service", []int{200}, &inputs, false)
	id_found := 0
	var instance_id string
	for i := 0; i < len(inputs.Instances["active"]); i++ {
		a, err := json.Marshal(inputs.Instances["active"][i])
		if err != nil {
			fmt.Println("error:", err)
		}
		var id api.MicroserviceInstanceOutput
		json.Unmarshal((a), &id)
		if strings.Contains(id.SpecRef, RefURL) {
			instance_id = id.InstanceId
			fmt.Println(instance_id)
			id_found = 1
			break
		}
	}
	if id_found == 0 {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("Unable to retreive service ID"))
	}
	for {
		// Open syslog
		file, err := os.Open("/var/log/syslog")
		if err != nil {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("/var/log/syslog could not be opened or does not exist", err))
		}
		// Check file stats - size of file
		fi, err := file.Stat()
		if err != nil {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("/var/log/syslog could not get stats", err))
		}
		file_size := fi.Size()
		defer file.Close()
		reader := bufio.NewReader(file)
		for {
			// open file again to re-check stats in case it was logrotated
			new_file, err := os.Open("/var/log/syslog")
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
			fi, err := new_file.Stat()
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
			new_file_size := fi.Size()
			// check if file was logrotated
			if new_file_size >= file_size {
				file_size = new_file_size
			} else {
				// new file detected
				if ServiceNameTail != "" {
					break
				}
				time.Sleep(30 * time.Second)
			}
			// read line from file
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					time.Sleep(1 * time.Second)
				} else {
					break
				}
			}
			// if keyword matches part of line, print to consol.
			if strings.Contains(line, instance_id) {
				fmt.Print(string(line))
			}
		}
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

	msgPrinter.Printf("Suspending %v, cancelling releated agreements, stopping related service containers...", msg_part)
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

	cliutils.HorizonPutPost(http.MethodPost, "service/configstate", []int{201, 200}, apiInput)

	msgPrinter.Printf("Service suspending request sucessfully sent, please use 'hzn agreement' and 'docker ps' to make sure the related agreements and service containers are removed. It may take a couple of minutes.")
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

	cliutils.HorizonPutPost(http.MethodPost, "service/configstate", []int{201, 200}, apiInput)

	msgPrinter.Printf("Service resuming request sucessfully sent, please use 'hzn agreement' and 'docker ps' to make sure the related agreements and service containers are started. It may take a couple of minutes.")
	msgPrinter.Println()
}
