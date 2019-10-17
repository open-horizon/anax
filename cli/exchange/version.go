package exchange

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
)

func Version(org, userPw string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	credToUse := ""
	var output []byte
	// Note: the base exchange does not need creds for this call (although is tolerant of it), but some front-ends to the exchange might
	if userPw !="" {
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "admin/version", cliutils.OrgAndCreds(org, userPw), []int{200}, &output)
	} else {
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "admin/version", credToUse, []int{200}, &output)
	}	
	fmt.Print(string(output))
}
