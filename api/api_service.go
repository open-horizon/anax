package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"io/ioutil"
	"net/http"
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
		getDevice := exchange.GetHTTPDeviceHandler(a)
		patchDevice := exchange.GetHTTPPatchDeviceHandler(a)

		// Input should be: Service type w/ zero or more Attribute types
		var service Service
		body, _ := ioutil.ReadAll(r.Body)

		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		if err := decoder.Decode(&service); err != nil {
			errorhandler(NewAPIUserInputError(fmt.Sprintf("Input body couldn't be deserialized to %v object: %v, error: %v", resource, string(body), err), "service"))
			return
		}

		create_service_error_handler := func(err error) bool {
			service_url := ""
			if service.Url != nil {
				service_url = *service.Url
			}
			LogServiceEvent(a.db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_CONFIG_SVC, service_url, err), persistence.EC_ERROR_SERVICE_CONFIG, &service)
			return errorhandler(err)
		}

		// Validate and create the service object and all of the service specific attributes in the body
		// of the request.
		errHandled, newService, msg := CreateService(&service, create_service_error_handler, getPatterns, resolveService, getService, getDevice, patchDevice, nil, a.db, a.Config, true)
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

// For gettting or changing the service configstate. The supported stated are "suspended" and "active"
func (a *API) service_configstate(w http.ResponseWriter, r *http.Request) {

	resource := "service/configstate"
	errorhandler := GetHTTPErrorHandler(w)

	_, errWritten := a.existingDeviceOrError(w)
	if errWritten {
		return
	}

	switch r.Method {
	case "GET":

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		getServicesConfigState := exchange.GetHTTPServicesConfigStateHandler(a)

		errorHandled, out := FindServiceConfigStateForOutput(errorhandler, getServicesConfigState, a.db)
		if errorHandled {
			return
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "POST":

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		var service_cs exchange.ServiceConfigState
		body, _ := ioutil.ReadAll(r.Body)

		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		if err := decoder.Decode(&service_cs); err != nil {
			errorhandler(NewAPIUserInputError(fmt.Sprintf("Input body couldn't be deserialized to %v object: %v, error: %v", resource, string(body), err), "service"))
			return
		}

		// error handler to save the event log and then pass the error to the default error handler.
		service_configstate_error_handler := func(err error) bool {
			LogServiceEvent(a.db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_CHANGE_SVC_CONFIGSTATE, resource, err), persistence.EC_ERROR_CHANGING_SERVICE_CONFIGSTATE, NewService(service_cs.Url, service_cs.Org, "", cutil.ArchString(), ""))
			return errorhandler(err)
		}

		s_string := ""
		if service_cs.Url == "" {
			if service_cs.Org == "" {
				s_string = "all registered services"
			} else {
				s_string = "all registered services under the organization " + service_cs.Org
			}
		} else {
			s_string = cutil.FormOrgSpecUrl(service_cs.Url, service_cs.Org)
		}
		LogServiceEvent(a.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_START_CHANGE_SVC_CONFIGSTATE, service_cs.ConfigState, s_string), persistence.EC_START_CHANGING_SERVICE_CONFIGSTATE, NewService(service_cs.Url, service_cs.Org, "", cutil.ArchString(), ""))

		getDevice := exchange.GetHTTPDeviceHandler(a)
		postDeviceSCS := exchange.GetHTTPPostDeviceServicesConfigStateHandler(a)
		errorHandled, suspended_services := ChangeServiceConfigState(&service_cs, service_configstate_error_handler, getDevice, postDeviceSCS, a.db)
		if errorHandled {
			return
		} else {
			// fire event to handle the newly suspended services if any
			if len(suspended_services) != 0 {
				a.Messages() <- events.NewServiceConfigStateChangeMessage(events.SERVICE_SUSPENDED, suspended_services)
			}

			LogServiceEvent(a.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_COMPLETE_CHANGE_SVC_CONFIGSTATE, service_cs.ConfigState, s_string), persistence.EC_CHANGING_SERVICE_CONFIGSTATE_COMPLETE, NewService(service_cs.Url, service_cs.Org, "", cutil.ArchString(), ""))
			w.WriteHeader(http.StatusOK)
		}

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

		// Gather all the policies from the local filesystem and format them for output.
		if out, err := findPoliciesForOutput(a.pm, a.db); err != nil {
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
