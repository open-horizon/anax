// +build unit

package agreementbot

import (
	//"fmt"
	"github.com/open-horizon/anax/events"
	"github.com/stretchr/testify/assert"
	"testing"
)

// Ensure that the secret manager queueing is working.
func Test_SUM_Queue(t *testing.T) {

	pn := make([]string, 0)
	pn = append(pn, "p1")
	pn = append(pn, "p2")
	pn = append(pn, "p3")

	su1 := events.NewSecretUpdate("org1", "mysecret1", 100, pn)

	pn[1] = "p4"

	su2 := events.NewSecretUpdate("org1", "mysecret2", 100, pn)

	sus := events.NewSecretUpdates()
	sus.AddSecretUpdate(su1)
	sus.AddSecretUpdate(su2)

	// Now test the secret update manager.
	sum := NewSecretUpdateManager()
	sum.SetUpdateEvent(sus)

	assert.True(t, len(sum.PendingUpdates) == 1, "There should be 1 pending update")

	ne := sum.GetNextUpdateEvent()

	assert.True(t, len(sum.PendingUpdates) == 0, "There should be 0 pending updates")
	assert.True(t, ne.Updates[0].SecretFullName == "mysecret1", "The first secret with an update should be mysecret1")


}