package exchange

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
)

func Version() {
	var output []byte
	cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "admin/version", "", []int{200}, &output)
	fmt.Print(string(output))
}
