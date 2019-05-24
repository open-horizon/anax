package sync_service

import (
	"github.com/open-horizon/anax/cli/cliutils"
)

func Status(org string, mmsUserPw string) {
	cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "Command 'hzn mms status' is not supported yet")
}
