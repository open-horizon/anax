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
		},
		AgreementBot: AGConfig{
			ExchangeURL: "zoo",
		},
	}

	if val := os.Getenv(ExchangeURLEnvvarName); val != "" {
		t.Errorf("Test environment error: please unset envvar %s before executing the test", ExchangeURLEnvvarName)
	}

	// test that there is no error produced by enriching w/ an unset exchange URL value until the time that we require it
	if err := enrichFromEnvvars(&config); err != nil || config.Edge.ExchangeURL != "goo" || config.AgreementBot.ExchangeURL != "zoo" {
		t.Errorf("Config enrichment failed passthrough test")
	}

	exVal := "fooozzzzz"
	if err := os.Setenv(ExchangeURLEnvvarName, exVal); err != nil {
		t.Errorf("Failed to set envvar in test environment. Error: %v", err)
	}

	if err := enrichFromEnvvars(&config); err != nil || config.Edge.ExchangeURL != exVal || config.AgreementBot.ExchangeURL != exVal {
		t.Errorf("Config enrichment did not set exchange URL from envvar as expected")
	}
}
