package api

import (
	"encoding/json"
	"net/http"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
)

func tokenRandom(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		str, err := cutil.SecureRandomString()
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		out := map[string]string{
			"token": str,
		}

		serial, err := json.Marshal(out)
		if err != nil {
			glog.Error(apiLogString(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write(serial); err != nil {
			glog.Error(apiLogString(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
