package exchange

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
)

func Version(org, userPw string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	var output []byte
	// Note: the base exchange doesn't need creds for this call (altho is tolerant of it), but some front-ends to the exchange might
	cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "admin/version", cliutils.OrgAndCreds(org, userPw), []int{200}, &output)
	fmt.Print(string(output))
}
