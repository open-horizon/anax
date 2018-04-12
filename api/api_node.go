package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/version"
	"github.com/open-horizon/anax/worker"
)

func (a *API) node(w http.ResponseWriter, r *http.Request) {

	resource := "node"

	errorHandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		if out, err := FindHorizonDeviceForOutput(a.db); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "HEAD":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		if out, err := FindHorizonDeviceForOutput(a.db); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else if serial, errWritten := serializeResponse(w, out); !errWritten {
			w.Header().Add("Content-Length", strconv.Itoa(len(serial)))
			w.WriteHeader(http.StatusOK)
		}

	case "POST":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// make sure current exchange version meet the requirement
		if err := version.VerifyExchangeVersion(a.GetHTTPFactory(), a.GetExchangeURL(), a.GetExchangeId(), a.GetExchangeToken(), false); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error verifiying exchange version. error: %v", err)))
			return
		}

		orgHandler := exchange.GetHTTPExchangeOrgHandlerWithContext(a.Config)
		patternHandler := exchange.GetHTTPExchangePatternHandlerWithContext(a.Config)

		// Read in the HTTP body and pass the device registration off to be validated and created.
		var newDevice HorizonDevice
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &newDevice); err != nil {
			errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body couldn't be deserialized to %v object: %v, error: %v", resource, string(body), err), "device"))
			return
		}

		// Validate and create the new device registration.
		errHandled, device, exDev := CreateHorizonDevice(&newDevice, errorHandler, orgHandler, patternHandler, a.em, a.db)
		if errHandled {
			return
		}

		a.EC = worker.NewExchangeContext(fmt.Sprintf("%v/%v", *device.Org, *device.Id), *device.Token, a.Config.Edge.ExchangeURL, false, a.Config.Collaborators.HTTPClientFactory)

		a.Messages() <- events.NewEdgeRegisteredExchangeMessage(events.NEW_DEVICE_REG, *device.Id, *device.Token, *device.Org, *device.Pattern)

		writeResponse(w, exDev, http.StatusCreated)

	case "PATCH":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// make sure current exchange version meet the requirement
		if err := version.VerifyExchangeVersion(a.GetHTTPFactory(), a.GetExchangeURL(), a.GetExchangeId(), a.GetExchangeToken(), false); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error verifiying exchange version. error: %v", err)))
			return
		}

		var device HorizonDevice
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &device); err != nil {
			errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body couldn't be deserialized to %v object: %v, error: %v", resource, string(body), err), "device"))
			return
		}

		// Validate the PATCH input and update the object in the database.
		errHandled, dev, exDev := UpdateHorizonDevice(&device, errorHandler, a.db)
		if errHandled {
			return
		}

		a.EC = worker.NewExchangeContext(fmt.Sprintf("%v/%v", *device.Org, *device.Id), *dev.Token, a.Config.Edge.ExchangeURL, a.GetServiceBased(), a.Config.Collaborators.HTTPClientFactory)

		writeResponse(w, exDev, http.StatusOK)

	case "DELETE":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Retrieve the optional query parameter
		removeNode := r.URL.Query().Get("removeNode")
		block := r.URL.Query().Get("block")

		// Validate the DELETE request and delete the object from the database.
		errHandled := DeleteHorizonDevice(removeNode, block, a.em, a.Messages(), errorHandler, a.db)
		if errHandled {
			return
		}

		if a.shutdownError != "" {
			errorHandler(NewSystemError(fmt.Sprintf("received error handling %v on resource %v, error: %v", r.Method, resource, a.shutdownError)))
			return
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handled %v on resource %v", r.Method, resource)))

		w.WriteHeader(http.StatusNoContent)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, POST, PATCH, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) nodeconfigstate(w http.ResponseWriter, r *http.Request) {

	resource := "node/configstate"

	errorHandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		if out, err := FindConfigstateForOutput(a.db); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "HEAD":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		if out, err := FindConfigstateForOutput(a.db); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else if serial, errWritten := serializeResponse(w, out); !errWritten {
			w.Header().Add("Content-Length", strconv.Itoa(len(serial)))
			w.WriteHeader(http.StatusOK)
		}

	case "PUT":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// make sure current exchange version meet the requirement
		if err := version.VerifyExchangeVersion(a.GetHTTPFactory(), a.GetExchangeURL(), a.GetExchangeId(), a.GetExchangeToken(), false); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error verifiying exchange version. error: %v", err)))
			return
		}

		microserviceHandler := exchange.GetHTTPMicroserviceHandler(a)
		patternHandler := exchange.GetHTTPExchangePatternHandler(a)
		workloadResolver := exchange.GetHTTPWorkloadResolverHandler(a)
		serviceResolver := exchange.GetHTTPServiceResolverHandler(a)
		getService := exchange.GetHTTPServiceHandler(a)

		// Read in the HTTP body and pass the device registration off to be validated and created.
		var configState Configstate
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &configState); err != nil {
			errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body couldn't be deserialized to %v object: %v, error: %v", resource, string(body), err), "configstate"))
			return
		}

		// Validate and update the config state.
		errHandled, cfg, msgs := UpdateConfigstate(&configState, errorHandler, microserviceHandler, patternHandler, workloadResolver, serviceResolver, getService, a.db, a.Config)
		if errHandled {
			return
		}

		pDevice, err := persistence.FindExchangeDevice(a.db)
		if err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err)))
			return
		}

		// Send out all messages
		for _, msg := range msgs {
			a.Messages() <- msg
		}

		// Send out the config complete message that enables the device for agreements
		a.Messages() <- events.NewEdgeConfigCompleteMessage(events.NEW_DEVICE_CONFIG_COMPLETE, pDevice.IsServiceBased())

		writeResponse(w, cfg, http.StatusCreated)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, POST, PATCH, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
