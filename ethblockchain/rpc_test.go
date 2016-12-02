package ethblockchain

import (
    // "fmt"
    "testing"
    )

func TestClient_Constructor(t *testing.T) {
    rpc_client := RPC_Connection_Factory("localhost",8545,"")
    if c := RPC_Client_Factory(rpc_client); c == nil {
        t.Errorf("Factory returned nil, but should not.\n")
    } else if c.Get_connection().Get_fullURL() != "http://localhost:8545" {
        t.Errorf("Factory returned a client that does not point to the right connection URL: %v\n", c.Get_connection().Get_fullURL())
    }
}

func TestBadClientConstructor(t *testing.T) {
    if c := RPC_Client_Factory(nil); c != nil {
        t.Errorf("Factory did not return nil, but should have.\n")
    }
}