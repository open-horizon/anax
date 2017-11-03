package register

import (
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/api"
	"strings"
	"github.com/open-horizon/anax/exchange"
	"fmt"
)


// DoIt registers this node to Horizon
func DoIt(org string, nodeId string, nodeToken string, pattern string, userPw string, inputFile string) {
	// Get the exchange url from the anax api
	status := api.Info{}
	cliutils.HorizonGet("status", 200, &status)
	exchUrlBase := strings.TrimSuffix(status.Configuration.ExchangeAPI, "/")
	fmt.Printf("Horizon Exchange base URL: %s\n", exchUrlBase)

	// See if the node exists in the exchange
	node := exchange.GetDevicesResponse{}
	httpCode := cliutils.ExchangeGet(exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, org+"/"+nodeId+":"+nodeToken, 0, &node)
	//cliutils.Verbose("node: %v", node)
	if httpCode != 200 {
		if userPw == "" { cliutils.Fatal(3, "node %s/%s does not exist in the exchange and the -u flag was not specified to provide exchange user credentials to create it.", org, nodeId) }
		fmt.Printf("Node %s/%s does not exists in the exchange, creating it...\n", org, nodeId)
		putNodeReq := exchange.PutDeviceRequest{Token: nodeToken, Name: nodeId, SoftwareVersions: make(map[string]string), PublicKey: []byte("")}		// we only need to set the token
		cliutils.ExchangePutPost("PUT", exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, org+"/"+userPw, 201, putNodeReq)
	} else {
		fmt.Printf("Node %s/%s exists in the exchange\n", org, nodeId)
	}

	// Read input file and call /service and /workloadconfig to set the specified variables
	fmt.Printf("Setting microservice and workload variables from %s...\n", inputFile)

	// Set the pattern and register the node
	fmt.Printf("Setting pattern %s and registering this node with Horizon...\n", pattern)
}
