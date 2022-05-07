//go:build unit
// +build unit

package apicommon

import (
	"github.com/open-horizon/anax/events"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func init_test() map[string]map[string]BlockchainState {
	bc_state := make(map[string]map[string]BlockchainState)

	bc_state["btype1"] = make(map[string]BlockchainState)
	bc_state["btype1"]["instance1"] = BlockchainState{
		ready:       true,
		writable:    true,
		service:     "blockchain11",
		servicePort: "1001",
	}
	bc_state["btype1"]["instance2"] = BlockchainState{
		ready:       true,
		writable:    false,
		service:     "blockchain12",
		servicePort: "1002",
	}
	bc_state["btype2"] = make(map[string]BlockchainState)
	bc_state["btype2"]["instance1"] = BlockchainState{
		ready:       true,
		writable:    true,
		service:     "blockchain21",
		servicePort: "2001",
	}

	return bc_state
}

func Test_GetBCNameMap(t *testing.T) {

	bc_state := init_test()

	bc_ret1 := GetBCNameMap("btype1", bc_state)
	assert.Equal(t, 2, len(bc_ret1))

	bc_ret2 := GetBCNameMap("btype2", bc_state)
	assert.Equal(t, 1, len(bc_ret2))

	bc_ret3 := GetBCNameMap("btype3", bc_state)
	assert.Equal(t, 0, len(bc_ret3))
}

func Test_HandleNewBCInit(t *testing.T) {

	bc_state := init_test()
	lock := sync.Mutex{}
	ev := events.NewBlockchainClientInitializedMessage(events.BC_CLIENT_INITIALIZED, "btype3", "instance1", "myorg", "blockchain31", "3001", "/var")

	HandleNewBCInit(ev, bc_state, &lock)
	bc_ret3 := GetBCNameMap("btype3", bc_state)
	assert.Equal(t, 1, len(bc_ret3))

	instMap, ok := bc_ret3["instance1"]
	assert.True(t, ok)
	assert.Equal(t, instMap.GetService(), "blockchain31")
	assert.Equal(t, instMap.GetServicePort(), "3001")
}

func Test_HandleStoppingBC(t *testing.T) {

	bc_state := init_test()
	lock := sync.Mutex{}

	// btype1 has 2 instances
	bc_ret := GetBCNameMap("btype1", bc_state)
	assert.Equal(t, 2, len(bc_ret))

	// instance1 got removed
	ev1 := events.NewBlockchainClientStoppingMessage(events.BC_CLIENT_STOPPING, "btype1", "instance1", "myorg")
	HandleStoppingBC(ev1, bc_state, &lock)
	bc_ret1 := GetBCNameMap("btype1", bc_state)
	assert.Equal(t, 1, len(bc_ret1))

	_, ok := bc_ret1["instance1"]
	assert.False(t, ok)

	// removing non existance type does not affect the existing bc info
	ev2 := events.NewBlockchainClientStoppingMessage(events.BC_CLIENT_STOPPING, "btype3", "instance1", "myorg")
	HandleStoppingBC(ev2, bc_state, &lock)
	bc_ret2 := GetBCNameMap("btype1", bc_state)
	assert.Equal(t, 1, len(bc_ret2))

	_, ok = bc_ret1["instance2"]
	assert.True(t, ok)

	bc_ret3 := GetBCNameMap("btype3", bc_state)
	assert.Equal(t, 0, len(bc_ret3))

}
