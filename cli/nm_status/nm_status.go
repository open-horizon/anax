package nm_status

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/i18n"
	"net/http"
	"strings"
)

// List the status of the NMPs for the local node
func List(nmpName string, long bool) {
	msgPrinter := i18n.GetMessagePrinter()

	if nmpName == "*" {
		nmpName = ""
	}

	managementStatuses := new(map[string]*exchangecommon.NodeManagementPolicyStatus)
	var httpCode int
	if nmpName != "" {
		// Get the node management status for a nmp
		httpCode, _ = cliutils.HorizonGet("nodemanagement/status/"+nmpName, []int{200, 404}, &managementStatuses, false)
	} else {
		// Get the node management status for a nmp
		httpCode, _ = cliutils.HorizonGet("nodemanagement/status", []int{200, 404}, &managementStatuses, false)
	}

	if httpCode == 404 {
		fmt.Println("{}")
		return
	}

	// Output the info
	var output string
	var err error
	if long {
		output, err = cliutils.DisplayAsJson(managementStatuses)
	} else {
		nmpStatusNames := make(map[string]string, 0)
		for nmpStatusName, nmpStatus := range *managementStatuses {
			nmpStatusNames[nmpStatusName] = nmpStatus.Status()
		}
		output, err = cliutils.DisplayAsJson(nmpStatusNames)
	}

	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn nmp status' output: %v", err))
	}
	fmt.Println(output)
}

// Re-evaluate the given NMPs for the local node
func Reset(nmpName string) {
	msgPrinter := i18n.GetMessagePrinter()

	if nmpName == "*" {
		nmpName = ""
	}

	if nmpName != "" {
		// re-evaluate the given nmp
		cliutils.HorizonPutPost(http.MethodPut, "nodemanagement/reset/"+nmpName, []int{201, 200}, nil, true)
		msgPrinter.Printf("The following node management policy will be re-evaluated: %v.", nmpName)
	} else {
		// Get all the node management status for this node
		managementStatuses := new(map[string]*exchangecommon.NodeManagementPolicyStatus)
		httpCode, _ := cliutils.HorizonGet("nodemanagement/status", []int{200, 404}, &managementStatuses, false)
		if httpCode == 404 {
			msgPrinter.Printf("No node management policy for this node.")
		} else {
			allIDs := []string{}
			for nmpID, _ := range *managementStatuses {
				allIDs = append(allIDs, nmpID)
			}

			cliutils.HorizonPutPost(http.MethodPut, "nodemanagement/reset", []int{201, 200}, nil, true)

			if len(allIDs) == 1 {
				msgPrinter.Printf("The following node management policy will be re-evaluated: %v.", allIDs[0])
			} else {
				msgPrinter.Printf("The following node management policies will be re-evaluated: %v.", strings.Join(allIDs, ","))
			}
		}
	}

	msgPrinter.Println()
}
