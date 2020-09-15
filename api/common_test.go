package api

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/policy"
	"io/ioutil"
	"os"
	"path"
	"time"
)

// ========================================================================================
// These are functions which are used across the set of API unit tests

func getDummyServiceResolver() exchange.ServiceResolverHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, *exchange.ServiceDefinition, []string, error) {
		return nil, nil, []string{}, nil
	}
}

func getDummyServiceDefResolver() exchange.ServiceDefResolverHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (map[string]exchange.ServiceDefinition, *exchange.ServiceDefinition, string, error) {
		return nil, nil, "", nil
	}
}

func getDummyServiceHandler() exchange.ServiceHandler {
	return func(mUrl string, mOrg string, mVersion string, mArch string) (*exchange.ServiceDefinition, string, error) {
		return nil, "", nil
	}
}

func getDummyDeviceHandler() exchange.DeviceHandler {
	return func(id string, token string) (*exchange.Device, error) {
		return nil, nil
	}
}

func getDummyPatchDeviceHandler() exchange.PatchDeviceHandler {
	return func(deviceId string, deviceToken string, pdr *exchange.PatchDeviceRequest) error {
		return nil
	}
}

func getBasicConfig() *config.HorizonConfig {
	return &config.HorizonConfig{
		Edge: config.Config{
			DefaultServiceRegistrationRAM: 256,
			PolicyPath:                    "/tmp/",
			DockerEndpoint:                "unix:///var/run/docker.sock",
			UserPublicKeyPath:             "/tmp/",
		},
		AgreementBot: config.AGConfig{},
		Collaborators: config.Collaborators{
			HTTPClientFactory: nil,
		},
	}
}

func getDummyGetOrg() exchange.OrgHandlerWithContext {
	return func(org string, id string, token string) (*exchange.Organization, error) {
		return nil, nil
	}
}

func getDummyGetPatterns() exchange.PatternHandler {
	return func(org string, pattern string) (map[string]exchange.Pattern, error) {
		return nil, nil
	}
}

func getDummyGetPatternsWithContext() exchange.PatternHandlerWithContext {
	return func(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
		return nil, nil
	}
}

func getDummyGetExchangeVersion() exchange.ExchangeVersionHandler {
	return func(id string, token string) (string, error) {
		return "20.0.0", nil
	}
}

// Use these variable functions when you need the business logic to do something specific and you need to verify something specific.
func getVariablePatternHandler(service exchange.ServiceReference) exchange.PatternHandler {
	return func(org string, pattern string) (map[string]exchange.Pattern, error) {
		patid := fmt.Sprintf("%v/%v", org, pattern)
		return map[string]exchange.Pattern{
			patid: exchange.Pattern{
				Label:              "label",
				Description:        "desc",
				Public:             true,
				Services:           []exchange.ServiceReference{service},
				AgreementProtocols: []exchange.AgreementProtocol{},
			},
		}, nil
	}
}

func getVariableServiceHandler(mUserInput exchange.UserInput) exchange.ServiceHandler {
	return func(mUrl string, mOrg string, mVersion string, mArch string) (*exchange.ServiceDefinition, string, error) {
		md := exchange.ServiceDefinition{
			Owner:         "owner",
			Label:         "label",
			Description:   "desc",
			URL:           mUrl,
			Version:       mVersion,
			Arch:          mArch,
			Sharable:      exchange.MS_SHARING_MODE_EXCLUSIVE,
			MatchHardware: exchange.HardwareRequirement{},
			UserInputs:    []exchange.UserInput{mUserInput},
			LastUpdated:   "today",
		}
		return &md, "service-id", nil
	}
}

func getVariableServiceResolver(mUrl, mOrg, mVersion, mArch string, ui *exchange.UserInput) exchange.ServiceResolverHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, *exchange.ServiceDefinition, []string, error) {
		sl := policy.APISpecList{}
		sd := []exchange.ServiceDependency{}
		if mUrl != "" {
			sl = policy.APISpecList{
				policy.APISpecification{
					SpecRef:         mUrl,
					Org:             mOrg,
					Version:         mVersion,
					ExclusiveAccess: true,
					Arch:            mArch,
				},
			}
			sd = append(sd, exchange.ServiceDependency{
				URL:     mUrl,
				Org:     mOrg,
				Version: mVersion,
				Arch:    mArch,
			})
		}
		uis := []exchange.UserInput{}
		if ui != nil {
			uis = []exchange.UserInput{*ui}
		}
		wl := exchange.ServiceDefinition{
			Owner:               "owner",
			Label:               "label",
			Description:         "desc",
			Public:              false,
			URL:                 wUrl,
			Version:             wVersion,
			Arch:                wArch,
			Sharable:            "multiple",
			RequiredServices:    sd,
			UserInputs:          uis,
			Deployment:          "",
			DeploymentSignature: "",
			LastUpdated:         "updated",
		}
		return &sl, &wl, []string{"x1", "x2"}, nil
	}
}

func getVariableServiceDefResolver(mUrl, mOrg, mVersion, mArch string, ui *exchange.UserInput) exchange.ServiceDefResolverHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (map[string]exchange.ServiceDefinition, *exchange.ServiceDefinition, string, error) {
		sd := []exchange.ServiceDependency{}
		dep_defs := map[string]exchange.ServiceDefinition{}
		if mUrl != "" {
			dep := exchange.ServiceDefinition{
				Owner:               "owner",
				Label:               "label_dep",
				Description:         "desc_dep",
				Public:              false,
				URL:                 mUrl,
				Version:             mVersion,
				Arch:                mArch,
				Sharable:            "multiple",
				RequiredServices:    []exchange.ServiceDependency{},
				UserInputs:          []exchange.UserInput{},
				Deployment:          "",
				DeploymentSignature: "",
				LastUpdated:         "updated",
			}
			dep_defs[mOrg+"/x2"] = dep
			sd = append(sd, exchange.ServiceDependency{
				URL:     mUrl,
				Org:     mOrg,
				Version: mVersion,
				Arch:    mArch,
			})
		}

		uis := []exchange.UserInput{}
		if ui != nil {
			uis = []exchange.UserInput{*ui}
		}
		wl := exchange.ServiceDefinition{
			Owner:               "owner",
			Label:               "label",
			Description:         "desc",
			Public:              false,
			URL:                 wUrl,
			Version:             wVersion,
			Arch:                wArch,
			Sharable:            "multiple",
			RequiredServices:    sd,
			UserInputs:          uis,
			Deployment:          "",
			DeploymentSignature: "",
			LastUpdated:         "updated",
		}
		return dep_defs, &wl, "x1", nil
	}
}

var ExchangeNodePolicyLastUpdated = ""

func getDummyPutNodePolicyHandler() exchange.PutNodePolicyHandler {
	return func(deviceId string, ep *exchange.ExchangePolicy) (*exchange.PutDeviceResponse, error) {
		ExchangeNodePolicyLastUpdated += "blah"
		return nil, nil
	}
}

func getDummyNodePolicyHandler(ep *externalpolicy.ExternalPolicy) exchange.NodePolicyHandler {
	return func(deviceId string) (*exchange.ExchangePolicy, error) {
		if ep != nil {
			return &exchange.ExchangePolicy{ExternalPolicy: *ep, LastUpdated: ExchangeNodePolicyLastUpdated}, nil
		} else {
			return &exchange.ExchangePolicy{ExternalPolicy: externalpolicy.ExternalPolicy{}, LastUpdated: ExchangeNodePolicyLastUpdated}, nil
		}
	}
}

func getDummyDeleteNodePolicyHandler() exchange.DeleteNodePolicyHandler {
	return func(deviceId string) error {
		return nil
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
