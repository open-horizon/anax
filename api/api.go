package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"

	"github.com/boltdb/bolt"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/governance"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
)

var HORIZON_SERVERS = [...]string{"firmware.bluehorizon.network", "images.bluehorizon.network"}

var ILLEGAL_INPUT_CHAR_REGEX = `[^-() _\w\d.@,]`

type API struct {
	worker.Manager // embedded field
	db             *bolt.DB
}

type Firmware struct {
	Definition   string `json:"definition"`
	FlashVersion string `json:"flash_version"`
}

type Container struct {
	Status    string    `json:"status"`
	ImageTags *[]string `json:"image_tags"`
}

type Geth struct {
	NetPeerCount   int64    `json:"net_peer_count"`
	EthSyncing     bool     `json:"eth_syncing"`
	EthBlockNumber int64    `json:"eth_block_number"`
	EthAccounts    []string `json:"eth_accounts"`
}

type Info struct {
	Geth         *Geth                `json:"geth"`
	Firmware     *Firmware            `json:"firmware"`
	Colonus      map[string]Container `json:"colonus"`
	Connectivity map[string]bool      `json:"connectivity"`
}

// "input" is flexible; could be a field name or other. Note: this is intended to be consumed by humans, either API consumers or developers of the UI. Add enum codes if these are to be evaluated in frontend code
type APIUserInputError struct {
	Error string `json:"error"`
	Input string `json:"input"`
}

func InputIsIllegal(str string) (string, error) {
	reg, err := regexp.Compile(ILLEGAL_INPUT_CHAR_REGEX)
	if err != nil {
		return "", fmt.Errorf("Unable to compile regex: %v, returning false for input check. Error: %v", ILLEGAL_INPUT_CHAR_REGEX, err)
	}

	maxLen := 32
	if reg.MatchString(str) {
		return fmt.Sprintf("Value violates regex illegal char match: %v", ILLEGAL_INPUT_CHAR_REGEX), nil
	} else if len([]byte(str)) > maxLen {
		return fmt.Sprintf("Value > max length: %v bytes", maxLen), nil
	}

	// a-ok!
	return "", nil
}

// returns: faulty value, msg, error
func MapInputIsIllegal(m map[string]string) (string, string, error) {
	for k, v := range m {
		if bogus, err := InputIsIllegal(k); err != nil || bogus != "" {
			return k, bogus, err
		}
		if bogus, err := InputIsIllegal(v); err != nil || bogus != "" {
			return fmt.Sprintf("%v: %v", k, v), bogus, err
		}
	}

	// all good
	return "", "", nil
}

func NewInfo(gethRunning bool) *Info {
	return &Info{
		Geth: &Geth{
			NetPeerCount:   -1,
			EthSyncing:     false,
			EthBlockNumber: -1,
		},
		Firmware: &Firmware{
			Definition:   "",
			FlashVersion: "",
		},
		Colonus:      map[string]Container{},
		Connectivity: map[string]bool{},
	}
}

type Account struct {
	Email *string `json:"email"`
}

func NewAPIListener(config *config.Config, db *bolt.DB) *API {
	messages := make(chan events.Message)

	listener := &API{
		Manager: worker.Manager{
			Config:   config,
			Messages: messages,
		},

		db: db,
	}

	listener.listen(config.APIListen)
	return listener
}

func (a *API) listen(apiListen string) {
	glog.Info("Starting Anax API server")

	nocache := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Add("Pragma", "no-cache, no-store")
			h.ServeHTTP(w, r)
		})
	}

	go func() {
		router := mux.NewRouter()

		router.HandleFunc("/agreement/{id}", a.agreement).Methods("DELETE")
		router.HandleFunc("/agreement/{id}/latestmicropayment", a.latestmicropayment)
		router.HandleFunc("/contract", a.contract)
		router.HandleFunc("/contract/names", a.contractNames)
		router.HandleFunc("/workload", a.workload)
		router.HandleFunc("/micropayment", a.micropayment)
		router.HandleFunc("/info", a.info)
		router.HandleFunc("/account", account)

		router.PathPrefix("/js/").Handler(http.StripPrefix("/js/", http.FileServer(http.Dir(path.Join(a.Config.StaticWebContent, "js")))))
		router.PathPrefix("/styles/").Handler(http.StripPrefix("/styles/", http.FileServer(http.Dir(path.Join(a.Config.StaticWebContent, "styles")))))
		router.PathPrefix("/images/").Handler(http.StripPrefix("/images/", http.FileServer(http.Dir(path.Join(a.Config.StaticWebContent, "images")))))

		// paths to pages
		router.PathPrefix("/status/").Handler(http.StripPrefix("/status/", http.FileServer(http.Dir(path.Join(a.Config.StaticWebContent, "status")))))
		router.PathPrefix("/registration/").Handler(http.StripPrefix("/registration/", http.FileServer(http.Dir(path.Join(a.Config.StaticWebContent, "registration")))))

		// redir root
		router.HandleFunc("/", a.redir)

		glog.Infof("Serving static web content from: %v", a.Config.StaticWebContent)
		http.ListenAndServe(apiListen, nocache(router))
	}()
}

func (a *API) redir(w http.ResponseWriter, r *http.Request) {
	reg := func() {
		http.Redirect(w, r, "/registration/", http.StatusTemporaryRedirect)
	}

	switch r.URL.String() {
	case "/":
		// redirect to status page if they've already registered, otherwise serve registration page
		if names, err := allContractNames(a.db); err != nil {
			glog.Error(err)
		} else if len(names) != 0 {
			glog.Infof("User has already registered, redirecting to status page")

			http.Redirect(w, r, "/registration/status.html", http.StatusTemporaryRedirect)
			return
		}
		reg()

	default:
		reg()
	}
}

func (a *API) info(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":

		info := NewInfo(false)

		if err := WriteGethStatus(a.Config.GethURL, info.Geth); err != nil {
			glog.Errorf("Unable to determine geth service facts: %v", err)
		}

		if err := WriteConnectionStatus(info); err != nil {
			glog.Errorf("Unable to get connectivity status: %v", err)
		}

		if serial, err := json.Marshal(info); err != nil {
			glog.Errorf("Failed to serialize status object: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")

			if _, err := w.Write(serial); err != nil {
				glog.Errorf("Failed to write response: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}

	case "OPTIONS":
		w.Header().Set("Allow", "OPTIONS, GET")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

func allContractNames(db *bolt.DB) ([]string, error) {
	if eContracts, err := persistence.FindEstablishedContracts(db, []persistence.ECFilter{}); err != nil {
		return nil, fmt.Errorf("Error fetching established contracts: %v", err)
	} else if pContracts, err := persistence.FindPendingContracts(db); err != nil {
		return nil, fmt.Errorf("Error fetching pending contracts: %v", err)
	} else {
		var names []string

		for _, eContract := range eContracts {
			names = append(names, eContract.Name)
		}

		for _, pContract := range pContracts {
			names = append(names, *pContract.Name)
		}

		return names, nil
	}
}

func (a *API) contractNames(w http.ResponseWriter, r *http.Request) {
	if names, err := allContractNames(a.db); err != nil {
		glog.Error(err)
		w.WriteHeader(http.StatusInternalServerError)

	} else {

		wrap := make(map[string][]string, 0)
		wrap["names"] = names

		if serial, err := json.Marshal(wrap); err != nil {
			glog.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")

			if _, err := w.Write(serial); err != nil {
				glog.Error(err)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}
}

func account(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		var account Account
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &account); err != nil {
			glog.Infof("User submitted data couldn't be deserialized to Account struct: %v. Error: %v", string(body), err)
			w.WriteHeader(http.StatusBadRequest)
		} else if account.Email != nil {
			// writing email address to disk

			// TODO: change to SNAP_USER_COMMON if this can be a multi-user thing
			if f, err := os.Create(path.Join(os.Getenv("SNAP_COMMON"), "contact")); err != nil {
				glog.Error(err)
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				defer f.Close()
				if _, err := f.WriteString(*account.Email); err != nil {
					glog.Errorf("Error writing account detail to fs: %v", err)
				}
			}
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) workload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if client, err := dockerclient.NewClient(a.Config.DockerEndpoint); err != nil {
			glog.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			opts := dockerclient.ListContainersOptions{
				All: true,
			}

			if containers, err := client.ListContainers(opts); err != nil {
				glog.Error(err)
				w.WriteHeader(http.StatusInternalServerError)
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
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					w.Header().Set("Content-Type", "application/json")
					if _, err := w.Write(serial); err != nil {
						glog.Error(err)
						w.WriteHeader(http.StatusInternalServerError)
					}
				}
			}
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

func (a *API) micropayment(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if micropayments, err := persistence.ActiveAgreementMicropayments(a.db); err != nil {
			glog.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {

			wrap := make(map[string][]persistence.Micropayments, 0)
			wrap["micropayments"] = micropayments

			if serial, err := json.Marshal(wrap); err != nil {
				glog.Error(err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {

				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(serial); err != nil {
					glog.Error(err)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

func (a *API) latestmicropayment(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		if paymentTime, paymentValue, _, err := persistence.LastMicropayment(a.db, id); err != nil {
			glog.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			var wrap persistence.LatestMicropayment
			wrap.PaymentTime = paymentTime
			wrap.PaymentValue = paymentValue
			wrap.AgreementId = id

			if serial, err := json.Marshal(wrap); err != nil {
				glog.Error(err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {

				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(serial); err != nil {
					glog.Error(err)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

func (a *API) agreement(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)
	id := pathVars["id"]

	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
	} else {
		glog.V(3).Infof("Handling DELETE of agreement: %v", r)

		filters := make([]persistence.ECFilter, 0)
		filters = append(filters, persistence.UnarchivedECFilter())
		filters = append(filters, persistence.AgreementECFilter(id))

		if contracts, err := persistence.FindEstablishedContracts(a.db, filters); err != nil {
			glog.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
		} else if len(contracts) == 0 {
			w.WriteHeader(http.StatusNotFound)
		} else {
			// write message
			ct := contracts[0]

			a.Messages <- governance.NewGovernanceCancelationMessage(events.CONTRACT_ENDED, events.CT_TERMINATED, ct.ContractAddress, ct.CurrentAgreementId, &ct.CurrentDeployment, ct.PreviousAgreements)
			w.WriteHeader(http.StatusOK)
		}
	}
}

func (a *API) contract(w http.ResponseWriter, r *http.Request) {

	writeInputErr := func(status int, inputErr *APIUserInputError) {
		if serial, err := json.Marshal(inputErr); err != nil {
			glog.Infof("Error serializing contract output: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(status)
			if _, err := w.Write(serial); err != nil {
				glog.Infof("Error writing response: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			w.Header().Set("Content-Type", "application/json")
		}
	}

	// TODO: refactor

	switch r.Method {
	case "GET":
		// really only for the purpose of determining if contracts were registered

		if contracts, err := persistence.FindEstablishedContracts(a.db, []persistence.ECFilter{}); err != nil {
			glog.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			wrap := make(map[string][]persistence.EstablishedContract, 0)
			wrap["contracts"] = contracts

			if serial, err := json.Marshal(wrap); err != nil {
				glog.Infof("Error serializing contract output: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(serial); err != nil {
					glog.Infof("Error writing response: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}
		}
	case "OPTIONS":
		w.Header().Set("Allow", "OPTIONS, POST, GET")
		w.WriteHeader(http.StatusOK)
	case "POST":
		// Check if it has internet connection
		if err := checkConnectivity(HORIZON_SERVERS[0]); err != nil {
			glog.Errorf("Cannot register the contract because this device does not have internet connection. %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		} else {
			var contract persistence.PendingContract

			body, _ := ioutil.ReadAll(r.Body) //slurp it up
			if err := json.Unmarshal(body, &contract); err != nil {
				glog.Infof("User submitted data that couldn't be deserialized to Pending Contract: %v. Error: %v", string(body), err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			nErrMsg := "null and must not be"

			// TODO: programmatically determine the field label strings when serialized given the struct's member name; at the time of this writing the field label is simply known to match those specified in persistence
			if contract.Name == nil {
				writeInputErr(http.StatusBadRequest, &APIUserInputError{Input: "name", Error: nErrMsg})
				return
			}
			if inputErr, err := InputIsIllegal(*contract.Name); err != nil {
				glog.Errorf("Failed to check input: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			} else if inputErr != "" {
				writeInputErr(http.StatusBadRequest, &APIUserInputError{Input: "name", Error: inputErr})
				return
			}
			if contract.RAM == nil {
				writeInputErr(http.StatusBadRequest, &APIUserInputError{Input: "ram", Error: nErrMsg})
				return
			}
			if contract.HourlyCostBacon == nil {
				writeInputErr(http.StatusBadRequest, &APIUserInputError{Input: "hourly_cost_bacon", Error: nErrMsg})
				return
			}
			if *contract.HourlyCostBacon < 60 {
				writeInputErr(http.StatusBadRequest, &APIUserInputError{Input: "hourly_cost_bacon", Error: "Value is < 60 and shouldn't be"})
				return
			}
			if contract.AppAttributes == nil {
				writeInputErr(http.StatusBadRequest, &APIUserInputError{Input: "app_attributes", Error: nErrMsg})
				return
			}
			if len(*contract.AppAttributes) == 0 {
				// TODO: expand to pick out required keys
				writeInputErr(http.StatusBadRequest, &APIUserInputError{Input: "", Error: "Object missing required keys"})
				return
			}
			if value, inputErr, err := MapInputIsIllegal(*contract.AppAttributes); err != nil {
				glog.Errorf("Failed to check input: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			} else if inputErr != "" {
				writeInputErr(http.StatusBadRequest, &APIUserInputError{Input: fmt.Sprintf("app_attributes.%v", value), Error: inputErr})
				return
			}
			if contract.PrivateAppAttributes == nil {
				writeInputErr(http.StatusBadRequest, &APIUserInputError{Input: "private_app_attributes", Error: nErrMsg})
				return
			}
			if value, inputErr, err := MapInputIsIllegal(*contract.PrivateAppAttributes); err != nil {
				glog.Errorf("Failed to check input: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			} else if inputErr != "" {
				writeInputErr(http.StatusBadRequest, &APIUserInputError{Input: fmt.Sprintf("private_app_attributes.%v", value), Error: inputErr})
				return
			}

			// input was ok!!

			if _, laset := (*contract.PrivateAppAttributes)["lat"]; laset {
				if _, loset := (*contract.PrivateAppAttributes)["lon"]; loset {
					contract.IsLocEnabled = true
				}
			}

			contract.Arch = runtime.GOARCH
			glog.V(2).Infof("Using discovered architecture tag: %v", contract.Arch)

			contract.CPUs = runtime.NumCPU()
			glog.V(2).Infof("Using discovered CPU count: %v", contract.CPUs)

			// TODO: move this into the persistence module
			duplicate := false

			// check for duplicate pending and established contracts
			pErr := a.db.View(func(tx *bolt.Tx) error {
				bp := tx.Bucket([]byte(persistence.P_CONTRACTS))
				if bp != nil {
					duplicate = (bp.Get([]byte(*contract.Name)) != nil)
				}

				return nil

			})

			nameFilter := func(name string) persistence.ECFilter {
				return func(e persistence.EstablishedContract) bool { return e.Name == name }
			}

			contracts, eErr := persistence.FindEstablishedContracts(a.db, []persistence.ECFilter{nameFilter(*contract.Name)})

			if pErr != nil || eErr != nil {
				// TODO: sloppy, fix
				glog.Errorf("Error checking duplicates of %v from db. Errors: %v %v", contract, pErr, eErr)
				w.WriteHeader(http.StatusInternalServerError)
			} else if len(contracts) != 0 || duplicate {
				writeInputErr(http.StatusConflict, &APIUserInputError{Input: *contract.Name, Error: "Duplicate record found in pending or established contracts"})
				return
			}

			writeErr := a.db.Update(func(tx *bolt.Tx) error {
				b, err := tx.CreateBucketIfNotExists([]byte(persistence.P_CONTRACTS))
				if err != nil {
					return err
				}

				if serial, err := json.Marshal(&contract); err != nil {
					glog.Errorf("Failed to serialize pending contract: %v. Error: %v", &contract, err)
				} else {
					return b.Put([]byte(*contract.Name), serial)
				}
				return nil // end tx
			})

			if writeErr != nil {
				glog.Errorf("Error storing contract (%v) to db. Error: %v", &contract, writeErr)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				glog.Infof("New pending contract: %v", &contract)
				w.WriteHeader(http.StatusCreated)
			}
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
