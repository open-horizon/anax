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
	"runtime"
	"os/exec"
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
	refUrl := serviceName
	// Get the list of running services from the agent.
	type AllServices struct {
		Instances map[string][]api.MicroserviceInstanceOutput `json:"instances"` // The service instances that are running
	}
	var runningServices AllServices
	cliutils.HorizonGet("service", []int{200}, &runningServices, false)
	// Search the list of services to find one that matches the input service name. The service's instance Id
	// is what appears in the syslog, so we need to save that.
	serviceFound := false
	var instanceId string
	for _, serviceInstance := range runningServices.Instances["active"] {
		if strings.Contains(serviceInstance.SpecRef, refUrl) {
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
	if runtime.GOOS == "darwin" {
		dockerCommand := "docker logs $(docker ps -q --filter name="+instanceId+")"
		if tailing {
			dockerCommand = "docker logs -f $(docker ps -q --filter name="+instanceId+")"
		}
		fmt.Print(dockerCommand)
		fmt.Print("\n")
		cmd := exec.Command("/bin/sh", "-c", dockerCommand)
		cmdReader, err := cmd.StdoutPipe()
		if err != nil {
			cliutils.Fatal(cliutils.EXEC_CMD_ERROR, msgPrinter.Sprintf("Error creating StdoutPipe for command: %v", err))
		}
		// Assign a single pipe to Command.Stdout and Command.Stderr
		cmd.Stderr = cmd.Stdout
		scanner := bufio.NewScanner(cmdReader)
		// Goroutine to print Stdout and Stderr while Docker logs command is running
		go func() {
			for scanner.Scan() {
				msg := scanner.Text()
				fmt.Println(msg)
			}
		}()
		err = cmd.Start()
		if err != nil {
			cliutils.Fatal(cliutils.EXEC_CMD_ERROR, msgPrinter.Sprintf("Error starting command: %v", err))
		}
		err = cmd.Wait()
		if err != nil {
			cliutils.Fatal(cliutils.EXEC_CMD_ERROR, msgPrinter.Sprintf("Error waiting for command: %v", err))
		}
		return
	}
	// The requested service is running, so grab the records from syslog for this service.
	file, err := os.Open("/var/log/syslog")
	if err != nil {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("/var/log/syslog could not be opened or does not exist: %v", err))
	}
	defer file.Close()
	// Check file stats and capture the current size of the file if we will be tailing it.
	var file_size int64
	if tailing {
		fi, err := file.Stat()
		if err != nil {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("/var/log/syslog could not get stats: %v", err))
		}
		file_size = fi.Size()
	}
	// Setup a file reader
	reader := bufio.NewReader(file)
	// Start reading records. The syslog could be rotated while we're tailing it. Log rotation occurs when
	// the current syslog file reaches its maximum size. When this happens, the current syslog is copied
	// to another file and a new (empty) syslog file is created. The only way we can tell that a log
	// rotation happened is when the size of the file gets smaller as we are reading it.
	for {
		// Get a record (delimited by EOL) from syslog.
		if line, err := reader.ReadString('\n'); err != nil {
			// Any error we get back, even EOF, is treated the same if we are not tailing. Just return to the caller.
			if !tailing {
				return
			}
			// When we're tailing and we hit EOF, briefly sleep to allow more records to appear in syslog.
			if err == io.EOF {
				time.Sleep(1 * time.Second)
			} else {
				// If the error is not EOF then we assume the error is due to log rotation so we silently
				// ignore the error and keep trying.
				cliutils.Verbose(msgPrinter.Sprintf("Error reading from /var/log/syslog: %v", err))
			}
		} else if strings.Contains(line, instanceId) {
			// If the requested service id is in the current syslog record, display it.
			fmt.Print(string(line))
		}
		// Re-check syslog file size via stats in case syslog was logrotated.
		// If were tailing and there was a non-EOF error, we will always come here.
		if tailing {
			fi_new, err := os.Stat("/var/log/syslog")
			if err != nil {
				cliutils.Verbose(msgPrinter.Sprintf("Unable to state /var/log/syslog: %v", err))
				time.Sleep(1 * time.Second)
				continue
			}
			new_file_size := fi_new.Size()
			// If syslog is smaller than the last time we checked, then a log rotation has occurred.
			if new_file_size >= file_size {
				file_size = new_file_size
			} else {
				// Log rotation has occurred. Re-open the new syslog file and capture the current size.
				file.Close()
				file, err = os.Open("/var/log/syslog")
				if err != nil {
					cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("/var/log/syslog could not be opened or does not exist: %v", err))
				}
				defer file.Close()
				// Setup a new reader on the new file.
				reader = bufio.NewReader(file)
				fi, err := file.Stat()
				if err != nil {
					cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("/var/log/syslog could not get stats: %v", err))
				}
				file_size = fi.Size()
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
