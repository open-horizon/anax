// +build unit

package config

import (
	"os"
	"testing"
)

func Test_enrichFromEnvvars_success(t *testing.T) {

	// to be enriched
	config := HorizonConfig{
		Edge: Config{
			ExchangeURL: "goo",
			FileSyncService: FSSConfig{
				CSSURL: "https://mycomp/css/",
			},
		},
		AgreementBot: AGConfig{
			ExchangeURL: "zoo",
		},
	}

	// Save the current env var value for restoration at the end.
	saveEURL := os.Getenv(ExchangeURLEnvvarName)
	saveCSSURL := os.Getenv(FileSyncServiceCSSURLEnvvarName)

	restore := func() {
		// Restore the env var to what it was at the beginning of the test
		if err := os.Setenv(ExchangeURLEnvvarName, saveEURL); err != nil {
			t.Errorf("Failed to set envvar in test environment. Error: %v", err)
		}
		if err := os.Setenv(FileSyncServiceCSSURLEnvvarName, saveCSSURL); err != nil {
			t.Errorf("Failed to set envvar in test environment. Error: %v", err)
		}
	}

	defer restore()

	// Clear it for the test
	if err := os.Unsetenv(ExchangeURLEnvvarName); err != nil {
		t.Errorf("Failed to clear %v for test environment. Error: %v", ExchangeURLEnvvarName, err)
	}
	if err := os.Unsetenv(FileSyncServiceCSSURLEnvvarName); err != nil {
		t.Errorf("Failed to clear %v for test environment. Error: %v", FileSyncServiceCSSURLEnvvarName, err)
	}

	// test that there is no error produced by enriching w/ an unset exchange URL value until the time that we require it
	if err := enrichFromEnvvars(&config); err != nil || config.Edge.ExchangeURL != "goo" || config.AgreementBot.ExchangeURL != "zoo" || config.Edge.FileSyncService.CSSURL != "https://mycomp/css/" {
		t.Errorf("Config enrichment failed passthrough test")
	}

	exVal := "fooozzzzz"
	exCSSUTL := "edge.cssurl"
	if err := os.Setenv(ExchangeURLEnvvarName, exVal); err != nil {
		t.Errorf("Failed to set envvar in test environment. Error: %v", err)
	}
	if err := os.Setenv(FileSyncServiceCSSURLEnvvarName, exCSSUTL); err != nil {
		t.Errorf("Failed to set envvar in test environment. Error: %v", err)
	}

	if err := enrichFromEnvvars(&config); err != nil || config.Edge.ExchangeURL != exVal || config.AgreementBot.ExchangeURL != exVal || config.Edge.FileSyncService.CSSURL != exCSSUTL {
		t.Errorf("Config enrichment did not set exchange URL or File Sync Server css url from envvar as expected")
	}

}
