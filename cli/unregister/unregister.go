package unregister

import (
	"encoding/json"
	"errors"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/agreement"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"net/http"
	"os"
	"path"
	"strings"
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

	if deepClean {
		err := backupEventLogs()
		if err != nil {
			msgPrinter.Printf("Cannot backup eventlogs: %v", err)
			msgPrinter.Println()
		}
	}

	if horDevice.Org == nil || *horDevice.Org == "" {
		msgPrinter.Printf("The node is not registered.")
		msgPrinter.Println()

		// still allow deep clean, just in case the node is in a strange state and the user really want to clean it up.
		if deepClean {
			if err := DeepClean(); err != nil {
				fmt.Println(err.Error())
			}
		}
	} else {
		// start unregistering the node
		msgPrinter.Printf("Unregistering this node, cancelling all agreements, stopping all workloads, and restarting Horizon...")
		msgPrinter.Println()

		// call horizon DELETE /node api, default timeout is to wait forever.
		unregErr := DeleteHorizonNode(removeNodeUnregister, deepClean, timeout)

		// deep clean if anax failed to do it
		if unregErr != nil {
			if deepClean {
				// Don't show anax's node state errors, as we can do external deepClean
				if !strings.Contains(unregErr.Error(), "INVALID_NODE_STATE") {
					msgPrinter.Printf("Node unregistering using anax API failed. External deep clean will be attempted. Specific anax API error is: %v", unregErr.Error())
					msgPrinter.Println()
				}

				if err := DeepClean(); err != nil {
					fmt.Println(err.Error())
				} else {
					unregErr = nil
				}
			} else {
				msgPrinter.Printf("The node was not successfully unregistered, please use 'hzn unregister -D' to ensure the node is completely reset. Specific anax API error is: %v", unregErr.Error())
				msgPrinter.Println()
			}
		}

		// check the new node config state
		if unregErr == nil {
			if err := CheckNodeConfigState(180); err != nil {
				if !deepClean {
					errmsg := msgPrinter.Sprintf("%v\nThe node was not successfully unregistered, please use 'hzn unregister -D' to ensure the node is completely reset.", err)
					cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, errmsg)
				} else {
					cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
				}
			} else {
				msgPrinter.Printf("Horizon node unregistered. You may now run 'hzn register ...' again, if desired.")
				msgPrinter.Println()
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
		httpCode, err := cliutils.HorizonDelete("node?block=true"+removeNodeOption+deepCleanOption, []int{200, 204}, []int{503}, true)
		if httpCode == http.StatusServiceUnavailable {
			msgPrinter.Printf("WARNING: The node is unregistered, but an error occurred during unregistration.")
			msgPrinter.Println()
			msgPrinter.Printf("The error was: %v", err)
			msgPrinter.Println()
			c <- "partial"
		} else if err != nil {
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
			} else if output == "partial" {
				return nil
			} else {
				return fmt.Errorf("%v", output)
			}
		case <-time.After(time.Duration(channelWait) * time.Second):

			// Get a list of all agreements so that we can display progress.
			ags := agreement.GetAgreements(false)

			if timeout != 0 {
				totalWait = totalWait - channelWait
				if totalWait <= 0 {
					return fmt.Errorf("Timeout unregistering the node.")
				}
				updateStatus := msgPrinter.Sprintf("Timeout in %v seconds ...", totalWait)
				msgPrinter.Printf("Waiting for Horizon node unregister to complete: %v", updateStatus)
				msgPrinter.Println()
				if len(ags) != 0 {
					msgPrinter.Printf("%v agreements remaining to be terminated.", len(ags))
					msgPrinter.Println()
				}
			} else {
				updateStatus := msgPrinter.Sprintf("No Timeout specified ...")
				msgPrinter.Printf("Waiting for Horizon node unregister to complete: %v", updateStatus)
				msgPrinter.Println()
				if len(ags) != 0 {
					msgPrinter.Printf("%v agreements remaining to be terminated.", len(ags))
					msgPrinter.Println()
				}
			}

		}
	}
}

// remove local db, policy files and all the service containers
func DeepClean() error {

	// detect the node type
	nodeType := persistence.DEVICE_TYPE_DEVICE
	if _, err := cutil.NewKubeConfig(); err == nil {
		nodeType = persistence.DEVICE_TYPE_CLUSTER
	}

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Printf("Starting external deep clean ...")
	msgPrinter.Println()

	if nodeType == persistence.DEVICE_TYPE_CLUSTER {
		msgPrinter.Printf("Deleting local horizon DB...")
		msgPrinter.Println()
		cliutils.RunCmd(nil, "bash", "-c", "rm -f /var/horizon/*.db")
		cliutils.RunCmd(nil, "bash", "-c", "rm -Rf /etc/horizon/policy.d/*")

		// kill anax inside the agent container, it will get restarted by the nax.service script
		msgPrinter.Printf("Restarting anax...")
		msgPrinter.Println()
		cliutils.RunCmd(nil, "pkill", "-f", "/usr/horizon/bin/anax")

	} else {
		cliutils.Verbose(msgPrinter.Sprintf("Stopping horizon..."))
		cliutils.RunCmd(nil, "systemctl", "stop", "horizon.service")

		msgPrinter.Printf("Deleting local horizon DB...")
		msgPrinter.Println()
		cliutils.RunCmd(nil, "bash", "-c", "rm -f /var/horizon/*.db")
		cliutils.RunCmd(nil, "bash", "-c", "rm -Rf /etc/horizon/policy.d/*")

		msgPrinter.Printf("Deleting service containers...")
		msgPrinter.Println()
		if err := RemoveServiceContainers(); err != nil {
			fmt.Printf(err.Error())
		}

		msgPrinter.Printf("Starting horizon...")
		msgPrinter.Println()
		cliutils.RunCmd(nil, "systemctl", "start", "horizon.service")

	}

	return nil
}

// make sure the configuration state is back to "unconfigured"
func CheckNodeConfigState(timeout uint64) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Printf("Waiting for agent service to restart and checking the node configuration state...")
	msgPrinter.Println()
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

// backupEventLogs loads eventlogs from eventlog API , marshals them into the JSON format
// and saves the bkp file into horizon folder with name of backup time
func backupEventLogs() error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Println("Backing up eventlogs...")

	// get the eventlog from anax
	elogs := make([]persistence.EventLogRaw, 0)
	cliutils.HorizonGet("eventlog/all", []int{200}, &elogs, false)

	elogsJson, err := json.MarshalIndent(elogs, "", cliutils.JSON_INDENT)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("cannot marshal eventlogs from local anax DB, eventlogs will not be saved"))
	}

	fileName := path.Join("/tmp/", fmt.Sprintf("eventlogs_bkp_%s.txt", time.Now().Format(time.RFC3339)))
	file, err := os.Create(fileName)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("failed to backup eventlogs file %v. %v", fileName, err))
	}
	defer file.Close()

	if _, err := file.Write(elogsJson); err != nil {
		return errors.New(msgPrinter.Sprintf("failed to save the eventlogs to file %v. %v", fileName, err))
	}

	msgPrinter.Printf("Saved eventlog into file %s", fileName)
	msgPrinter.Println()

	return nil
}
