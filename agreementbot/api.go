package agreementbot

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"io/ioutil"
	"net/http"
	"sort"
	"sync"
)

type API struct {
	worker.Manager // embedded field
	name           string
	db             *bolt.DB
	pm             *policy.PolicyManager
	bcState        map[string]map[string]apicommon.BlockchainState
	bcStateLock    sync.Mutex
	EC             *worker.BaseExchangeContext
}

func NewAPIListener(name string, config *config.HorizonConfig, db *bolt.DB) *API {
	messages := make(chan events.Message)

	listener := &API{
		Manager: worker.Manager{
			Config:   config,
			Messages: messages,
		},

		name: name,
		db:   db,
		EC:   worker.NewExchangeContext(config.AgreementBot.ExchangeId, config.AgreementBot.ExchangeToken, config.AgreementBot.ExchangeURL, false, config.Collaborators.HTTPClientFactory),
	}

	listener.listen(config.AgreementBot.APIListen)
	return listener
}

// Worker framework functions
func (a *API) Messages() chan events.Message {
	return a.Manager.Messages
}

func (a *API) NewEvent(ev events.Message) {

	switch ev.(type) {
	case *events.BlockchainClientInitializedMessage:
		msg, _ := ev.(*events.BlockchainClientInitializedMessage)
		switch msg.Event().Id {
		case events.BC_CLIENT_INITIALIZED:
			apicommon.HandleNewBCInit(msg, a.bcState, &a.bcStateLock)
			glog.V(3).Infof(APIlogString(fmt.Sprintf("API Worker processed BC initialization for %v", msg)))
		}

	case *events.BlockchainClientStoppingMessage:
		msg, _ := ev.(*events.BlockchainClientStoppingMessage)
		switch msg.Event().Id {
		case events.BC_CLIENT_STOPPING:
			apicommon.HandleStoppingBC(msg, a.bcState, &a.bcStateLock)
			glog.V(3).Infof(APIlogString(fmt.Sprintf("API Worker processed BC stopping for %v", msg)))
		}
	case *events.NodeShutdownCompleteMessage:
		// Now remove myself from the worker dispatch list. When the anax process terminates,
		// the socket listener will terminate also. This is done on a separate thread so that
		// the message dispatcher doesnt get blocked. This worker isnt actually a full blown
		// worker and doesnt have a command thread that it can run on.
		go func() {
			a.Messages() <- events.NewWorkerStopMessage(events.WORKER_STOP, a.GetName())
		}()
	}

	return
}

func (a *API) GetName() string {
	return a.name
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
		return ""
	}
}

func (a *API) GetServiceBased() bool {
	if a.EC != nil {
		return a.EC.ServiceBased
	} else {
		return false
	}
}

func (a *API) GetHTTPFactory() *config.HTTPClientFactory {
	if a.EC != nil {
		return a.EC.HTTPFactory
	} else {
		return a.Config.Collaborators.HTTPClientFactory
	}
}

func (a *API) listen(apiListen string) {
	glog.Info("Starting AgreementBot API server")

	// If there is no Agbot config, we will terminate
	if apiListen == "" {
		glog.Errorf("AgreementBotWorker API terminating, no AgreementBot API config.")
		return
	} else if a.db == nil {
		glog.Errorf("AgreementBotWorker API terminating, no AgreementBot database configured.")
		return
	}

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

	// This routine does not need to be a subworker because it will terminate on its own when the main
	// anax process terminates.
	go func() {
		router := mux.NewRouter()

		router.HandleFunc("/agreement", a.agreement).Methods("GET", "OPTIONS")
		router.HandleFunc("/agreement/{id}", a.agreement).Methods("GET", "DELETE", "OPTIONS")
		router.HandleFunc("/policy", a.policy).Methods("GET", "OPTIONS")
		router.HandleFunc("/policy/{org}", a.policy).Methods("GET", "OPTIONS")
		router.HandleFunc("/policy/{org}/{name}", a.policy).Methods("GET", "OPTIONS")
		router.HandleFunc("/policy/{name}/upgrade", a.policy).Methods("POST", "OPTIONS")
		router.HandleFunc("/workloadusage", a.workloadusage).Methods("GET", "OPTIONS")
		router.HandleFunc("/status", a.status).Methods("GET", "OPTIONS")
		router.HandleFunc("/node", a.node).Methods("GET", "OPTIONS")

		http.ListenAndServe(apiListen, nocache(router))
	}()
}

func (a *API) agreement(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		if id != "" {
			if ag, err := FindSingleAgreementByAgreementIdAllProtocols(a.db, id, policy.AllAgreementProtocols(), []AFilter{}); err != nil {
				glog.Error(APIlogString(fmt.Sprintf("error finding agreement %v, error: %v", id, err)))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			} else if ag == nil {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "id", Error: "agreement id not found"})
			} else {
				// write output
				writeResponse(w, *ag, http.StatusOK)
			}
		} else {
			var agreementsKey = "agreements"
			var archivedKey = "archived"
			var activeKey = "active"

			wrap := make(map[string]map[string][]Agreement, 0)
			wrap[agreementsKey] = make(map[string][]Agreement, 0)
			wrap[agreementsKey][archivedKey] = []Agreement{}
			wrap[agreementsKey][activeKey] = []Agreement{}

			for _, agp := range policy.AllAgreementProtocols() {
				if ags, err := FindAgreements(a.db, []AFilter{}, agp); err != nil {
					glog.Error(APIlogString(fmt.Sprintf("error finding all agreements, error: %v", err)))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				} else {

					for _, agreement := range ags {
						// The archived agreements and the agreements being terminated are returned as archived.
						if agreement.Archived || agreement.AgreementTimedout != 0 {
							wrap[agreementsKey][archivedKey] = append(wrap[agreementsKey][archivedKey], agreement)
						} else {
							wrap[agreementsKey][activeKey] = append(wrap[agreementsKey][activeKey], agreement)
						}
					}

				}
			}

			// do sorts
			sort.Sort(AgreementsByAgreementCreationTime(wrap[agreementsKey][activeKey]))
			sort.Sort(AgreementsByAgreementTimeoutTime(wrap[agreementsKey][archivedKey]))

			// write output
			writeResponse(w, wrap, http.StatusOK)
		}

	case "DELETE":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		glog.V(3).Infof(APIlogString(fmt.Sprintf("handling DELETE of agreement: %v", r)))

		if ag, err := FindSingleAgreementByAgreementIdAllProtocols(a.db, id, policy.AllAgreementProtocols(), []AFilter{UnarchivedAFilter()}); err != nil {
			glog.Error(APIlogString(fmt.Sprintf("error finding agreement %v, error: %v", id, err)))
			w.WriteHeader(http.StatusInternalServerError)
		} else if ag == nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "id", Error: "agreement id not found"})
		} else {
			if ag.AgreementTimedout == 0 {
				// Update the database
				if _, err := AgreementTimedout(a.db, ag.CurrentAgreementId, ag.AgreementProtocol); err != nil {
					glog.Errorf(APIlogString(fmt.Sprintf("error marking agreement %v terminated: %v", ag.CurrentAgreementId, err)))
				}
				a.Messages() <- events.NewABApiAgreementCancelationMessage(events.AGREEMENT_ENDED, ag.AgreementProtocol, ag.CurrentAgreementId)
			} else {
				glog.V(3).Infof(APIlogString(fmt.Sprintf("agreement %v not deleted, already timed out at %v", id, ag.AgreementTimedout)))
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

func (a *API) policy(w http.ResponseWriter, r *http.Request) {
	workloadResolver := func(wURL string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, error) {
		asl, _, err := exchange.GetHTTPWorkloadResolverHandler(a)(wURL, wOrg, wVersion, wArch)
		if err != nil {
			glog.Errorf(APIlogString(fmt.Sprintf("unable to resolve workload, error %v", err)))
		}
		return asl, err
	}

	serviceResolver := func(wURL string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, error) {
		asl, _, err := exchange.GetHTTPServiceResolverHandler(a)(wURL, wOrg, wVersion, wArch)
		if err != nil {
			glog.Errorf(APIlogString(fmt.Sprintf("unable to resolve service, error %v", err)))
		}
		return asl, err
	}

	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		org := pathVars["org"]
		name := pathVars["name"]

		// get a list of hosted policy names
		if pm, err := policy.Initialize(a.Config.AgreementBot.PolicyPath, a.Config.ArchSynonyms, workloadResolver, serviceResolver, false, false); err != nil {
			glog.Error(APIlogString(fmt.Sprintf("error initializing policy manager, error: %v", err)))
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			if org == "" {
				// get all the policy names
				response := pm.GetAllPolicyNames()
				writeResponse(w, response, http.StatusOK)
			} else if name == "" {
				// get the policy names for the given org
				response := pm.GetPolicyNamesForOrg(org)

				if len(response) > 0 && len(response[org]) > 0 {
					writeResponse(w, response, http.StatusOK)
				} else if _, err = exchange.GetOrganization(a.GetHTTPFactory(), org, a.GetExchangeURL(), a.GetExchangeId(), a.GetExchangeToken()); err != nil {
					// org does not exists
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "org", Error: "organization does not exist in the exchange."})
				} else {
					// org exists but no policy files
					res := make(map[string][]string, 1)
					res[org] = make([]string, 0)
					writeResponse(w, res, http.StatusOK)
				}
			} else {
				if response := pm.GetPolicy(org, name); response != nil {
					writeResponse(w, response, http.StatusOK)
				} else if _, err = exchange.GetOrganization(a.GetHTTPFactory(), org, a.GetExchangeURL(), a.GetExchangeId(), a.GetExchangeToken()); err != nil {
					// org does not exists
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "org", Error: "organization does not exist in the exchange."})
				} else {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "name", Error: "policy not found."})
				}
			}
		}

	case "POST":
		pathVars := mux.Vars(r)
		policyName := pathVars["name"]

		if policyName == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		glog.V(3).Infof(APIlogString(fmt.Sprintf("handling POST of policy: %v", policyName)))

		// Demarshal the input body and verify it.
		var upgrade UpgradeDevice
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &upgrade); err != nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "body", Error: fmt.Sprintf("user submitted data couldn't be deserialized to struct: %v. Error: %v", string(body), err)})
			return
		} else if ok, msg := upgrade.IsValid(); !ok {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "body", Error: msg})
			return
		}

		// Verify the input policy name. It can be either the name of the policy within the header of the policy file or the name
		// of the file itself.
		found := false
		if pm, err := policy.Initialize(a.Config.AgreementBot.PolicyPath, a.Config.ArchSynonyms, workloadResolver, serviceResolver, false, false); err != nil {
			glog.Error(APIlogString(fmt.Sprintf("error initializing policy manager, error: %v", err)))
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			if pol := pm.GetPolicy(upgrade.Org, policyName); pol != nil {
				found = true
			} else if name := pm.WatcherContent.GetPolicyName(upgrade.Org, policyName); name != "" {
				found = true
				policyName = name
			}
		}

		if !found {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "policy name", Error: fmt.Sprintf("no policies with the name %v", policyName)})
			return
		}

		protocol := ""
		// The body is syntacticly correct, verify that the agreement id matches up with the device id and policy name.
		if upgrade.AgreementId != "" {
			if ag, err := FindSingleAgreementByAgreementIdAllProtocols(a.db, upgrade.AgreementId, policy.AllAgreementProtocols(), []AFilter{UnarchivedAFilter()}); err != nil {
				glog.Error(APIlogString(fmt.Sprintf("error finding agreement %v, error: %v", upgrade.AgreementId, err)))
				w.WriteHeader(http.StatusInternalServerError)
				return
			} else if ag == nil {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementId", Error: "agreement id not found"})
				return
			} else if ag.AgreementTimedout != 0 {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementId", Error: fmt.Sprintf("agreement %v not upgraded, already timed out at %v", upgrade.AgreementId, ag.AgreementTimedout)})
				return
			} else if upgrade.Device != "" && ag.DeviceId != upgrade.Device {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementId", Error: fmt.Sprintf("agreement %v not upgraded, not with specified device id %v", upgrade.AgreementId, upgrade.Device)})
				return
			} else if ag.PolicyName != policyName {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementId", Error: fmt.Sprintf("agreement %v not upgraded, not using policy %v", upgrade.AgreementId, policyName)})
				return
			} else {
				// We have a valid agreement. Make sure we get the device id to pass along in the event if it isnt in the input.
				if upgrade.Device == "" {
					upgrade.Device = ag.DeviceId
				}
				protocol = ag.AgreementProtocol
			}

		}

		// Verfiy that the device is using the workload rollback feature
		if upgrade.Device != "" {
			if wlUsage, err := FindSingleWorkloadUsageByDeviceAndPolicyName(a.db, upgrade.Device, policyName); err != nil {
				glog.Error(APIlogString(fmt.Sprintf("error finding workload usage record for %v with policy %v, error: %v", upgrade.AgreementId, policyName, err)))
				w.WriteHeader(http.StatusInternalServerError)
				return
			} else if wlUsage == nil {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "device", Error: fmt.Sprintf("device %v with policy %v is not using the workload rollback feature", upgrade.Device, policyName)})
				return
			}
		}

		// If we got this far, begin workload upgrade processing.
		a.Messages() <- events.NewABApiWorkloadUpgradeMessage(events.WORKLOAD_UPGRADE, protocol, upgrade.AgreementId, upgrade.Device, policyName)
		w.WriteHeader(http.StatusOK)

	case "OPTIONS":
		w.Header().Set("Allow", "POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) workloadusage(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		if wlusages, err := FindWorkloadUsages(a.db, []WUFilter{}); err != nil {
			glog.Error(APIlogString(fmt.Sprintf("error finding all workload usages, error: %v", err)))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else {

			// do sort
			sort.Sort(WorkloadUsagesByDeviceId(wlusages))

			// write output
			writeResponse(w, wlusages, http.StatusOK)
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) status(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":

		info := apicommon.NewInfo(a.GetHTTPFactory(), a.GetExchangeURL(), a.GetExchangeId(), a.GetExchangeToken())

		if err := apicommon.WriteConnectionStatus(info); err != nil {
			glog.Errorf(APIlogString(fmt.Sprintf("Unable to get connectivity status: %v", err)))
		}

		a.bcStateLock.Lock()
		defer a.bcStateLock.Unlock()

		for _, bc := range a.bcState[policy.Ethereum_bc] {
			geth := apicommon.NewGeth()

			gethURL := fmt.Sprintf("http://%v:%v", bc.GetService(), bc.GetServicePort())
			if err := apicommon.WriteGethStatus(gethURL, geth); err != nil {
				glog.Errorf(APIlogString(fmt.Sprintf("Unable to determine geth service facts: %v", err)))
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

func (a *API) node(w http.ResponseWriter, r *http.Request) {

	resource := "node"

	switch r.Method {
	case "GET":
		glog.V(5).Infof(APIlogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		id_org := a.Config.AgreementBot.ExchangeId
		var id, org string
		if id_org != "" {
			id = exchange.GetId(id_org)
			org = exchange.GetOrg(id_org)
		}
		agbot := NewHorizonAgbot(id, org)
		writeResponse(w, agbot, http.StatusOK)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// ==========================================================================================
// Utility functions used by many of the API endpoints.
//
type HorizonAgbot struct {
	Id  string `json:"agbot_id"`
	Org string `json:"organization"`
}

func NewHorizonAgbot(id string, org string) *HorizonAgbot {
	return &HorizonAgbot{
		Id:  id,
		Org: org,
	}
}

func getAgbotInfo(config *config.HorizonConfig) {

}

type APIUserInputError struct {
	Error string `json:"error"`
	Input string `json:"input,omitempty"`
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

// Helper functions for sorting agreements
type AgreementsByAgreementCreationTime []Agreement

func (s AgreementsByAgreementCreationTime) Len() int {
	return len(s)
}

func (s AgreementsByAgreementCreationTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s AgreementsByAgreementCreationTime) Less(i, j int) bool {
	return s[i].AgreementInceptionTime < s[j].AgreementInceptionTime
}

type AgreementsByAgreementTimeoutTime []Agreement

func (s AgreementsByAgreementTimeoutTime) Len() int {
	return len(s)
}

func (s AgreementsByAgreementTimeoutTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s AgreementsByAgreementTimeoutTime) Less(i, j int) bool {
	return s[i].AgreementTimedout < s[j].AgreementTimedout
}

// Helper functions for sorting workload usages
type WorkloadUsagesByDeviceId []WorkloadUsage

func (s WorkloadUsagesByDeviceId) Len() int {
	return len(s)
}

func (s WorkloadUsagesByDeviceId) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s WorkloadUsagesByDeviceId) Less(i, j int) bool {
	return s[i].DeviceId < s[j].DeviceId
}

// Log string prefix api
var APIlogString = func(v interface{}) string {
	return fmt.Sprintf("AgreementBotWorker API %v", v)
}

type UpgradeDevice struct {
	Device      string `json:"device"`
	AgreementId string `json:"agreementId"`
	Org         string `json:"org"`
}

func (b *UpgradeDevice) IsValid() (bool, string) {
	if b.Device == "" && b.AgreementId == "" {
		return false, "must specify either device or agreementId"
	}
	return true, ""
}

// Utility functions used by all the http handlers for each API path.
func serializeResponse(w http.ResponseWriter, payload interface{}) ([]byte, bool) {
	glog.V(6).Infof(APIlogString(fmt.Sprintf("response payload before serialization (%T): %v", payload, payload)))

	serial, err := json.Marshal(payload)
	if err != nil {
		glog.Error(APIlogString(err))
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
		glog.Error(APIlogString(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
