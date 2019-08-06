package sync_service

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/edge-sync-service/common"
	"path"
)

type MMSHealth struct {
	General  common.HealthStatusInfo   `json:"general"`
	DBHealth common.DBHealthStatusInfo `json:"dbHealth"`
}

func Status(org string, userPw string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if userPw == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify exchange credentials to access the model management service"))
	}

	// Set the API key env var if that's what we're using.
	cliutils.SetWhetherUsingApiKey(userPw)

	// Display the minimal health status.
	var healthData MMSHealth

	// Construct the URL path from the input pieces. They are required inputs so we know they are at least non-null.
	urlPath := path.Join("api/v1/health")

	// Call the MMS service over HTTP
	httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{200}, &healthData)
	if httpCode != 200 {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("health status API returned HTTP code %v", httpCode))
	}
	output := cliutils.MarshalIndent(healthData, "mms health")
	fmt.Println(output)
}
