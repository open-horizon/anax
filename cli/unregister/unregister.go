package unregister

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
)

type ApiAttribute struct {
	Id string	`json:"id"`
}

type ApiAttributes struct {
	Attributes []ApiAttribute	`json:"attributes"`
}


// DoIt unregisters this Horizon edge node and resets it so it can be registered again
//todo: currently this is implemented quickly in a hacky way, because we will soon switch to use the unconfigure api
func DoIt() {
	/*
	// Get the list of attribute resources and delete each
	fmt.Println("Removing attributes...")
	apiOutput := ApiAttributes{}
	cliutils.HorizonGet("attribute", 200, &apiOutput)
	for _, a := range apiOutput.Attributes {
		cliutils.HorizonDelete("attribute/"+a.Id, []int{200})
	}
	*/

	//stdoutBytes, _ := cliutils.RunCmd(nil, "bash", "-c", "ls /Users/bp/*")
	//fmt.Printf("output: %s\n", string(stdoutBytes))

	fmt.Println("Stopping horizon...")
	cliutils.RunCmd(nil, "systemctl", "stop", "horizon.service")

	fmt.Println("Stopping workload and microservice containers...")
	stdoutBytes, _ := cliutils.RunCmd(nil, "docker", "ps", "-qa")
	cliutils.RunCmd(stdoutBytes, "xargs", "docker", "stop")

	fmt.Println("Deleting local horizon DB...")
	cliutils.RunCmd(nil, "bash", "-c", "rm -f /var/horizon/*.db")
	cliutils.RunCmd(nil, "bash", "-c", "rm -Rf /etc/horizon/policy.d/*")

	fmt.Println("Starting horizon...")
	cliutils.RunCmd(nil, "systemctl", "start", "horizon.service")

	fmt.Println("Done.")
}
