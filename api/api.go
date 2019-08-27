package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
)

type API struct {
	worker.Manager // embedded field
	name           string
	db             *bolt.DB
	pm             *policy.PolicyManager
	em             *events.EventStateManager
	bcState        map[string]map[string]apicommon.BlockchainState
	bcStateLock    sync.Mutex
	shutdownError  string
	EC             *worker.BaseExchangeContext
}

type BlockchainState struct {
	ready       bool   // the blockchain is ready
	writable    bool   // the blockchain is writable
	service     string // the network endpoint name of the container
	servicePort string // the network port of the container
}

func NewAPIListener(name string, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager) *API {
	messages := make(chan events.Message)

	listener := &API{
		Manager: worker.Manager{
			Config:   cfg,
			Messages: messages,
		},

		name:        name,
		db:          db,
		pm:          pm,
		em:          events.NewEventStateManager(),
		bcState:     make(map[string]map[string]apicommon.BlockchainState),
		bcStateLock: sync.Mutex{},
		EC:          nil,
	}

	// setup the exchange context if the device is set
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		glog.Errorf(apiLogString(fmt.Sprintf("Unable to read node object, error %v", err)))
	} else if pDevice != nil {
		listener.EC = worker.NewExchangeContext(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token, cfg.Edge.ExchangeURL, cfg.GetCSSURL(), cfg.Collaborators.HTTPClientFactory)
	}

	listener.listen(cfg)
	return listener
}

func (a *API) router(includeStaticRedirects bool) *mux.Router {
	router := mux.NewRouter()

	// For working with global and microservice specific attributes directly
	router.HandleFunc("/attribute", a.attribute).Methods("OPTIONS", "HEAD", "GET", "POST")
	router.HandleFunc("/attribute/{id}", a.attribute).Methods("OPTIONS", "HEAD", "GET", "PUT", "PATCH", "DELETE")

	// For working with existing or archived agreements
	router.HandleFunc("/agreement", a.agreement).Methods("GET", "OPTIONS")
	router.HandleFunc("/agreement/{id}", a.agreement).Methods("GET", "DELETE", "OPTIONS")

	// For obtaining microservice info or configuring a microservice (sensor) userInput variables
	router.HandleFunc("/service", a.service).Methods("GET", "OPTIONS")
	router.HandleFunc("/service/config", a.serviceconfig).Methods("GET", "POST", "OPTIONS")
	router.HandleFunc("/service/configstate", a.service_configstate).Methods("GET", "POST", "OPTIONS")
	router.HandleFunc("/service/policy", a.servicepolicy).Methods("GET", "OPTIONS")

	// Connectivity and blockchain status info
	router.HandleFunc("/status", a.status).Methods("GET", "OPTIONS")
	router.HandleFunc("/status/workers", a.workerstatus).Methods("GET", "OPTIONS")

	// Used by the Registration UI to obtain a random token string
	router.HandleFunc("/token/random", tokenRandom).Methods("GET", "OPTIONS")

	// Used to configure a node to participate in the Horizon platform
	router.HandleFunc("/node", a.node).Methods("GET", "HEAD", "POST", "PATCH", "DELETE", "OPTIONS")
	router.HandleFunc("/node/configstate", a.nodeconfigstate).Methods("GET", "HEAD", "PUT", "OPTIONS")
	router.HandleFunc("/node/policy", a.nodepolicy).Methods("GET", "HEAD", "PUT", "POST", "PATCH", "DELETE", "OPTIONS")
	router.HandleFunc("/node/userinput", a.nodeuserinput).Methods("GET", "HEAD", "PUT", "POST", "PATCH", "DELETE", "OPTIONS")

	// Used to get the event logs on this node.
	// get the eventlogs for current registration.
	router.HandleFunc("/eventlog", a.eventlog).Methods("GET", "OPTIONS")
	// get the eventlogs for all registrations.
	router.HandleFunc("/eventlog/all", a.eventlog).Methods("GET", "OPTIONS")
	//get the active surface errors for this node
	router.HandleFunc("/eventlog/surface", a.surface).Methods("GET", "OPTIONS")

	// For importing workload public signing keys (RSA-PSS key pair public key)
	router.HandleFunc("/{p:(?:publickey|trust)}", a.publickey).Methods("GET", "OPTIONS")
	router.HandleFunc("/{p:(?:publickey|trust)}/{filename}", a.publickey).Methods("GET", "PUT", "DELETE", "OPTIONS")

	if includeStaticRedirects {
		// redirect to index.html because SPA
		router.HandleFunc(`/{p:[\w\/]+}`, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		})
		router.PathPrefix("/").Handler(http.FileServer(http.Dir(a.Config.Edge.StaticWebContent)))
		glog.Infof(apiLogString(fmt.Sprintf("Include static redirects: %v", includeStaticRedirects)))
		glog.Infof(apiLogString(fmt.Sprintf("Serving static web content from: %v", a.Config.Edge.StaticWebContent)))
	}
	return router
}

func (a *API) listen(cfg *config.HorizonConfig) {
	glog.Info(apiLogString(fmt.Sprintf("Starting Anax API server")))

	nocache := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Add("Pragma", "no-cache, no-store")
			w.Header().Add("Access-Control-Allow-Origin", "*")
			w.Header().Add("Access-Control-Allow-Headers", "X-Requested-With, content-type, Authorization")
			w.Header().Add("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			h.ServeHTTP(w, r)
		})
	}

	// This routine does not need to be a subworker because there is no way to terminate it. It will terminate when
	// the main anax process goes away.
	go func() {
		if err := http.ListenAndServe(cfg.Edge.APIListen, nocache(a.router(true))); err != nil {
			glog.Fatalf(apiLogString(fmt.Sprintf("Failed to start listener on %v, error %v", cfg.Edge.APIListen, err)))
		}
	}()

}

// Worker framework functions
func (a *API) Messages() chan events.Message {
	return a.Manager.Messages
}

func (a *API) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.BlockchainClientInitializedMessage:
		msg, _ := incoming.(*events.BlockchainClientInitializedMessage)
		switch msg.Event().Id {
		case events.BC_CLIENT_INITIALIZED:
			apicommon.HandleNewBCInit(msg, a.bcState, &a.bcStateLock)
			glog.V(3).Infof(apiLogString(fmt.Sprintf("API Worker processed BC initialization for %v", msg)))
		}

	case *events.BlockchainClientStoppingMessage:
		msg, _ := incoming.(*events.BlockchainClientStoppingMessage)
		switch msg.Event().Id {
		case events.BC_CLIENT_STOPPING:
			apicommon.HandleStoppingBC(msg, a.bcState, &a.bcStateLock)
			glog.V(3).Infof(apiLogString(fmt.Sprintf("API Worker processed BC stopping for %v", msg)))
		}

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			a.em.RecordEvent(msg, func(m events.Message) { a.saveShutdownError(m) })
			// Now remove myself from the worker dispatch list. When the anax process terminates,
			// the socket listener will terminate also. This is done on a separate thread so that
			// the message dispatcher doesnt get blocked. This worker isnt actually a full blown
			// worker and doesnt have a command thread that it can run on.
			go func() {
				a.Messages() <- events.NewWorkerStopMessage(events.WORKER_STOP, a.GetName())
			}()
		}

	}
	return
}

func (a *API) GetName() string {
	return a.name
}

func (a *API) saveShutdownError(msg events.Message) {
	switch msg.(type) {
	case *events.NodeShutdownCompleteMessage:
		m, _ := msg.(*events.NodeShutdownCompleteMessage)
		a.shutdownError = m.Err()
	}
}

// A local implementation of the ExchangeContext interface because the API object is not an anax worker.
func (a *API) GetExchangeId() string {
	if a.EC != nil {
		return a.EC.Id
	} else {
		return ""
	}
}

func (a *API) GetExchangeToken() string {
	if a.EC != nil {
		return a.EC.Token
	} else {
		return ""
	}
}

func (a *API) GetExchangeURL() string {
	if a.EC != nil {
		return a.EC.URL
	} else {
		return a.Config.Edge.ExchangeURL
	}
}

func (a *API) GetCSSURL() string {
	if a.EC != nil {
		return a.EC.CSSURL
	} else {
		return a.Config.GetCSSURL()
	}
}

func (a *API) GetHTTPFactory() *config.HTTPClientFactory {
	if a.EC != nil {
		return a.EC.HTTPFactory
	} else {
		return a.Config.Collaborators.HTTPClientFactory
	}
}

// Utility functions used by all the http handlers for each API path.
func serializeResponse(w http.ResponseWriter, payload interface{}) ([]byte, bool) {
	glog.V(6).Infof(apiLogString(fmt.Sprintf("response payload before serialization (%T): %v", payload, payload)))

	serial, err := json.Marshal(payload)
	if err != nil {
		glog.Error(apiLogString(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil, true
	}

	return serial, false
}

func writeResponse(w http.ResponseWriter, payload interface{}, successStatusCode int) {

	serial, errWritten := serializeResponse(w, payload)
	if errWritten {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(successStatusCode)

	if _, err := w.Write(serial); err != nil {
		glog.Error(apiLogString(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (a *API) existingDeviceOrError(w http.ResponseWriter) (*persistence.ExchangeDevice, bool) {

	var statusWritten bool
	existingDevice, err := persistence.FindExchangeDevice(a.db)

	if err != nil {
		glog.Errorf(apiLogString(fmt.Sprintf("Failed fetching existing exchange device, error: %v", err)))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		statusWritten = true
	} else if existingDevice == nil {
		writeInputErr(w, http.StatusFailedDependency, NewAPIUserInputError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node"))
		statusWritten = true
	}

	return existingDevice, statusWritten
}

var apiLogString = func(v interface{}) string {
	return fmt.Sprintf("API: %v", v)
}
