//go:build unit
// +build unit

package imagefetch

import (
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/persistence"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func Test_authDockerFile(t *testing.T) {

	host_only := true
	publishable := false

	// docker auths
	meta_data1 := persistence.AttributeMeta{
		Label:       "test1",
		Publishable: &publishable,
		HostOnly:    &host_only,
		Type:        reflect.TypeOf(persistence.DockerRegistryAuthAttributes{}).Name(),
	}
	var auth_array1 []persistence.Auth
	auth_array1 = append(auth_array1, persistence.Auth{Registry: "myrepo1.com", Token: "t11"}, persistence.Auth{Registry: "myrepo1.com", UserName: "iamapikey", Token: "t12"})
	auth_attrib1 := persistence.DockerRegistryAuthAttributes{
		Meta:  &meta_data1,
		Auths: auth_array1,
	}

	meta_data2 := persistence.AttributeMeta{
		Label:       "test2",
		Publishable: &publishable,
		HostOnly:    &host_only,
		Type:        reflect.TypeOf(persistence.DockerRegistryAuthAttributes{}).Name(),
	}
	var auth_array2 []persistence.Auth
	auth_array2 = append(auth_array2, persistence.Auth{Registry: "myrepo2.com", UserName: "token", Token: "t21"}, persistence.Auth{Registry: "myrepo2.com", Token: "t22"}, persistence.Auth{Registry: "myrepo2.com", Token: "t23"})
	auth_attrib2 := persistence.DockerRegistryAuthAttributes{
		Meta:  &meta_data2,
		Auths: auth_array2,
	}

	dockerAuthConfigurations := make(map[string][]docker.AuthConfiguration, 0)

	attrib_array := make([]persistence.Attribute, 0)
	attrib_array = append(attrib_array, auth_attrib1, auth_attrib2)
	err := ExtractAuthAttributes(attrib_array, dockerAuthConfigurations)

	var config config.Config
	auth_file_name := "./test/docker_auths.json"
	config.DockerCredFilePath = auth_file_name

	err = authDockerFile(config, dockerAuthConfigurations)
	assert.Nil(t, err, "Should return nil.")
	assert.Equal(t, 3, len(dockerAuthConfigurations), "the docker auth should have 3 elements.")
	assert.Equal(t, 2, len(dockerAuthConfigurations["myrepo1.com"]), "The docker auth array should have 2 items.")
	assert.Equal(t, 4, len(dockerAuthConfigurations["myrepo2.com"]), "The docker auth array should have 4 items.")
	assert.Equal(t, 1, len(dockerAuthConfigurations["myrepo3.com"]), "The docker auth array should have 1 items.")

}
