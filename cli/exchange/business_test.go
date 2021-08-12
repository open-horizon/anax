package exchange

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

func Test_ValidateSecretBindingForSvcAndDep(t *testing.T) {
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

	serviceDefResolver := getVariableServiceDefResolverHandler(dep_url, dep_org, dep_ver, dep_arch, secrets_top, secrets_dep)
	selectedServices := getVariableSelectedServicesHandler([]string{"amd64", "arm64"})

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
		Secrets:             []exchangecommon.BoundSecret{},
	}
	sb_dep := exchangecommon.SecretBinding{
		ServiceOrgid:        dep_org,
		ServiceUrl:          dep_url,
		ServiceArch:         dep_arch,
		ServiceVersionRange: dep_ver,
		Secrets:             []exchangecommon.BoundSecret{},
	}

	// good case
	sb_top.Secrets = []exchangecommon.BoundSecret{vb_top1, vb_top2}
	sb_dep.Secrets = []exchangecommon.BoundSecret{vb_dep1, vb_dep2}
	secretBindings := []exchangecommon.SecretBinding{sb_top, sb_dep}
	index_map, err := ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, top_arch,
		true, serviceDefResolver, selectedServices, nil)
	if err != nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should not have returned error. Error: %v", err)
	} else if _, ok := index_map[0]["mysecret_top1"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[0][mysecret_top1] but not. index_map=%v", index_map)
	} else if _, ok := index_map[0]["mysecret_both"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[0][mysecret_both] but not. index_map=%v", index_map)
	} else if _, ok := index_map[1]["mysecret_dep1"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[1][mysecret_dep1] but not. index_map=%v", index_map)
	} else if _, ok := index_map[1]["mysecret_both"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[1][mysecret_both] but not. index_map=%v", index_map)
	}

	// good case, ServiceArch="*"
	sb_top.ServiceArch = "*"
	sb_top.Secrets = []exchangecommon.BoundSecret{vb_top1, vb_top2}
	sb_dep.Secrets = []exchangecommon.BoundSecret{vb_dep1, vb_dep2}
	secretBindings = []exchangecommon.SecretBinding{sb_top, sb_dep}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, top_arch,
		true, serviceDefResolver, selectedServices, nil)
	if err != nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should not have returned error. Error: %v", err)
	}

	// good case, ServiceArch=""
	sb_top.ServiceArch = "*"
	sb_top.Secrets = []exchangecommon.BoundSecret{vb_top1, vb_top2}
	sb_dep.Secrets = []exchangecommon.BoundSecret{vb_dep1, vb_dep2}
	secretBindings = []exchangecommon.SecretBinding{sb_top, sb_dep}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, top_arch,
		true, serviceDefResolver, selectedServices, nil)
	if err != nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should not have returned error. Error: %v", err)
	} else if _, ok := index_map[0]["mysecret_top1"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[0][mysecret_top1] but not. index_map=%v", index_map)
	} else if _, ok := index_map[0]["mysecret_both"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[0][mysecret_both] but not. index_map=%v", index_map)
	} else if _, ok := index_map[1]["mysecret_dep1"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[1][mysecret_dep1] but not. index_map=%v", index_map)
	} else if _, ok := index_map[1]["mysecret_both"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[1][mysecret_both] but not. index_map=%v", index_map)
	}

	// good case, arch="*", but checkAllArches is false
	sb_top.ServiceArch = "amd64"
	sb_top.Secrets = []exchangecommon.BoundSecret{vb_top1, vb_top2}
	sb_dep.Secrets = []exchangecommon.BoundSecret{vb_dep1, vb_dep2}
	secretBindings = []exchangecommon.SecretBinding{sb_top, sb_dep}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, "*",
		false, serviceDefResolver, selectedServices, nil)
	if err != nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should not have returned error. Error: %v", err)
	} else if _, ok := index_map[0]["mysecret_top1"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[0][mysecret_top1] but not. index_map=%v", index_map)
	} else if _, ok := index_map[0]["mysecret_both"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[0][mysecret_both] but not. index_map=%v", index_map)
	} else if _, ok := index_map[1]["mysecret_dep1"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[1][mysecret_dep1] but not. index_map=%v", index_map)
	} else if _, ok := index_map[1]["mysecret_both"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[1][mysecret_both] but not. index_map=%v", index_map)
	}

	// bad case, arch="*", checkAllArches is true
	sb_top.ServiceArch = "amd64"
	sb_top.Secrets = []exchangecommon.BoundSecret{vb_top1, vb_top2}
	sb_dep.Secrets = []exchangecommon.BoundSecret{vb_dep1, vb_dep2}
	secretBindings = []exchangecommon.SecretBinding{sb_top, sb_dep}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, "*",
		true, serviceDefResolver, selectedServices, nil)
	if err == nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should return error but not.")
	}

	// both top and dependent services have secrets, but no secret bindings provided
	secretBindings = []exchangecommon.SecretBinding{}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, top_arch,
		true, serviceDefResolver, selectedServices, nil)
	if err == nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should have returned error but not. Returned: (%v, %v)", index_map, err)
	} else if !strings.Contains(err.Error(), "No secret binding found for") {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'No secret binding found for', but got: %v", err)
	} else if !strings.Contains(err.Error(), "top_svc_id") || !strings.Contains(err.Error(), secrets_top[0]) || !strings.Contains(err.Error(), secrets_top[1]) {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'top_svc_id', '%v' and '%v' , but got: %v", secrets_top[0], secrets_top[1], err)
	}

	// no secret bindings provided for the dependent service
	sb_top.Secrets = []exchangecommon.BoundSecret{vb_top1, vb_top2}
	secretBindings = []exchangecommon.SecretBinding{sb_top}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, top_arch,
		true, serviceDefResolver, selectedServices, nil)
	if err == nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should have returned error but not. Returned: (%v, %v)", index_map, err)
	} else if !strings.Contains(err.Error(), "No secret binding found for") {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'No secret binding found for', but got: %v", err)
	} else if !strings.Contains(err.Error(), "dep_svc_id") || !strings.Contains(err.Error(), secrets_dep[0]) || !strings.Contains(err.Error(), secrets_dep[1]) {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'dep_svc_id', '%v' and '%v' , but got: %v", secrets_dep[0], secrets_dep[1], err)
	}

	// no secret bindings provided for the top level service
	sb_dep.Secrets = []exchangecommon.BoundSecret{vb_dep1, vb_dep2}
	secretBindings = []exchangecommon.SecretBinding{sb_dep}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, top_arch,
		true, serviceDefResolver, selectedServices, nil)
	if err == nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should have returned error but not. Returned: (%v, %v)", index_map, err)
	} else if !strings.Contains(err.Error(), "No secret binding found for") {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'No secret binding found for', but got: %v", err)
	} else if !strings.Contains(err.Error(), "top_svc_id") || !strings.Contains(err.Error(), secrets_top[0]) || !strings.Contains(err.Error(), secrets_top[1]) {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'top_svc_id', '%v' and '%v' , but got: %v", secrets_top[0], secrets_top[1], err)
	}

	// missing one secret binding for the dependent service
	sb_top.Secrets = []exchangecommon.BoundSecret{vb_top1, vb_top2}
	sb_dep.Secrets = []exchangecommon.BoundSecret{vb_dep1}
	secretBindings = []exchangecommon.SecretBinding{sb_top, sb_dep}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, top_arch,
		true, serviceDefResolver, selectedServices, nil)
	if err == nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should have returned error but not. Returned: (%v, %v)", index_map, err)
	} else if !strings.Contains(err.Error(), "No secret binding found for") {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'No secret binding found for', but got: %v", err)
	} else if !strings.Contains(err.Error(), "dep_svc_id") || !strings.Contains(err.Error(), secrets_dep[1]) {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'dep_svc_id' and '%v', but got: %v", secrets_dep[1], err)
	}

	// missing one secret binding for the top level service
	sb_top.Secrets = []exchangecommon.BoundSecret{vb_top2}
	sb_dep.Secrets = []exchangecommon.BoundSecret{vb_dep1, vb_dep2}
	secretBindings = []exchangecommon.SecretBinding{sb_top, sb_dep}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, top_arch,
		true, serviceDefResolver, selectedServices, nil)
	if err == nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should have returned error but not. Returned: (%v, %v)", index_map, err)
	} else if !strings.Contains(err.Error(), "No secret binding found for") {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'No secret binding found for', but got: %v", err)
	} else if !strings.Contains(err.Error(), "top_svc_id") || !strings.Contains(err.Error(), secrets_top[0]) {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'top_svc_id' and '%v', but got: %v", secrets_top[0], err)
	}

	// invalid vault secret name
	vb_dep1x := map[string]string{secrets_dep[0]: "openhorizon/myorg/user/fred/sd1"}
	sb_top.Secrets = []exchangecommon.BoundSecret{vb_top1, vb_top2}
	sb_dep.Secrets = []exchangecommon.BoundSecret{vb_dep1x, vb_dep2}
	secretBindings = []exchangecommon.SecretBinding{sb_top, sb_dep}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, top_arch,
		true, serviceDefResolver, selectedServices, nil)
	if err == nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should have returned error but not. Returned: (%v, %v)", index_map, err)
	} else if !strings.Contains(err.Error(), "Invalid format") {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'Invalid format', but got: %v", err)
	} else if !strings.Contains(err.Error(), "dep_svc_id") {
		t.Errorf("The error message returned by ValidateSecretBindingForSvcAndDep should have contained 'dep_svc_id' and '%v', but got: %v", secrets_dep[1], err)
	}

	// extra secret binding for top level service
	sb_top.Secrets = []exchangecommon.BoundSecret{vb_top1, vb_top2, map[string]string{"mysecret_top3": "s3"}}
	sb_dep.Secrets = []exchangecommon.BoundSecret{vb_dep1, vb_dep2}
	secretBindings = []exchangecommon.SecretBinding{sb_top, sb_dep}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, top_arch,
		true, serviceDefResolver, selectedServices, nil)
	if err != nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should not have returned error. Error: %v", err)
	} else if len(index_map) != 2 || len(index_map[0]) != 2 {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep is incorrect. %v", index_map)
	} else if _, ok := index_map[0]["mysecret_top1"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[0][mysecret_top1] but not. index_map=%v", index_map)
	} else if _, ok := index_map[0]["mysecret_both"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[0][mysecret_both] but not. index_map=%v", index_map)
	} else if _, ok := index_map[1]["mysecret_dep1"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[1][mysecret_dep1] but not. index_map=%v", index_map)
	} else if _, ok := index_map[1]["mysecret_both"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[1][mysecret_both] but not. index_map=%v", index_map)
	}

	// extra secret binding for a different service
	sb_extra := exchangecommon.SecretBinding{
		ServiceOrgid:        "extra_url",
		ServiceUrl:          dep_url,
		ServiceArch:         dep_arch,
		ServiceVersionRange: dep_ver,
		Secrets:             []exchangecommon.BoundSecret{map[string]string{"svc_secret": "vault_secret"}},
	}
	sb_top.Secrets = []exchangecommon.BoundSecret{vb_top1, vb_top2}
	sb_dep.Secrets = []exchangecommon.BoundSecret{vb_dep1, vb_dep2}
	secretBindings = []exchangecommon.SecretBinding{sb_top, sb_dep, sb_extra}
	index_map, err = ValidateSecretBindingForSvcAndDep(secretBindings, top_org, top_url, top_ver, top_arch,
		true, serviceDefResolver, selectedServices, nil)
	if err != nil {
		t.Errorf("ValidateSecretBindingForSvcAndDep should not have returned error. Error: %v", err)
	} else if len(index_map) != 2 || len(index_map[0]) != 2 {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep is incorrect. %v", index_map)
	} else if _, ok := index_map[0]["mysecret_top1"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[0][mysecret_top1] but not. index_map=%v", index_map)
	} else if _, ok := index_map[0]["mysecret_both"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[0][mysecret_both] but not. index_map=%v", index_map)
	} else if _, ok := index_map[1]["mysecret_dep1"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[1][mysecret_dep1] but not. index_map=%v", index_map)
	} else if _, ok := index_map[1]["mysecret_both"]; !ok {
		t.Errorf("The index map returned by ValidateSecretBindingForSvcAndDep should have contained index_map[1][mysecret_both] but not. index_map=%v", index_map)
	}
}
