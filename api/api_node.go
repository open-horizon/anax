package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangesync"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
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

		// Read in the HTTP body and pass the device registration off to be validated and created.
		var newDevice HorizonDevice
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &newDevice); err != nil {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_API_ERR_PARSING_INPUT_FOR_NODE_REG, string(body), err.Error()),
				persistence.EC_API_USER_INPUT_ERROR, nil)
			errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body couldn't be deserialized to %v object: %v, error: %v", resource, string(body), err), "device"))
			return
		}

		orgHandler := exchange.GetHTTPExchangeOrgHandlerWithContext(a.Config)
		patternHandler := exchange.GetHTTPExchangePatternHandlerWithContext(a.Config)
		versionHandler := exchange.GetHTTPExchangeVersionHandler(a.Config)
		patchDeviceHandler := exchange.GetHTTPPatchDeviceHandler2(a.Config)
		getDeviceHandler := exchange.GetHTTPDeviceHandler2(a.Config)

		create_device_error_handler := func(err error) bool {
			dev_id := ""
			if newDevice.Id != nil {
				dev_id = *newDevice.Id
			}
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_IN_NODE_REG, dev_id, err.Error()), persistence.EC_ERROR_NODE_CONFIG_REG, &newDevice)
			return errorHandler(err)
		}

		// Validate and create the new device registration.
		errHandled, device, exDev := CreateHorizonDevice(&newDevice, create_device_error_handler, orgHandler, patternHandler, versionHandler, patchDeviceHandler, getDeviceHandler, a.em, a.db)
		if errHandled {
			return
		}

		a.EC = worker.NewExchangeContext(fmt.Sprintf("%v/%v", *device.Org, *device.Id), *device.Token, a.Config.Edge.ExchangeURL, a.Config.GetCSSURL(), a.Config.Collaborators.HTTPClientFactory)

		// sync the node policy and userinput with the exchange
		if err := exchangesync.NodeInitalSetup(a.db, exchange.GetHTTPDeviceHandler(a)); err != nil {
			create_device_error_handler(fmt.Errorf("Failed to initially set up local copy of the exchange node. %v", err))
		}
		if _, err := exchangesync.NodePolicyInitalSetup(a.db, a.Config, exchange.GetHTTPNodePolicyHandler(a), exchange.GetHTTPPutNodePolicyHandler(a)); err != nil {
			create_device_error_handler(fmt.Errorf("Failed to initially set up node policy. %v", err))
			return
		}
		if err := exchangesync.NodeUserInputInitalSetup(a.db, exchange.GetHTTPPatchDeviceHandler(a)); err != nil {
			create_device_error_handler(fmt.Errorf("Failed to initially set up node user input. %v", err))
			return
		}

		a.Messages() <- events.NewEdgeRegisteredExchangeMessage(events.NEW_DEVICE_REG, *device.Id, *device.Token, *device.Org, *device.Pattern, *device.NodeType)

		writeResponse(w, exDev, http.StatusCreated)

	case "PATCH":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		var device HorizonDevice
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &device); err != nil {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_API_ERR_PARSING_INPUT_FOR_NODE_UPDATE, body, err.Error()),
				persistence.EC_API_USER_INPUT_ERROR, nil)
			errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body couldn't be deserialized to %v object: %v, error: %v", resource, string(body), err), "device"))
			return
		}

		update_device_error_handler := func(err error) bool {
			dev_id := ""
			if device.Id != nil {
				dev_id = *device.Id
			}
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_IN_NODE_UPDATE, dev_id, err.Error()),
				persistence.EC_ERROR_NODE_UPDATE, &device)
			return errorHandler(err)
		}

		versionHandler := exchange.GetHTTPExchangeVersionHandler(a.Config)

		// Validate the PATCH input and update the object in the database.
		errHandled, dev, exDev := UpdateHorizonDevice(&device, update_device_error_handler, versionHandler, a.db)
		if errHandled {
			return
		}

		a.EC = worker.NewExchangeContext(fmt.Sprintf("%v/%v", *device.Org, *device.Id), *dev.Token, a.Config.Edge.ExchangeURL, a.Config.GetCSSURL(), a.Config.Collaborators.HTTPClientFactory)

		writeResponse(w, exDev, http.StatusOK)

	case "DELETE":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Retrieve the optional query parameter
		removeNode := r.URL.Query().Get("removeNode")
		deepClean := r.URL.Query().Get("deepClean")
		block := r.URL.Query().Get("block")

		// Validate the DELETE request and delete the object from the database.
		errHandled := DeleteHorizonDevice(removeNode, deepClean, block, a.em, a.Messages(), errorHandler, a.db)
		if errHandled {
			return
		}

		if a.shutdownError != "" {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_IN_NODE_UNREG, a.shutdownError),
				persistence.EC_ERROR_NODE_UNREG, nil)
			errorHandler(NewServiceUnavailableError(a.shutdownError))
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
			eventlog.LogExchangeEvent(a.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_API_ERR_IN_VERIFY_EXCH_VERSION, err.Error()),
				persistence.EC_EXCHANGE_ERROR, a.GetExchangeURL())
			errorHandler(NewSystemError(fmt.Sprintf("Error verifiying exchange version. error: %v", err)))
			return
		}

		patternHandler := exchange.GetHTTPExchangePatternHandler(a)
		serviceResolver := exchange.GetHTTPServiceDefResolverHandler(a)
		getService := exchange.GetHTTPServiceHandler(a)
		getDevice := exchange.GetHTTPDeviceHandler(a)
		patchDevice := exchange.GetHTTPPatchDeviceHandler(a)

		// Read in the HTTP body and pass the device registration off to be validated and created.
		var configState Configstate
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &configState); err != nil {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_API_ERR_PARSING_INPUT_FOR_NODE_UNREG, string(body), err.Error()),
				persistence.EC_API_USER_INPUT_ERROR, nil)
			errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body couldn't be deserialized to %v object: %v, error: %v", resource, string(body), err), "configstate"))
			return
		}

		// Validate and update the config state.
		errHandled, cfg, msgs := UpdateConfigstate(&configState, errorHandler, patternHandler, serviceResolver, getService, getDevice, patchDevice, a.db, a.Config)
		if errHandled {
			return
		}

		// Send out all messages
		for _, msg := range msgs {
			a.Messages() <- msg
		}

		// Send out the config complete message that enables the device for agreements
		a.Messages() <- events.NewEdgeConfigCompleteMessage(events.NEW_DEVICE_CONFIG_COMPLETE)

		writeResponse(w, cfg, http.StatusCreated)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, POST, PATCH, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) nodepolicy(w http.ResponseWriter, r *http.Request) {

	resource := "node/policy"

	errorHandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		if out, err := FindNodePolicyForOutput(a.db); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "HEAD":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		if out, err := FindNodePolicyForOutput(a.db); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else if serial, errWritten := serializeResponse(w, out); !errWritten {
			w.Header().Add("Content-Length", strconv.Itoa(len(serial)))
			w.WriteHeader(http.StatusOK)
		}

	case "PUT", "POST":
		// Because there is one node policy object, POST and PUT are interchangeable. Either can be used to create the object and either
		// can be used to update the existing object.
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Read in the HTTP body and pass the device registration off to be validated and created.
		var nodePolicy externalpolicy.ExternalPolicy
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &nodePolicy); err != nil {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_API_ERR_PARSING_INPUT_FOR_NODE_POLICY, string(body), err.Error()),
				persistence.EC_API_USER_INPUT_ERROR, nil)
			errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body could not be deserialized to %v object: %v, error: %v", resource, string(body), err), "body"))
			return
		}

		update_node_policy_error_handler := func(device interface{}, err error) bool {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_IN_NODE_POLICY_CREATE, err.Error()), persistence.EC_ERROR_NODE_POLICY_UPDATE, device)
			return errorHandler(err)
		}
		nodeGetPolicyHandler := exchange.GetHTTPNodePolicyHandler(a)
		nodePutPolicyHandler := exchange.GetHTTPPutNodePolicyHandler(a)

		// Validate and create or update the node policy.
		errHandled, cfg, msgs := UpdateNodePolicy(&nodePolicy, update_node_policy_error_handler, nodeGetPolicyHandler, nodePutPolicyHandler, a.db)
		if errHandled {
			return
		}

		// Send out all messages
		for _, msg := range msgs {
			a.Messages() <- msg
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handled %v on resource %v", r.Method, resource)))

		writeResponse(w, cfg, http.StatusCreated)

	case "PATCH":
		//Read in the HTTP message body and pass the policy update to be updated in the local db and the exchange
		//Patch object can be either a constraint expression or a property list
		//This reads in the message body and throws an error if it cannot be unmarshaled or if it is neither a constraint expression nor a property list
		var constraintExp map[string]externalpolicy.ConstraintExpression
		var propertyList map[string]externalpolicy.PropertyList
		body, _ := ioutil.ReadAll(r.Body)
		err := json.Unmarshal(body, &constraintExp)
		if _, ok := constraintExp["constraints"]; !ok || err != nil {
			err := json.Unmarshal(body, &propertyList)
			if err != nil {
				LogDeviceEvent(a.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_API_ERR_PARSING_INPUT_FOR_NODE_POLICY_PATCH, string(body), err.Error()),
					persistence.EC_API_USER_INPUT_ERROR, nil)
				errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body could not be deserialized to %v object: %v, error: %v", resource, string(body), err), "body"))
				return
			}
			if _, ok := propertyList["properties"]; !ok {
				LogDeviceEvent(a.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_API_ERR_POLICY_PATCH_INPUT_PROPERTY_ERROR, string(body), err.Error()),
					persistence.EC_API_USER_INPUT_ERROR, nil)
				errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body could not be deserialized to %v object: %v, error: %v", resource, string(body), err), "body"))
				return
			}
		}

		patch_node_policy_error_handler := func(device interface{}, err error) bool {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_IN_NODE_POLICY_PATCH, err.Error()), persistence.EC_ERROR_NODE_POLICY_PATCH, device)
			return errorHandler(err)
		}
		nodeGetPolicyHandler := exchange.GetHTTPNodePolicyHandler(a)
		nodePatchPolicyHandler := exchange.GetHTTPPutNodePolicyHandler(a)

		var patchObject interface{}
		if _, ok := constraintExp["constraints"]; ok {
			//var patchObject externalpolicy.ConstraintExpression
			patchObject = constraintExp["constraints"]
		} else {
			//var patchObject externalpolicy.PropertyList
			patchObject = propertyList["properties"]
		}

		//Validate the patch and update the policy
		errHandled, cfg, msgs := PatchNodePolicy(patchObject, patch_node_policy_error_handler, nodeGetPolicyHandler, nodePatchPolicyHandler, a.db)

		if errHandled {
			return
		}

		// Send out all messages
		for _, msg := range msgs {
			a.Messages() <- msg
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handled %v on resource %v", r.Method, resource)))

		writeResponse(w, cfg, http.StatusCreated)

	case "DELETE":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		delete_node_policy_error_handler := func(device interface{}, err error) bool {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_IN_NODE_POLICY_DEL, err.Error()), persistence.EC_ERROR_NODE_POLICY_UPDATE, device)
			return errorHandler(err)
		}
		nodeGetPolicyHandler := exchange.GetHTTPNodePolicyHandler(a)
		nodeDeletePolicyHandler := exchange.GetHTTPDeleteNodePolicyHandler(a)

		// Validate the DELETE request and delete the object from the database.
		errHandled, msgs := DeleteNodePolicy(delete_node_policy_error_handler, a.db, nodeGetPolicyHandler, nodeDeletePolicyHandler)
		if errHandled {
			return
		}

		// Send out all messages
		for _, msg := range msgs {
			a.Messages() <- msg
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handled %v on resource %v", r.Method, resource)))

		w.WriteHeader(http.StatusNoContent)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, HEAD, PUT, POST, PATCH, DELETE, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) nodeuserinput(w http.ResponseWriter, r *http.Request) {

	resource := "node/userinput"

	errorHandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		if out, err := FindNodeUserInputForOutput(a.db); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "HEAD":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		if out, err := FindNodeUserInputForOutput(a.db); err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else if serial, errWritten := serializeResponse(w, out); !errWritten {
			w.Header().Add("Content-Length", strconv.Itoa(len(serial)))
			w.WriteHeader(http.StatusOK)
		}

	case "PUT", "POST":
		// Because there is one node user input object, POST and PUT are interchangeable. Either can be used to create the object and either
		// can be used to update the existing object.
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Read in the HTTP body and pass the device registration off to be validated and created.
		var nodeUserInput []policy.UserInput
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &nodeUserInput); err != nil {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_API_ERR_PARSING_INPUT_FOR_NODE_UI, string(body), err.Error()),
				persistence.EC_API_USER_INPUT_ERROR, nil)
			errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body could not be deserialized to %v object: %v, error: %v", resource, string(body), err), "body"))
			return
		}

		update_node_userinput_error_handler := func(device interface{}, err error) bool {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_IN_NODE_UI_UPDATE, err.Error()), persistence.EC_ERROR_NODE_USERINPUT_UPDATE, device)
			return errorHandler(err)
		}
		getDevice := exchange.GetHTTPDeviceHandler(a)
		patchDevice := exchange.GetHTTPPatchDeviceHandler(a)
		getService := exchange.GetHTTPServiceHandler(a)

		// Validate and create or update the node policy.
		errHandled, cfg, msgs := UpdateNodeUserInput(nodeUserInput, update_node_userinput_error_handler, getDevice, patchDevice, getService, a.db)
		if errHandled {
			return
		}

		// Send out all messages
		for _, msg := range msgs {
			a.Messages() <- msg
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handled %v on resource %v", r.Method, resource)))

		writeResponse(w, cfg, http.StatusCreated)

	case "PATCH":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		//Read in the HTTP message body and pass the user input update to be updated in the local db and the exchange
		var nodeUserInput []policy.UserInput
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &nodeUserInput); err != nil {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_API_ERR_PARSING_INPUT_FOR_NODE_UI, string(body), err.Error()),
				persistence.EC_API_USER_INPUT_ERROR, nil)
			errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body could not be deserialized to %v object: %v, error: %v", resource, string(body), err), "body"))
			return
		}

		patch_node_userinput_error_handler := func(device interface{}, err error) bool {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_IN_NODE_UI_PATCH, err.Error()), persistence.EC_ERROR_NODE_USERINPUT_PATCH, device)
			return errorHandler(err)
		}

		getDevice := exchange.GetHTTPDeviceHandler(a)
		patchDevice := exchange.GetHTTPPatchDeviceHandler(a)
		getService := exchange.GetHTTPServiceHandler(a)

		//Validate the patch and update the policy
		errHandled, cfg, msgs := PatchNodeUserInput(nodeUserInput, patch_node_userinput_error_handler, getDevice, patchDevice, getService, a.db)

		if errHandled {
			return
		}

		// Send out all messages
		for _, msg := range msgs {
			a.Messages() <- msg
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handled %v on resource %v", r.Method, resource)))

		writeResponse(w, cfg, http.StatusCreated)

	case "DELETE":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		delete_node_userinput_error_handler := func(device interface{}, err error) bool {
			LogDeviceEvent(a.db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_IN_NODE_UI_DEL, err.Error()), persistence.EC_ERROR_NODE_USERINPUT_UPDATE, device)
			return errorHandler(err)
		}
		getDevice := exchange.GetHTTPDeviceHandler(a)
		patchDevice := exchange.GetHTTPPatchDeviceHandler(a)

		// Validate the DELETE request and delete the object from the database.
		errHandled, msgs := DeleteNodeUserInput(delete_node_userinput_error_handler, a.db, getDevice, patchDevice)
		if errHandled {
			return
		}

		// Send out all messages
		for _, msg := range msgs {
			a.Messages() <- msg
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handled %v on resource %v", r.Method, resource)))

		w.WriteHeader(http.StatusNoContent)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, HEAD, PUT, POST, PATCH, DELETE, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
