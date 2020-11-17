package exchange

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"strings"
)

func Version(org string, userPw string) {
	output := LoadExchangeVersion(true, org, userPw)
	fmt.Println(output)
}

// Loads exchange version based on optional org and optional set of prioritized credentials (first non-empty credentials will be used)
// If loadWithoutCredentials is false and credentials are missing - the version loading will not be executed (empty string will be returned)
func LoadExchangeVersion(loadWithoutCredentials bool, org string, userPws ...string) string {
	var credToUse string
	for _, cred := range userPws {
		if cred != "" {
			credToUse = cred
			break
		}
	}
	var output []byte
	// Note: the base exchange does not need creds for this call (although is tolerant of it), but some front-ends to the exchange might
	if credToUse != "" {
		cliutils.SetWhetherUsingApiKey(credToUse)
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "admin/version", cliutils.OrgAndCreds(org, credToUse), []int{200}, &output)
	} else if loadWithoutCredentials {
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "admin/version", credToUse, []int{200}, &output)
	}
	return strings.TrimSpace(string(output))
}
