// Command line interface to the horizon agent. Provide sub-commands to register an edge node, display info about the node, etc.
package main

import (
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/importkey"
	"github.com/open-horizon/anax/cli/register"
	"github.com/open-horizon/anax/cli/show"
	"github.com/open-horizon/anax/cli/unregister"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
)

func main() {
	// Command flags and args - see https://github.com/alecthomas/kingpin
	app := kingpin.New("hzn", "Command line interface for Horizon agent.")
	cliutils.Opts.Verbose = app.Flag("verbose", "Verbose output.").Short('v').Bool()

	registerCmd := app.Command("register", "Register this edge node with Horizon.")
	nodeIdTok := registerCmd.Flag("node-id-tok", "The Horizon exchange node ID and token. The node ID must be unique within the organization. If not specified, the node ID will be created by Horizon from the machine serial number or fully qualified hostname. If the token is not specified, Horizon will create a random token. If node resource in the exchange identified by the ID and token does not yet exist, you must also specify the -u flag so it can be created.").Short('n').PlaceHolder("ID:TOK").String()
	userPw := registerCmd.Flag("user-pw", "User credentials to create the node resource in the Horizon exchange if it does not already exist.").Short('u').PlaceHolder("USER:PW").String()
	inputFile := registerCmd.Flag("input-file", "A JSON file that sets or overrides variables needed by the node, workloads, and microservices that are part of this pattern. See /usr/horizon/samples/input.json. Specify -f- to read from stdin.").Short('f').String()
	org := registerCmd.Arg("organization", "The Horizon exchange organization ID.").Required().String()
	pattern := registerCmd.Arg("pattern", "The Horizon exchange pattern that describes what workloads that should be deployed to this node.").Required().String()

	importkeyCmd := app.Command("importkey", "Import a public key to verify signed microservices and workloads.")
	keyFile := importkeyCmd.Arg("keyfile", "The public PEM file.").Required().String()

	showCmd := app.Command("show", "Display information about this Horizon edge node.")
	showNodeCmd := showCmd.Command("node", "Show general information about this Horizon edge node.")
	showAgreementsCmd := showCmd.Command("agreements", "Show the active or archived agreements this edge node has made with a Horizon agreement bot.")
	archivedAgreements := showAgreementsCmd.Flag("archived", "Show archived agreements instead of the active agreements.").Short('r').Bool()
	showMeteringCmd := showCmd.Command("metering", "Show metering (payment) information for the active or archived agreements.")
	archivedMetering := showMeteringCmd.Flag("archived", "Show archived agreement metering information instead of metering for the active agreements.").Short('r').Bool()
	showKeysCmd := showCmd.Command("keys", "Show the public signing keys that have been imported to this Horizon edge node.")
	showAttributesCmd := showCmd.Command("attributes", "Show the global attributes that are currently registered on this Horizon edge node.")
	showServicesCmd := showCmd.Command("services", "Show the microservices that are currently registered on this Horizon edge node.")
	showWorkloadsCmd := showCmd.Command("workloads", "Show the workloads that are currently registered on this Horizon edge node.")

	unregisterCmd := app.Command("unregister", "Unregister and reset this Horizon edge node so that it is ready to be registered again. Warning: this will stop all the Horizon workloads running on this edge node, and restart the Horizon agent.")
	forceUnregister := unregisterCmd.Flag("force", "Skip the 'are you sure?' prompt.").Short('f').Bool()
	removeNodeUnregister := unregisterCmd.Flag("remove", "Also remove this node resource from the Horizon exchange (because you no longer want to use this node with Horizon).").Short('r').Bool()

	app.Version("0.0.2") //todo: get the real version of anax

	// Decide which command to run
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case registerCmd.FullCommand():
		register.DoIt(*org, *pattern, *nodeIdTok, *userPw, *inputFile)
	case importkeyCmd.FullCommand():
		importkey.DoIt(*keyFile)
	//case showCmd.FullCommand():   // <- I'd like to just show usage for hzn show, but don't know how to do that yet
	//	showCmd.?
	case showNodeCmd.FullCommand():
		show.Node()
	case showAgreementsCmd.FullCommand():
		show.Agreements(*archivedAgreements)
	case showMeteringCmd.FullCommand():
		show.Metering(*archivedMetering)
	case showKeysCmd.FullCommand():
		show.Keys()
	case showAttributesCmd.FullCommand():
		show.Attributes()
	case showServicesCmd.FullCommand():
		show.Services()
	case showWorkloadsCmd.FullCommand():
		show.Workloads()
	case unregisterCmd.FullCommand():
		unregister.DoIt(*forceUnregister, *removeNodeUnregister)
	}
}
