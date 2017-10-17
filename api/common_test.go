package api

import (
	"fmt"
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
	return func(wUrl string, wOrg string, wVersion string, wArch string, id string, token string) (*policy.APISpecList, *exchange.WorkloadDefinition, error) {
		return nil, nil, nil
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

// Use these variable functions when you need the business logic to do something specific and you need to verify something specific.
func getVariablePatternHandler(workload exchange.WorkloadReference) func(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
	return func(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
		patid := fmt.Sprintf("%v/%v", org, pattern)
		return map[string]exchange.Pattern{
			patid: exchange.Pattern{
				Label:              "label",
				Description:        "desc",
				Public:             true,
				Workloads:          []exchange.WorkloadReference{workload},
				AgreementProtocols: []exchange.AgreementProtocol{},
			},
		}, nil
	}
}

func getVariableWorkloadResolver(mUrl, mOrg, mVersion, mArch string, ui *exchange.UserInput) func(wUrl string, wOrg string, wVersion string, wArch string, id string, token string) (*policy.APISpecList, *exchange.WorkloadDefinition, error) {
	return func(wUrl string, wOrg string, wVersion string, wArch string, id string, token string) (*policy.APISpecList, *exchange.WorkloadDefinition, error) {
		sl := policy.APISpecList{
			policy.APISpecification{
				SpecRef:         mUrl,
				Org:             mOrg,
				Version:         mVersion,
				ExclusiveAccess: true,
				Arch:            mArch,
			},
		}
		es := exchange.APISpec{
			SpecRef: mUrl,
			Org:     mOrg,
			Version: mVersion,
			Arch:    mArch,
		}
		uis := []exchange.UserInput{}
		if ui != nil {
			uis = []exchange.UserInput{*ui}
		}
		wl := exchange.WorkloadDefinition{
			Owner:       "owner",
			Label:       "label",
			Description: "desc",
			WorkloadURL: wUrl,
			Version:     wVersion,
			Arch:        wArch,
			DownloadURL: "",
			APISpecs:    []exchange.APISpec{es},
			UserInputs:  uis,
			Workloads:   []exchange.WorkloadDeployment{},
			LastUpdated: "updated",
		}
		return &sl, &wl, nil
	}
}

func getVariableMicroserviceHandler(mUserInput exchange.UserInput) func(mUrl string, mOrg string, mVersion string, mArch string, id string, token string) (*exchange.MicroserviceDefinition, error) {
	return func(mUrl string, mOrg string, mVersion string, mArch string, id string, token string) (*exchange.MicroserviceDefinition, error) {
		md := exchange.MicroserviceDefinition{
			Owner:         "owner",
			Label:         "label",
			Description:   "desc",
			SpecRef:       mUrl,
			Version:       mVersion,
			Arch:          mArch,
			Sharable:      exchange.MS_SHARING_MODE_EXCLUSIVE,
			DownloadURL:   "",
			MatchHardware: exchange.HardwareMatch{},
			UserInputs:    []exchange.UserInput{mUserInput},
			Workloads:     []exchange.WorkloadDeployment{},
			LastUpdated:   "today",
		}
		return &md, nil
	}
}

func utsetup() (string, *bolt.DB, error) {
	dir, err := ioutil.TempDir("", "utdb-")
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
