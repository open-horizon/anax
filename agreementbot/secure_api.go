// Package agreementbot Agreement Bot Secure API
//
// This is the secure API for the agreement bot.
//
//   schemes: https
//   host: localhost
//   basePath: https://host:port/
//   version: 0.0.1
//
//   consumes:
//   - application/json
//
//   produces:
//   - application/json
//
// swagger:meta
package agreementbot

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/agreementbot/secrets"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/worker"
	"golang.org/x/text/message"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type SecureAPI struct {
	worker.Manager // embedded field
	name           string
	db             persistence.AgbotDatabase
	httpClient     *http.Client // a shared HTTP client instance for this worker
	em             *events.EventStateManager
	shutdownError  string
	secretProvider secrets.AgbotSecrets
}

func NewSecureAPIListener(name string, config *config.HorizonConfig, db persistence.AgbotDatabase, s secrets.AgbotSecrets) *SecureAPI {
	messages := make(chan events.Message)

	listener := &SecureAPI{
		Manager: worker.Manager{
			Config:   config,
			Messages: messages,
		},
		httpClient:     newHTTPClientFactory().NewHTTPClient(nil),
		name:           name,
		db:             db,
		em:             events.NewEventStateManager(),
		secretProvider: s,
	}

	listener.listen()
	return listener
}

// Worker framework functions
func (a *SecureAPI) Messages() chan events.Message {
	return a.Manager.Messages
}

func (a *SecureAPI) NewEvent(ev events.Message) {

	switch ev.(type) {
	case *events.NodeShutdownCompleteMessage:
		msg, _ := ev.(*events.NodeShutdownCompleteMessage)
		// Now remove myself from the worker dispatch list. When the anax process terminates,
		// the socket listener will terminate also. This is done on a separate thread so that
		// the message dispatcher doesnt get blocked. This worker isnt actually a full blown
		// worker and doesnt have a command thread that it can run on.
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			// This is for the situation where the agbot is running on a node.
			go func() {
				a.Messages() <- events.NewWorkerStopMessage(events.WORKER_STOP, a.GetName())
			}()
		case events.AGBOT_QUIESCE_COMPLETE:
			a.em.RecordEvent(msg, func(m events.Message) { a.saveShutdownError(m) })
			// This is for the situation where the agbot is running stand alone.
			go func() {
				a.Messages() <- events.NewWorkerStopMessage(events.WORKER_STOP, a.GetName())
			}()
		}

	}

	return
}

func (a *SecureAPI) saveShutdownError(msg events.Message) {
	switch msg.(type) {
	case *events.NodeShutdownCompleteMessage:
		m, _ := msg.(*events.NodeShutdownCompleteMessage)
		a.shutdownError = m.Err()
	}
}

func (a *SecureAPI) GetName() string {
	return a.name
}

func (a *SecureAPI) createUserExchangeContext(userId string, passwd string) exchange.ExchangeContext {
	return exchange.NewCustomExchangeContext(userId, passwd, a.Config.AgreementBot.ExchangeURL, a.Config.GetAgbotCSSURL(), newHTTPClientFactory())
}

func (a *SecureAPI) setCommonHeaders(w http.ResponseWriter) http.ResponseWriter {
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Pragma", "no-cache, no-store")
	w.Header().Add("Access-Control-Allow-Headers", "X-Requested-With, content-type, Authorization")
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
	return w
}

// This function sets up the agbot secure http server
func (a *SecureAPI) listen() {
	glog.Info("Starting AgreementBot SecureAPI server")

	// If there is no invalid Agbot config, we will terminate
	apiListenHost := a.Config.AgreementBot.SecureAPIListenHost
	apiListenPort := a.Config.AgreementBot.SecureAPIListenPort
	certFile := a.Config.AgreementBot.SecureAPIServerCert
	keyFile := a.Config.AgreementBot.SecureAPIServerKey
	if apiListenHost == "" {
		glog.Errorf("AgreementBotWorker SecureAPI terminating, no AgreementBot SecureAPIListenHost config.")
		return
	} else if apiListenPort == "" {
		glog.Errorf("AgreementBotWorker SecureAPI terminating, no AgreementBot SecureAPIListenPort config.")
		return
	} else if a.db == nil {
		glog.Errorf("AgreementBotWorker SecureAPI terminating, no AgreementBot database configured.")
		return
	} else if certFile != "" && !fileExists(certFile) {
		glog.Errorf("AgreementBotWorker SecureAPI terminating, secure API server certificate file %v does not exist.", certFile)
		return
	} else if keyFile != "" && !fileExists(keyFile) {
		glog.Errorf("AgreementBotWorker SecureAPI terminating, secure API server key file %v does not exist.", keyFile)
		return
	}

	bSecure := true
	var nocache func(h http.Handler) http.Handler
	if certFile == "" || keyFile == "" {
		glog.V(3).Infof(APIlogString(fmt.Sprintf("Starting AgreementBot Remote API server in non TLS mode with address: %v:%v. The server cert file or key file is not specified in the configuration file.", apiListenHost, apiListenPort)))
		bSecure = false

		nocache = func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w = a.setCommonHeaders(w)
				h.ServeHTTP(w, r)
			})
		}
	} else {
		glog.V(3).Infof(APIlogString(fmt.Sprintf("Starting AgreementBot Remote API server in secure (TLS) mode with address: %v:%v, cert file: %v, key file: %v", apiListenHost, apiListenPort, certFile, keyFile)))

		nocache = func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w = a.setCommonHeaders(w)
				w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
				h.ServeHTTP(w, r)
			})
		}
	}

	// This routine does not need to be a subworker because it will terminate on its own when the main
	// anax process terminates.
	go func() {
		router := mux.NewRouter()

		router.HandleFunc("/deploycheck/policycompatible", a.policy_compatible).Methods("GET", "OPTIONS")
		router.HandleFunc("/deploycheck/userinputcompatible", a.userinput_compatible).Methods("GET", "OPTIONS")
		router.HandleFunc("/deploycheck/deploycompatible", a.deploy_compatible).Methods("GET", "OPTIONS")
		router.HandleFunc("/deploycheck/secretbindingcompatible", a.secretbinding_compatible).Methods("GET", "OPTIONS")
		router.HandleFunc("/org/{org}/secrets/user/{user}", a.userSecrets).Methods("LIST", "OPTIONS")
		router.HandleFunc(`/org/{org}/secrets/user/{user}/{secret:[\w\/\-]+}`, a.userSecret).Methods("GET", "LIST", "PUT", "POST", "DELETE", "OPTIONS")
		router.HandleFunc("/org/{org}/secrets", a.orgSecrets).Methods("LIST", "OPTIONS")
		router.HandleFunc(`/org/{org}/secrets/{secret:[\w\/\-]+}`, a.orgSecret).Methods("GET", "LIST", "PUT", "POST", "DELETE", "OPTIONS")

		apiListen := fmt.Sprintf("%v:%v", apiListenHost, apiListenPort)

		var err error
		if bSecure {
			err = http.ListenAndServeTLS(apiListen, certFile, keyFile, nocache(router))
		} else {
			err = http.ListenAndServe(apiListen, nocache(router))
		}
		if err != nil {
			glog.Fatalf(APIlogString(fmt.Sprintf("failed to start listener on %v, error %v", apiListen, err)))
		}
	}()
}

// This function does policy compatibility check.
func (a *SecureAPI) policy_compatible(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	// swagger:operation GET /deploycheck/policycompatible deployCheckPolicyCompatible
	//
	// Check the policy compatibility
	//
	// This API does the policy compatibility check for the given deployment policy, node policy and service policy. The deployment policy and the service policy will be merged to check against the node policy. If the result is compatible, it means that, when deployed, the node will form an agreement with the agbot and the service will be running on the node.
	//
	// ---
	// consumes:
	//  - application/json
	// produces:
	//  - application/json
	// parameters:
	//  - name: checkAll
	//    in: query
	//    type: bool
	//    required: false
	//    description: "Return the compatibility check result for all the service versions referenced in the deployment policy or pattern."
	//  - name: long
	//    in: query
	//    type: bool
	//    required: false
	//    description: "Show the input which was used to come up with the result."
	//  - name: node_id
	//    in: body
	//    type: string
	//    required: false
	//    description: "The exchange id of the node. Mutually exclusive with node_policy."
	//  - name: node_arch
	//    in: body
	//    type: string
	//    required: false
	//    description: "The architecture of the node."
	//  - name: node_policy
	//    in: body
	//    required: false
	//    description: "The node policy that will be put in the exchange. Mutually exclusive with node_id."
	//    schema:
	//     "$ref": "#/definitions/ExternalPolicy"
	//  - name: business_policy_id
	//    in: body
	//    type: string
	//    required: false
	//    description: "The exchange id of the deployment policy. Mutually exclusive with business_policy."
	//  - name: business_policy
	//    in: body
	//    required: false
	//    description: "The defintion of the deployment policy that will be put in the exchange. Mutually exclusive with business_policy_id."
	//    schema:
	//     "$ref": "#/definitions/BusinessPolicy"
	//  - name: service_policy
	//    in: body
	//    required: false
	//    description: "The service policy that will be put in the exchange. They are for the top level service referenced in the deployment policy. If omitted, the service policy will be retrieved from the exchange. The service policy has the same format as the node policy."
	//    schema:
	//     "$ref": "#/definitions/ExternalPolicy"
	// responses:
	//  '200':
	//    description: "Success"
	//    schema:
	//     type: compcheck.CompCheckOutput
	//     "$ref": "#/definitions/CompCheckOutput"
	//  '400':
	//    description: "Failure - No input found"
	//    schema:
	//     type: string
	//  '501':
	//    description: "Failure - Failed to authenticate"
	//    schema:
	//     type: string
	//  '500':
	//    description: "Failure - Error"
	//    schema:
	//      type: string
	case "GET":
		glog.V(5).Infof(APIlogString(fmt.Sprintf("/deploycheck/policycompatible called.")))

		// check user cred
		if user_ec, _, msgPrinter, ok := a.processUserCred("/deploycheck/policycompatible", w, r); ok {
			body, _ := ioutil.ReadAll(r.Body)
			if len(body) == 0 {
				glog.Errorf(APIlogString(fmt.Sprintf("No input found.")))
				writeResponse(w, msgPrinter.Sprintf("No input found."), http.StatusBadRequest)
			} else if input, err := a.decodePolicyCheckBody(body, msgPrinter); err != nil {
				writeResponse(w, err.Error(), http.StatusBadRequest)
			} else {
				// if checkAll is set, then check all the services defined in the deployment policy for compatibility.
				checkAll := r.URL.Query().Get("checkAll")

				// do policy compatibility check
				output, err := compcheck.PolicyCompatible(user_ec, input, (checkAll != ""), msgPrinter)

				// nil out the policies in the output if 'long' is not set in the request
				long := r.URL.Query().Get("long")
				if long == "" && output != nil {
					output.Input = nil
				}

				// write the output
				a.writeCompCheckResponse(w, output, err, msgPrinter)
			}
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *SecureAPI) userinput_compatible(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	// swagger:operation GET /deploycheck/userinputcompatible userinputCompatible
	//
	// Check the user input compatibility.
	//
	// This API does the user input compatibility check for the given deployment policy (or a pattern), service definition and node user input. The user input values in the deployment policy and the node will be merged to check against the service uer input requirement defined in the service definition. If the result is compatible, it means that, when deployed, the node will form an agreement with the agbot and the service will be running on the node.
	//
	// ---
	// consumes:
	//  - application/json
	// produces:
	//  - application/json
	// parameters:
	//  - name: checkAll
	//    in: query
	//    type: bool
	//    required: false
	//    description: "Return the compatibility check result for all the service versions referenced in the deployment policy or pattern."
	//  - name: long
	//    in: query
	//    type: bool
	//    required: false
	//    description: "Show the input which was used to come up with the result."
	//  - name: node_id
	//    in: body
	//    type: string
	//    required: false
	//    description: "The exchange id of the node. Mutually exclusive with node_user_input."
	//  - name: node_arch
	//    in: body
	//    type: string
	//    required: false
	//    description: "The architecture of the node."
	//  - name: node_user_input
	//    in: body
	//    required: false
	//    description: "The user input that will be put in the exchange for the services. Mutually exclusive with node_id."
	//    schema:
	//     "$ref": "#/definitions/UserInput"
	//  - name: business_policy_id
	//    in: body
	//    type: string
	//    required: false
	//    description: "The exchange id of the deployment policy. Mutually exclusive with business_policy. Mutually exclusive with pattern_id and pattern."
	//  - name: business_policy
	//    in: body
	//    required: false
	//    description: "The defintion of the deployment policy that will be put in the exchange. Mutually exclusive with business_policy_id. Mutually exclusive with pattern_id and pattern."
	//    schema:
	//     "$ref": "#/definitions/BusinessPolicy"
	//  - name: pattern_id
	//    in: body
	//    type: string
	//    required: false
	//    description: "The exchange id of the pattern. Mutually exclusive with pattern. Mutually exclusive with business_policy_id and business_policy."
	//  - name: pattern
	//    in: body
	//    required: false
	//    description: "The pattern that will be put in the exchange. Mutually exclusive with pattern_id. Mutually exclusive with business_policy_id and business_policy."
	//    schema:
	//     "$ref": "#/definitions/PatternFile"
	//  - name: service
	//    in: body
	//    required: false
	//    description: "An array of the top level services that will be put in the exchange. They are refrenced in the deployment policy or pattern. If omitted, the services will be retrieved from the exchange."
	//    schema:
	//     "$ref": "#/definitions/ServiceFile"
	// responses:
	//  '200':
	//    description: "Success"
	//    schema:
	//     type: compcheck.CompCheckOutput
	//     "$ref": "#/definitions/CompCheckOutput"
	//  '400':
	//    description: "Failure - No input found"
	//    schema:
	//     type: string
	//  '401':
	//    description: "Failure - Failed to authenticate"
	//    schema:
	//     type: string
	//  '500':
	//    description: "Failure - Error"
	//    schema:
	//      type: string
	case "GET":
		glog.V(5).Infof(APIlogString(fmt.Sprintf("/deploycheck/userinputcompatible called.")))

		if user_ec, _, msgPrinter, ok := a.processUserCred("/deploycheck/userinputcompatible", w, r); ok {
			body, _ := ioutil.ReadAll(r.Body)
			if len(body) == 0 {
				glog.Errorf(APIlogString(fmt.Sprintf("No input found.")))
				writeResponse(w, msgPrinter.Sprintf("No input found."), http.StatusBadRequest)
			} else if input, err := a.decodeUserInputCheckBody(body, msgPrinter); err != nil {
				writeResponse(w, err.Error(), http.StatusBadRequest)
			} else {
				// if checkAll is set, then check all the services defined in the deployment policy for compatibility.
				checkAll := r.URL.Query().Get("checkAll")

				// do user input compatibility check
				output, err := compcheck.UserInputCompatible(user_ec, input, (checkAll != ""), msgPrinter)

				// nil out the details in the output if 'long' is not set in the request
				long := r.URL.Query().Get("long")
				if long == "" && output != nil {
					output.Input = nil
				}

				// write the output
				a.writeCompCheckResponse(w, output, err, msgPrinter)
			}
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// This function does secret binding compatibility check.
func (a *SecureAPI) secretbinding_compatible(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	// swagger:operation GET /deploycheck/secretbindingcompatible secretbinding_compatible
	//
	// Check the secret binding compatibility. 
	//
	// This API does the secret binding compatibility check for the given deployment policy (or a pattern) and service definition. It checks if each secret defined in a serice has a binding associated in the given deployment policy (or pattern) and each bound secret exists in the secret manager. If the result is compatible, it means that, when deployed, the node will form an agreement with the agbot and the service will be running on the node.
	//
	// ---
	// consumes: 
	//  - application/json 
	// produces: 
	//  - application/json
	// parameters:
	//  - name: checkAll
	//    in: query
	//    type: bool
	//    required: false
	//    description: "Return the compatibility check result for all the service versions referenced in the deployment policy or pattern."
	//  - name: long
	//    in: query
	//    type: bool     
	//    required: false
	//    description: "Show the input which was used to come up with the result."
	//  - name: node_id
	//    in: body
	//    type: string   
	//    required: false
	//    description: "The exchange id of the node. Mutually exclusive with node_user_input."
	//  - name: node_arch
	//    in: body
	//    type: string   
	//    required: false
	//    description: "The architecture of the node."
	//  - name: node_org
	//    in: body
	//    type: string
	//    required: false
	//    description: "The organization of the node."
	//  - name: business_policy_id
	//    in: body
	//    type: string   
	//    required: false
	//    description: "The exchange id of the deployment policy. Mutually exclusive with business_policy. Mutually exclusive with pattern_id and pattern."
	//  - name: business_policy
	//    in: body
	//    required: false
	//    description: "The defintion of the deployment policy that will be put in the exchange. Mutually exclusive with business_policy_id. Mutually exclusive with pattern_id and pattern."
	//    schema:
	//     "$ref": "#/definitions/BusinessPolicy"
	//  - name: pattern_id
	//    in: body
	//    type: string   
	//    required: false
	//    description: "The exchange id of the pattern. Mutually exclusive with pattern. Mutually exclusive with business_policy_id and business_policy."
	//  - name: pattern
	//    in: body
	//    required: false
	//    description: "The pattern that will be put in the exchange. Mutually exclusive with pattern_id. Mutually exclusive with business_policy_id and business_policy."
	//    schema:
	//     "$ref": "#/definitions/PatternFile"
	//  - name: service
	//    in: body
	//    required: false
	//    description: "An array of the top level services that will be put in the exchange. They are refrenced in the deployment policy or pattern. If omitted, the services will be retrieved from the exchange."
	//    schema:
	//     "$ref": "#/definitions/ServiceFile"
	// responses:
	//  '200':
	//    description: "Success"
	//    schema:
	//     type: compcheck.CompCheckOutput
	//     "$ref": "#/definitions/CompCheckOutput"
	//  '400':
	//    description: "Failure - No input found"
	//    schema:
	//     type: string
	//  '401':
	//    description: "Failure - Failed to authenticate"
	//    schema:
	//     type: string
	//  '500':
	//    description: "Failure - Error"
	//    schema:
	//      type: string
	case "GET":
		glog.V(5).Infof(APIlogString(fmt.Sprintf("/deploycheck/secretbindingcompatible called.")))

		if user_ec, exUser, msgPrinter, ok := a.processUserCred("/deploycheck/secretbindingcompatible", w, r); ok {
			body, _ := ioutil.ReadAll(r.Body)
			if len(body) == 0 {
				glog.Errorf(APIlogString(fmt.Sprintf("No input found.")))
				writeResponse(w, msgPrinter.Sprintf("No input found."), http.StatusBadRequest)
			} else if input, err := a.decodeSecretBindingCheckBody(body, msgPrinter); err != nil {
				glog.Errorf(APIlogString(err.Error()))
				writeResponse(w, err.Error(), http.StatusBadRequest)
			} else {
				// if checkAll is set, then check all the services defined in the deployment policy for compatibility.
				checkAll := r.URL.Query().Get("checkAll")

				// do user input compatibility check
				output, err := compcheck.SecretBindingCompatible(user_ec, "", input, (checkAll != ""), msgPrinter)

				// do the bound secret name varification in the secret manager
				if err == nil && output != nil {
					neededSB := output.Input.NeededSB
					if neededSB != nil && len(neededSB) != 0 {
						if ok, msg, err := a.verifySecretNames(user_ec, exUser, neededSB, output.Input.NodeOrg, msgPrinter); err != nil {
							glog.Errorf(APIlogString(err.Error()))
							writeResponse(w, err.Error(), http.StatusInternalServerError)
							return
						} else if !ok {
							output.Compatible = false
							output.Reason["general"] = msg
						}
					}

					// nil out the details in the output if 'long' is not set in the request
					long := r.URL.Query().Get("long")
					if long == "" {
						output.Input = nil
					}
				}

				// write the output
				a.writeCompCheckResponse(w, output, err, msgPrinter)
			}
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// This function does policy and userinput compatibility check.
func (a *SecureAPI) deploy_compatible(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	// swagger:operation GET /deploycheck/deploycompatible deploy_compatible
	//
	// Check deployment compatibility. 
	// 
	// This API does compatibility check for the given deployment policy (or a pattern), service definition, node policy and node user input. It does both policy compatibility check and user input compatibility check. If the result is compatible, it means that, when deployed, the node will form an agreement with the agbot and the service will be running on the node.
	//
	// ---
	// consumes: 
	//  - application/json 
	// produces: 
	//  - application/json
	// parameters:
	//  - name: checkAll
	//    in: query
	//    type: bool
	//    required: false
	//    description: "Return the compatibility check result for all the service versions referenced in the deployment policy or pattern."
	//  - name: long
	//    in: query
	//    type: bool     
	//    required: false
	//    description: "Show the input which was used to come up with the result."
	//  - name: node_id
	//    in: body
	//    type: string   
	//    required: false
	//    description: "The exchange id of the node. Mutually exclusive with node_policy and node_user_input."
	//  - name: node_arch
	//    in: body
	//    type: string   
	//    required: false
	//    description: "The architecture of the node."
	//  - name: node_org
	//    in: body
	//    type: string
	//    required: false
	//    description: "The organization of the node."
	//  - name: node_policy
	//    in: body
	//    required: false
	//    description: "The node policy that will be put in the exchange. Mutually exclusive with node_id."
	//    schema:
	//     "$ref": "#/definitions/ExternalPolicy"
	//  - name: node_user_input
	//    in: body
	//    required: false
	//    description: "The user input that will be put in the exchange for the services. Mutually exclusive with node_id."
	//    schema:
	//     "$ref": "#/definitions/UserInput"
	//  - name: business_policy_id
	//    in: body
	//    type: string   
	//    required: false
	//    description: "The exchange id of the deployment policy. Mutually exclusive with business_policy. Mutually exclusive with pattern_id and pattern."
	//  - name: business_policy
	//    in: body
	//    required: false
	//    description: "The defintion of the deployment policy that will be put in the exchange. Mutually exclusive with business_policy_id. Mutually exclusive with pattern_id and pattern."
	//    schema:
	//     "$ref": "#/definitions/BusinessPolicy"
	//  - name: pattern_id
	//    in: body
	//    type: string   
	//    required: false
	//    description: "The exchange id of the pattern. Mutually exclusive with pattern. Mutually exclusive with business_policy_id and business_policy."
	//  - name: pattern
	//    in: body
	//    required: false
	//    description: "The pattern that will be put in the exchange. Mutually exclusive with pattern_id. Mutually exclusive with business_policy_id and business_policy."
	//    schema:
	//     "$ref": "#/definitions/PatternFile"
	//  - name: service_policy
	//    in: body
	//    required: false
	//    description: "The service policy that will be put in the exchange. They are for the top level service referenced in the deployment policy. If omitted, the service policy will be retrieved from the exchange. The service policy has the same format as the node policy."
	//    schema:
	//     "$ref": "#/definitions/ExternalPolicy"
	//  - name: service
	//    in: body
	//    required: false
	//    description: "An array of the top level services that will be put in the exchange. They are refrenced in the deployment policy or pattern. If omitted, the services will be retrieved from the exchange."
	//    schema:
	//     "$ref": "#/definitions/ServiceFile"
	// responses:
	//  '200':
	//    description: "Success"
	//    schema:
	//     type: compcheck.CompCheckOutput
	//     "$ref": "#/definitions/CompCheckOutput"
	//  '400':
	//    description: "Failure - No input found"
	//    schema:
	//     type: string
	//  '401':
	//    description: "Failure - Failed to authenticate"
	//    schema:
	//     type: string
	//  '500':
	//    description: "Failure - Error"
	//    schema:
	//      type: string
	case "GET":
		glog.V(5).Infof(APIlogString(fmt.Sprintf("/deploycheck/deploycompatible called.")))

		if user_ec, exUser, msgPrinter, ok := a.processUserCred("/deploycheck/deploycompatible", w, r); ok {
			body, _ := ioutil.ReadAll(r.Body)
			if len(body) == 0 {
				glog.Errorf(APIlogString(fmt.Sprintf("No input found.")))
				writeResponse(w, msgPrinter.Sprintf("No input found."), http.StatusBadRequest)
			} else if input, err := a.decodeCompCheckBody(body, msgPrinter); err != nil {
				writeResponse(w, err.Error(), http.StatusBadRequest)
			} else {
				// if checkAll is set, then check all the services defined in the deployment policy for compatibility.
				checkAll := r.URL.Query().Get("checkAll")

				// do user input compatibility check
				output, err := compcheck.DeployCompatible(user_ec, "", input, (checkAll != ""), msgPrinter)

				// do the bound secret name varification in the secret manager
				if err == nil && output != nil {
					neededSB := output.Input.NeededSB
					if neededSB != nil && len(neededSB) != 0 {
						if ok, msg, err := a.verifySecretNames(user_ec, exUser, neededSB, output.Input.NodeOrg, msgPrinter); err != nil {
							glog.Errorf(APIlogString(err.Error()))
							writeResponse(w, err.Error(), http.StatusInternalServerError)
							return
						} else if !ok {
							output.Compatible = false
							output.Reason["general"] = msg
						}
					}

					// nil out the details in the output if 'long' is not set in the request
					long := r.URL.Query().Get("long")
					if long == "" {
						output.Input = nil
					}
				}

				// write the output
				a.writeCompCheckResponse(w, output, err, msgPrinter)
			}
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// This function checks user cred and writes corrsponding response. It also creates a message printer with given language from the http request.
func (a *SecureAPI) processUserCred(resource string, w http.ResponseWriter, r *http.Request) (exchange.ExchangeContext, string, *message.Printer, bool) {
	// get message printer with the language passed in from the header
	lan := r.Header.Get("Accept-Language")
	if lan == "" {
		lan = i18n.DEFAULT_LANGUAGE
	}
	msgPrinter := i18n.GetMessagePrinterWithLocale(lan)

	// check user cred
	userId, userPasswd, ok := r.BasicAuth()
	if !ok {
		glog.Errorf(APIlogString(fmt.Sprintf("%v is called without exchange authentication.", resource)))
		writeResponse(w, msgPrinter.Sprintf("Unauthorized. No exchange user id is supplied."), http.StatusUnauthorized)
		return nil, "", nil, false
	} else if user_ec, exUser, err := a.authenticateWithExchange(userId, userPasswd, msgPrinter); err != nil {
		glog.Errorf(APIlogString(fmt.Sprintf("Failed to authenticate user %v with the Exchange. %v", userId, err)))
		writeResponse(w, msgPrinter.Sprintf("Failed to authenticate the user with the Exchange. %v", err), http.StatusUnauthorized)
		return nil, "", nil, false
	} else {
		return user_ec, exUser, msgPrinter, true
	}
}

// This function checks if file exits or not
func fileExists(filename string) bool {
	fileinfo, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	if fileinfo.IsDir() {
		return false
	}

	return true
}

// Verify the comcheck input body from the /deploycheck/policycompatible api and convert it to compcheck.PolicyCheck
// It will give meaningful error as much as possible
func (a *SecureAPI) decodePolicyCheckBody(body []byte, msgPrinter *message.Printer) (*compcheck.PolicyCheck, error) {

	var js map[string]interface{}
	if err := json.Unmarshal(body, &js); err != nil {
		glog.Errorf(APIlogString(fmt.Sprintf("Input body couldn't be deserialized to JSON object. %v", err)))
		return nil, fmt.Errorf(msgPrinter.Sprintf("Input body couldn't be deserialized to JSON object. %v", err))
	} else {
		var input compcheck.PolicyCheck
		if err := json.Unmarshal(body, &input); err != nil {
			glog.Errorf(APIlogString(fmt.Sprintf("Input body couldn't be deserialized to PolicyCheck object. %v", err)))
			return nil, fmt.Errorf(msgPrinter.Sprintf("Input body couldn't be deserialized to PolicyCheck object. %v", err))
		} else {
			// verification of the input is done in the compcheck component, no need to validate the policies here.
			return &input, nil
		}
	}
}

// Verify the comcheck input body from the /deploycheck/userinputcompatible api and convert it to compcheck.UserInputCheck
// It will give meaningful error as much as possible
func (a *SecureAPI) decodeUserInputCheckBody(body []byte, msgPrinter *message.Printer) (*compcheck.UserInputCheck, error) {

	var js map[string]interface{}
	if err := json.Unmarshal(body, &js); err != nil {
		glog.Errorf(APIlogString(fmt.Sprintf("Input body couldn't be deserialized to JSON object. %v", err)))
		return nil, fmt.Errorf(msgPrinter.Sprintf("Input body couldn't be deserialized to JSON object. %v", err))
	} else {
		var input compcheck.UserInputCheck
		if err := json.Unmarshal(body, &input); err != nil {
			glog.Errorf(APIlogString(fmt.Sprintf("Input body couldn't be deserialized to UserInputCheck object. %v", err)))
			return nil, fmt.Errorf(msgPrinter.Sprintf("Input body couldn't be deserialized to UserInputCheck object. %v", err))
		} else {
			// verification of the input is done in the compcheck component, no need to validate the policies here.
			return &input, nil
		}
	}
}

// Verify the comcheck input body from the /deploycheck/secretbindingcompatible api and convert it to compcheck.UserInputCheck
// It will give meaningful error as much as possible
func (a *SecureAPI) decodeSecretBindingCheckBody(body []byte, msgPrinter *message.Printer) (*compcheck.SecretBindingCheck, error) {

	var js map[string]interface{}
	if err := json.Unmarshal(body, &js); err != nil {
		glog.Errorf(APIlogString(fmt.Sprintf("Input body couldn't be deserialized to JSON object. %v", err)))
		return nil, fmt.Errorf(msgPrinter.Sprintf("Input body couldn't be deserialized to JSON object. %v", err))
	} else {
		var input compcheck.SecretBindingCheck
		if err := json.Unmarshal(body, &input); err != nil {
			glog.Errorf(APIlogString(fmt.Sprintf("Input body couldn't be deserialized to SecretBindingCheck object. %v", err)))
			return nil, fmt.Errorf(msgPrinter.Sprintf("Input body couldn't be deserialized to SecretBindingCheck object. %v", err))
		} else {
			// verification of the input is done in the compcheck component, no need to validate the policies here.
			return &input, nil
		}
	}
}

// Verify the comcheck input body from the /deploycheck/userinputcompatible api and convert it to compcheck.UserInputCheck
// It will give meaningful error as much as possible
func (a *SecureAPI) decodeCompCheckBody(body []byte, msgPrinter *message.Printer) (*compcheck.CompCheck, error) {

	var js map[string]interface{}
	if err := json.Unmarshal(body, &js); err != nil {
		glog.Errorf(APIlogString(fmt.Sprintf("Input body couldn't be deserialized to JSON object. %v", err)))
		return nil, fmt.Errorf(msgPrinter.Sprintf("Input body couldn't be deserialized to JSON object. %v", err))
	} else {
		var input compcheck.CompCheck
		if err := json.Unmarshal(body, &input); err != nil {
			glog.Errorf(APIlogString(fmt.Sprintf("Input body couldn't be deserialized to CompCheck object. %v", err)))
			return nil, fmt.Errorf(msgPrinter.Sprintf("Input body couldn't be deserialized to CompCheck object. %v", err))
		} else {
			// verification of the input is done in the compcheck component, no need to validate the policies here.
			return &input, nil
		}
	}
}

// This function verifies the given exchange user name and password.
// The user must be in the format of orgId/userId.
func (a *SecureAPI) authenticateWithExchange(user string, userPasswd string, msgPrinter *message.Printer) (exchange.ExchangeContext, string, error) {
	glog.V(5).Infof(APIlogString(fmt.Sprintf("authenticateWithExchange called with user %v", user)))

	orgId, userId := cutil.SplitOrgSpecUrl(user)
	if userId == "" {
		return nil, "", fmt.Errorf(msgPrinter.Sprintf("No exchange user id is supplied."))
	} else if orgId == "" {
		return nil, "", fmt.Errorf(msgPrinter.Sprintf("No exchange user organization id is supplied."))
	} else if userPasswd == "" {
		return nil, "", fmt.Errorf(msgPrinter.Sprintf("No exchange user password or api key is supplied."))
	}

	// create the exchange context with the provided user and password
	user_ec := a.createUserExchangeContext(user, userPasswd)

	// Invoke the exchange API to verify the user.
	retryCount := user_ec.GetHTTPFactory().RetryCount
	retryInterval := user_ec.GetHTTPFactory().GetRetryInterval()
	for {
		retryCount = retryCount - 1

		var resp interface{}
		resp = new(exchange.GetUsersResponse)
		targetURL := fmt.Sprintf("%vorgs/%v/users/%v", user_ec.GetExchangeURL(), orgId, userId)

		if err, tpErr := exchange.InvokeExchange(a.httpClient, "GET", targetURL, user, userPasswd, nil, &resp); err != nil {
			glog.Errorf(APIlogString(err.Error()))

			if strings.Contains(err.Error(), "401") {
				return nil, "", fmt.Errorf(msgPrinter.Sprintf("Wrong organization id, user id or password."))
			} else {
				return nil, "", err
			}
		} else if tpErr != nil {
			glog.Warningf(APIlogString(tpErr.Error()))

			if retryCount <= 0 {
				return nil, "", fmt.Errorf("Exceeded %v retries for error: %v", retryCount, tpErr)
			}
			time.Sleep(time.Duration(retryInterval) * time.Second)
			continue
		} else {
			// iterate through the users returned by the Exchange (should only be one)
			users, _ := resp.(*exchange.GetUsersResponse)
			for key := range users.Users {
				// key should be in the format {org}/{user}
				orgAndUsername := strings.Split(key, "/")
				if len(orgAndUsername) != 2 {
					return nil, "", fmt.Errorf(msgPrinter.Sprintf("Exchange user %s is not in the correct format, should be org/username.", key))
				}
				return user_ec, orgAndUsername[1], nil
			}
		}
	}
}

// write response for compcheck call output
func (a *SecureAPI) writeCompCheckResponse(w http.ResponseWriter, output interface{}, err error, msgPrinter *message.Printer) {
	if err != nil {
		switch err.(type) {
		case *compcheck.CompCheckError:
			httpCode := getHTTPStatusCode(err.(*compcheck.CompCheckError).ErrCode)
			writeResponse(w, err.Error(), httpCode)
		default:
			writeResponse(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if output != nil {
			if serial, errWritten := serializeResponse(w, output); !errWritten {
				if _, err := w.Write(serial); err != nil {
					glog.Error(APIlogString(err))
					http.Error(w, msgPrinter.Sprintf("Internal server error"), http.StatusInternalServerError)
				}
			}
		}
	}
}

type SecretRequestInfo struct {

	// information about the user making the request
	org    string                   // the organization of the user making the request
	ec     exchange.ExchangeContext // holds credential information
	exUser string                   // the real username of the user in the exchange
	// if the user is authenticated with iamapikey or iamtoken, this will be different
	// from ec.GetExchangeId() (which will be iamapikey/iamtoken)

	// information about the resources being accessed
	user            string // if applicable, the user whose resources are being accessed
	vaultSecretName string // the name of the secret being accessed

	msgPrinter *message.Printer
}

// swagger:operation GET /org/{org}/secrets/* secrets_setup
//
// Common setup required before using the vault to manage secrets.
// 
// Authenticates the node user with the exchange. Checks if the vault plugin being used is ready. 
// Performs sanity checks on the secret user and secret name provided.
//
// ---
// consumes: 
//  - application/json 
// parameters:
//  - name: org
//    in: query
//    type: string
//    required: true
//    description: "The organisation name the secret belongs to. Must be the same as the org the user node belongs to."
//  - name: user
//    in: query
//    type: string 
//    required: false
//    description: "The user owning the secret."
//  - name: secret
//    in: query
//    type: string
//    required: false
//    description: "The secret key (name)."
// responses:
//  '400':
//    description: "Secret org or name does not meet constraints."
//    schema:
//     type: string
//  '503':
//    description: "Secret provider not ready or not configured."
//    schema:
//     type: string
func (a *SecureAPI) secretsSetup(w http.ResponseWriter, r *http.Request) *SecretRequestInfo {

	// Process in the inputs and verify that they are consistent with the logged in user.
	pathVars := mux.Vars(r)
	org := pathVars["org"]
	user := pathVars["user"]
	vaultSecretName := pathVars["secret"]
	api_url := "/org/%v/secrets/%v"
	resourceString := fmt.Sprintf(api_url, "{org}", "{secret}")
	if user != "" {
		api_url = "/org/%v/secrets/user/%v/%v"
		resourceString = fmt.Sprintf(api_url, "{org}", "{user}", "{secret}")
	}

	ec, exUser, msgPrinter, userAuthenticated := a.processUserCred(resourceString, w, r)
	if !userAuthenticated {
		return nil
	}

	// Check if vault is configured in the management hub, and the provider is ready to handle requests.
	if a.secretProvider == nil {
		glog.Errorf(APIlogString("There is no secrets provider configured, secrets are unavailable."))
		writeResponse(w, msgPrinter.Sprintf("There is no secrets provider configured, secrets are unavailable."), http.StatusServiceUnavailable)
		return nil
	}

	if !a.secretProvider.IsReady() {
		unavailMsg := "The secrets provider is not ready. The caller should retry this API call a small number of times with a short delay between calls to ensure that the secrets provider is unavailable."
		glog.Errorf(APIlogString(unavailMsg))
		writeResponse(w, msgPrinter.Sprintf(unavailMsg), http.StatusServiceUnavailable)
	}

	if user != "" {
		glog.V(5).Infof(APIlogString(fmt.Sprintf("%v %v called.", r.Method, fmt.Sprintf(api_url, org, user, vaultSecretName))))
	} else {
		glog.V(5).Infof(APIlogString(fmt.Sprintf("%v %v called.", r.Method, fmt.Sprintf(api_url, org, vaultSecretName))))
	}

	// pre processing
	if err, httpCode := a.vaultSecretPreCheck(ec, fmt.Sprint(r.URL), vaultSecretName, org, user, msgPrinter); err != nil {
		glog.Errorf(APIlogString(err.Error()))
		writeResponse(w, err.Error(), httpCode)
		return nil
	}

	if r.Method == "GET" || r.Method == "PUT" || r.Method == "POST" || r.Method == "DELETE" {
		if vaultSecretName == "" {
			glog.Errorf(APIlogString(fmt.Sprintf("Secret name must be provided")))
			writeResponse(w, msgPrinter.Sprintf("Secret name must be provided"), http.StatusBadRequest)
			return nil
		}
	}

	return &SecretRequestInfo{org, ec, exUser, user, vaultSecretName, msgPrinter}
}

func parseSecretDetails(w http.ResponseWriter, r *http.Request, msgPrinter *message.Printer) *secrets.SecretDetails {
	var input secrets.SecretDetails
	if body, err := ioutil.ReadAll(r.Body); err != nil {
		glog.Errorf(APIlogString(fmt.Sprintf("Unable to read request body, error: %v.", err)))
		writeResponse(w, msgPrinter.Sprintf("Unable to read request body, error: %v.", err), http.StatusInternalServerError)
		return nil
	} else if len(body) == 0 {
		glog.Errorf(APIlogString(fmt.Sprintf("Request body is empty.")))
		writeResponse(w, msgPrinter.Sprintf("Request body is empty."), http.StatusBadRequest)
		return nil
	} else if uerr := json.Unmarshal(body, &input); uerr != nil {
		glog.Errorf(APIlogString(fmt.Sprintf("Request body parse error, %v", uerr)))
		writeResponse(w, msgPrinter.Sprintf("Request body parse error, %v", uerr), http.StatusBadRequest)
		return nil
	}
	return &input
}

func secretExists(secretName string, secretList []string) bool {
	for _, secret := range secretList {
		if secretName == secret {
			return true
		}
	}
	return false
}

func (a *SecureAPI) errCheck(err error, action string, info *SecretRequestInfo) (*secrets.SecretsProviderError, string) {
	if serr := secrets.WrapSecretsError(err); serr != nil {

		// log the actual error
		glog.Errorf(APIlogString(fmt.Sprintf("Unable to access secrets provider: %v", err.Error())))

		// build the original secret name
		var secretName string
		if info.user != "" {
			secretName = "user/" + info.user + cliutils.AddSlash(info.vaultSecretName)
		} else {
			secretName = info.vaultSecretName
		}

		// case on the error and output the right message
		var errMsg string
		switch e := serr.Err.(type) {
		case *secrets.PermissionDenied:
			if secretName == "" {
				errMsg = info.msgPrinter.Sprintf("Permission denied, user \"%s\" cannot %s secrets in organization \"%s\"", info.exUser, action, info.org)
			} else {
				errMsg = info.msgPrinter.Sprintf("Permission denied, user \"%s\" cannot %s secret \"%s\" in organization \"%s\"", info.exUser, action, secretName, info.org)
			}

		case *secrets.InvalidResponse:
			if e.ReadError != nil {
				errMsg = info.msgPrinter.Sprintf("Unable to read the vault response: %s", e.ReadError.Error())
			} else {
				// e.ParseError != nil
				errMsg = info.msgPrinter.Sprintf("Unable to parse the vault response \"%s\": %s", e.Response, e.ParseError.Error())
			}
		case *secrets.BadRequest:
			if e.ResponseCode == 405 {
				// method not supported
				if secretName == "" {
					errMsg = info.msgPrinter.Sprintf("Unable to %s secrets in organization \"%s\", operation not supported by the secrets provider.", action, info.org)
				} else {
					errMsg = info.msgPrinter.Sprintf("Unable to %s secret \"%s\" in organization \"%s\", operation not supported by the secrets provider.", action, secretName, info.org)
				}
			} else {
				// e.ResponseCode == 400
				errMsg = info.msgPrinter.Sprintf("Secrets provider received a bad request, please check all the provided inputs.")
				errMsg += info.msgPrinter.Sprintf("\nResponse: %s", secrets.RespToString(e.Response))
			}
		case *secrets.NoSecretFound:
			if secretName == "" {
				errMsg = info.msgPrinter.Sprintf("No secret(s) found in organization \"%s\"", info.org)
			} else {
				errMsg = info.msgPrinter.Sprintf("No secret(s) found under secret name \"%s\"", secretName)
			}
		case *secrets.Unknown:
			errMsg = info.msgPrinter.Sprintf("An unknown error occurred. Response code %d received from the secrets provider.", e.ResponseCode)
			errMsg += info.msgPrinter.Sprintf("\nResponse: %s", secrets.RespToString(e.Response))
		default:
			errMsg = e.Error()
		}

		// write the response
		return serr, errMsg
	} else {
		return nil, ""
	}
}

func (a *SecureAPI) orgSecrets(w http.ResponseWriter, r *http.Request) {
	info := a.secretsSetup(w, r)
	if info == nil {
		return
	}

	// handle API options
	switch r.Method {
	// swagger:operation LIST /org/{org}/secrets orgSecrets
	// 
	// List all secrets belonging to the org.
	// 
	// ---
	// consumes:
	//   - application/json
	// produces:
	//   - application/json
	// responses:
	//  '200':
	//    description: "Success or no secrets found."
	//    type: array
	//    items: string
	//  '401':
	//    description: "Unauthenticated user."
	//    type: string
	//  '403':
	//    description: "Secrets permission denied to user."
	//    type: string
	//  '503':
	//    description: "Secret provider unavailable"
	//    type: string
	//  '500':
	//    description: "Invalid vault response"
	//    type: string
	case "LIST":
		if payload, err, httpCode := a.listVaultSecret(info); err != nil {
			glog.Errorf(APIlogString(err.Error()))
			writeResponse(w, err.Error(), httpCode)
		} else {
			writeResponse(w, payload, http.StatusOK)
		}
	case "OPTIONS":
		w.Header().Set("Allow", "LIST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handler for /org/<org>/secrets/<secret> - GET, LIST, PUT, POST, DELETE, OPTIONS
func (a *SecureAPI) orgSecret(w http.ResponseWriter, r *http.Request) {
	// check the provided secret name, <secret> can sometimes bind to user/<user>
	pathVars := mux.Vars(r)
	if strings.HasPrefix(pathVars["secret"], "user/") {
		writeResponse(w, fmt.Sprintf("Incorrect secret name provided: \"%s\" cannot refer to a secret in the secrets manager.", pathVars["secret"]), http.StatusBadRequest)
		return
	}

	// perform regular setup
	info := a.secretsSetup(w, r)
	if info == nil {
		return
	}

	// handle API options
	switch r.Method {
	case "GET":
		// pull details for an org-level secret
		secretDetails, err := a.secretProvider.GetSecretDetails(info.ec.GetExchangeId(), info.ec.GetExchangeToken(), info.org, "", info.vaultSecretName)
		if serr, errMsg := a.errCheck(err, "read", info); serr == nil {
			writeResponse(w, secretDetails, http.StatusOK)
		} else {
			writeResponse(w, errMsg, serr.ResponseCode)
		}
	case "LIST":
		// check existence of an org-level secret
		if payload, err, httpCode := a.listVaultSecret(info); err != nil {
			glog.Errorf(APIlogString(err.Error()))
			writeResponse(w, err.Error(), httpCode)
		} else {
			writeResponse(w, payload, http.StatusOK)
		}
	case "PUT":
		fallthrough
	case "POST":
		// create an org-level secret

		// parse the request body
		input := parseSecretDetails(w, r, info.msgPrinter)
		if input == nil {
			return
		}

		// create the secret
		err := a.secretProvider.CreateOrgSecret(info.ec.GetExchangeId(), info.ec.GetExchangeToken(), info.org, info.vaultSecretName, *input)
		if serr, errMsg := a.errCheck(err, "create", info); serr == nil {
			writeResponse(w, "Secret created/updated.", http.StatusCreated)
		} else {
			writeResponse(w, errMsg, serr.ResponseCode)
		}
	case "DELETE":
		// delete an org-level secret
		err := a.secretProvider.DeleteOrgSecret(info.ec.GetExchangeId(), info.ec.GetExchangeToken(), info.org, info.vaultSecretName)
		if serr, errMsg := a.errCheck(err, "remove", info); serr == nil {
			writeResponse(w, "Secret is deleted.", http.StatusNoContent)
		} else {
			writeResponse(w, errMsg, serr.ResponseCode)
		}
	case "OPTIONS":
		w.Header().Set("Allow", "LIST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handler for /org/<org>/secrets/user/<user> - LIST, OPTIONS
func (a *SecureAPI) userSecrets(w http.ResponseWriter, r *http.Request) {
	info := a.secretsSetup(w, r)
	if info == nil {
		return
	}

	// handle API options
	switch r.Method {
	case "LIST":
		// list user-level secrets
		if payload, err, httpCode := a.listVaultSecret(info); err != nil {
			glog.Errorf(APIlogString(err.Error()))
			writeResponse(w, err.Error(), httpCode)
		} else {
			writeResponse(w, payload, http.StatusOK)
		}
	case "OPTIONS":
		w.Header().Set("Allow", "LIST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handler for /org/<org>/secrets/user/<user>/<secret> - GET, LIST, PUT, POST, DELETE, OPTIONS
func (a *SecureAPI) userSecret(w http.ResponseWriter, r *http.Request) {
	info := a.secretsSetup(w, r)
	if info == nil {
		return
	}

	// handle API options
	userPath := "user/" + info.user + cliutils.AddSlash(info.vaultSecretName)
	switch r.Method {
	case "GET":
		// pull details for a user-level secret
		secretDetails, err := a.secretProvider.GetSecretDetails(info.ec.GetExchangeId(), info.ec.GetExchangeToken(), info.org, info.user, info.vaultSecretName)
		if serr, errMsg := a.errCheck(err, "read", info); serr == nil {
			writeResponse(w, secretDetails, http.StatusOK)
		} else {
			writeResponse(w, errMsg, serr.ResponseCode)
		}
	case "LIST":
		// check existence of a user-level secret
		if payload, err, httpCode := a.listVaultSecret(info); err != nil {
			glog.Errorf(APIlogString(err.Error()))
			writeResponse(w, err.Error(), httpCode)
		} else {
			writeResponse(w, payload, http.StatusOK)
		}
	case "PUT":
		fallthrough
	case "POST":
		// create a user-level secret

		// parse the request body
		input := parseSecretDetails(w, r, info.msgPrinter)
		if input == nil {
			return
		}

		// create the secret
		err := a.secretProvider.CreateOrgUserSecret(info.ec.GetExchangeId(), info.ec.GetExchangeToken(), info.org, userPath, *input)
		if serr, errMsg := a.errCheck(err, "create", info); serr == nil {
			writeResponse(w, "Secret created/updated.", http.StatusCreated)
		} else {
			writeResponse(w, errMsg, serr.ResponseCode)
		}
	case "DELETE":
		err := a.secretProvider.DeleteOrgUserSecret(info.ec.GetExchangeId(), info.ec.GetExchangeToken(), info.org, userPath)
		if serr, errMsg := a.errCheck(err, "remove", info); serr == nil {
			writeResponse(w, "Secret created/updated.", http.StatusNoContent)
		} else {
			writeResponse(w, errMsg, serr.ResponseCode)
		}
	case "OPTIONS":
		w.Header().Set("Allow", "LIST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// This function does preprocessing for the secret APIs.
// It returns error and http response code.
func (a *SecureAPI) vaultSecretPreCheck(ec exchange.ExchangeContext,
	apiUrl string, vaultSecretName string, org string, user string,
	msgPrinter *message.Printer) (error, int) {

	// Check if vault is configured in the management hub, and the provider is ready to handle requests.
	if a.secretProvider == nil {
		return fmt.Errorf(msgPrinter.Sprintf("There is no secrets provider configured, secrets are unavailable.")), http.StatusServiceUnavailable
	}

	if !a.secretProvider.IsReady() {
		unavailMsg := "The secrets provider is not ready. The caller should retry this API call a small number of times with a short delay between calls to ensure that the secrets provider is unavailable."
		return fmt.Errorf(msgPrinter.Sprintf(unavailMsg)), http.StatusServiceUnavailable
	}

	if org == "" {
		return fmt.Errorf(msgPrinter.Sprintf("Organization must be specified in the API path")), http.StatusBadRequest
	} else if (strings.Contains(apiUrl, "/user/") && user == "") ||
		(vaultSecretName == "user" && user == "") {
		return fmt.Errorf(msgPrinter.Sprintf("User must be specified in the API path")), http.StatusBadRequest
	}

	return nil, 0
}

// Call the plugged in secrets provider to list the secret(s) for the input org.
// It returns the payload, error and httpcode.
func (a *SecureAPI) listVaultSecret(info *SecretRequestInfo) (interface{}, error, int) {

	userPath := "user/" + info.user + cliutils.AddSlash(info.vaultSecretName)
	if info.vaultSecretName == "" {
		var secretNames []string
		var err error
		if info.user != "" {
			// listing user level secrets
			secretNames, err = a.secretProvider.ListOrgUserSecrets(info.ec.GetExchangeId(), info.ec.GetExchangeToken(), info.org, userPath)
		} else {
			// listing org level secrets
			secretNames, err = a.secretProvider.ListOrgSecrets(info.ec.GetExchangeId(), info.ec.GetExchangeToken(), info.org, info.vaultSecretName)
		}

		// check the error output, ignore 404
		if serr, errMsg := a.errCheck(err, "list", info); serr == nil {
			// no error
			return secretNames, nil, http.StatusOK
		} else {
			// ignore NoSecretFound error
			_, ok := serr.Err.(*secrets.NoSecretFound)
			if ok {
				// 404, should return an empty list
				return secretNames, nil, http.StatusOK
			} else {
				// error
				return nil, fmt.Errorf(errMsg), serr.ResponseCode
			}
		}
	} else {
		var err error
		if info.user != "" {
			err = a.secretProvider.ListOrgUserSecret(info.ec.GetExchangeId(), info.ec.GetExchangeToken(), info.org, userPath)
		} else {
			err = a.secretProvider.ListOrgSecret(info.ec.GetExchangeId(), info.ec.GetExchangeToken(), info.org, info.vaultSecretName)
		}

		// check the error output, ignore 404
		if serr, errMsg := a.errCheck(err, "list", info); serr == nil {
			// no error, secret exists
			return map[string]bool{"exists": true}, nil, http.StatusOK
		} else {
			// ignore NoSecretFound error
			_, ok := serr.Err.(*secrets.NoSecretFound)
			if ok {
				// 404, should return false
				return map[string]bool{"exists": (serr.ResponseCode != http.StatusNotFound)}, nil, http.StatusOK
			} else {
				// error
				return nil, fmt.Errorf(errMsg), serr.ResponseCode
			}
		}
	}
}

// Given an array of secret bindings, make sure the bound secrets exist
// in the secret manager(/provider).
func (a *SecureAPI) verifySecretNames(ec exchange.ExchangeContext, exUser string,
	secretBinding []exchangecommon.SecretBinding,
	nodeOrg string, msgPrinter *message.Printer) (bool, string, error) {

	// take the user org as the default node org
	if nodeOrg == "" {
		nodeOrg = exchange.GetOrg(ec.GetExchangeId())
	}

	for _, sn := range secretBinding {
		for _, vbind := range sn.Secrets {
			_, secretName := vbind.GetBinding()

			//parse the bound secret name
			secretUser, shortSecretName, err := compcheck.ParseVaultSecretName(secretName, msgPrinter)
			if err != nil {
				return false, "", err
			}

			// make sure the scret manager is ready. Forming the agbot api url so that it
			// can use the agbot secure API precheck function vaultSecretPreCheck
			agbotApiUrl := ""
			fullSecretName := ""
			if secretUser == "" {
				agbotApiUrl = fmt.Sprintf("/org/%v/secrets/%v", nodeOrg, shortSecretName)
				fullSecretName = fmt.Sprintf("/openhorizon/%v/%v", nodeOrg, shortSecretName)
			} else {
				agbotApiUrl = fmt.Sprintf("/org/%v/secrets/user/%v/%v", nodeOrg, secretUser, shortSecretName)
				fullSecretName = fmt.Sprintf("/openhorizon/%v/user/%v/%v", nodeOrg, secretUser, shortSecretName)
			}
			if err, _ := a.vaultSecretPreCheck(ec, agbotApiUrl, shortSecretName, nodeOrg, secretUser, msgPrinter); err != nil {
				return false, "", err
			}

			// check if the bound secret exists or not{}
			payload, err, _ := a.listVaultSecret(&SecretRequestInfo{nodeOrg, ec, exUser, secretUser, shortSecretName, msgPrinter})
			if err != nil {
				return false, "", fmt.Errorf(msgPrinter.Sprintf("Error checking secret %v in the secret manager.", fullSecretName))
			} else if payload != nil {
				if p, ok := payload.(map[string]bool); !ok {
					// this should never happen, but check it just in case
					return false, "", fmt.Errorf(msgPrinter.Sprintf("Wrong type returned checking the secret name in the secret manager: %v", payload))
				} else if p["exists"] == false {
					return false, msgPrinter.Sprintf("Secret %v does not exist in the secret manager.", fullSecretName), nil
				}
			}
		}
	}

	return true, "", nil
}

// convert the policy check error code to http status code
func getHTTPStatusCode(code int) int {
	var httpCode int
	switch code {
	case compcheck.COMPCHECK_INPUT_ERROR, compcheck.COMPCHECK_VALIDATION_ERROR:
		httpCode = http.StatusBadRequest
	default:
		httpCode = http.StatusInternalServerError
	}
	return httpCode
}

func newHTTPClientFactory() *config.HTTPClientFactory {
	clientFunc := func(overrideTimeoutS *uint) *http.Client {
		var timeoutS uint
		if overrideTimeoutS != nil {
			timeoutS = *overrideTimeoutS
		} else {
			timeoutS = config.HTTPRequestTimeoutS
		}

		httpClient := cliutils.GetHTTPClient(int(timeoutS))
		if err := cliutils.TrustIcpCert(httpClient); err != nil {
			glog.Errorf(APIlogString(err.Error()))
		}

		return httpClient
	}

	return &config.HTTPClientFactory{
		NewHTTPClient: clientFunc,
		RetryCount:    5,
		RetryInterval: 2,
	}
}
