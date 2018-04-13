package exchange

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
)

func Status(org, userPw string) {
	var output string
	cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "admin/status", cliutils.OrgAndCreds(org, userPw), []int{200}, &output)
	fmt.Println(output)
}
