// Package show displays various information about the Horizon edge node.
// The information is mostly obtained from the Horizon API, but in many cases massaged to be more human consumable.
package show

import (
	"fmt"
	"github.com/open-horizon/anax/api"
	"encoding/json"
	"github.com/open-horizon/anax/cli/cliutils"
)

 /*
func Usage(exitCode int) {
	usageStr := "Usage: "+cliutils.GetShortBinaryName()+` show {node|agreements|metering|keys} [options] [args...]

Sub-Commands:
  node - Show general information about this Horizon edge node.
  agreements - Show the active or archived agreements this edge node has made with a Horizon agreement bot.
  metering - Show metering (payment) information for the active or archived agreements.
  keys - Show the public signing keys that have been imported to this Horizon edge node.

Run '`+cliutils.GetShortBinaryName()+` show <sub-command> -h' to get more information on a command.`
	fmt.Println(usageStr)
	os.Exit(exitCode)
}
 */


func Node() {
	status := api.Info{}
	cliutils.HorizonGet("status", &status)
	jsonBytes, err := json.MarshalIndent(status, "", "    ")
	if err != nil { cliutils.Fatal(3, "failed to marshaling 'show node' output: %v", err) }
	fmt.Printf("%s\n", jsonBytes)
}

func Agreements() {
	fmt.Println("implement me")
}

func Metering() {
	fmt.Println("implement me")
}

func Keys() {
	fmt.Println("implement me")
}
