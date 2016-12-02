package ethblockchain

import (
    // "fmt"
    "testing"
    )

func TestConstructor(t *testing.T) {
    if c := RPC_Connection_Factory("localhost",8545,""); c == nil {
        t.Errorf("Factory returned nil, but should not.\n")
    } else if c.Get_fullURL() != "http://localhost:8545" {
        t.Errorf("Factory did not correctly construct URL, returned %v\n",c.Get_fullURL())
    } else if c := RPC_Connection_Factory("",0,"http://localhost:8545"); c == nil {
        t.Errorf("Factory returned nil, but should not.\n")
    } else if c.Get_fullURL() != "http://localhost:8545" {
        t.Errorf("Factory did not correctly construct URL, returned %v\n",c.Get_fullURL())
    }
}

func TestBadURLConstructor(t *testing.T) {
    if c := RPC_Connection_Factory("",0,"blah blah"); c != nil {
        t.Errorf("Factory should have returned nil, but did not.\n")
    } else if c := RPC_Connection_Factory("",0,"other://host:1234"); c != nil {
        t.Errorf("Factory should have returned nil, but did not.\n")
    } else if c := RPC_Connection_Factory("",0,"host:1234"); c != nil {
        t.Errorf("Factory should have returned nil, but did not.\n")
    } else if c := RPC_Connection_Factory("",0,"://host:1234"); c != nil {
        t.Errorf("Factory should have returned nil, but did not.\n")
    } else if c := RPC_Connection_Factory("",0,"//host:1234"); c != nil {
        t.Errorf("Factory should have returned nil, but did not.\n")
    } else if c := RPC_Connection_Factory("",0,"http://local_host:1234"); c != nil {
        t.Errorf("Factory should have returned nil, but did not.\n")
    } else if c := RPC_Connection_Factory("",0,"http://host:host"); c != nil {
        t.Errorf("Factory should have returned nil, but did not.\n")
    }
}

func TestBadConstructor(t *testing.T) {
    if c := RPC_Connection_Factory("",0,""); c != nil {
        t.Errorf("Factory should have returned nil, but did not.\n")
    } else if c := RPC_Connection_Factory("local_host",0,""); c != nil {
        t.Errorf("Factory should have returned nil, but did not.\n")
    } else if c := RPC_Connection_Factory("local_host",1,""); c != nil {
        t.Errorf("Factory should have returned nil, but did not.\n")
    }
}
