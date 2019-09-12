package unregister

import (
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"time"
)

type ApiAttribute struct {
	Id string `json:"id"`
}

type ApiAttributes struct {
	Attributes []ApiAttribute `json:"attributes"`
}

// DoIt unregisters this Horizon edge node and resets it so it can be registered again
func DoIt(forceUnregister, removeNodeUnregister bool, deepClean bool, timeout int) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if !forceUnregister {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to unregister this Horizon node?"))
	}

	// get the node
	horDevice := api.HorizonDevice{}
	cliutils.HorizonGet("node", []int{200}, &horDevice, false)
	if horDevice.Org == nil || *horDevice.Org == "" {
		msgPrinter.Println("The node is not registered.")

		// still allow deep clean, just in case the node is in a strange state and the user really want to clean it up.
		if deepClean {
			if err := DeepClean(); err != nil {
				fmt.Println(err.Error())
			}
		}
	} else {
		// start unregistering the node
		msgPrinter.Println("Unregistering this node, cancelling all agreements, stopping all workloads, and restarting Horizon...")

		//call horizon DELETE /node api, timeout in 3 minutes.
		unregErr := DeleteHorizonNode(removeNodeUnregister, deepClean, timeout)
		if unregErr != nil {
			fmt.Println(unregErr.Error())
		}

		// deep clean if anax failed to do it
		if deepClean && unregErr != nil {
			if err := DeepClean(); err != nil {
				fmt.Println(err.Error())
			} else {
				unregErr = nil
			}
		}

		// check the new node config state
		if unregErr == nil {
			if err := CheckNodeConfigState(180); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
			} else {
				msgPrinter.Println("Horizon node unregistered. You may now run 'hzn register ...' again, if desired.")
			}
		}
	}
}

//call horizon DELETE /node api, timeout in 3 minutes.
func DeleteHorizonNode(removeNodeUnregister bool, deepClean bool, timeout int) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	removeNodeOption := ""
	if removeNodeUnregister {
		removeNodeOption = "&removeNode=true"
	}
	deepCleanOption := ""
	if deepClean {
		deepCleanOption = "&deepClean=true"
	}

	c := make(chan string, 1)
	go func() {
		_, err := cliutils.HorizonDelete("node?block=true"+removeNodeOption+deepCleanOption, []int{200, 204}, true)
		if err != nil {
			c <- err.Error()
		} else {
			c <- "done"
		}
	}()

	// Block the CLI until the node shutdown process is complete, or timeout occurs. Given an update every 15 seconds.
	channelWait := 15
	totalWait := timeout * 60

	for {
		select {
		case output := <-c:
			if output == "done" {
				cliutils.Verbose(msgPrinter.Sprintf("Horizon node delete call successful with return code: %v", output))
				return nil
			} else {
				return fmt.Errorf("%v", output)
			}
		case <-time.After(time.Duration(channelWait) * time.Second):
			if timeout != 0 {
				totalWait = totalWait - channelWait
				if totalWait <= 0 {
					return fmt.Errorf("Timeout unregistering the node.")
				}
				updateStatus := msgPrinter.Sprintf("Timeout in %v seconds ...", totalWait)
				msgPrinter.Printf("Waiting for Horizon node unregister to complete: %v", updateStatus)
				msgPrinter.Println()
			} else {
				updateStatus := msgPrinter.Sprintf("No Timeout specified ...")
				msgPrinter.Printf("Waiting for Horizon node unregister to complete: %v", updateStatus)
				msgPrinter.Println()
			}

		}
	}
}

// remove local db, policy files and all the service containers
func DeepClean() error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Println("Starting external deep clean ...")
	cliutils.Verbose(msgPrinter.Sprintf("Stopping horizon..."))
	cliutils.RunCmd(nil, "systemctl", "stop", "horizon.service")

	msgPrinter.Println("Deleting local horizon DB...")
	cliutils.RunCmd(nil, "bash", "-c", "rm -f /var/horizon/*.db")
	cliutils.RunCmd(nil, "bash", "-c", "rm -Rf /etc/horizon/policy.d/*")

	msgPrinter.Println("Deleting service containers...")
	if err := RemoveServiceContainers(); err != nil {
		fmt.Printf(err.Error())
	}

	msgPrinter.Println("Starting horizon...")
	cliutils.RunCmd(nil, "systemctl", "start", "horizon.service")
	return nil
}

// make sure the configuration state is back to "unconfigured"
func CheckNodeConfigState(timeout uint64) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Println("Checking the node configuration state...")
	now := uint64(time.Now().Unix())
	for uint64(time.Now().Unix())-now < timeout {
		horDevice := api.HorizonDevice{}
		_, err := cliutils.HorizonGet("node", []int{200}, &horDevice, true)
		if err == nil && horDevice.Config != nil && horDevice.Config.State != nil {
			cliutils.Verbose(msgPrinter.Sprintf("Node configuration state: %v", *horDevice.Config.State))
			if *horDevice.Config.State == "unconfigured" {
				return nil
			}
		}
		time.Sleep(time.Duration(3) * time.Second)
	}
	return fmt.Errorf(msgPrinter.Sprintf("Timeout waiting for node change to 'unconfigured' state."))
}

// Remove all the horizon service containers and networks.
// Note: it will also remove any containers from another horizon instance
// if there are multiple horizon running on the same node.
func RemoveServiceContainers() error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get docker client
	dockerEP := "unix:///var/run/docker.sock"
	client, derr := docker.NewClient(dockerEP)
	if derr != nil {
		return derr
	}

	// get all the containers
	listOptions := docker.ListContainersOptions{All: true, Filters: map[string][]string{}}
	containers, err := client.ListContainers(listOptions)
	if err != nil {
		return fmt.Errorf(msgPrinter.Sprintf("unable to list containers, %v", err))
	}

	if containers == nil || len(containers) == 0 {
		return nil
	}

	err_string := ""
	for _, c := range containers {
		if c.Labels == nil || len(c.Labels) == 0 {
			continue
		} else {
			for k, _ := range c.Labels {
				if k == "openhorizon.anax.service_name" {
					if err := client.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, RemoveVolumes: true, Force: true}); err != nil {
						err_string += msgPrinter.Sprintf("Error deleting container %v. %v\n", c.Names[0], err)
					} else {
						cliutils.Verbose(msgPrinter.Sprintf("Removed service container: %v", c.Names[0]))
					}
					break
				}
			}
		}
	}

	// remove all the unused docker networks
	if _, err := client.PruneNetworks(docker.PruneNetworksOptions{}); err != nil {
		err_string += msgPrinter.Sprintf("Error pruning docker networks. %v\n", err)
	}

	if err_string == "" {
		return nil
	} else {
		return fmt.Errorf(err_string)
	}
}
