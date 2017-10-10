package api

import (
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"io/ioutil"
	"os"
	"path"
	"time"
)

// ========================================================================================
// These are functions which are used across the set of API unit tests

func getDummyWorkloadResolver() WorkloadResolverHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string, id string, token string) (*policy.APISpecList, error) {
		return nil, nil
	}
}

func getDummyMicroserviceHandler() MicroserviceHandler {
	return func(mUrl string, mOrg string, mVersion string, mArch string, id string, token string) (*exchange.MicroserviceDefinition, error) {
		return nil, nil
	}
}

func getBasicConfig() *config.HorizonConfig {
	return &config.HorizonConfig{
		Edge: config.Config{
			DefaultServiceRegistrationRAM: 256,
			PolicyPath:                    "/tmp/",
		},
		AgreementBot: config.AGConfig{},
		Collaborators: config.Collaborators{
			HTTPClientFactory: nil,
		},
	}
}

func getDummyGetOrg() OrgHandler {
	return func(org string, id string, token string) (*exchange.Organization, error) {
		return nil, nil
	}
}

func getDummyGetPatterns() PatternHandler {
	return func(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
		return nil, nil
	}
}

func setup() (string, *bolt.DB, error) {
	dir, err := ioutil.TempDir("", "horizondevice-")
	if err != nil {
		return "", nil, err
	}

	db, err := bolt.Open(path.Join(dir, "anax-int.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		return dir, nil, err
	}

	return dir, db, nil
}

// Make a deferred call to this function after calling setup(), passing the output dirpath of the setup() function.
func cleanTestDir(dirPath string) error {
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(dirPath); err != nil {
			return err
		}
	}
	return nil
}
