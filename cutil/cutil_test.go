// +build unit

package cutil

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_ParseDockerImagePath_Tags(t *testing.T) {

	var image_name, domain, path, tag, digest string
	// normal case
	image_name = "mydomain.com/x86_64/hellomicroservice:v1.0"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Equal(t, "mydomain.com", domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "x86_64/hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Equal(t, "v1.0", tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Empty(t, digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// docker hub with user name
	image_name = "username/hellomicroservice:v1.0"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Empty(t, domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "username/hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Equal(t, "v1.0", tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Empty(t, digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// docker hub without user name
	image_name = "hellomicroservice:v1.0"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Empty(t, domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Equal(t, "v1.0", tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Empty(t, digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// normal case no tags
	image_name = "mydomain.com/x86_64/hellomicroservice"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Equal(t, "mydomain.com", domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "x86_64/hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Empty(t, tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Empty(t, digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// docker hub without user name without tags
	image_name = "hellomicroservice"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Empty(t, domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Empty(t, tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Empty(t, digest, fmt.Sprintf("Wrong digest in %v.", image_name))
}

func Test_ParseDockerImagePath_Digests(t *testing.T) {

	var image_name, domain, path, tag, digest string

	// normal case
	image_name = "mydomain.com/x86_64/hellomicroservice@sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Equal(t, "mydomain.com", domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "x86_64/hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Empty(t, tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Equal(t, "sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567", digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// normal case with port
	image_name = "mydomain.com:8080/x86_64/hellomicroservice@sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Equal(t, "mydomain.com:8080", domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "x86_64/hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Empty(t, tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Equal(t, "sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567", digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// docker hub with user name
	image_name = "username/hellomicroservice@sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Empty(t, domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "username/hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Empty(t, tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Equal(t, "sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567", digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// docker hub without user name
	image_name = "hellomicroservice@sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Empty(t, domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Empty(t, tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Equal(t, "sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567", digest, fmt.Sprintf("Wrong digest in %v.", image_name))
}

func Test_ParseDockerImagePath_Tags_and_Digests(t *testing.T) {

	var image_name, domain, path, tag, digest string

	// normal case
	image_name = "mydomain.com/x86_64/hellomicroservice:v1.0@sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Equal(t, "mydomain.com", domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "x86_64/hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Equal(t, "v1.0", tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Equal(t, "sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567", digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// normal case with port
	image_name = "mydomain.com:8080/x86_64/hellomicroservice:v1.0@sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Equal(t, "mydomain.com:8080", domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "x86_64/hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Equal(t, "v1.0", tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Equal(t, "sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567", digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// docker hub with user name
	image_name = "username/hellomicroservice:v1.0@sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Empty(t, domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "username/hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Equal(t, "v1.0", tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Equal(t, "sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567", digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// docker hub without user name
	image_name = "hellomicroservice:v1.0@sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Empty(t, domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Equal(t, "v1.0", tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Equal(t, "sha256:15315df0677ab1c7291a822290731032b19462a9d29bdd4d4619df7cb0c0f567", digest, fmt.Sprintf("Wrong digest in %v.", image_name))
}

func Test_ParseDockerImagePath_Other_Cases(t *testing.T) {

	var image_name, domain, path, tag, digest string

	// has / in the repo digest
	image_name = "mydomain.com/x86_64/hellomicroservice@sha256:15315df0677ab1c7291/a822290731032b19462a9d29bdd4d4619df7cb0c0f567"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Equal(t, "mydomain.com", domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "x86_64/hellomicroservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Empty(t, tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Equal(t, "sha256:15315df0677ab1c7291/a822290731032b19462a9d29bdd4d4619df7cb0c0f567", digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// has . in path
	image_name = "mydomain.com:8080/x86_64/hello.microservice:v1.0"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Equal(t, "mydomain.com:8080", domain, fmt.Sprintf("Wrong domain name in %v.", image_name))
	assert.Equal(t, "x86_64/hello.microservice", path, fmt.Sprintf("Wrong path name in %v.", image_name))
	assert.Equal(t, "v1.0", tag, fmt.Sprintf("Wrong tag name in %v.", image_name))
	assert.Empty(t, digest, fmt.Sprintf("Wrong digest in %v.", image_name))

	// no path and domain
	image_name = ":v1.0"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Empty(t, path, fmt.Sprintf("Wrong path name in %v.", image_name))

	// no path and domain -- invalid
	image_name = "@sha256:15315df0677ab1c7291/a822290731032b19462a9d29bdd4d4619df7cb0c0f567"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Empty(t, path, fmt.Sprintf("Wrong path name in %v.", image_name))

	// no path -- invalid
	image_name = "mydomain:8080/@sha256:15315df0677ab1c7291/a822290731032b19462a9d29bdd4d4619df7cb0c0f567"
	domain, path, tag, digest = ParseDockerImagePath(image_name)
	assert.Empty(t, path, fmt.Sprintf("Wrong path name in %v.", image_name))

}

func Test_TruncateDisplayString(t *testing.T) {
	s1 := "1234567890"
	assert.Equal(t, "12...", TruncateDisplayString(s1, 2), fmt.Sprintf("Should only show the first 2 charactors"))
	assert.Equal(t, "12345678...", TruncateDisplayString(s1, 8), fmt.Sprintf("Should only show the first 8 charactors"))
	assert.Equal(t, "1234567890", TruncateDisplayString(s1, 10), fmt.Sprintf("Should only show all 10 charactors"))
	assert.Equal(t, "1234567890", TruncateDisplayString(s1, 15), fmt.Sprintf("Should only show all 10 charactors"))
}

func Test_GetAllIPAddresses_nofilter(t *testing.T) {

	_, err := GetAllHostIPv4Addresses([]NetFilter{})
	if err != nil {
		t.Errorf("error returned: %v", err)
	}

}

func Test_GetAllIPAddresses_loopbackfilter(t *testing.T) {

	allips, err := GetAllHostIPv4Addresses([]NetFilter{})
	if err != nil {
		t.Errorf("error returned: %v", err)
	}

	nlbips, nlberr := GetAllHostIPv4Addresses([]NetFilter{OmitLoopback})
	if nlberr != nil {
		t.Errorf("error returned: %v", nlberr)
	}

	if len(allips) <= len(nlbips) {
		t.Errorf("there should be fewer ips returned when the loopback interface is filtered out. All: %v, filtered: %v", len(allips), len(nlbips))
	}

}

func Test_GetAllIPAddresses_upfilter(t *testing.T) {

	allips, err := GetAllHostIPv4Addresses([]NetFilter{})
	if err != nil {
		t.Errorf("error returned: %v", err)
	}

	nupips, nuperr := GetAllHostIPv4Addresses([]NetFilter{OmitUp})
	if nuperr != nil {
		t.Errorf("error returned: %v", nuperr)
	}

	if len(allips) <= len(nupips) {
		t.Errorf("there should be fewer ips returned when the interface state of UP is filtered out. All: %v, filtered: %v", len(allips), len(nupips))
	}

}

func Test_GetAllIPAddresses_downfilter(t *testing.T) {

	allips, err := GetAllHostIPv4Addresses([]NetFilter{})
	if err != nil {
		t.Errorf("error returned: %v", err)
	}

	ndips, nderr := GetAllHostIPv4Addresses([]NetFilter{OmitDown})
	if nderr != nil {
		t.Errorf("error returned: %v", nderr)
	}

	if len(allips) != len(ndips) {
		t.Errorf("this test assumes that all network interfaces on the host are up. All: %v, filtered: %v", len(allips), len(ndips))
	}

}

func Test_FormOrgSpecUrl(t *testing.T) {

	s := FormOrgSpecUrl("service_url", "")
	if s != "service_url" {
		t.Errorf("FormOrgSpecUrl should have returned 'service_url', but got '%v'", s)
	}

	s = FormOrgSpecUrl("service_url", "myorg")
	if s != "myorg/service_url" {
		t.Errorf("FormOrgSpecUrl should have returned 'myorg/service_url', but got '%v'", s)
	}

	s = FormOrgSpecUrl("http://service_url", "myorg")
	if s != "myorg/http://service_url" {
		t.Errorf("FormOrgSpecUrl should have returned 'myorg/http://service_url', but got '%v'", s)
	}
}

func Test_SplitOrgSpecUrl(t *testing.T) {

	org, url := SplitOrgSpecUrl("service_url")
	if org != "" || url != "service_url" {
		t.Errorf("SplitOrgSpecUrl should have returned '(, service_url)', but got '(%v, %v)'", org, url)
	}

	org, url = SplitOrgSpecUrl("myorg/service_url")
	if org != "myorg" || url != "service_url" {
		t.Errorf("SplitOrgSpecUrl should have returned '(myorg, service_url)', but got '(%v, %v)'", org, url)
	}

	org, url = SplitOrgSpecUrl("myorg/http://service_url")
	if org != "myorg" || url != "http://service_url" {
		t.Errorf("SplitOrgSpecUrl should have returned '(myorg, http://service_url)', but got '(%v, %v)'", org, url)
	}
}

func Test_MakeMSInstanceKey(t *testing.T) {

	s := MakeMSInstanceKey("service_url", "myorg", "1.0.0", "123b-4512ef")
	if s != "myorg_service_url_1.0.0_123b-4512ef" {
		t.Errorf("MakeMSInstanceKey should have returned 'myorg_service_url_1.0.0_123b-4512ef', but got '%v'", s)
	}

	s = MakeMSInstanceKey("htp://service_url", "myorg", "1.0.0", "123b-4512ef")
	if s != "myorg_service_url_1.0.0_123b-4512ef" {
		t.Errorf("MakeMSInstanceKey should have returned 'myorg_service_url_1.0.0_123b-4512ef', but got '%v'", s)
	}

	s = MakeMSInstanceKey("htp://service/myurl", "myorg", "1.0.0", "123b-4512ef")
	if s != "myorg_service-myurl_1.0.0_123b-4512ef" {
		t.Errorf("MakeMSInstanceKey should have returned 'myorg_service-myurl_1.0.0_123b-4512ef', but got '%v'", s)
	}

	s = MakeMSInstanceKey("htp://service/my%%url", "my@org*#", "1.0.0", "123b-45@#$%12ef")
	if s != "my-org--_service-my--url_1.0.0_123b-45----12ef" {
		t.Errorf("MakeMSInstanceKey should have returned 'my-org--_service-my--url_1.0.0_123b-45----12ef', but got '%v'", s)
	}

	s = MakeMSInstanceKey("htp://service/my%%url", "%%my@org*#", "1.0.0", "123b-45@#$%12ef")
	if s != "0-my-org--_service-my--url_1.0.0_123b-45----12ef" {
		t.Errorf("MakeMSInstanceKey should have returned '0-my-org--_service-my--url_1.0.0_123b-45----12ef', but got '%v'", s)
	}

}
