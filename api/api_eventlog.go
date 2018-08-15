package api

import (
	"fmt"
	"github.com/golang/glog"
	"net/http"
	"strings"
)

// get the eventlogs for current registration.
func (a *API) eventlog(w http.ResponseWriter, r *http.Request) {

	resource := "eventlog"

	errorHandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":
		all_loags := false
		if r.URL != nil && strings.Contains(r.URL.Path, "all") {
			all_loags = true
		}

		if err := r.ParseForm(); err != nil {
			errorHandler(NewAPIUserInputError(fmt.Sprintf("Error parsing the selections %v. %v", r.Form, err), "selection"))
			return
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v with selection %v", r.Method, resource, r.Form)))

		if out, err := FindEventLogsForOutput(a.db, all_loags, r.Form); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}
