package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
)

func (a *API) workload(w http.ResponseWriter, r *http.Request) {

	resource := "workload"
	errorhandler := GetHTTPErrorHandler(w)

	_, errWritten := a.existingDeviceOrError(w)
	if errWritten {
		return
	}

	switch r.Method {
	case "GET":

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Gather all the agreements from the local database and format them for output.
		if out, err := FindWorkloadForOutput(a.db, a.Config); err != nil {
			errorhandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
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

func (a *API) workloadConfig(w http.ResponseWriter, r *http.Request) {

	existingDevice, errWritten := a.existingDeviceOrError(w)
	if errWritten {
		return
	}

	resource := "workload/config"
	errorhandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Gather all the agreements from the local database and format them for output.
		if out, err := FindWorkloadConfigForOutput(a.db); err != nil {
			errorhandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "POST":

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		getWorkload := a.exchHandlers.GetHTTPWorkloadHandler()

		// Demarshal the input body
		var cfg WorkloadConfig
		body, _ := ioutil.ReadAll(r.Body)

		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		if err := decoder.Decode(&cfg); err != nil {
			errorhandler(NewAPIUserInputError(fmt.Sprintf("Input body could not be demarshalled, error: %v", err), "workloadConfig"))
			return
		}

		// Validate and create the workloadconfig object in the body of the request.
		errHandled, newWC := CreateWorkloadconfig(&cfg, existingDevice, errorhandler, getWorkload, a.db)
		if errHandled {
			return
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handled %v on resource %v", r.Method, resource)))

		// Write the new workloadconfig back to the caller.
		writeResponse(w, newWC, http.StatusCreated)

	case "DELETE":

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Demarshal the input body. Use the same body as the POST but ignore the variables section.
		var cfg WorkloadConfig
		body, _ := ioutil.ReadAll(r.Body)

		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		if err := decoder.Decode(&cfg); err != nil {
			errorhandler(NewAPIUserInputError(fmt.Sprintf("Input body could not be demarshalled, error: %v", err), "workloadConfig"))
			return
		}

		// Validate and create the workloadconfig object in the body of the request.
		errHandled := DeleteWorkloadconfig(&cfg, errorhandler, a.db)
		if errHandled {
			return
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handled %v on resource %v", r.Method, resource)))

		w.WriteHeader(http.StatusNoContent)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
