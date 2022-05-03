package api

import (
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	"net/http"
)

func (a *API) status(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// get the cert and config versions for the local db
		cert_version := ""
		config_version := ""
		pDevice, err := persistence.FindExchangeDevice(a.db)
		if err == nil && pDevice != nil {
			sw_version := pDevice.SoftwareVersions
			if sw_version != nil {
				cert_version, _ = sw_version[persistence.CERT_VERSION]
				config_version, _ = sw_version[persistence.CONFIG_VERSION]
			}
		}

		info := apicommon.NewInfo(a.GetHTTPFactory(), a.GetExchangeURL(), a.GetCSSURL(),
			a.GetExchangeId(), a.GetExchangeToken(), cert_version, config_version)

		writeResponse(w, info, http.StatusOK)
	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) workerstatus(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		status := worker.GetWorkerStatusManager()
		writeResponse(w, status, http.StatusOK)
	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
