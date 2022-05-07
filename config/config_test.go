//go:build unit
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
			Vault: VaultConfig{
				VaultURL: "http://vault/v1",
			},
		},
	}

	testVars := []string{ExchangeURLEnvvarName, FileSyncServiceCSSURLEnvvarName, VaultURLEnvvarName}
	// Save the current env var value for restoration at the end.
	currentVarValue := make(map[string]string)
	for _, v := range testVars {
		currentVarValue[v] = os.Getenv(v)
	}

	restore := func() {
		// Restore the env var to what it was at the beginning of the test
		for _, v := range testVars {
			if err := os.Setenv(v, currentVarValue[v]); err != nil {
				t.Errorf("Failed to set envvar in test environment. Error: %v", err)
			}
		}
	}

	defer restore()

	// Clear it for the test
	for _, v := range testVars {
		if err := os.Unsetenv(v); err != nil {
			t.Errorf("Failed to clear %v for test environment. Error: %v", v, err)
		}
	}

	// test that there is no error produced by enriching w/ an unset exchange URL value until the time that we require it
	if err := enrichFromEnvvars(&config); err != nil || config.Edge.ExchangeURL != "goo" || config.AgreementBot.ExchangeURL != "zoo" || config.Edge.FileSyncService.CSSURL != "https://mycomp/css/" || config.AgreementBot.Vault.VaultURL != "http://vault/v1" {
		t.Errorf("Config enrichment failed passthrough test")
	}

	exVal := "fooozzzzz"
	exCSSURL := "edge.cssurl"
	exVaultURL := "https://vault/v1"
	newVarValues := []string{exVal, exCSSURL, exVaultURL}

	for i, v := range testVars {
		if err := os.Setenv(v, newVarValues[i]); err != nil {
			t.Errorf("Failed to set envvar in test environment. Error: %v", err)
		}
	}

	if err := enrichFromEnvvars(&config); err != nil || config.Edge.ExchangeURL != exVal || config.AgreementBot.ExchangeURL != exVal || config.Edge.FileSyncService.CSSURL != exCSSURL || config.AgreementBot.Vault.VaultURL != exVaultURL {
		t.Errorf("Config enrichment did not set exchange URL or File Sync Server css url from envvar as expected")
	}

}
