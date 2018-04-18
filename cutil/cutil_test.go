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
