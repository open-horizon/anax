// Package main Cloud Syncronization Service
//
// This is the main package of the cloud synchronization service
//
//	schemes: http
//	host: localhost
//	basePath: /
//	version: 0.0.1
//
//	consumes:
//	- application/json
//
//	produces:
//	- application/json
//
// swagger:meta
package main

//go:generate swagger generate spec

import (
	"github.com/open-horizon/anax/css"
	"github.com/open-horizon/edge-sync-service/core/base"
)

func main() {
	base.ConfigStandaloneSyncService()
	base.StandaloneSyncService(&css.HorizonAuthenticate{})
}
