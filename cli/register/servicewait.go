package register

import (
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/agreement"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"strings"
	"time"
)

// Wait for the specified service to start running on this node.
func WaitForService(org string, waitService string, waitTimeout int) {

	const UpdateThreshold = 5    // How many service check iterations before updating the user with a msg on the console.
	const ServiceUpThreshold = 5 // How many service check iterations before deciding that the service is up.

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Verify that the input makes sense.
	if waitTimeout < 0 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("--timeout must be a positive integer."))
	}

	// 1. Wait for the /service API to return a service with url that matches the input
	// 2. While waiting, report when at least 1 agreement is formed

	msgPrinter.Printf("Waiting for up to %v seconds for service %v/%v to start...", waitTimeout, org, waitService)
	msgPrinter.Println()

	// Save the most recent set of services here.
	services := api.AllServices{}

	// Start monitoring the agent's /service API, looking for the presence of the input waitService.
	updateCounter := UpdateThreshold
	serviceUp := 0
	serviceFailed := false
	now := uint64(time.Now().Unix())
	for (uint64(time.Now().Unix())-now < uint64(waitTimeout) || serviceUp > 0) && !serviceFailed {
		time.Sleep(time.Duration(3) * time.Second)
		if _, err := cliutils.HorizonGet("service", []int{200}, &services, true); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
		}

		// Active services are services that have at least been started. When the execution time becomes non-zero
		// it means the service container is started. The container could still fail quickly after it is started.
		instances := services.Instances["active"]
		for _, serviceInstance := range instances {

			if !(serviceInstance.SpecRef == waitService && serviceInstance.Org == org) {
				// Skip elements for other services
				continue

			} else if serviceInstance.ExecutionStartTime != 0 {
				// The target service is started. If stays up then declare victory and return.
				if serviceUp >= ServiceUpThreshold {
					msgPrinter.Printf("Service %v/%v is started.", org, waitService)
					msgPrinter.Println()
					return
				}

				// The service could fail quickly if we happened to catch it just as it was starting, so make sure
				// the service stays up.
				serviceUp += 1

			} else if serviceUp > 0 {
				// The service has been up for at least 1 iteration, so it's absence means that it failed.
				serviceUp = 0
				msgPrinter.Printf("The service %v/%v has failed.", org, waitService)
				msgPrinter.Println()
				serviceFailed = true
			}

			break
		}

		// Service is not there yet. Update the user on progress, and wait for a bit.
		updateCounter = updateCounter - 1
		if updateCounter <= 0 && !serviceFailed {
			updateCounter = UpdateThreshold
			msgPrinter.Printf("Waiting for service %v/%v to start executing.", org, waitService)
			msgPrinter.Println()
		}
	}

	// If we got to this point, then there is a problem.
	msgPrinter.Printf("Timeout waiting for service %v/%v to successfully start. Analyzing possible reasons for the timeout...", org, waitService)
	msgPrinter.Println()

	// Let's see if we can provide the user with some help figuring out what's going on.
	found := false
	for _, serviceInstance := range services.Instances["active"] {

		// 1. Maybe the service is there but just hasnt started yet.
		if serviceInstance.SpecRef == waitService && serviceInstance.Org == org {
			msgPrinter.Printf("Service %v/%v is deployed to the node, but not executing yet.", org, waitService)
			msgPrinter.Println()
			found = true

			// 2. Maybe the service has encountered an error.
			if serviceInstance.ExecutionStartTime == 0 && serviceInstance.ExecutionFailureCode != 0 {
				msgPrinter.Printf("Service %v/%v execution failed: %v.", org, waitService, serviceInstance.ExecutionFailureDesc)
				msgPrinter.Println()
				serviceFailed = true
			} else {
				msgPrinter.Printf("Service %v/%v might need more time to start executing, continuing analysis.", org, waitService)
				msgPrinter.Println()
			}
			break

		}
	}

	// 3. The service might not even be there at all.
	if !found {
		msgPrinter.Printf("Service %v/%v is not deployed to the node, continuing analysis.", org, waitService)
		msgPrinter.Println()
	}

	// 4. Are there any agreements being made? Check for only non-archived agreements. Skip this if we know the service failed
	// because we know there are agreements.
	if !serviceFailed {
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
		msgPrinter.Printf("Use the 'hzn deploycheck all' command to verify that node, service, pattern and policy configuration is compatible.")
		msgPrinter.Println()
	} else {
		msgPrinter.Printf("The following errors were found in the node's event log and are related to %v/%v. Use 'hzn eventlog list -s severity=error -l' to see the full detail of the errors.", org, waitService)
		msgPrinter.Println()

		// Scan the log for events related to the service we're waiting for.
		sel := persistence.Selector{
			Op:         "=",
			MatchValue: waitService,
		}
		match := make(map[string][]persistence.Selector)
		match["service_url"] = []persistence.Selector{sel}

		for _, el := range eLogs {
			t := time.Unix(int64(el.Timestamp), 0)
			printLog := false
			if strings.Contains(el.Message, waitService) {
				printLog = true
			} else if es, err := persistence.GetRealEventSource(el.SourceType, el.Source); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "unable to convert eventlog source, error: %v", err)
			} else if (*es).Matches(match) {
				printLog = true
			}

			// Put relevant events on the console.
			if printLog {
				msgPrinter.Printf("%v: %v", t.Format("2006-01-02 15:04:05"), el.Message)
				msgPrinter.Println()
			}
		}
	}

	// Done analyzing
	msgPrinter.Printf("Analysis complete.")
	msgPrinter.Println()

	return
}
