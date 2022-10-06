//go:build unit
// +build unit

package cliutils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ValidateOrg(t *testing.T) {

	// Invalid org name containing [_]
	org_name := "test_org"
	invalidFlag := ValidateOrg(org_name)
	assert.Equal(t, true, invalidFlag, fmt.Sprintf("%s should be flagged as invalid org name.", org_name))

	// Invalid org name containing [space]
	org_name = "test org"
	invalidFlag = ValidateOrg(org_name)
	assert.Equal(t, true, invalidFlag, fmt.Sprintf("%s should be flagged as invalid org name.", org_name))

	// Invalid org name containing [']
	org_name = "test'org"
	invalidFlag = ValidateOrg(org_name)
	assert.Equal(t, true, invalidFlag, fmt.Sprintf("%s should be flagged as invalid org name.", org_name))

	// Invalid org name containing [?]
	org_name = "test?org"
	invalidFlag = ValidateOrg(org_name)
	assert.Equal(t, true, invalidFlag, fmt.Sprintf("%s should be flagged as invalid org name.", org_name))

	// Valid org name
	org_name = "test-org"
	invalidFlag = ValidateOrg(org_name)
	assert.Equal(t, false, invalidFlag, fmt.Sprintf("%s should be a valid org name.", org_name))

}
