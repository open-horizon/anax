package api

import (
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/worker"
)

func (a *API) status(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":

		info := apicommon.NewInfo(a.GetHTTPFactory(), a.GetExchangeURL(), a.GetCSSURL(), a.GetExchangeId(), a.GetExchangeToken())

		if err := apicommon.WriteConnectionStatus(info); err != nil {
			glog.Errorf(apiLogString(fmt.Sprintf("Unable to get connectivity status: %v", err)))
		}

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
