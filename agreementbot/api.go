package agreementbot

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/open-horizon/anax/agreementbot/persistence"
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
	"time"
)

type API struct {
	worker.Manager // embedded field
	name           string
	db             persistence.AgbotDatabase
	pm             *policy.PolicyManager
	bcState        map[string]map[string]apicommon.BlockchainState
	bcStateLock    sync.Mutex
	EC             *worker.BaseExchangeContext
	em             *events.EventStateManager
	shutdownError  string
	configFile     string
}

func NewAPIListener(name string, config *config.HorizonConfig, db persistence.AgbotDatabase, configFile string) *API {
	messages := make(chan events.Message)

	listener := &API{
		Manager: worker.Manager{
			Config:   config,
			Messages: messages,
		},

		name:       name,
		db:         db,
		EC:         worker.NewExchangeContext(config.AgreementBot.ExchangeId, config.AgreementBot.ExchangeToken, config.AgreementBot.ExchangeURL, config.GetAgbotCSSURL(), config.Collaborators.HTTPClientFactory),
		em:         events.NewEventStateManager(),
		configFile: configFile,
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

func (a *API) saveShutdownError(msg events.Message) {
	switch msg.(type) {
	case *events.NodeShutdownCompleteMessage:
		m, _ := msg.(*events.NodeShutdownCompleteMessage)
		a.shutdownError = m.Err()
	}
}

func (a *API) GetName() string {
	return a.name
}

type ServedOrgs struct {
	ServedPatterns map[string]exchange.ServedPattern        `json:"servedPatterns"`
	ServedPolicies map[string]exchange.ServedBusinessPolicy `json:"servedPolicies"`
}

func (a *API) ListServedOrgs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		pmServedPatterns := patternManager.GetServedPatterns()
		bmServedPolicies := businessPolManager.GetServedPolicies()
		// retrieve info to write
		info := ServedOrgs{ServedPatterns: pmServedPatterns, ServedPolicies: bmServedPolicies}
		writeResponse(w, info, http.StatusOK)
	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) ListPatterns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		org := pathVars["org"]
		name := pathVars["name"]
		long := r.URL.Query().Get("long")
		pmOrgPats := patternManager.GetOrgPatterns()

		if org == "" {
			// if no org, display detailed or undetailed all orgs and names
			if long != "" {
				writeResponse(w, pmOrgPats, http.StatusOK)
			} else {
				cachedPatterns := make(map[string][]string)
				for org, patterns := range pmOrgPats {
					patternIds := make([]string, 0, len(patterns))
					for k := range patterns {
						patternIds = append(patternIds, k)
					}
					cachedPatterns[org] = patternIds
				}

				writeResponse(w, cachedPatterns, http.StatusOK)
			}
		} else if patternManager.hasOrg(org) {
			if name == "" {
				// if org is specified and valid and no name is specified, display all patterns under org
				if long != "" {
					writeResponse(w, pmOrgPats, http.StatusOK)
				} else {
					cachedPatterns := make(map[string][]string)
					for o, patterns := range pmOrgPats {
						patternIds := make([]string, 0, len(patterns))
						for k := range patterns {
							patternIds = append(patternIds, k)
						}
						cachedPatterns[o] = patternIds
					}
					writeResponse(w, cachedPatterns[org], http.StatusOK)
				}
				// if name is specified and valid, display detailed
			} else if _, hasName := pmOrgPats[org][name]; hasName {
				writeResponse(w, pmOrgPats[org][name], http.StatusOK)
				// else throw 404 error
			} else {
				writeResponse(w, "pattern not found in the pattern management cache.", http.StatusNotFound)
			}
		} else {
			writeResponse(w, "organization not found in the pattern management cache.", http.StatusNotFound)
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) ListDeploy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		org := pathVars["org"]
		name := pathVars["name"]
		long := r.URL.Query().Get("long")
		bmOrgPols := businessPolManager.GetOrgPolicies()
		if org == "" {
			// if no org, display detailed or undetailed all orgs and names
			if long != "" {
				writeResponse(w, bmOrgPols, http.StatusOK)
			} else {
				cachedPol := make(map[string][]string)
				for org, policies := range bmOrgPols {
					polIds := make([]string, 0, len(policies))
					for k := range policies {
						polIds = append(polIds, k)
					}
					cachedPol[org] = polIds
				}

				writeResponse(w, cachedPol, http.StatusOK)
			}
		} else if businessPolManager.hasOrg(org) {
			if name == "" {
				// if org is specified and valid and no name is specified, display all deployment policies under org
				if long != "" {
					writeResponse(w, bmOrgPols[org], http.StatusOK)
				} else {
					cachedPol := make(map[string][]string)
					for o, policies := range bmOrgPols {
						polIds := make([]string, 0, len(policies))
						for k := range policies {
							polIds = append(polIds, k)
						}
						cachedPol[o] = polIds
					}

					writeResponse(w, cachedPol[org], http.StatusOK)
				}
				// if name is specified and valid, display detailed
			} else if _, hasName := bmOrgPols[org][name]; hasName {
				writeResponse(w, bmOrgPols[org][name], http.StatusOK)
				// else throw error
			} else {
				writeResponse(w, "policy not found in the deployment policy management cache.", http.StatusNotFound)
			}
		} else {
			writeResponse(w, "organization not found in the deployment policy management cache.", http.StatusNotFound)
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
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
		return ""
	}
}

func (a *API) GetCSSURL() string {
	if a.EC != nil {
		return a.EC.CSSURL
	} else {
		return ""
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
		router.HandleFunc("/partition", a.partition).Methods("GET", "OPTIONS")
		router.HandleFunc("/policy", a.policy).Methods("GET", "OPTIONS")
		router.HandleFunc("/policy/{org}", a.policy).Methods("GET", "OPTIONS")
		router.HandleFunc("/policy/{org}/{name}", a.policy).Methods("GET", "OPTIONS")
		router.HandleFunc("/policy/{name}/upgrade", a.policy).Methods("POST", "OPTIONS")
		router.HandleFunc("/workloadusage", a.workloadusage).Methods("GET", "OPTIONS")
		router.HandleFunc("/status", a.status).Methods("GET", "OPTIONS")
		router.HandleFunc("/health", a.health).Methods("GET", "OPTIONS")
		router.HandleFunc("/status/workers", a.workerstatus).Methods("GET", "OPTIONS")
		router.HandleFunc("/node", a.node).Methods("GET", "DELETE", "OPTIONS")
		router.HandleFunc("/config", a.config).Methods("GET", "OPTIONS")
		router.HandleFunc("/cache/servedorg", a.ListServedOrgs).Methods("GET", "OPTIONS")
		router.HandleFunc("/cache/pattern", a.ListPatterns).Methods("GET", "OPTIONS")
		router.HandleFunc("/cache/pattern/{org}", a.ListPatterns).Methods("GET", "OPTIONS")
		router.HandleFunc("/cache/pattern/{org}/{name}", a.ListPatterns).Methods("GET", "OPTIONS")
		router.HandleFunc("/cache/deploymentpol", a.ListDeploy).Methods("GET", "OPTIONS")
		router.HandleFunc("/cache/deploymentpol/{org}", a.ListDeploy).Methods("GET", "OPTIONS")
		router.HandleFunc("/cache/deploymentpol/{org}/{name}", a.ListDeploy).Methods("GET", "OPTIONS")

		if err := http.ListenAndServe(apiListen, nocache(router)); err != nil {
			glog.Fatalf(APIlogString(fmt.Sprintf("failed to start listener on %v, error %v", apiListen, err)))
		}
	}()
}

func (a *API) agreement(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		if id != "" {
			if ag, err := a.db.FindSingleAgreementByAgreementIdAllProtocols(id, policy.AllAgreementProtocols(), []persistence.AFilter{}); err != nil {
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

			wrap := make(map[string]map[string][]persistence.Agreement, 0)
			wrap[agreementsKey] = make(map[string][]persistence.Agreement, 0)
			wrap[agreementsKey][archivedKey] = []persistence.Agreement{}
			wrap[agreementsKey][activeKey] = []persistence.Agreement{}

			for _, agp := range policy.AllAgreementProtocols() {
				if ags, err := a.db.FindAgreements([]persistence.AFilter{}, agp); err != nil {
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

		if ag, err := a.db.FindSingleAgreementByAgreementIdAllProtocols(id, policy.AllAgreementProtocols(), []persistence.AFilter{persistence.UnarchivedAFilter()}); err != nil {
			glog.Error(APIlogString(fmt.Sprintf("error finding agreement %v, error: %v", id, err)))
			w.WriteHeader(http.StatusInternalServerError)
		} else if ag == nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "id", Error: "agreement id not found"})
		} else {
			if ag.AgreementTimedout == 0 {
				// Update the database
				if _, err := a.db.AgreementTimedout(ag.CurrentAgreementId, ag.AgreementProtocol); err != nil {
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

	serviceResolver := func(wURL string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, error) {
		asl, _, _, err := exchange.GetHTTPServiceResolverHandler(a)(wURL, wOrg, wVersion, wArch)
		if err != nil {
			glog.Errorf(APIlogString(fmt.Sprintf("unable to resolve %v %v, error %v", wURL, wOrg, err)))
		}
		return asl, err
	}

	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		org := pathVars["org"]
		name := pathVars["name"]

		// get a list of hosted policy names
		if pm, err := policy.Initialize(a.Config.AgreementBot.PolicyPath, a.Config.ArchSynonyms, serviceResolver, false, false); err != nil {
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
		if pm, err := policy.Initialize(a.Config.AgreementBot.PolicyPath, a.Config.ArchSynonyms, serviceResolver, false, false); err != nil {
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
			if ag, err := a.db.FindSingleAgreementByAgreementIdAllProtocols(upgrade.AgreementId, policy.AllAgreementProtocols(), []persistence.AFilter{persistence.UnarchivedAFilter()}); err != nil {
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
			if wlUsage, err := a.db.FindSingleWorkloadUsageByDeviceAndPolicyName(upgrade.Device, policyName); err != nil {
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
		if wlusages, err := a.db.FindWorkloadUsages([]persistence.WUFilter{}); err != nil {
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

		info := apicommon.NewInfo(a.GetHTTPFactory(), a.GetExchangeURL(), a.GetCSSURL(), a.GetExchangeId(), a.GetExchangeToken())

		// Augment the common status with agbot specific stuff
		health := &apicommon.HealthTimestamps{}
		var err error
		if health.LastDBHeartbeatTime, err = a.db.GetHeartbeat(); err != nil {
			glog.Errorf(APIlogString(fmt.Sprintf("Unable to get DB heartbeat, error: %v", err)))
		}
		info.LiveHealth = health

		writeResponse(w, info, http.StatusOK)
	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		info := apicommon.NewLocalInfo(a.GetExchangeURL(), a.GetCSSURL(), a.GetExchangeId(), a.GetExchangeToken())

		// Augment the common status with agbot specific stuff
		health := &apicommon.HealthTimestamps{}
		var err error
		if health.LastDBHeartbeatTime, err = a.db.GetHeartbeat(); err != nil {
			glog.Errorf(APIlogString(fmt.Sprintf("Unable to get DB heartbeat, error: %v", err)))
		}
		info.LiveHealth = health

		writeResponse(w, info, http.StatusOK)
	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) workerstatus(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		status := worker.GetWorkerStatusManager()
		writeResponse(w, status, http.StatusOK)
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

	case "DELETE":
		glog.V(5).Infof(APIlogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Get the blocking option from the URL query parameters. If blocking is true, then the API will block
		// until the Agbot quiesce is complete. True is the default.
		block := r.URL.Query().Get("block")
		if block != "" && block != "true" && block != "false" {
			glog.Error(APIlogString(fmt.Sprintf("%v is an incorrect value for block, must be true or false", block)))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		} else if block == "" {
			block = "true"
		}

		blocking := true
		if block == "false" {
			blocking = false
		}

		// Quiesce the agbot. This means:
		// a) stop the search for nodes to make agreements with, and then
		// b) make sure all this agbot's agreements are in a steady state, meaning archived or finalized

		// Fire the NodeShutdown event to get the agbot to quiesce itself.
		ns := events.NewNodeShutdownMessage(events.START_AGBOT_QUIESCE, blocking, false)
		a.Messages() <- ns

		// Wait (if allowed) for the ShutdownComplete event
		if block == "true" {
			se := events.NewNodeShutdownCompleteMessage(events.AGBOT_QUIESCE_COMPLETE, "")
			for {
				if a.em.ReceivedEvent(se, nil) {
					break
				}
				glog.V(5).Infof(APIlogString(fmt.Sprintf("Waiting for agbot shutdown to complete")))
				time.Sleep(5 * time.Second)
			}
		}

		glog.V(5).Infof(APIlogString(fmt.Sprintf("Handled %v on resource %v", r.Method, resource)))

		w.WriteHeader(http.StatusNoContent)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

//Get Agbot config info
func (a *API) config(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if cfg, err := a.GetHorizonAgbotConfig(); err != nil {
			// ConfigFile does not exist
			glog.Error(APIlogString(fmt.Sprintf("error with File System Config File, error: %v", err)))
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			writeResponse(w, cfg, http.StatusOK)
		}
	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) partition(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":

		// For each partition, how many agreements and other objects are in it. The top level keys in the output
		// are the partition names, the sub maps are for each of agreements, workload usage, etc.
		const PARTITION_OWNER = "owner"
		const AGREEMENT_ACTIVE_KEY = "active agreements"
		const AGREEMENT_ARCHIVED_KEY = "archived agreements"
		const WORKLOAD_USAGES_KEY = "workload usages"

		output := make(map[string]map[string]interface{}, 0)

		if partitions, err := a.db.FindPartitions(); err != nil {
			glog.Error(APIlogString(fmt.Sprintf("error finding all partitions, error: %v", err)))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else {

			// For each partition, get a count of records in the partition.
			for _, p := range partitions {
				partitionMaps := make(map[string]interface{}, 0)

				// First get the partition owner.
				if owner, err := a.db.GetPartitionOwner(p); err != nil {
					glog.Error(APIlogString(fmt.Sprintf("error finding partition %v owner, error: %v", p, err)))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				} else {
					partitionMaps[PARTITION_OWNER] = owner
				}

				// Then get the agreement count.
				if active, archived, err := a.db.GetAgreementCount(p); err != nil {
					glog.Error(APIlogString(fmt.Sprintf("error finding agreement count in partition %v, error: %v", p, err)))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				} else {
					partitionMaps[AGREEMENT_ACTIVE_KEY] = active
					partitionMaps[AGREEMENT_ARCHIVED_KEY] = archived
				}

				// Then get the workload_usage count.
				if num, err := a.db.GetWorkloadUsagesCount(p); err != nil {
					glog.Error(APIlogString(fmt.Sprintf("error finding workload usage count in partition %v, error: %v", p, err)))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				} else {
					partitionMaps[WORKLOAD_USAGES_KEY] = num
				}

				// Set the values for the current partition
				output[p] = partitionMaps

			}

			writeResponse(w, output, http.StatusOK)
		}

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

type HorizonAgbotConfig struct {
	InMemoryConfig   config.AGConfig `json:"InMemoryConfig"`
	FileSystemConfig config.AGConfig `json:"FileSystemConfig"`
}

func (a *API) GetHorizonAgbotConfig() (*HorizonAgbotConfig, error) {
	cfg, err := config.Read(a.configFile)
	if err != nil {
		glog.Error(APIlogString(fmt.Sprintf("error finding File System Config File %v, error: %v", a.configFile, err)))
	}
	return &HorizonAgbotConfig{
		InMemoryConfig:   a.Config.AgreementBot,
		FileSystemConfig: cfg.AgreementBot,
	}, err
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
type AgreementsByAgreementCreationTime []persistence.Agreement

func (s AgreementsByAgreementCreationTime) Len() int {
	return len(s)
}

func (s AgreementsByAgreementCreationTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s AgreementsByAgreementCreationTime) Less(i, j int) bool {
	return s[i].AgreementInceptionTime < s[j].AgreementInceptionTime
}

type AgreementsByAgreementTimeoutTime []persistence.Agreement

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
type WorkloadUsagesByDeviceId []persistence.WorkloadUsage

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
