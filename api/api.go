package api

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/microservice"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
)

type API struct {
	worker.Manager // embedded field
	db             *bolt.DB
	pm             *policy.PolicyManager
	bcState        map[string]map[string]BlockchainState
	bcStateLock    sync.Mutex
}

type BlockchainState struct {
	ready       bool   // the blockchain is ready
	writable    bool   // the blockchain is writable
	service     string // the network endpoint name of the container
	servicePort string // the network port of the container
}

func NewAPIListener(config *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager) *API {
	messages := make(chan events.Message)

	listener := &API{
		Manager: worker.Manager{
			Config:   config,
			Messages: messages,
		},

		db:          db,
		pm:          pm,
		bcState:     make(map[string]map[string]BlockchainState),
		bcStateLock: sync.Mutex{},
	}

	listener.listen(config.Edge.APIListen)
	return listener
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
			a.handleNewBCInit(msg)
			glog.V(3).Infof("API Worker processed BC initialization for %v", msg)
		}

	case *events.BlockchainClientStoppingMessage:
		msg, _ := incoming.(*events.BlockchainClientStoppingMessage)
		switch msg.Event().Id {
		case events.BC_CLIENT_STOPPING:
			a.handleStoppingBC(msg)
			glog.V(3).Infof("API Worker processed BC stopping for %v", msg)
		}
	}

	return
}

func (a *API) handleNewBCInit(ev *events.BlockchainClientInitializedMessage) {

	a.bcStateLock.Lock()
	defer a.bcStateLock.Unlock()

	nameMap := a.getBCNameMap(ev.BlockchainType())
	namedBC, ok := nameMap[ev.BlockchainInstance()]
	if !ok {
		nameMap[ev.BlockchainInstance()] = BlockchainState{
			ready:       true,
			writable:    false,
			service:     ev.ServiceName(),
			servicePort: ev.ServicePort(),
		}
	} else {
		namedBC.ready = true
		namedBC.service = ev.ServiceName()
		namedBC.servicePort = ev.ServicePort()
	}

}

func (a *API) handleStoppingBC(ev *events.BlockchainClientStoppingMessage) {

	a.bcStateLock.Lock()
	defer a.bcStateLock.Unlock()

	nameMap := a.getBCNameMap(ev.BlockchainType())
	delete(nameMap, ev.BlockchainInstance())

}

func (a *API) getBCNameMap(typeName string) map[string]BlockchainState {
	nameMap, ok := a.bcState[typeName]
	if !ok {
		a.bcState[typeName] = make(map[string]BlockchainState)
		nameMap = a.bcState[typeName]
	}
	return nameMap
}

func (a *API) router(includeStaticRedirects bool) *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/attribute", a.attribute).Methods("OPTIONS", "HEAD", "GET", "POST")
	router.HandleFunc("/attribute/{id}", a.attribute).Methods("OPTIONS", "HEAD", "GET", "PUT", "PATCH", "DELETE")

	router.HandleFunc("/agreement", a.agreement).Methods("GET", "OPTIONS")
	router.HandleFunc("/agreement/{id}", a.agreement).Methods("GET", "DELETE", "OPTIONS")

	// N.B. the following two paths are the primary registration endpoints as of v2.1.0; these notions
	// get split apart when a proper microservice / workload prefs split is established in the future

	// for declaring microservices (just opting into using it, it's defined elsewhere (in exchange)); variables need to be set on the microservice in the exchange; the values of the variables need to be filled in by the caller
	router.HandleFunc("/service", a.service).Methods("GET", "POST", "OPTIONS")

	router.HandleFunc("/microservice", a.microservice).Methods("GET", "OPTIONS")

	router.HandleFunc("/status", a.status).Methods("GET", "OPTIONS")
	router.HandleFunc("/token/random", tokenRandom).Methods("GET", "OPTIONS")
	router.HandleFunc("/horizondevice", a.horizonDevice).Methods("GET", "POST", "PATCH", "OPTIONS")
	router.HandleFunc("/workload", a.workload).Methods("GET", "OPTIONS") // for getting running stuff info
	router.HandleFunc("/publickey", a.publickey).Methods("GET", "OPTIONS")
	router.HandleFunc("/publickey/{filename}", a.publickey).Methods("GET", "PUT", "DELETE", "OPTIONS")
	router.HandleFunc("/workloadconfig", a.workloadConfig).Methods("GET", "POST", "DELETE", "OPTIONS")

	if includeStaticRedirects {
		// redirect to index.html because SPA
		router.HandleFunc(`/{p:[\w\/]+}`, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		})
		router.PathPrefix("/").Handler(http.FileServer(http.Dir(a.Config.Edge.StaticWebContent)))
		glog.Infof("Include static redirects: %v", includeStaticRedirects)
		glog.Infof("Serving static web content from: %v", a.Config.Edge.StaticWebContent)
	}
	return router
}

func (a *API) listen(apiListen string) {
	glog.Info("Starting Anax API server")

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

	go func() {
		http.ListenAndServe(apiListen, nocache(a.router(true)))
	}()
}

func (a *API) agreement(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		// we don't support getting just one yet
		if id != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		agreements, err := persistence.FindEstablishedAgreementsAllProtocols(a.db, policy.AllAgreementProtocols(), []persistence.EAFilter{})
		if err != nil {
			glog.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		var agreementsKey = "agreements"
		var archivedKey = "archived"
		var activeKey = "active"

		wrap := make(map[string]map[string][]persistence.EstablishedAgreement, 0)
		wrap[agreementsKey] = make(map[string][]persistence.EstablishedAgreement, 0)
		wrap[agreementsKey][archivedKey] = []persistence.EstablishedAgreement{}
		wrap[agreementsKey][activeKey] = []persistence.EstablishedAgreement{}

		for _, agreement := range agreements {
			// The archived agreements and the agreements being terminated are returned as archived.
			if agreement.Archived || agreement.AgreementTerminatedTime != 0 {
				wrap[agreementsKey][archivedKey] = append(wrap[agreementsKey][archivedKey], agreement)
			} else {
				wrap[agreementsKey][activeKey] = append(wrap[agreementsKey][activeKey], agreement)
			}
		}

		// do sorts
		sort.Sort(EstablishedAgreementsByAgreementCreationTime(wrap[agreementsKey][activeKey]))
		sort.Sort(EstablishedAgreementsByAgreementTerminatedTime(wrap[agreementsKey][archivedKey]))

		serial, err := json.Marshal(wrap)
		if err != nil {
			glog.Infof("Error serializing agreement output: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(serial); err != nil {
			glog.Infof("Error writing response: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	case "DELETE":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		glog.V(3).Infof("Handling DELETE of agreement: %v", r)

		var filters []persistence.EAFilter
		filters = append(filters, persistence.UnarchivedEAFilter())
		filters = append(filters, persistence.IdEAFilter(id))

		if agreements, err := persistence.FindEstablishedAgreementsAllProtocols(a.db, policy.AllAgreementProtocols(), filters); err != nil {
			glog.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
		} else if len(agreements) == 0 {
			w.WriteHeader(http.StatusNotFound)
		} else {
			// write message
			ct := agreements[0]
			if ct.AgreementTerminatedTime == 0 {
				a.Messages() <- events.NewApiAgreementCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ct.AgreementProtocol, ct.CurrentAgreementId, ct.CurrentDeployment)
			}
			w.WriteHeader(http.StatusOK)

		}
	case "OPTIONS":
		w.Header().Set("Allow", "GET, DELETE, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func serializeResponse(w http.ResponseWriter, payload interface{}) ([]byte, bool) {
	glog.V(6).Infof("response payload before serialization (%T): %v", payload, payload)

	serial, err := json.Marshal(payload)
	if err != nil {
		glog.Error(err)
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
		glog.Error(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (a *API) horizonDevice(w http.ResponseWriter, r *http.Request) {

	// returns existing device ref and boolean if error occured during fetch (error output handled by this func)
	fetch := func(device *HorizonDevice) (*persistence.ExchangeDevice, bool) {
		existing, err := persistence.FindExchangeDevice(a.db)
		if err != nil {
			glog.Errorf("Failed fetching existing exchange device. Error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return nil, true
		}

		return existing, false
	}

	writeDevice := func(exDevice *persistence.ExchangeDevice, successStatusCode int) {

		var outModel *HorizonDevice

		if exDevice == nil {
			device_id := os.Getenv("CMTN_DEVICE_ID")
			outModel = &HorizonDevice{
				Id: &device_id,
			}
		} else {
			// assume input struct is well-formed, should come from persisted record
			outModel = &HorizonDevice{
				Name:               &exDevice.Name,
				Org:                &exDevice.Org,
				Pattern:            &exDevice.Pattern,
				Id:                 &exDevice.Id,
				TokenValid:         &exDevice.TokenValid,
				TokenLastValidTime: &exDevice.TokenLastValidTime,
				HADevice:           &exDevice.HADevice,
			}
		}

		writeResponse(w, outModel, successStatusCode)
	}

	switch r.Method {
	case "GET":
		existingDevice, errWritten := a.existingDeviceOrError(w)
		if errWritten {
			return
		}

		writeDevice(existingDevice, http.StatusOK)

	case "POST":
		var device HorizonDevice

		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &device); err != nil {
			glog.Infof("User submitted data couldn't be deserialized to Device struct: %v. Error: %v", string(body), err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if bail := checkInputString(w, "device.organization", device.Org); bail {
			return
		}
		// Device pattern is optional
		if device.Pattern != nil && *device.Pattern != "" {
			if bail := checkInputString(w, "device.pattern", device.Pattern); bail {
				return
			}
		}
		if bail := checkInputString(w, "device.name", device.Name); bail {
			return
		}
		if device.Token == nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "device.token", Error: "null and must not be"})
			return
		}

		if device.Id == nil || *device.Id == "" {
			device_id := os.Getenv("CMTN_DEVICE_ID")
			if device_id == "" {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "device.id", Error: "Either setup CMTN_DEVICE_ID environmental variable or specify device.id."})
				return
			}
			device.Id = &device_id
		}
		if bail := checkInputString(w, "device.id", device.Id); bail {
			return
		}

		// don't bother sanitizing token data; we *never* output it, and we *never* compute it

		// Verify that the input organization exists in the exchange
		deviceId := fmt.Sprintf("%v/%v", *device.Org, *device.Id)
		if _, err := exchange.GetOrganization(a.Config.Collaborators.HTTPClientFactory, *device.Org, a.Config.Edge.ExchangeURL, deviceId, *device.Token); err != nil {
			glog.Errorf("Organization %v not found in exchange, error: %v", *device.Org, err)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "organization", Error: fmt.Sprintf("organization %v not found in exchange, error: %v", *device.Org, err)})
			return
		}

		// Verify that the input pattern is defined in the exchange. A device (or node) canonly use patterns that are defined within its own org.
		if device.Pattern != nil && *device.Pattern != "" {
			if patternDefs, err := exchange.GetPatterns(a.Config.Collaborators.HTTPClientFactory, *device.Org, *device.Pattern, a.Config.Edge.ExchangeURL, deviceId, *device.Token); err != nil {
				glog.Errorf("Error searching for pattern %v for %v in exchange, error: %v", *device.Pattern, *device.Org, err)
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "pattern", Error: fmt.Sprintf("error searching for pattern %v in exchange, error: %v", *device.Pattern, err)})
				return
			} else if _, ok := patternDefs[fmt.Sprintf("%v/%v", *device.Org, *device.Pattern)]; !ok {
				glog.Errorf("Pattern %v for %v not found in exchange, error: %v", *device.Pattern, *device.Org, err)
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "pattern", Error: fmt.Sprintf("pattern %v not found in exchange, error: %v", *device.Pattern, err)})
				return
			}
		}

		// Check for the device already in the local database
		existing, fetchErrWritten := fetch(&device)
		if fetchErrWritten {
			// errors already written to response writer by fetch function call
			return
		}

		// handle conflict here; should never be a conflict in POST method, PATCH is for update
		if existing != nil {
			w.WriteHeader(http.StatusConflict)
			return
		}

		haDevice := false
		if device.HADevice != nil && *device.HADevice == true {
			haDevice = true
		}

		exDev, err := persistence.SaveNewExchangeDevice(a.db, *device.Id, *device.Token, *device.Name, haDevice, *device.Org, *device.Pattern)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			glog.Errorf("Error persisting new exchange device: %v", err)
			return
		}

		a.Messages() <- events.NewEdgeRegisteredExchangeMessage(events.NEW_DEVICE_REG, *device.Id, *device.Token, *device.Org, *device.Pattern)

		writeDevice(exDev, http.StatusCreated)

	case "PATCH":
		var device HorizonDevice

		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &device); err != nil {
			glog.Infof("User submitted data couldn't be deserialized to Device struct: %v. Error: %v", string(body), err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if bail := checkInputString(w, "device.id", device.Id); bail {
			return
		}
		if device.Token == nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "device.token", Error: "null and must not be"})
			return
		}

		existing, fetchErrWritten := fetch(&device)
		if fetchErrWritten {
			// errors already written to response writer by fetch function call
			return
		}

		if existing == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		updatedDevice, err := persistence.SetExchangeDeviceToken(a.db, *device.Id, *device.Token)
		if err != nil {
			glog.Errorf("Error doing token update on horizon device object: %v. Error: %v", existing, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}

		writeDevice(updatedDevice, http.StatusOK)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, POST, PATCH, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) existingDeviceOrError(w http.ResponseWriter) (*persistence.ExchangeDevice, bool) {

	var statusWritten bool
	existingDevice, err := persistence.FindExchangeDevice(a.db)

	if err != nil {
		glog.Errorf("Failed fetching existing exchange device. Error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		statusWritten = true
	} else if existingDevice == nil {
		writeInputErr(w, http.StatusFailedDependency, &APIUserInputError{Error: "Exchange registration not recorded. Complete account and device registration with an exchange and then record device registration using this API's /horizondevice path."})
		statusWritten = true
	}

	return existingDevice, statusWritten
}

func (a *API) attribute(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	glog.V(5).Infof("Attribute vars: %v", vars)
	id := vars["id"]

	existingDevice, errWritten := a.existingDeviceOrError(w)
	if errWritten {
		return
	}

	var decodedID string
	if id != "" {
		var err error
		decodedID, err = url.PathUnescape(id)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// shared logic between payload-handling update functions
	handlePayload := func(permitPartial bool, doModifications func(permitPartial bool, attr persistence.Attribute)) {
		defer r.Body.Close()

		if attrs, inputErr, err := payloadToAttributes(w, r.Body, permitPartial, existingDevice); err != nil {
			glog.Error("Error processing incoming attributes. ", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else if !inputErr {
			glog.V(6).Infof("persistent-type attributes: %v", attrs)

			if len(attrs) != 1 {
				// only one attr may be specified to add at a time
				w.WriteHeader(http.StatusBadRequest)
			} else {
				doModifications(permitPartial, attrs[0])
			}
		}
	}

	handleUpdateFn := func() func(bool, persistence.Attribute) {
		return func(permitPartial bool, attr persistence.Attribute) {
			if added, err := persistence.SaveOrUpdateAttribute(a.db, attr, decodedID, permitPartial); err != nil {
				switch err.(type) {
				case *persistence.OverwriteCandidateNotFound:
					glog.V(3).Infof("User attempted attribute update but there isn't a matching persisting attribute to modify.")
					w.WriteHeader(http.StatusNotFound)
				default:
					glog.Error("Error persisting attribute. ", err)
					w.WriteHeader(http.StatusInternalServerError)
				}
			} else if added != nil {
				writeResponse(w, toOutModel(*added), http.StatusOK)
			} else {
				glog.Error("Attribute was not successfully persisted but no error was returned from persistence module")
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}

	switch r.Method {
	case "OPTIONS":
		w.Header().Set("Allow", "OPTIONS, HEAD, GET, POST, PUT, PATCH, DELETE")

	case "HEAD":
		returned, err := persistence.FindAttributeByKey(a.db, decodedID)
		if err != nil {
			glog.Error("Attribute was not successfully deleted. ", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		out := wrapAttributesForOutput([]persistence.Attribute{*returned}, decodedID)

		if serial, errWritten := serializeResponse(w, out); !errWritten {
			w.Header().Add("Content-Length", strconv.Itoa(len(serial)))
			w.WriteHeader(http.StatusOK)
		}

	case "GET":
		out, err := FindAndWrapAttributesForOutput(a.db, decodedID)
		glog.V(5).Infof("returning %v for query of %v", out, decodedID)
		if err != nil {
			glog.Error("Error reading persisted attributes", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "POST":
		// can't POST with an id, POST is only for new records
		if decodedID != "" {
			w.WriteHeader(http.StatusBadRequest)
		} else {

			// call handlePayload with function to do additions
			handlePayload(false, func(permitPartial bool, attr persistence.Attribute) {

				if added, err := persistence.SaveOrUpdateAttribute(a.db, attr, decodedID, permitPartial); err != nil {
					glog.Infof("Got error from attempted save: <%T>, %v", err, err == nil)
					switch err.(type) {
					case *persistence.ConflictingAttributeFound:
						w.WriteHeader(http.StatusConflict)
					default:
						glog.Error("Error persisting attribute. ", err)
						w.WriteHeader(http.StatusInternalServerError)
					}
				} else if added != nil {
					writeResponse(w, toOutModel(*added), http.StatusCreated)
				} else {
					glog.Error("Attribute was not successfully persisted but no error was returned from persistence module")
					w.WriteHeader(http.StatusInternalServerError)
				}
			})
		}

	case "PUT":
		// must PUT with an id, this is a complete replacement of the document body
		if decodedID == "" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			// call handlePayload with function to do updates but prohibit partial updates
			handlePayload(false, handleUpdateFn())
		}

	case "PATCH":
		if decodedID == "" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			// call handlePayload with function to do updates and allow partial updates
			handlePayload(true, handleUpdateFn())
		}

	case "DELETE":
		if decodedID == "" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			deleted, err := persistence.DeleteAttribute(a.db, decodedID)
			if err != nil {
				glog.Error("Attribute was not successfully deleted. ", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else if deleted == nil {
				// nothing deleted, 200 w/ no return
				w.WriteHeader(http.StatusOK)
			} else {
				writeResponse(w, toOutModel(*deleted), http.StatusOK)
			}
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// for registering what *should* be microservices but as of v2.1.0, are more
// like the old contracts
func (a *API) service(w http.ResponseWriter, r *http.Request) {

	findAdditions := func(attrs []persistence.Attribute, incoming []persistence.Attribute) []persistence.Attribute {

		toAdd := []persistence.Attribute{}

		for _, in := range incoming {
			c := false
			for _, attr := range attrs {
				if in.GetMeta().Id == attr.GetMeta().Id {
					c = true
					break
				}
			}

			if !c {
				toAdd = append(toAdd, in)
			}
		}

		// return the mutated copy
		return toAdd
	}

	switch r.Method {
	case "GET":
		type outServiceWrapper struct {
			Policy     policy.Policy           `json:"policy"`
			Attributes []persistence.Attribute `json:"attributes"`
		}

		outServices := make(map[string]interface{}, 0)

		allOrgs := a.pm.GetAllPolicyOrgs()
		for _, org := range allOrgs {

			allPolicies := a.pm.GetAllPolicies(org)
			for _, pol := range allPolicies {

				var applicable []persistence.Attribute

				for _, apiSpec := range pol.APISpecs {
					pAttr, err := persistence.FindApplicableAttributes(a.db, apiSpec.SpecRef)
					if err != nil {
						glog.Errorf("Failed fetching attributes. Error: %v", err)
						http.Error(w, "Internal server error", http.StatusInternalServerError)
						return
					}

					applicable = append(applicable, findAdditions(applicable, pAttr)...)
				}

				// TODO: consider sorting the attributes returned
				outServices[pol.Header.Name] = outServiceWrapper{
					Policy:     pol,
					Attributes: applicable,
				}
			}
		}

		wrapper := make(map[string]map[string]interface{}, 0)
		wrapper["services"] = outServices

		serial, err := json.Marshal(wrapper)
		if err != nil {
			glog.Infof("Error serializing agreement output: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(serial); err != nil {
			glog.Infof("Error writing response: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

	case "POST":
		existingDevice, errWritten := a.existingDeviceOrError(w)
		if errWritten {
			return
		}

		// input should be: Service type w/ zero or more Attribute types
		var service Service
		body, _ := ioutil.ReadAll(r.Body)

		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		if err := decoder.Decode(&service); err != nil {
			glog.Errorf("User submitted data that couldn't be deserialized to service: %v. Error: %v", string(body), err)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "service.attribute", Error: fmt.Sprintf("could not be demarshalled, error: %v", err)})
			return
		}

		glog.V(5).Infof("Service POST: %v", &service)

		if bail := checkInputString(w, "sensor_url", service.SensorUrl); bail {
			return
		}

		if bail := checkInputString(w, "sensor_name", service.SensorName); bail {
			return
		}

		// Default sensor version if not specified
		if service.SensorVersion == nil {
			def := "0.0.0"
			service.SensorVersion = &def
		}

		// Convert the sensor version to a version expression
		vExp, err := policy.Version_Expression_Factory(*service.SensorVersion)
		if err != nil {
			glog.Errorf("Unable to convert %v to a version expression, error %v", *service.SensorVersion, err)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "sensor_version", Error: fmt.Sprintf("sensor_version %v cannot be converted to a version expression, error %v", *service.SensorVersion, err)})
			return
		}

		// Use the device's org if org not specified in the POST body.
		if service.SensorOrg == nil {
			service.SensorOrg = &existingDevice.Org
		} else if bail := checkInputString(w, "sensor_org", service.SensorOrg); bail {
			return
		}

		var msdef *persistence.MicroserviceDefinition

		// Verify with the exchange to make sure the service exists
		e_msdef, err := exchange.GetMicroservice(a.Config.Collaborators.HTTPClientFactory, *service.SensorUrl, *service.SensorOrg, vExp.Get_expression(), cutil.ArchString(), a.Config.Edge.ExchangeURL, existingDevice.GetId(), existingDevice.Token)
		if err != nil || e_msdef == nil {
			glog.Errorf("Unable to find the microservice definition in the exchange: %v", err)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Error: fmt.Sprintf("Unable to find the microservice definition for '%v' on the exchange. Please verify sensor_url and sensor_version.", *service.SensorName)})
			return
		}
		// Convert it to persistent format so that it can be saved to the db.
		msdef, err = microservice.ConvertToPersistent(e_msdef, *service.SensorOrg)
		if err != nil {
			glog.Errorf("Error converting the microservice metadata to persistent.MicroserviceDefinition for %v version %v. %v", e_msdef.SpecRef, e_msdef.Version, err)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Error: fmt.Sprintf("Error converting the microservice metadata to persistent.MicroserviceDefinition for %v version %v. %v", e_msdef.SpecRef, e_msdef.Version, err)})
			return

		}
		// Save some of the items in the MicroserviceDefinition object for use in the upgrading process.
		msdef.Name = *service.SensorName
		msdef.UpgradeVersionRange = *service.SensorVersion
		if service.AutoUpgrade != nil {
			msdef.AutoUpgrade = *service.AutoUpgrade
		}
		if service.ActiveUpgrade != nil {
			msdef.ActiveUpgrade = *service.ActiveUpgrade
		}

		service.SensorVersion = &msdef.Version

		// Check if the microservice has been registered or not (currently only support one microservice registration)
		if pms, err := persistence.FindMicroserviceDefs(a.db, []persistence.MSFilter{persistence.UrlMSFilter(*service.SensorUrl)}); err != nil {
			glog.Errorf("Error accessing db to find microservice definition: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if pms != nil && len(pms) > 0 {
			glog.Errorf("Duplicate registration for %v. Anax only supports one registration for each microservice now.", *service.SensorUrl)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Error: fmt.Sprintf("Duplicate registration for %v. Anax only supports one registration for each microservice now.", *service.SensorUrl)})
			return
		}

		msdefAttributeVerifier := func(w http.ResponseWriter, attr persistence.Attribute) (bool, error) {

			// Verfiy that all non-defaulted userInput variables in the microservice definition are specified in a mapped property attribute
			// of this service invocation.
			if msdef != nil && attr.GetMeta().Type == "MappedAttributes" {
				for _, ui := range msdef.UserInputs {
					if ui.DefaultValue != "" {
						continue
					} else if _, ok := attr.GetGenericMappings()[ui.Name]; !ok {
						// There is a config variable missing from the generic mapped attributes
						glog.Errorf("Variable %v defined in microservice %v %v is missing from the service definition.", ui.Name, msdef.SpecRef, msdef.Version)
						writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "service.[attribute].mapped", Error: fmt.Sprintf("variable %v is missing from mappings", ui.Name)})
						return true, nil
					}
				}
			}

			return false, nil
		}

		patternedDeviceAttributeVerifier := func(w http.ResponseWriter, attr persistence.Attribute) (bool, error) {
			// If the device declared itself to be using a pattern, then it CANNOT specify any attributes that generate policy
			// settings . All policy is controlled by the pattern definition.
			if existingDevice.Pattern != "" {
				if attr.GetMeta().Type == "MeteringAttributes" || attr.GetMeta().Type == "PropertyAttributes" || attr.GetMeta().Type == "CounterPartyPropertyAttributes" || attr.GetMeta().Type == "AgreementProtocolAttributes" {
					glog.Errorf("device is using a pattern %v, policy attributes are not supported.", existingDevice.Pattern)
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "service.[attribute].type", Error: fmt.Sprintf("device is using a pattern %v, policy attributes are not supported.", existingDevice.Pattern)})
					return true, nil
				}
			}

			return false, nil
		}

		var attributes []persistence.Attribute
		if service.Attributes != nil {
			// build a serviceAttribute for each one
			var err error
			var inputErrWritten bool

			attributes, inputErrWritten, err = toPersistedAttributesAttachedToService(w, existingDevice, a.Config.Edge.DefaultServiceRegistrationRAM, *service.Attributes, *service.SensorUrl, []AttributeVerifier{msdefAttributeVerifier, patternedDeviceAttributeVerifier})

			// log even if there was an inputErr already written to the response
			if err != nil {
				glog.Errorf("Failure deserializing attributes: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if inputErrWritten {
				return
			}
		}

		// Information advertised in the edge node policy file
		var policyArch string
		var haPartner []string
		var meterPolicy policy.Meter
		var counterPartyProperties policy.RequiredProperty
		var properties map[string]interface{}
		var globalAgreementProtocols []interface{}

		// props to store in file; stuff that is enforced; need to convert from serviceattributes to props. *CAN NEVER BE* unpublishable ServiceAttributes
		props := make(map[string]interface{})

		// There might be device wide attributes. Check for them and grab the values to use as defaults.
		if allAttrs, err := persistence.FindApplicableAttributes(a.db, ""); err != nil {
			glog.Errorf("Unable to fetch workload preferences. Err: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		} else {
			for _, attr := range allAttrs {

				// Extract ha property
				if attr.GetMeta().Type == "HAAttributes" && len(attr.GetMeta().SensorUrls) == 0 {
					haPartner = attr.(persistence.HAAttributes).Partners
					glog.V(5).Infof("Found default global ha attribute %v", attr)
				}

				// Global policy attributes are ignored for devices that are using a pattern. All policy is controlled
				// by the pattern definition.
				if existingDevice.Pattern == "" {
					// Extract global metering property
					if attr.GetMeta().Type == "MeteringAttributes" && len(attr.GetMeta().SensorUrls) == 0 {
						// found a global metering entry
						meterPolicy = policy.Meter{
							Tokens:                attr.(persistence.MeteringAttributes).Tokens,
							PerTimeUnit:           attr.(persistence.MeteringAttributes).PerTimeUnit,
							NotificationIntervalS: attr.(persistence.MeteringAttributes).NotificationIntervalS,
						}
						glog.V(5).Infof("Found default global metering attribute %v", attr)
					}

					// Extract global counterparty property
					if attr.GetMeta().Type == "CounterPartyPropertyAttributes" && len(attr.GetMeta().SensorUrls) == 0 {
						counterPartyProperties = attr.(persistence.CounterPartyPropertyAttributes).Expression
						glog.V(5).Infof("Found default global counterpartyproperty attribute %v", attr)
					}

					// Extract global properties
					if attr.GetMeta().Type == "PropertyAttributes" && len(attr.GetMeta().SensorUrls) == 0 {
						properties = attr.(persistence.PropertyAttributes).Mappings
						glog.V(5).Infof("Found default global properties %v", properties)
					}

					// Extract global agreement protocol attribute
					if attr.GetMeta().Type == "AgreementProtocolAttributes" && len(attr.GetMeta().SensorUrls) == 0 {
						agpl := attr.(persistence.AgreementProtocolAttributes).Protocols
						globalAgreementProtocols = agpl.([]interface{})
						glog.V(5).Infof("Found default global agreement protocol attribute %v", globalAgreementProtocols)
					}
				}
			}
		}

		// ha device has no ha attribute from either device wide or service wide attributes
		haType := reflect.TypeOf(persistence.HAAttributes{}).Name()
		if existingDevice.HADevice && len(haPartner) == 0 {
			if attr := attributesContains(attributes, *service.SensorUrl, haType); attr == nil {
				glog.Errorf("HA device %v can only support HA enabled services %v", existingDevice, service)
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "service.[attribute].type", Error: "services on an HA device must specify an HA partner."})
				return
			}
		}

		var serviceAgreementProtocols []policy.AgreementProtocol
		// persist all prefs; while we're at it, fetch the props we want to publish and the arch
		for _, attr := range attributes {

			_, err := persistence.SaveOrUpdateAttribute(a.db, attr, "", false)
			if err != nil {
				glog.Errorf("Error saving attribute: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			switch attr.(type) {
			case *persistence.ComputeAttributes:
				compute := attr.(*persistence.ComputeAttributes)
				props["cpus"] = strconv.FormatInt(compute.CPUs, 10)
				props["ram"] = strconv.FormatInt(compute.RAM, 10)

			case *persistence.ArchitectureAttributes:
				policyArch = attr.(*persistence.ArchitectureAttributes).Architecture

			case *persistence.HAAttributes:
				haPartner = attr.(*persistence.HAAttributes).Partners

			case *persistence.MeteringAttributes:
				meterPolicy = policy.Meter{
					Tokens:                attr.(*persistence.MeteringAttributes).Tokens,
					PerTimeUnit:           attr.(*persistence.MeteringAttributes).PerTimeUnit,
					NotificationIntervalS: attr.(*persistence.MeteringAttributes).NotificationIntervalS,
				}

			case *persistence.CounterPartyPropertyAttributes:
				counterPartyProperties = attr.(*persistence.CounterPartyPropertyAttributes).Expression

			case *persistence.PropertyAttributes:
				properties = attr.(*persistence.PropertyAttributes).Mappings

			case *persistence.AgreementProtocolAttributes:
				agpl := attr.(*persistence.AgreementProtocolAttributes).Protocols
				serviceAgreementProtocols = agpl.([]policy.AgreementProtocol)

			default:
				glog.V(4).Infof("Unhandled attr type (%T): %v", attr, attr)
			}
		}

		// add the PropertyAttributes to props
		if len(properties) > 0 {
			for key, val := range properties {
				glog.V(5).Infof("Adding property %v=%v with value type %T", key, val, val)
				props[key] = val
			}
		}

		glog.V(5).Infof("Complete Attr list for registration of service %v: %v", *service.SensorUrl, attributes)

		// Establish the correct agreement protocol list
		var agpList *[]policy.AgreementProtocol
		if len(serviceAgreementProtocols) != 0 {
			agpList = &serviceAgreementProtocols
		} else if list, err := policy.ConvertToAgreementProtocolList(globalAgreementProtocols); err != nil {
			glog.Errorf("Error converting global agreement protocol list attribute %v to agreement protocol list, error: %v", globalAgreementProtocols, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			agpList = list
		}

		// Save ms def in local db
		if err := persistence.SaveOrUpdateMicroserviceDef(a.db, msdef); err != nil { // save to db
			glog.Errorf("Error saving microservice definition %v into db: %v", *msdef, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Get max number of agreements for policy
		maxAgreements := 1
		if msdef.Sharable == exchange.MS_SHARING_MODE_SINGLE || msdef.Sharable == exchange.MS_SHARING_MODE_MULTIPLE {
			maxAgreements = 2 // hard coded to 2 for now. will change to 0 later
		}

		// Generate a policy based on all the attributes and the service definition
		if genErr := policy.GeneratePolicy(a.Messages(), *service.SensorUrl, *service.SensorOrg, *service.SensorName, *service.SensorVersion, policyArch, &props, haPartner, meterPolicy, counterPartyProperties, *agpList, maxAgreements, a.Config.Edge.PolicyPath, existingDevice.Org); genErr != nil {
			glog.Errorf("Error: %v", genErr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// TODO: when there is a way to represent services for output, write it out w/ the 201
		w.WriteHeader(http.StatusCreated)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) status(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":

		info := NewInfo(a.Config)

		if err := WriteConnectionStatus(info); err != nil {
			glog.Errorf("Unable to get connectivity status: %v", err)
		}

		a.bcStateLock.Lock()
		defer a.bcStateLock.Unlock()

		for _, bc := range a.bcState[policy.Ethereum_bc] {
			geth := NewGeth()

			gethURL := fmt.Sprintf("http://%v:%v", bc.service, bc.servicePort)
			if err := WriteGethStatus(gethURL, geth); err != nil {
				glog.Errorf("Unable to determine geth service facts: %v", err)
			}

			info.AddGeth(geth)
		}

		writeResponse(w, info, http.StatusOK)
	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

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
			glog.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write(serial); err != nil {
			glog.Error(err)
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

func (a *API) workload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if client, err := dockerclient.NewClient(a.Config.Edge.DockerEndpoint); err != nil {
			glog.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else {
			opts := dockerclient.ListContainersOptions{
				All: true,
			}

			if containers, err := client.ListContainers(opts); err != nil {
				glog.Error(err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			} else {
				ret := make(map[string][]dockerclient.APIContainers, 0)
				ret["workloads"] = []dockerclient.APIContainers{}

				for _, c := range containers {
					if _, exists := c.Labels["network.bluehorizon.colonus.service_name"]; exists {
						ret["workloads"] = append(ret["workloads"], c)
					}
				}

				if serial, err := json.Marshal(ret); err != nil {
					glog.Error(err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				} else {
					w.Header().Set("Content-Type", "application/json")
					if _, err := w.Write(serial); err != nil {
						glog.Error(err)
						http.Error(w, "Internal server error", http.StatusInternalServerError)
					}
				}
			}
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

func (a *API) publickey(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":

		pathVars := mux.Vars(r)
		fileName := pathVars["filename"]

		// Get a list of all valid public key PEM files in the configured location
		pubKeyDir := a.Config.UserPublicKeyPath()
		files, err := getPemFiles(pubKeyDir)
		if err != nil {
			glog.Errorf("APIWorker %v /publickey unable to read public key directory %v, error: %v", r.Method, pubKeyDir, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}

		if fileName != "" {

			// If the input file name is not in the list of valid pem files, then return an error
			found := false
			for _, f := range files {
				if f.Name() == fileName {
					found = true
				}
			}
			if !found {
				glog.Errorf("APIWorker %v /publickey unable to find input file %v", r.Method, fileName)
				w.WriteHeader(http.StatusNotFound)
				return
			}

			// Open the file so that we can read any header info that might be there.
			pemFile, err := os.Open(pubKeyDir + "/" + fileName)
			defer pemFile.Close()

			if err != nil {
				glog.Errorf("APIWorker %v /publickey unable to open requested key file %v, error: %v", r.Method, fileName, err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Get the Content-Type of the file.
			fileHeader := make([]byte, 512)
			pemFile.Read(fileHeader)
			fileContentType := http.DetectContentType(fileHeader)

			// Get the file size.
			fileStat, _ := pemFile.Stat()
			fileSize := strconv.FormatInt(fileStat.Size(), 10)

			// Set the headers for a file atachment.
			w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
			w.Header().Set("Content-Type", fileContentType)
			w.Header().Set("Content-Length", fileSize)

			// Reset the file so that we can read from the beginning again.
			pemFile.Seek(0, 0)
			io.Copy(w, pemFile)
			w.WriteHeader(http.StatusOK)
			return

		} else {
			files, err := getPemFiles(pubKeyDir)
			if err != nil {
				glog.Errorf("APIWorker %v /publickey unable to read public key directory %v, error: %v", r.Method, pubKeyDir, err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}

			response := make(map[string][]string)
			response["pem"] = make([]string, 0, 10)
			for _, pf := range files {
				response["pem"] = append(response["pem"], pf.Name())
			}

			serial, err := json.Marshal(response)
			if err != nil {
				glog.Errorf("APIWorker %v /publickey unable to serialize response %v, error %v", r.Method, response, err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write(serial); err != nil {
				glog.Errorf("APIWorker %v /publickey error writing response: %v, error %v", r.Method, serial, err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)

		}

	case "PUT":

		pathVars := mux.Vars(r)
		fileName := pathVars["filename"]

		if fileName == "" {
			glog.Errorf("APIWorker %v /publickey unable to upload, no file name specfied", r.Method)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "public key file", Error: "no filename specified"})
			return
		} else if !strings.HasSuffix(fileName, ".pem") {
			glog.Errorf("APIWorker %v /publickey unable to upload, file must have .pem suffix", r.Method)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "public key file", Error: "filename must have .pem suffix"})
			return
		}

		glog.V(3).Infof("APIWorker %v /publickey of %v", r.Method, fileName)
		targetPath := a.Config.UserPublicKeyPath()
		targetFile := targetPath + "/" + fileName

		// Receive the uploaded file content and verify that it is a valid public key. If it's valid then
		// save it into the configured PublicKeyPath location from the config. The name of the uploaded file
		// is specified on the HTTP PUT. It does not have to have the same file name used by the HTTP caller.

		if nkBytes, err := ioutil.ReadAll(r.Body); err != nil {
			glog.Errorf("APIWorker %v /publickey unable to read uploaded public key file, error: %v", r.Method, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else if nkBlock, _ := pem.Decode(nkBytes); nkBlock == nil {
			glog.Errorf("APIWorker %v /publickey unable to extract pem block from uploaded public key file", r.Method)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "public key file", Error: "not a pem encoded file"})
			return
		} else if _, err := x509.ParsePKIXPublicKey(nkBlock.Bytes); err != nil {
			glog.Errorf("APIWorker %v /publickey unable to parse uploaded public key, error: %v", r.Method, err)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "public key file", Error: "not a PKIX public key"})
			return
		} else if err := os.MkdirAll(targetPath, 0644); err != nil {
			glog.Errorf("APIWorker %v /publickey unable to create user key directory, error %v", r.Method, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else if err := ioutil.WriteFile(targetFile, nkBytes, 0644); err != nil {
			glog.Errorf("APIWorker %v /publickey unable to write uploaded public key file %v, error: %v", r.Method, targetFile, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else {
			glog.V(5).Infof("APIWorker %v /publickey successfully uploaded and verified public key in %v", r.Method, targetFile)
			w.WriteHeader(http.StatusOK)
		}

	case "DELETE":

		pathVars := mux.Vars(r)
		fileName := pathVars["filename"]

		if fileName == "" {
			glog.Errorf("APIWorker %v /publickey unable to delete, no file name specfied", r.Method)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "public key file", Error: "no filename specified"})
			return
		}
		glog.V(3).Infof("APIWorker %v /publickey of %v", r.Method, fileName)

		// Get a list of all valid public key PEM files in the configured location
		pubKeyDir := a.Config.UserPublicKeyPath()
		files, err := getPemFiles(pubKeyDir)
		if err != nil {
			glog.Errorf("APIWorker %v /publickey unable to read public key directory %v, error: %v", r.Method, pubKeyDir, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}

		// If the input file name is not in the list of valid pem files, then return an error
		found := false
		for _, f := range files {
			if f.Name() == fileName {
				found = true
			}
		}
		if !found {
			glog.Errorf("APIWorker %v /publickey unable to find input file %v", r.Method, fileName)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// The input file is a valid public key, remove it
		err = os.Remove(pubKeyDir + "/" + fileName)
		if err != nil {
			glog.Errorf("APIWorker %v /publickey unable to delete public key file %v, error: %v", r.Method, fileName, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusNoContent)
		return

	case "OPTIONS":
		w.Header().Set("Allow", "GET, PUT, DELETE, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

func writeInputErr(writer http.ResponseWriter, status int, inputErr *APIUserInputError) {
	if serial, err := json.Marshal(inputErr); err != nil {
		glog.Infof("Error serializing agreement output: %v", err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
	} else {
		writer.WriteHeader(status)
		writer.Header().Set("Content-Type", "application/json")
		if _, err := writer.Write(serial); err != nil {
			glog.Infof("Error writing response: %v", err)
			http.Error(writer, "Internal server error", http.StatusInternalServerError)
		}
	}
}

func getPemFiles(homePath string) ([]os.FileInfo, error) {

	res := make([]os.FileInfo, 0, 10)

	if files, err := ioutil.ReadDir(homePath); err != nil && !os.IsNotExist(err) {
		return res, errors.New(fmt.Sprintf("Unable to get list of PEM files in %v, error: %v", homePath, err))
	} else if os.IsNotExist(err) {
		return res, nil
	} else {
		for _, fileInfo := range files {
			if strings.HasSuffix(fileInfo.Name(), ".pem") && !fileInfo.IsDir() {
				fName := homePath + "/" + fileInfo.Name()
				if pubKeyData, err := ioutil.ReadFile(fName); err != nil {
					continue
				} else if block, _ := pem.Decode(pubKeyData); block == nil {
					continue
				} else if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
					continue
				} else {
					res = append(res, fileInfo)
				}
			}
		}
		return res, nil
	}
}

func (a *API) workloadConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":

		// Only "get all" is supported
		wrap := make(map[string][]persistence.WorkloadConfig)

		// Retrieve all workload configs from the db
		cfgs, err := persistence.FindWorkloadConfigs(a.db, []persistence.WCFilter{})
		if err != nil {
			glog.Error(err)
			http.Error(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
			return
		}

		wrap["active"] = cfgs

		// Sort the output by workload URL and then within that by version
		sort.Sort(WorkloadConfigByWorkloadURLAndVersion(wrap["active"]))

		// Create the response body and send it back
		serial, err := json.Marshal(wrap)
		if err != nil {
			glog.Infof("Error serializing agreement output: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		glog.V(5).Infof("WorkloadConfig GET returns: %v", string(serial))

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(serial); err != nil {
			glog.Infof("Error writing response: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

	case "POST":

		// Demarshal the input body
		var cfg WorkloadConfig
		body, _ := ioutil.ReadAll(r.Body)

		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		if err := decoder.Decode(&cfg); err != nil {
			glog.Errorf("User submitted data that couldn't be deserialized to workload config: %v. Error: %v", string(body), err)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "workloadConfig", Error: fmt.Sprintf("could not be demarshalled, error: %v", err)})
			return
		}

		glog.V(5).Infof("WorkloadConfig POST input: %v", &cfg)

		existingDevice, errWritten := a.existingDeviceOrError(w)
		if errWritten {
			return
		}

		// Validate the input strings. The variables map can be empty if the device owner wants
		// the workload to use all default values, so we wont validate that map.
		if cfg.WorkloadURL == "" {
			glog.Errorf("WorkloadConfig workload_url is empty: %v", cfg)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "workload_url", Error: "not specified"})
			return
		} else if cfg.Version == "" {
			glog.Errorf("WorkloadConfig workload_version is empty: %v", cfg)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "workload_version", Error: "not specified"})
			return
		} else if !policy.IsVersionString(cfg.Version) && !policy.IsVersionExpression(cfg.Version) {
			glog.Errorf("WorkloadConfig workload_version is not a valid version string or expression: %v", cfg)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "workload_version", Error: fmt.Sprintf("workload_version %v is not a valid version string or expression", cfg.Version)})
			return
		}

		// Convert the input version to a full version expression if it is not already a full expression.
		vExp, verr := policy.Version_Expression_Factory(cfg.Version)
		if verr != nil {
			glog.Errorf("WorkloadConfig workload_version %v error converting to full version expression, error: %v", cfg.Version, verr)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "workload_version", Error: fmt.Sprintf("workload_version %v error converting to full version expression, error: %v", cfg.Version, verr)})
			return
		}

		// Use the device org if not explicitly specified. Otherwise verify that the specified org exists.
		org := cfg.Org
		if cfg.Org == "" {
			org = existingDevice.Org
		} else if _, err := exchange.GetOrganization(a.Config.Collaborators.HTTPClientFactory, org, a.Config.Edge.ExchangeURL, existingDevice.GetId(), existingDevice.Token); err != nil {
			glog.Errorf("WorkloadConfig organization %v not found in exchange, error: %v", cfg.Org, err)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "organization", Error: fmt.Sprintf("organization %v not found in exchange, error: %v", cfg.Org, err)})
			return
		}

		// Reject the POST if there is already a config for this workload and version range
		existingCfg, err := persistence.FindWorkloadConfig(a.db, cfg.WorkloadURL, vExp.Get_expression())
		if err != nil {
			glog.Error(err)
			http.Error(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
			return
		} else if existingCfg != nil {
			glog.Errorf("WorkloadConfig workload config already exists: %v", cfg)
			http.Error(w, "Resource already exists", http.StatusConflict)
			return
		}

		// Get the workload metadata from the exchange and verify the userInput against the variables in the POST body.
		workloadDef, err := exchange.GetWorkload(a.Config.Collaborators.HTTPClientFactory, cfg.WorkloadURL, org, vExp.Get_expression(), cutil.ArchString(), a.Config.Edge.ExchangeURL, existingDevice.GetId(), existingDevice.Token)
		if err != nil || workloadDef == nil {
			glog.Errorf("Unable to find the workload definition using version %v in the exchange: %v", vExp.Get_expression(), err)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Error: fmt.Sprintf("Unable to find the workload definition using version %v in the exchange.", vExp.Get_expression())})
			return
		}

		// Loop through each input variable and verify that it is defined in the workload's user input section, and that the
		// type matches.
		for varName, varValue := range cfg.Variables {
			glog.V(5).Infof("WorkloadConfig checking input variable: %v", varName)
			if ui := workloadDef.GetUserInputName(varName); ui != nil {
				errMsg := ""
				switch varValue.(type) {
				case string:
					if ui.Type != "string" {
						errMsg = fmt.Sprintf("WorkloadConfig variable %v is type %T, expecting %v", varName, varValue, ui.Type)
					}
				case json.Number:
					strNum := varValue.(json.Number).String()
					if ui.Type != "int" && ui.Type != "float" {
						errMsg = fmt.Sprintf("WorkloadConfig variable %v is a number, expecting %v", varName, ui.Type)
					} else if strings.Contains(strNum, ".") && ui.Type == "int" {
						errMsg = fmt.Sprintf("WorkloadConfig variable %v is a float, expecting int", varName)
					}
					cfg.Variables[varName] = strNum
				case []interface{}:
					if ui.Type != "list of strings" {
						errMsg = fmt.Sprintf("WorkloadConfig variable %v is type %T, expecting %v", varName, varValue, ui.Type)
					} else {
						for _, e := range varValue.([]interface{}) {
							if _, ok := e.(string); !ok {
								errMsg = fmt.Sprintf("WorkloadConfig variable %v is not []string", varName)
								break
							}
						}
					}
				default:
					errMsg = fmt.Sprintf("WorkloadConfig variable %v is type %T, but is an unexpected type.", varName, varValue)
				}
				if errMsg != "" {
					glog.Error(errMsg)
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Error: errMsg})
					return
				}
			} else {
				glog.Errorf("Unable to find the workload config variable %v in workload definition userInputs: %v", varName, workloadDef.UserInputs)
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Error: fmt.Sprintf("Unable to find the workload config variable %v in workload definition", varName)})
				return
			}
		}

		// Loop through each userInput variable in the workload definition to make sure variables without default values have been set.
		for _, ui := range workloadDef.UserInputs {
			glog.V(5).Infof("WorkloadConfig checking workload userInput: %v", ui)
			if _, ok := cfg.Variables[ui.Name]; ok {
				// User Input variable is defined in the workload config request
				continue
			} else if !ok && ui.DefaultValue != "" {
				// User Input variable is not defined in the workload config request but it has a default in the workload definition. Save
				// the default into the workload config so that we dont have to query the exchange for the value when the workload starts.
				cfg.Variables[ui.Name] = ui.DefaultValue
			} else {
				// User Input variable is not defined in the workload config request and doesnt have a default, that's a problem.
				glog.Errorf("WorkloadConfig does not set %v, which has no default value in the workload", ui.Name)
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Error: fmt.Sprintf("WorkloadConfig does not set %v, which has no default value", ui.Name)})
				return
			}
		}

		// Persist the workload configuration to the database
		glog.V(5).Infof("WorkloadConfig persisting variables: %v", cfg.Variables)

		_, err = persistence.NewWorkloadConfig(a.db, cfg.WorkloadURL, vExp.Get_expression(), cfg.Variables)
		if err != nil {
			glog.Error(err)
			http.Error(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

	case "DELETE":

		// Demarshal the input body. Use the same body as the POST but ignore the variables section.
		var cfg WorkloadConfig
		body, _ := ioutil.ReadAll(r.Body)

		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		if err := decoder.Decode(&cfg); err != nil {
			glog.Errorf("User submitted data that couldn't be deserialized to workload config: %v. Error: %v", string(body), err)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "workloadConfig", Error: fmt.Sprintf("could not be demarshalled, error: %v", err)})
			return
		}

		glog.V(5).Infof("WorkloadConfig DELETE: %v", &cfg)

		// Validate the input strings. The variables map is ignored.
		if cfg.WorkloadURL == "" {
			glog.Errorf("WorkloadConfig workload_url is empty: %v", cfg)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "workload_url", Error: "not specified"})
			return
		} else if cfg.Version == "" {
			glog.Errorf("WorkloadConfig workload_version is empty: %v", cfg)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "workload_version", Error: "not specified"})
			return
		} else if !policy.IsVersionString(cfg.Version) && !policy.IsVersionExpression(cfg.Version) {
			glog.Errorf("WorkloadConfig workload_version is not a valid version string: %v", cfg)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "workload_version", Error: fmt.Sprintf("workload_version %v is not a valid version string", cfg.Version)})
			return
		}

		// Convert the input version to a full version expression if it is not already a full expression.
		vExp, verr := policy.Version_Expression_Factory(cfg.Version)
		if verr != nil {
			glog.Errorf("WorkloadConfig workload_version %v error converting to full version expression, error: %v", cfg.Version, verr)
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "workload_version", Error: fmt.Sprintf("workload_version %v error converting to full version expression, error: %v", cfg.Version, verr)})
			return
		}

		// Find the target record
		existingCfg, err := persistence.FindWorkloadConfig(a.db, cfg.WorkloadURL, vExp.Get_expression())
		if err != nil {
			glog.Error(err)
			http.Error(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
			return
		} else if existingCfg == nil {
			http.Error(w, "WorkloadConfig not found", http.StatusNotFound)
			return
		} else {
			glog.V(5).Infof("WorkloadConfig deleting: %v", &cfg)
			persistence.DeleteWorkloadConfig(a.db, cfg.WorkloadURL, vExp.Get_expression())
			w.WriteHeader(http.StatusNoContent)
			return
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) microservice(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		// we don't support getting just one yet
		if id != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		msinsts, err := persistence.FindMicroserviceInstances(a.db, []persistence.MIFilter{})
		if err != nil {
			glog.Error(err)
			http.Error(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
			return
		}

		msdefs, err := persistence.FindMicroserviceDefs(a.db, []persistence.MSFilter{})
		if err != nil {
			glog.Error(err)
			http.Error(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
			return
		}

		var msinstKey = "instances"
		var msdefKey = "definitions"
		var archivedKey = "archived"
		var activeKey = "active"

		wrap := make(map[string]map[string][]interface{}, 0)

		wrap[msinstKey] = make(map[string][]interface{}, 0)
		wrap[msinstKey][archivedKey] = []interface{}{}
		wrap[msinstKey][activeKey] = []interface{}{}

		wrap[msdefKey] = make(map[string][]interface{}, 0)
		wrap[msdefKey][archivedKey] = make([]interface{}, 0)
		wrap[msdefKey][activeKey] = make([]interface{}, 0)

		for _, msinst := range msinsts {
			if msinst.Archived {
				wrap[msinstKey][archivedKey] = append(wrap[msinstKey][archivedKey], msinst)
			} else {
				wrap[msinstKey][activeKey] = append(wrap[msinstKey][activeKey], msinst)
			}
		}

		for _, msdef := range msdefs {
			if msdef.Archived {
				wrap[msdefKey][archivedKey] = append(wrap[msdefKey][archivedKey], msdef)
			} else {
				wrap[msdefKey][activeKey] = append(wrap[msdefKey][activeKey], msdef)
			}
		}

		// do sorts
		sort.Sort(MicroserviceInstanceByMicroserviceDefId(wrap[msinstKey][activeKey]))
		sort.Sort(MicroserviceInstanceByCleanupStartTime(wrap[msinstKey][archivedKey]))
		sort.Sort(MicroserviceDefById(wrap[msdefKey][activeKey]))
		sort.Sort(MicroserviceDefByUpgradeStartTime(wrap[msdefKey][archivedKey]))

		serial, err := json.Marshal(wrap)
		if err != nil {
			glog.Infof("Error serializing microservice output: %v", err)
			http.Error(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(serial); err != nil {
			glog.Infof("Error writing response: %v", err)
			http.Error(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
			return
		}
	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
