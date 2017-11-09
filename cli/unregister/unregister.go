package unregister

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"strings"
	"os"
)

type ApiAttribute struct {
	Id string	`json:"id"`
}

type ApiAttributes struct {
	Attributes []ApiAttribute	`json:"attributes"`
}


// DoIt unregisters this Horizon edge node and resets it so it can be registered again
func DoIt(forceUnregister, removeNodeUnregister bool) {
	if !forceUnregister {
		// Prompt the user to make sure he/she wants to do this
		fmt.Print("Are you sure you want to unregister this Horizon node? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.TrimSpace(response) != "y" {
			fmt.Println("Exiting.")
			os.Exit(0)
		}
	}

	fmt.Println("Unregistering this node, cancelling all agreements, stopping all workloads, and restarting Horizon...")

	removeNodeOption := ""
	if removeNodeUnregister { removeNodeOption = "&removeNode=true" }

	cliutils.HorizonDelete("horizondevice?block=true"+removeNodeOption, []int{200,204})
	fmt.Println("Horizon node unregistered.")

	/* This does the same thing more manually. Want to keep for reference...
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
	*/
}
