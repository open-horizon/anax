package register

import (
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/agreement"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/deploycheck"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"strings"
	"time"
)

type serviceSpec struct {
	name      string
	org       string
	status    WaitingStatus
	serviceUp int
}

// Check if service type and node type match
func CheckService(name string, org string, arch string, version string, nodeType string, userOrg string, userPw string) bool {
	var services exchange.GetServicesResponse

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get service from exchange
	id := cutil.FormExchangeIdForService(name, version, arch)
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+id, cliutils.OrgAndCreds(userOrg, userPw), []int{200, 404}, &services)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("service '%s' not found in org %s", id, org))
	}
	// Check in the service type
	for _, s := range services.Services {
		serviceType := s.GetServiceType()
		return nodeType == serviceType || serviceType == exchange.SERVICE_TYPE_BOTH
	}
	return false
}

type WaitingStatus int

const (
	Failed WaitingStatus = iota - 1
	NoAgreements
	AgreementFormed
	AgreementAccepted
	ServiceCreated
	ExecutionStarted
	Success
)

func DisplayServiceStatus(servSpecArr []serviceSpec, displayStarOrg bool) {
	statusDetails := []string{
		"Failed",
		"Waiting: no agreements formed yet",
		"Waiting: agreement is formed",
		"Waiting: agreement is accepted",
		"Waiting: service is created",
		"Waiting: execution is started",
		"Success"}

	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Printf("Status of the services you are watching:")
	msgPrinter.Println()
	for _, ss := range servSpecArr {
		if ss.org != "*" || displayStarOrg {
			msgPrinter.Printf("\t%v/%v \t%v", ss.org, ss.name, statusDetails[ss.status+1])
			msgPrinter.Println()
		}
	}
}

// returns two values:
// allSucces 	- 	true if all services are successful
// needWait 	-	true if any service is still in progress
func ServiceAllSucess(servSpecArr []serviceSpec) (bool, bool) {
	// true if all services have started
	allSuccess := true

	// true if any service has not shown up yet
	needWait := false

	// for each service that we are monitoring, check if all succeeds
	for _, ss := range servSpecArr {
		if ss.status != Success {
			allSuccess = false
			needWait = true

		} else if ss.status == Failed {
			allSuccess = false
		}
	}

	return allSuccess, needWait
}

// Wait for the specified service to start running on this node.
func WaitForService(org string, waitService string, waitTimeout int, pattern string, pat exchange.Pattern, nodeType string, nodeArch, userOrg string, userPw string) {

	const UpdateThreshold = 5    // How many service check iterations before updating the user with a msg on the console.
	const ServiceUpThreshold = 5 // How many service check iterations before deciding that the service is up.

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Verify that the input makes sense.
	if waitTimeout < 0 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("--timeout must be a positive integer."))
	}

	servSpecArr := []serviceSpec{}
	if pattern != "" {
		for _, s := range pat.Services {
			if (waitService == "*" || waitService == s.ServiceURL) && (org == "*" || org == s.ServiceOrg) && (s.ServiceArch == "*" || nodeArch == s.ServiceArch) {
				if len(s.ServiceVersions) > 0 && CheckService(s.ServiceURL, s.ServiceOrg, nodeArch, s.ServiceVersions[0].Version, nodeType, userOrg, userPw) {
					servSpecArr = append(servSpecArr, serviceSpec{name: s.ServiceURL, org: s.ServiceOrg, status: 0, serviceUp: 0})
				}
			}
		}
	} else {
		servSpecArr = append(servSpecArr, serviceSpec{name: waitService, org: org, status: 0, serviceUp: 0})
	}

	// 1. Wait for the /service API to return a service with url that matches the input
	// 2. While waiting, report when at least 1 agreement is formed

	msgPrinter.Printf("Waiting for up to %v seconds for following services to start:", waitTimeout)
	msgPrinter.Println()

	for _, ss := range servSpecArr {
		msgPrinter.Printf("\t%v/%v", ss.org, ss.name)
		msgPrinter.Println()

	}
	// Save the most recent set of services here.
	services := api.AllServices{}

	// Start monitoring the agent's /service API, looking for the presence of the input waitService.
	updateCounter := UpdateThreshold
	now := uint64(time.Now().Unix())
	for uint64(time.Now().Unix())-now < uint64(waitTimeout) {
		time.Sleep(time.Duration(3) * time.Second)
		if _, err := cliutils.HorizonGet("service", []int{200}, &services, true); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
		}

		// Active services are services that have at least been started. When the execution time becomes non-zero
		// it means the servie container is started. The container could still fail quickly after it is started.
		instances := services.Instances["active"]

		// Get active agreements to check status of services
		ags := agreement.GetAgreements(false)

		serviceStatusUpdated := false

		// for each service that we are monitoring, check related agreements and if the services are up
		for i, ss := range servSpecArr {
			serviceStatus := NoAgreements
			for _, ag := range ags {
				if ag.RunningWorkload.URL == ss.name && (ag.RunningWorkload.Org == ss.org || ss.org == "*") {
					if ag.AgreementAcceptedTime == 0 {
						serviceStatus = AgreementFormed
					} else {
						serviceStatus = AgreementAccepted
					}
				}
			}
			if ss.status >= AgreementAccepted {
				for _, serviceInstance := range instances {
					if serviceInstance.SpecRef == ss.name && (serviceInstance.Org == ss.org || ss.org == "*") {
						serviceStatus = ServiceCreated

						if ss.org == "*" {
							servSpecArr[i].org = serviceInstance.Org
						}

						if serviceInstance.ExecutionStartTime != 0 {
							serviceStatus = ExecutionStarted
							// If the service stays up then declare victory and return.
							if ss.serviceUp >= ServiceUpThreshold {
								serviceStatus = Success
							}

							// The service could fail quickly if we happened to catch it just as it was starting, so make sure
							// the service stays up.
							servSpecArr[i].serviceUp += 1
							break

						} else if ss.serviceUp > 0 {
							// The service has been up for at least 1 iteration, so it's absence means that it failed.
							servSpecArr[i].serviceUp = 0
							serviceStatus = Failed
							break
						}
					}
				}
			}
			if ss.status != serviceStatus {
				serviceStatusUpdated = true
				servSpecArr[i].status = serviceStatus
			}
		}

		// decrement or update updateCounter
		updateCounter = updateCounter - 1

		// exit if all services are started successfully
		allSuccess, needWait := ServiceAllSucess(servSpecArr)
		if allSuccess {
			DisplayServiceStatus(servSpecArr, false)
			return
		}

		if updateCounter <= 0 || serviceStatusUpdated {
			DisplayServiceStatus(servSpecArr, true)
			updateCounter = UpdateThreshold
		}

		if !needWait {
			break
		}
	}

	// If we got to this point, then there is a problem.
	msgPrinter.Printf("Timeout waiting for some services to successfully start. Analyzing possible reasons for the timeout...")
	msgPrinter.Println()

	// Save services to an array based on their status
	failedArr, failedDescArr, delayedArr, notDeployedArr := []serviceSpec{}, []string{}, []serviceSpec{}, []serviceSpec{}

	// Let's see if we can provide the user with some help figuring out what's going on.
	for i, ss := range servSpecArr {
		if ss.status <= 0 {
			found := false
			for _, serviceInstance := range services.Instances["active"] {
				// 1. Maybe the service is there but just hasnt started yet.
				if serviceInstance.SpecRef == ss.name && serviceInstance.Org == ss.org {
					found = true

					// 2. Maybe the service has encountered an error.
					if serviceInstance.ExecutionStartTime == 0 && serviceInstance.ExecutionFailureCode != 0 {
						failedArr = append(failedArr, ss)
						failedDescArr = append(failedDescArr, serviceInstance.ExecutionFailureDesc)
						servSpecArr[i].status = Failed
					} else {
						delayedArr = append(delayedArr, ss)
					}
					break
				}
			}

			// 3. The service might not even be there at all.
			if !found {
				notDeployedArr = append(notDeployedArr, ss)
			}
		}
	}

	// Print the status of each service
	if len(failedArr) > 0 {
		msgPrinter.Printf("The following services failed during execution:")
		msgPrinter.Println()
		for i, ss := range failedArr {
			msgPrinter.Printf("\t%v/%v: %v", ss.org, ss.name, failedDescArr[i])
			msgPrinter.Println()
		}
	}

	if len(delayedArr) > 0 {
		msgPrinter.Printf("The following services might need more time to start executing, continuing analysis:")
		msgPrinter.Println()
		for _, ss := range delayedArr {
			msgPrinter.Printf("\t%v/%v", ss.org, ss.name)
			msgPrinter.Println()
		}
	}

	if len(notDeployedArr) > 0 {
		msgPrinter.Printf("The following services are not deployed to the node, continuing analysis:")
		msgPrinter.Println()
		for _, ss := range notDeployedArr {
			msgPrinter.Printf("\t%v/%v", ss.org, ss.name)
			msgPrinter.Println()
		}
	}

	// 4. Are there any agreements being made? Check for only non-archived agreements. Skip this if we know the service failed
	// because we know there are agreements.
	if len(delayedArr) > 0 || len(notDeployedArr) > 0 {
		msgPrinter.Println()
		ags := agreement.GetAgreements(false)
		if len(ags) != 0 {
			msgPrinter.Printf("Currently, there are %v active agreements on this node. Use `hzn agreement list' to see the agreements that have been formed so far.", len(ags))
			msgPrinter.Println()
		} else {
			msgPrinter.Printf("Currently, there are no active agreements on this node.")
			msgPrinter.Println()
		}
	}
	// 5. Scan the event log for errors related to this service. This should always be done if the service did not come up
	// successfully.
	eLogs := make([]persistence.EventLogRaw, 0)
	cliutils.HorizonGet("eventlog?severity=error", []int{200}, &eLogs, true)
	msgPrinter.Println()
	if len(eLogs) == 0 {
		msgPrinter.Printf("Currently, there are no errors recorded in the node's event log.")
		msgPrinter.Println()
		if pattern == "" {
			msgPrinter.Printf("Use the 'hzn deploycheck all -b' or 'hzn deploycheck all -B' command to verify that node, service configuration and deployment policy is compatible.")
			msgPrinter.Println()
		} else {
			msgPrinter.Printf("Using the 'hzn deploycheck all -p' command to verify that node, service configuration and pattern is compatible.")
			msgPrinter.Println()
			deploycheck.AllCompatible(userOrg, userPw, "", nodeArch, nodeType, "", "",
				"", "", pattern, "", "", []string{}, false, false)
		}
	} else {
		for _, ss := range servSpecArr {
			logArr := []string{}

			if ss.status == Success {
				continue
			}
			// Scan the log for events related to the service we're waiting for.
			var serviceFullName = ss.name
			if ss.org != "*" {
				serviceFullName = fmt.Sprintf("%v/%v", ss.org, ss.name)
			}
			sel := persistence.Selector{
				Op:         "=",
				MatchValue: serviceFullName,
			}
			match := make(map[string][]persistence.Selector)
			match["service_url"] = []persistence.Selector{sel}

			for _, el := range eLogs {
				t := time.Unix(int64(el.Timestamp), 0)
				printLog := false
				if strings.Contains(el.Message, serviceFullName) {
					printLog = true
				} else if es, err := persistence.GetRealEventSource(el.SourceType, el.Source); err != nil {
					cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "unable to convert eventlog source, error: %v", err)
				} else if (*es).Matches(match) {
					printLog = true
				}

				// Put relevant events on the console.
				if printLog {
					logArr = append(logArr, msgPrinter.Sprintf("%v: %v", t.Format("2006-01-02 15:04:05"), el.Message))
				}
			}

			if len(logArr) > 0 {
				msgPrinter.Printf("The following errors were found in the node's event log and are related to %v/%v. Use 'hzn eventlog list -s severity=error -l' to see the full detail of the errors.", ss.org, ss.name)
				msgPrinter.Println()
				for _, log := range logArr {
					fmt.Printf("%v", log)
				}
			}
		}
	}

	// Done analyzing
	msgPrinter.Printf("Analysis complete.")
	msgPrinter.Println()

	return
}
