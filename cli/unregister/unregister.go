package unregister

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
)

type ApiAttribute struct {
	Id string `json:"id"`
}

type ApiAttributes struct {
	Attributes []ApiAttribute `json:"attributes"`
}

// DoIt unregisters this Horizon edge node and resets it so it can be registered again
func DoIt(forceUnregister, removeNodeUnregister bool) {
	if !forceUnregister {
		cliutils.ConfirmRemove("Are you sure you want to unregister this Horizon node?")
	}

	fmt.Println("Unregistering this node, cancelling all agreements, stopping all workloads, and restarting Horizon...")

	removeNodeOption := ""
	if removeNodeUnregister {
		removeNodeOption = "&removeNode=true"
	}

	cliutils.HorizonDelete("node?block=true"+removeNodeOption, []int{200, 204})
	fmt.Println("Horizon node unregistered. You may now run 'hzn register ...' again, if desired.")

	/* This does the same thing more manually. Want to keep for reference...
	fmt.Println("Stopping horizon...")
	cliutils.RunCmd(nil, "systemctl", "stop", "horizon.service")
	fmt.Println("Stopping service containers...")
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
