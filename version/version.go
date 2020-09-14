package version

import (
	"fmt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/semanticversion"
)

// the real version will be set by the horizon-deb-packager build process
const HORIZON_VERSION = "local build"

// the minimum exchange version
const MINIMUM_EXCHANGE_VERSION = "2.42.0"

// the preferred exchange version
const PREFERRED_EXCHANGE_VERSION = "2.42.0"

// This function verifies the exchange version to make sure it meets the requirement.
// It return nil if the exchange version is okay.
// or error if there is an error or current version is not okay.
// If a new feature needs the exchagne version higher than the minumum version, call this function with checkWithPreffered to true.
func VerifyExchangeVersion(httpClientFactory *config.HTTPClientFactory, exchangeUrl string, id string, token string, checkWithPreferred bool) error {
	if exch_version, err := exchange.GetExchangeVersion(httpClientFactory, exchangeUrl, id, token); err != nil {
		return fmt.Errorf("Failed to get exchange version from the exchange. %v", err)
	} else {
		return VerifyExchangeVersion1(exch_version, checkWithPreferred)
	}
}

func VerifyExchangeVersion1(exch_version string, checkWithPreferred bool) error {
	version_for_check := MINIMUM_EXCHANGE_VERSION
	if checkWithPreferred {
		version_for_check = PREFERRED_EXCHANGE_VERSION
	}

	if !semanticversion.IsVersionString(exch_version) {
		return fmt.Errorf("The current exchange version %v is not a valid version string.", exch_version)
	} else if comp, err := semanticversion.CompareVersions(exch_version, version_for_check); err != nil {
		return fmt.Errorf("Failed to compare the versions. %v", err)
	} else if comp < 0 {
		return fmt.Errorf("The current exchange version %v does not meet the requirement. The required version is %v or above. Please upgrade the exchange.", exch_version, version_for_check)
	} else {
		return nil
	}
}
