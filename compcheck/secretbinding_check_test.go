package compcheck

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/policy"
	"strings"
	"testing"
)

// create a deployment string with the given secrets and service name
func getDeploymentString(name string, secrets []string) string {
	services := map[string]*containermessage.Service{}

	secrets_for_svc := map[string]containermessage.Secret{}
	for _, s := range secrets {
		secrets_for_svc[s] = containermessage.Secret{Description: "blah"}
	}

	svc := containermessage.Service{
		Secrets: secrets_for_svc,
	}
	services[name] = &svc

	deploy := common.DeploymentConfig{Services: services}

	dbytes, _ := json.Marshal(deploy)

	return string(dbytes)
}

func getVariableSelectedServicesHandler(arches []string) exchange.SelectedServicesHandler {
	return func(mUrl string, mOrg string, mVersion string, mArch string) (map[string]exchange.ServiceDefinition, error) {
		services := map[string]exchange.ServiceDefinition{}

		if mArch == "" {
			for _, arch := range arches {
				s := exchange.ServiceDefinition{
					URL:              mUrl,
					Version:          mVersion,
					Arch:             arch,
					RequiredServices: []exchangecommon.ServiceDependency{},
					Deployment:       "",
				}

				sId := fmt.Sprintf("id_%s", arch)
				services[sId] = s
			}
		} else {
			s := exchange.ServiceDefinition{
				URL:              mUrl,
				Version:          mVersion,
				Arch:             mArch,
				RequiredServices: []exchangecommon.ServiceDependency{},
				Deployment:       "",
			}
			sId := fmt.Sprintf("id_%s", mArch)
			services[sId] = s
		}

		return services, nil
	}
}

func getVariableServiceDefResolverHandler(mUrl, mOrg, mVersion, mArch string, secrets_top []string, secrets_dep []string) exchange.ServiceDefResolverHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, map[string]exchange.ServiceDefinition, *exchange.ServiceDefinition, string, error) {
		sd := []exchangecommon.ServiceDependency{}
		dep_defs := map[string]exchange.ServiceDefinition{}
		if mUrl != "" {
			dep := exchange.ServiceDefinition{
				URL:              mUrl,
				Version:          mVersion,
				Arch:             mArch,
				RequiredServices: []exchangecommon.ServiceDependency{},
				Deployment:       getDeploymentString(wUrl, secrets_dep),
			}
			dep_defs[mOrg+"/dep_svc_id"] = dep
			sd = append(sd, exchangecommon.ServiceDependency{
				URL:     mUrl,
				Org:     mOrg,
				Version: mVersion,
				Arch:    mArch,
			})
		}

		wl := exchange.ServiceDefinition{
			URL:              wUrl,
			Version:          wVersion,
			Arch:             wArch,
			RequiredServices: sd,
			Deployment:       getDeploymentString(wUrl, secrets_top),
		}
		return nil, dep_defs, &wl, "top_svc_id", nil
	}
}

func Test_ParseVaultSecretName_good(t *testing.T) {

	vName := "mysecret/extra"
	sUser, sName, err := ParseVaultSecretName(vName, nil)
	if err != nil {
		t.Errorf("ParseVaultSecretName returned error but should not. Error: %v", err)
	}
	if sUser != "" || sName != "mysecret/extra" {
		t.Errorf("ParseVaultSecretName returned incorrect values for %v: (%v, %v)", vName, sUser, sName)
	}

	vName = "/mysecret/extra"
	sUser, sName, err = ParseVaultSecretName(vName, nil)
	if err != nil {
		t.Errorf("ParseVaultSecretName returned error but should not. Error: %v", err)
	}
	if sUser != "" || sName != "mysecret/extra" {
		t.Errorf("ParseVaultSecretName returned incorrect values for %v: (%v, %v)", vName, sUser, sName)
	}

	vName = "my/secret%*/2"
	sUser, sName, err = ParseVaultSecretName(vName, nil)
	if err != nil {
		t.Errorf("ParseVaultSecretName returned error but should not. Error: %v", err)
	}
	if sUser != "" || sName != "my/secret%*/2" {
		t.Errorf("ParseVaultSecretName returned incorrect values for %v: (%v, %v)", vName, sUser, sName)
	}

	vName = "user/myusername/mysecret/extra"
	sUser, sName, err = ParseVaultSecretName(vName, nil)
	if err != nil {
		t.Errorf("ParseVaultSecretName returned error but should not. Error: %v", err)
	}
	if sUser != "myusername" || sName != "mysecret/extra" {
		t.Errorf("ParseVaultSecretName returned incorrect values for %v: (%v, %v)", vName, sUser, sName)
	}

	vName = "user/myusername/myorgagain/extra/mysecret"
	sUser, sName, err = ParseVaultSecretName(vName, nil)
	if err != nil {
		t.Errorf("ParseVaultSecretName returned error but should not. Error: %v", err)
	}
	if sUser != "myusername" || sName != "myorgagain/extra/mysecret" {
		t.Errorf("ParseVaultSecretName returned incorrect values for %v: (%v, %v)", vName, sUser, sName)
	}

	vName = "/user/myusername/mysecret/extra"
	sUser, sName, err = ParseVaultSecretName(vName, nil)
	if err != nil {
		t.Errorf("ParseVaultSecretName returned error but should not. Error: %v", err)
	}
	if sUser != "myusername" || sName != "mysecret/extra" {
		t.Errorf("ParseVaultSecretName returned incorrect values for %v: (%v, %v)", vName, sUser, sName)
	}

}

func Test_ParseVaultSecretName_bad(t *testing.T) {
	errMsg := "Invalid format for the binding secret name"

	vName := "openhorizon/myorg/mysecret"
	_, _, err := ParseVaultSecretName(vName, nil)
	if err == nil {
		t.Errorf("ParseVaultSecretName should have returned error but not.")
	} else if !strings.Contains(err.Error(), errMsg) {
		t.Errorf("The error message returned by ParseVaultSecretName should contain '%v', but got: %v", errMsg, err)
	}

	vName = "openhorizon/myorg/user/myusername/mysecret"
	_, _, err = ParseVaultSecretName(vName, nil)
	if err == nil {
		t.Errorf("ParseVaultSecretName should have returned error but not.")
	} else if !strings.Contains(err.Error(), errMsg) {
		t.Errorf("The error message returned by ParseVaultSecretName should contain '%v'", errMsg)
	}

}

func Test_UpdateIndexMap(t *testing.T) {
	indexMap := map[int]map[string]bool{}
	indexMap[0] = map[string]bool{}
	indexMap[0]["top_sn1"] = true
	indexMap[0]["top_sn2"] = true
	indexMap[1] = map[string]bool{}
	indexMap[1]["dep_sn1"] = true
	indexMap[1]["dep_sn2"] = true

	UpdateIndexMap(indexMap, 0, []string{"top_sn1", "top_sn3"})
	if len(indexMap[0]) != 3 {
		t.Errorf("UpdateIndexMap: there should be 3 elements in indexMap[0] but got %v", len(indexMap[0]))
	} else if _, ok := indexMap[0]["top_sn3"]; !ok {
		t.Errorf("UpdateIndexMap: indexMap[0][top_sn3] should exist but not. %v", indexMap)
	}

	UpdateIndexMap(indexMap, 1, []string{"dep_sn3", "dep_sn1", "dep_sn4"})
	if len(indexMap[1]) != 4 {
		t.Errorf("UpdateIndexMap: there should be 4 elements in indexMap[0] but got %v", len(indexMap[0]))
	} else if _, ok := indexMap[1]["dep_sn3"]; !ok {
		t.Errorf("UpdateIndexMap: indexMap[1][dep_sn3] should exist but not. %v", indexMap)
	} else if _, ok := indexMap[1]["dep_sn4"]; !ok {
		t.Errorf("UpdateIndexMap: indexMap[1][dep_sn4] should exist but not. %v", indexMap)
	}

	UpdateIndexMap(indexMap, 5, []string{"extra_sn1", "extra_sn2"})
	if len(indexMap[5]) != 2 {
		t.Errorf("UpdateIndexMap: there should be 4 elements in indexMap[0] but got %v", len(indexMap[0]))
	} else if _, ok := indexMap[5]["extra_sn1"]; !ok {
		t.Errorf("UpdateIndexMap: indexMap[5][extra_sn1] should exist but not. %v", indexMap)
	} else if _, ok := indexMap[5]["extra_sn2"]; !ok {
		t.Errorf("UpdateIndexMap: indexMap[5][extra_sn2] should exist but not. %v", indexMap)
	}

	UpdateIndexMap(indexMap, 0, []string{})
	if len(indexMap[0]) != 3 {
		t.Errorf("UpdateIndexMap: there should be 3 elements in indexMap[0] but got %v", len(indexMap[0]))
	} else if _, ok := indexMap[0]["top_sn1"]; !ok {
		t.Errorf("UpdateIndexMap: indexMap[0][top_sn1] should exist but not. %v", indexMap)
	} else if _, ok := indexMap[0]["top_sn2"]; !ok {
		t.Errorf("UpdateIndexMap: indexMap[0][top_sn2] should exist but not. %v", indexMap)
	}

}

func Test_CombineIndexMap(t *testing.T) {
	indexMap := map[int]map[string]bool{}
	indexMap[0] = map[string]bool{}
	indexMap[0]["top_sn1"] = true
	indexMap[0]["top_sn2"] = true
	indexMap[1] = map[string]bool{}
	indexMap[1]["dep_sn1"] = true
	indexMap[1]["dep_sn2"] = true

	new_IndexMap := map[int]map[string]bool{}
	CombineIndexMap(indexMap, new_IndexMap)
	if len(indexMap[0]) != 2 {
		t.Errorf("CombineIndexMap: there should be 2 elements in indexMap[0] but got %v", len(indexMap[0]))
	} else if _, ok := indexMap[0]["top_sn1"]; !ok {
		t.Errorf("CombineIndexMap: indexMap[0][top_sn1] should exist but not. %v", indexMap)
	} else if _, ok := indexMap[0]["top_sn2"]; !ok {
		t.Errorf("CombineIndexMap: indexMap[0][top_sn2] should exist but not. %v", indexMap)
	}

	new_IndexMap[0] = map[string]bool{}
	new_IndexMap[0]["top_sn1"] = true
	new_IndexMap[0]["top_sn3"] = true
	new_IndexMap[5] = map[string]bool{}
	new_IndexMap[5]["extra_sn1"] = true
	new_IndexMap[5]["extra_sn2"] = true
	CombineIndexMap(indexMap, new_IndexMap)
	if len(indexMap) != 3 {
		t.Errorf("CombineIndexMap: there should be 3 elements in indexMap but got %v", len(indexMap))
	} else if len(indexMap[0]) != 3 {
		t.Errorf("CombineIndexMap: there should be 3 elements in indexMap[0] but got %v", len(indexMap[0]))
	} else if _, ok := indexMap[0]["top_sn1"]; !ok {
		t.Errorf("CombineIndexMap: indexMap[0][top_sn1] should exist but not. %v", indexMap)
	} else if _, ok := indexMap[0]["top_sn2"]; !ok {
		t.Errorf("CombineIndexMap: indexMap[0][top_sn2] should exist but not. %v", indexMap)
	} else if _, ok := indexMap[0]["top_sn3"]; !ok {
		t.Errorf("CombineIndexMap: indexMap[0][top_sn3] should exist but not. %v", indexMap)
	} else if len(indexMap[5]) != 2 {
		t.Errorf("CombineIndexMap: there should be 2 elements in indexMap[5] but got %v", len(indexMap[0]))
	} else if _, ok := indexMap[5]["extra_sn1"]; !ok {
		t.Errorf("CombineIndexMap: indexMap[5][extra_sn1] should exist but not. %v", indexMap)
	} else if _, ok := indexMap[5]["extra_sn2"]; !ok {
		t.Errorf("CombineIndexMap: indexMap[5][extra_sn2] should exist but not. %v", indexMap)
	}
}

func Test_GroupSecretBindings(t *testing.T) {
	// top service spec
	top_url := "mysvc"
	top_org := "myorg"
	top_ver := "1.0.1"
	top_arch := "amd64"
	secrets_top := []string{"mysecret_top1", "mysecret_both"}

	// dependent service spec
	dep_url := "dep1"
	dep_org := "deporg"
	dep_ver := "0.0.1"
	dep_arch := "amd64"
	secrets_dep := []string{"mysecret_dep1", "mysecret_both"}

	// secret bindings
	vb_top1 := map[string]string{secrets_top[0]: "s1"}
	vb_top2 := map[string]string{secrets_top[1]: "s2"}
	vb_dep1 := map[string]string{secrets_dep[0]: "user/fred/sd1"}
	vb_dep2 := map[string]string{secrets_dep[1]: "s2"}

	sb_top := exchangecommon.SecretBinding{
		ServiceOrgid:        top_org,
		ServiceUrl:          top_url,
		ServiceArch:         top_arch,
		ServiceVersionRange: top_ver,
		Secrets:             []exchangecommon.BoundSecret{vb_top1, vb_top2},
	}
	sb_dep := exchangecommon.SecretBinding{
		ServiceOrgid:        dep_org,
		ServiceUrl:          dep_url,
		ServiceArch:         dep_arch,
		ServiceVersionRange: dep_ver,
		Secrets:             []exchangecommon.BoundSecret{vb_dep1, vb_dep2},
	}
	secretBindings := []exchangecommon.SecretBinding{sb_top, sb_dep}

	// all needed
	indexMap := map[int]map[string]bool{}
	indexMap[0] = map[string]bool{}
	indexMap[0]["mysecret_top1"] = true
	indexMap[0]["mysecret_both"] = true
	indexMap[1] = map[string]bool{}
	indexMap[1]["mysecret_dep1"] = true
	indexMap[1]["mysecret_both"] = true

	neededSB, extraneousSB := GroupSecretBindings(secretBindings, indexMap)
	if len(neededSB) != 2 {
		t.Errorf("GroupSecretBindings: there should be 2 elements in neededSB but got %v", len(neededSB))
	} else if len(extraneousSB) != 0 {
		t.Errorf("GroupSecretBindings: there should be 2 elements in extraneousSB but got %v", len(extraneousSB))
	} else if len(neededSB[0].Secrets) != 2 {
		t.Errorf("GroupSecretBindings: there should be 2 elements in neededSB[0].Secrets but got %v", len(neededSB[0].Secrets))
	} else if len(neededSB[1].Secrets) != 2 {
		t.Errorf("GroupSecretBindings: there should be 2 elements in neededSB[1].Secrets but got %v", len(neededSB[1].Secrets))
	}

	// all extraneous
	indexMap = map[int]map[string]bool{}
	neededSB, extraneousSB = GroupSecretBindings(secretBindings, indexMap)
	if len(extraneousSB) != 2 {
		t.Errorf("GroupSecretBindings: there should be 2 elements in extraneousSB but got %v", len(extraneousSB))
	} else if len(neededSB) != 0 {
		t.Errorf("GroupSecretBindings: there should be 2 elements in extraneousSB but got %v", len(extraneousSB))
	} else if len(extraneousSB[0].Secrets) != 2 {
		t.Errorf("GroupSecretBindings: there should be 2 elements in extraneousSB[0].Secrets but got %v", len(extraneousSB[0].Secrets))
	} else if len(extraneousSB[1].Secrets) != 2 {
		t.Errorf("GroupSecretBindings: there should be 2 elements in extraneousSB[1].Secrets but got %v", len(extraneousSB[1].Secrets))
	}

	// split
	indexMap = map[int]map[string]bool{}
	indexMap[0] = map[string]bool{}
	indexMap[0]["mysecret_top1"] = true
	indexMap[1] = map[string]bool{}
	indexMap[1]["mysecret_both"] = true

	neededSB, extraneousSB = GroupSecretBindings(secretBindings, indexMap)
	if len(neededSB) != 2 {
		t.Errorf("GroupSecretBindings: there should be 2 elements in neededSB but got %v", len(neededSB))
	} else if len(extraneousSB) != 2 {
		t.Errorf("GroupSecretBindings: there should be 2 elements in extraneousSB but got %v", len(extraneousSB))
	} else if len(neededSB[0].Secrets) != 1 {
		t.Errorf("GroupSecretBindings: there should be 1 elements in neededSB[0].Secrets but got %v", len(neededSB[0].Secrets))
	} else if len(neededSB[1].Secrets) != 1 {
		t.Errorf("GroupSecretBindings: there should be 1 elements in neededSB[1].Secrets but got %v", len(neededSB[1].Secrets))
	} else if len(extraneousSB[0].Secrets) != 1 {
		t.Errorf("GroupSecretBindings: there should be 1 elements in extraneousSB[0].Secrets but got %v", len(extraneousSB[0].Secrets))
	} else if len(extraneousSB[1].Secrets) != 1 {
		t.Errorf("GroupSecretBindings: there should be 1 elements in extraneousSB[1].Secrets but got %v", len(extraneousSB[1].Secrets))
	} else if s, _ := neededSB[0].Secrets[0].GetBinding(); s != "mysecret_top1" {
		t.Errorf("GroupSecretBindings: neededSB[0].Secrets[0].Key should be 'mysecret_top1' but got %v", s)
	} else if s, _ := extraneousSB[0].Secrets[0].GetBinding(); s != "mysecret_both" {
		t.Errorf("GroupSecretBindings: extraneousSB[0].Secrets[0].Key should be 'mysecret_both' but got %v", s)
	} else if s, _ := neededSB[1].Secrets[0].GetBinding(); s != "mysecret_both" {
		t.Errorf("GroupSecretBindings: neededSB[1].Secrets[0].Key should be 'mysecret_both' but got %v", s)
	} else if s, _ := extraneousSB[1].Secrets[0].GetBinding(); s != "mysecret_dep1" {
		t.Errorf("GroupSecretBindings: extraneousSB[1].Secrets[0].Key should be 'mysecret_dep1' but got %v", s)
	}
}
