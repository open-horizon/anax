package api

import (
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/worker"
	"net/http"
)

func (a *API) status(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":

		info := apicommon.NewInfo(a.GetHTTPFactory(), a.GetExchangeURL(), a.GetCSSURL(), a.GetExchangeId(), a.GetExchangeToken(), a.Config.Edge.AgbotURL)

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
