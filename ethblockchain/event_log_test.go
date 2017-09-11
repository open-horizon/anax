package ethblockchain

import (
	"github.com/open-horizon/anax/config"
	"testing"
)

func httpClientFactory(t *testing.T) *config.HTTPClientFactory {
	collab, err := config.NewCollaborators(config.HorizonConfig{})
	if err != nil {
		t.Error(err)
	}

	return collab.HTTPClientFactory
}

func TestClientConstructor(t *testing.T) {
	if event_log := Event_Log_Factory(httpClientFactory(t), nil, "0x0123456789012345678901234567890123456789"); event_log == nil {
		t.Errorf("Factory returned nil, but should not.\n")
	}
}
