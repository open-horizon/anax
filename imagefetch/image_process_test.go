// +build unit

package imagefetch

import (
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func Test_AppendDockerAuth(t *testing.T) {
	dockerAuths := make(map[string][]docker.AuthConfiguration)
	a1 := docker.AuthConfiguration{
		Email:         "",
		Username:      "token",
		Password:      "11111",
		ServerAddress: "test1",
	}
	a2 := docker.AuthConfiguration{
		Email:         "",
		Username:      "token",
		Password:      "22222",
		ServerAddress: "test1",
	}
	a3 := docker.AuthConfiguration{
		Email:         "",
		Username:      "token",
		Password:      "33333",
		ServerAddress: "test2",
	}
	a4 := docker.AuthConfiguration{
		Email:         "",
		Username:      "token",
		Password:      "44444",
		ServerAddress: "test3",
	}
	a5 := docker.AuthConfiguration{
		Email:         "",
		Username:      "token",
		Password:      "55555",
		ServerAddress: "test3",
	}
	dockerAuths = AppendDockerAuth(dockerAuths, a1)
	dockerAuths = AppendDockerAuth(dockerAuths, a2)
	dockerAuths = AppendDockerAuth(dockerAuths, a3)
	dockerAuths = AppendDockerAuth(dockerAuths, a4)
	dockerAuths = AppendDockerAuth(dockerAuths, a5)

	assert.Equal(t, 3, len(dockerAuths), "The map should have 3 keys.")
	assert.Equal(t, 2, len(dockerAuths["test1"]), "The auth array for test1 should have 2 items.")
	assert.Equal(t, 1, len(dockerAuths["test2"]), "The auth array for test1 should have 1 items.")
	assert.Equal(t, 2, len(dockerAuths["test3"]), "The auth array for test1 should have 2 items.")
}

func Test_ExtractAuthAttributes(t *testing.T) {

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

	// http auth
	meta_data3 := persistence.AttributeMeta{
		Label:       "test3",
		Publishable: &publishable,
		HostOnly:    &host_only,
		Type:        reflect.TypeOf(persistence.HTTPSBasicAuthAttributes{}).Name(),
	}
	auth_attrib3 := persistence.HTTPSBasicAuthAttributes{
		Meta:     &meta_data3,
		Url:      "http://myrepo3.com",
		Username: "user3",
		Password: "password3",
	}

	meta_data4 := persistence.AttributeMeta{
		Label:       "test3",
		Publishable: &publishable,
		HostOnly:    &host_only,
		Type:        reflect.TypeOf(persistence.HTTPSBasicAuthAttributes{}).Name(),
	}
	auth_attrib4 := persistence.HTTPSBasicAuthAttributes{
		Meta:     &meta_data4,
		Url:      "http://myrepo4.com",
		Username: "user4",
		Password: "password4",
	}

	dockerAuthConfigurations := make(map[string][]docker.AuthConfiguration, 0)

	attrib_array := make([]persistence.Attribute, 0)
	attrib_array = append(attrib_array, auth_attrib1, auth_attrib3, auth_attrib2)
	err := ExtractAuthAttributes(attrib_array, dockerAuthConfigurations)
	assert.Nil(t, err, "Should return nil.")
	assert.Equal(t, 2, len(dockerAuthConfigurations), "the docker auth should have 2 elements.")
	assert.Equal(t, 2, len(dockerAuthConfigurations["myrepo1.com"]), "The docker auth array should have 2 items.")
	assert.Equal(t, 3, len(dockerAuthConfigurations["myrepo2.com"]), "The docker auth array should have 3 items.")

	// add duplicate
	attrib_array = make([]persistence.Attribute, 0)
	attrib_array = append(attrib_array, auth_attrib1, auth_attrib3, auth_attrib2, auth_attrib4)
	err = ExtractAuthAttributes(attrib_array, dockerAuthConfigurations)
	assert.Equal(t, 2, len(dockerAuthConfigurations), "the docker auth should have 2 elements.")
	assert.Equal(t, 2, len(dockerAuthConfigurations["myrepo1.com"]), "The docker auth array should have 2 items.")
	assert.Equal(t, 3, len(dockerAuthConfigurations["myrepo2.com"]), "The docker auth array should have 3 items.")
}

func Test_authExchange(t *testing.T) {

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

	// http auth
	meta_data3 := persistence.AttributeMeta{
		Label:       "test3",
		Publishable: &publishable,
		HostOnly:    &host_only,
		Type:        reflect.TypeOf(persistence.HTTPSBasicAuthAttributes{}).Name(),
	}
	auth_attrib3 := persistence.HTTPSBasicAuthAttributes{
		Meta:     &meta_data3,
		Url:      "http://myrepo3.com",
		Username: "user3",
		Password: "password3",
	}

	dockerAuthConfigurations := make(map[string][]docker.AuthConfiguration, 0)

	attrib_array := make([]persistence.Attribute, 0)
	attrib_array = append(attrib_array, auth_attrib1, auth_attrib3, auth_attrib2)
	err := ExtractAuthAttributes(attrib_array, dockerAuthConfigurations)

	img_auths := make([]events.ImageDockerAuth, 0)
	img_auths = append(img_auths, events.ImageDockerAuth{Registry: "myrepo5.com", UserName: "token", Password: "t51"},
		events.ImageDockerAuth{Registry: "myrepo1.com", UserName: "iamapikey", Password: "t12"},
		events.ImageDockerAuth{Registry: "myrepo2.com", UserName: "token", Password: "t24"})

	err = authExchange(img_auths, dockerAuthConfigurations)

	assert.Nil(t, err, "Should return nil.")
	assert.Equal(t, 3, len(dockerAuthConfigurations), "the docker auth should have 3 elements.")
	assert.Equal(t, 2, len(dockerAuthConfigurations["myrepo1.com"]), "The docker auth array should have 2 items.")
	assert.Equal(t, 4, len(dockerAuthConfigurations["myrepo2.com"]), "The docker auth array should have 4 items.")

}
