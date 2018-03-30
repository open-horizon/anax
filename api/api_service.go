package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
)

func (a *API) service(w http.ResponseWriter, r *http.Request) {

	resource := "service"
	errorhandler := GetHTTPErrorHandler(w)

	_, errWritten := a.existingDeviceOrError(w)
	if errWritten {
		return
	}

	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		// we don't support getting just one yet
		if id != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Get the persisted device to see if it's service or workload based.
		if pDevice, errWritten := a.existingDeviceOrError(w); errWritten {
			return
		} else if !pDevice.IsServiceBased() {
			writeResponse(w, *NewServiceOutput(), http.StatusOK)
			return
		}

		// Gather all the service info from the database and format for output.
		if out, err := FindServicesForOutput(a.pm, a.db, a.Config); err != nil {
			errorhandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else {
			writeResponse(w, *out, http.StatusOK)
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// For working with a node's representation of a service, including the policy and input variables of the service.
func (a *API) serviceconfig(w http.ResponseWriter, r *http.Request) {

	resource := "service/config"
	errorhandler := GetHTTPErrorHandler(w)

	_, errWritten := a.existingDeviceOrError(w)
	if errWritten {
		return
	}

	switch r.Method {
	case "GET":

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Get the persisted device to see if it's service or workload based.
		if pDevice, errWritten := a.existingDeviceOrError(w); errWritten {
			return
		} else if !pDevice.IsServiceBased() {
			out := make(map[string][]MicroserviceConfig)
			out["config"] = make([]MicroserviceConfig, 0, 10)
			writeResponse(w, out, http.StatusOK)
			return
		}

		if out, err := FindServiceConfigForOutput(a.pm, a.db); err != nil {
			errorhandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "POST":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		getService := exchange.GetHTTPServiceHandler(a)
		getPatterns := exchange.GetHTTPExchangePatternHandler(a)
		resolveService := exchange.GetHTTPServiceResolverHandler(a)

		// Input should be: Service type w/ zero or more Attribute types
		var service Service
		body, _ := ioutil.ReadAll(r.Body)

		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		if err := decoder.Decode(&service); err != nil {
			errorhandler(NewAPIUserInputError(fmt.Sprintf("Input body couldn't be deserialized to %v object: %v, error: %v", resource, string(body), err), "service"))
			return
		}

		// Validate and create the service object and all of the service specific attributes in the body
		// of the request.
		errHandled, newService, msg := CreateService(&service, errorhandler, getPatterns, resolveService, getService, a.db, a.Config, true)
		if errHandled {
			return
		}

		// Send the policy created message to the internal bus.
		if msg != nil {
			a.Messages() <- msg
		}

		// Write the new service back to the caller.
		writeResponse(w, newService, http.StatusCreated)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// For working with a node's policy files.
func (a *API) servicepolicy(w http.ResponseWriter, r *http.Request) {

	resource := "service/policy"
	errorhandler := GetHTTPErrorHandler(w)

	_, errWritten := a.existingDeviceOrError(w)
	if errWritten {
		return
	}

	switch r.Method {
	case "GET":

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Get the persisted device to see if it's service or workload based.
		if pDevice, errWritten := a.existingDeviceOrError(w); errWritten {
			return
		} else if !pDevice.IsServiceBased() {
			out := make(map[string]policy.Policy)
			writeResponse(w, out, http.StatusOK)
			return
		}

		// Gather all the policies from the local filesystem and format them for output.
		if out, err := FindPoliciesForOutput(a.pm, a.db); err != nil {
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
