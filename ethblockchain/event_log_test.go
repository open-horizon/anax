package ethblockchain

import (
	"testing"
)

func TestClientConstructor(t *testing.T) {
	if event_log := Event_Log_Factory(nil, "0x0123456789012345678901234567890123456789"); event_log == nil {
		t.Errorf("Factory returned nil, but should not.\n")
	}
}
