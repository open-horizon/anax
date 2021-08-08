// +build unit

package events

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

// This test ensures that array copy functions are working as expected.
func Test_Construction(t *testing.T) {

	// Make sure the policy array is copied when a new secret update is created.
	pn := make([]string, 0)
	pn = append(pn, "p1")
	pn = append(pn, "p2")
	pn = append(pn, "p3")

	su1 := NewSecretUpdate("org1", "mysecret1", 100, pn, []string{})

	pn[1] = "p4"

	su2 := NewSecretUpdate("org1", "mysecret2", 100, pn, []string{})

	assert.True(t, su1.PolicyNames[1] == "p2", "The second element of policy names in su1 should have the value 'p2'")
	assert.True(t, su2.PolicyNames[1] == "p4", "The second element of policy names in su2 should have the value 'p4'")

	// Make sure the secret array and policy arrays are consumed as expected.
	sus := NewSecretUpdates()
	sus.AddSecretUpdate(su1)
	sus.AddSecretUpdate(su2)

	// Verify that the updates structure ins unaffected
	assert.True(t, sus.Updates[0].SecretFullName == "mysecret1", "The secret name in the first update should be 'mysecret'")
	assert.True(t, sus.Updates[0].PolicyNames[1] == "p2", "The second element of policy names in su1 should have the value 'p2'")
	assert.True(t, sus.Updates[1].PolicyNames[1] == "p4", "The second element of policy names in su2 should have the value 'p4'")

	t.Log(fmt.Sprintf("%v", sus))

}
