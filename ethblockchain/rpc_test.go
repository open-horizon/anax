// +build unit

package ethblockchain

import (
	"testing"
)

func TestClient_Constructor(t *testing.T) {
	rpcClient := RPC_Connection_Factory("localhost", 8545, "")

	if c := RPC_Client_Factory(httpClientFactory(t), rpcClient); c == nil {
		t.Errorf("Factory returned nil, but should not.\n")
	} else if c.Get_connection().Get_fullURL() != "http://localhost:8545" {
		t.Errorf("Factory returned a client that does not point to the right connection URL: %v\n", c.Get_connection().Get_fullURL())
	}
}

func TestBadClientConstructor(t *testing.T) {

	if c := RPC_Client_Factory(httpClientFactory(t), nil); c != nil {
		t.Errorf("Factory did not return nil, but should have.\n")
	}
}
