// Package main Edge Syncronization Service
//
// This is the main package of the edge synchronization service used by the hzn dev tool.
//
package main

//go:generate swagger generate spec

import (
	"github.com/open-horizon/anax/ess"
	"github.com/open-horizon/edge-sync-service/core/base"
)

func main() {
	base.StandaloneSyncService(&ess.HZNDEVAuthenticate{})
}
