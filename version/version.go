package version

import (
	"fmt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
)

// the real version will be set by the horizon-deb-packager build process
const HORIZON_VERSION = "local build"

// the required exchange version
const REQUIRED_EXCHANGE_VERSION = "1.44.0"

// This function verifies the exchange version to make sure it meets the requirement.
// It return nil if the exchange version is okay.
// or error if there is an error or current version is not okay.
func VerifyExchangeVersion(httpClientFactory *config.HTTPClientFactory, exchangeUrl string) error {
	if exch_version, err := exchange.GetExchangeVersion(httpClientFactory, exchangeUrl); err != nil {
		return fmt.Errorf("Failed to get exchange version from the exchange. %v", err)
	} else if !policy.IsVersionString(exch_version) {
		return fmt.Errorf("The current exchange version %v is not a valid version string.", exch_version)
	} else if comp, err := policy.CompareVersions(exch_version, REQUIRED_EXCHANGE_VERSION); err != nil {
		return fmt.Errorf("Failed to compare the versions. %v", err)
	} else if comp < 0 {
		return fmt.Errorf("The current exchange version %v does not meet the requirement. The required minimum version is %v. Please upgrade the exchange.", exch_version, REQUIRED_EXCHANGE_VERSION)
	} else {
		return nil
	}
}
