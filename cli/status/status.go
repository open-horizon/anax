package status

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/worker"
	"os"
)

func getStatus(agbot bool) (apiOutput *worker.WorkerStatusManager) {
	apiOutput = worker.NewWorkerStatusManager()

	if agbot {
		// set env to call agbot url
		os.Setenv("HORIZON_URL", cliutils.AGBOT_HZN_API)
	}

	// Get horizon api worker status
	cliutils.HorizonGet("status/workers", []int{200}, apiOutput)

	return
}

// Display status for node or agbot
func DisplayStatus(details bool, agbot bool) {
	status := getStatus(agbot)

	if details {
		jsonBytes, err := json.MarshalIndent(status, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn status -l' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		workers := make(map[string]map[string]*worker.WorkerStatus)
		workers["workers"] = status.Workers

		jsonBytes, err := json.MarshalIndent(workers, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn status' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	}
}
