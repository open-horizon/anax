package api

import (
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

func (a *API) agreement(w http.ResponseWriter, r *http.Request) {

	resource := "agreement"
	errorhandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		// we don't support getting just one yet
		if id != "" {
			errorhandler(NewBadRequestError(fmt.Sprintf("path variables not suported on GET %v", resource)))
			return
		}

		// Gather all the agreements from the local database and format them for output.
		if out, err := FindAgreementsForOutput(a.db); err != nil {
			errorhandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "DELETE":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		if id == "" {
			errorhandler(NewBadRequestError(fmt.Sprintf("path variable missing on DELETE %v", resource)))
			return
		}

		// Gather all the agreements from the local database and format them for output.
		errHandled, msg := DeleteAgreement(errorhandler, id, a.db)
		if errHandled {
			return
		}

		if msg != nil {
			a.Messages() <- msg
		}

		w.WriteHeader(http.StatusOK)
		// TODO: Is NoContent more correct for a response to DELETE
		//w.WriteHeader(http.StatusNoContent)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, DELETE, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
