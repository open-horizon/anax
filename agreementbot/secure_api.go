// @APIVersion 1.0.0
// @APITitle Agreement Bot Secure API
// @APIDescription This is the secure API for the agreement bot.
// @BasePath https://host:port/
// @SubApi Deployment Check API [/deploycheck]

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
		httpClient: newHTTPClientFactory().NewHTTPClient(nil),
		name:       name,
		db:         db,
		em:         events.NewEventStateManager(),
		secretProvider:    s,
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

func (a *SecureAPI)	setCommonHeaders(w http.ResponseWriter) http.ResponseWriter {
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Pragma", "no-cache, no-store")
	w.Header().Add("Access-Control-Allow-Headers", "X-Requested-With, content-type, Authorization")
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
	return w
}

// This function sets up the agbot secure http server
func (a *SecureAPI) listen() {
	glog.Info("Starting AgreementBot SecureAPI server")

	// If there is no ir invalid Agbot config, we will terminate
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
		router.HandleFunc("/org/{org}/secrets/{vault-secret-name}", a.secrets).Methods("GET", "PUT", "POST", "DELETE", "OPTIONS")

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

// @Title policy_compatible
// @Description Check the policy compatibility. This API does the policy compatibility check for the given business policy, node policy and service policy. The business policy and the service policy will be merged to check against the node policy. If the result is compatible, it means that, when deployed, the node will form an agreement with the agbot and the service will be running on the node.
// @Accept  json
// @Produce json
// @Param   checkAll     		query    bool     false        "Return the compatibility check result for all the service versions referenced in the business policy or pattern."
// @Param   long         		query    bool     false        "Show the input which was used to come up with the result."
// @Param   node_id      		body     string   false        "The exchange id of the node. Mutually exclusive with node_policy."
// @Param   node_arch    		body     string   false        "The architecture of the node."
// @Param   node_policy  		body     externalpolicy.ExternalPolicy     false        "The node policy that will be put in the exchange. Mutually exclusive with node_id."
// @Param   business_policy_id  body     string   false        "The exchange id of the business policy. Mutually exclusive with business_policy."
// @Param   business_policy  	body     businesspolicy.BusinessPolicy     false        "The defintion of the business policy that will be put in the exchange. Mutually exclusive with business_policy_id."
// @Param   service_policy  	body     externalpolicy.ExternalPolicy     false        "The service policy that will be put in the exchange. They are for the top level service referenced in the business policy. If omitted, the service policy will be retrieved from the exchange. The service policy has the same format as the node policy."
// @Success 200 {object}  compcheck.CompCheckOutput
// @Failure 400 {object}  string      "No input found"
// @Failure 401 {object}  string      "Failed to authenticate"
// @Failure 500 {object}  string      "Error"
// @Resource /deploycheck
// @Router /deploycheck/policycompatible [get]
// This function does policy compatibility check.
func (a *SecureAPI) policy_compatible(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		glog.V(5).Infof(APIlogString(fmt.Sprintf("/deploycheck/policycompatible called.")))

		// check user cred
		if user_ec, msgPrinter, ok := a.processUserCred("/deploycheck/policycompatible", w, r); ok {
			body, _ := ioutil.ReadAll(r.Body)
			if len(body) == 0 {
				glog.Errorf(APIlogString(fmt.Sprintf("No input found.")))
				writeResponse(w, msgPrinter.Sprintf("No input found."), http.StatusBadRequest)
			} else if input, err := a.decodePolicyCheckBody(body, msgPrinter); err != nil {
				writeResponse(w, err.Error(), http.StatusBadRequest)
			} else {
				// if checkAll is set, then check all the services defined in the business policy for compatibility.
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

// @Title userinput_compatible
// @Description Check the user input compatibility. This API does the user input compatibility check for the given business policy (or a pattern), service definition and node user input. The user input values in the business policy and the node will be merged to check against the service uer input requirement defined in the service definition. If the result is compatible, it means that, when deployed, the node will form an agreement with the agbot and the service will be running on the node.
// @Accept  json
// @Produce json
// @Param   checkAll     		query    bool     false        "Return the compatibility check result for all the service versions referenced in the business policy or pattern."
// @Param   long         		query    bool     false        "Show the input which was used to come up with the result."
// @Param   node_id      		body     string   false        "The exchange id of the node. Mutually exclusive with node_user_input."
// @Param   node_arch    		body     string   false        "The architecture of the node."
// @Param   node_user_input  	body     policy.UserInput    				false        "The user input that will be put in the exchange for the services. Mutually exclusive with node_id."
// @Param   business_policy_id  body     string   false        "The exchange id of the business policy. Mutually exclusive with business_policy. Mutually exclusive with pattern_id and pattern."
// @Param   business_policy  	body     businesspolicy.BusinessPolicy     	false        "The defintion of the business policy that will be put in the exchange. Mutually exclusive with business_policy_id. Mutually exclusive with pattern_id and pattern."
// @Param   pattern_id      	body     string   false        "The exchange id of the pattern. Mutually exclusive with pattern. Mutually exclusive with business_policy_id and business_policy."
// @Param   pattern  			body     common.PatternFile     			false        "The pattern that will be put in the exchange. Mutually exclusive with pattern_id. Mutually exclusive with business_policy_id and business_policy."
// @Param   service  			body     common.ServiceFile    				false        "An array of the top level services that will be put in the exchange. They are refrenced in the business policy or pattern. If omitted, the services will be retrieved from the exchange."
// @Success 200 {object}  compcheck.CompCheckOutput
// @Failure 400 {object}  string      "No input found"
// @Failure 401 {object}  string      "Failed to authenticate"
// @Failure 500 {object}  string      "Error"
// @Resource /deploycheck
// @Router /deploycheck/userinputcompatible [get]
// This function does userinput compatibility check.
func (a *SecureAPI) userinput_compatible(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		glog.V(5).Infof(APIlogString(fmt.Sprintf("/deploycheck/userinputcompatible called.")))

		if user_ec, msgPrinter, ok := a.processUserCred("/deploycheck/userinputcompatible", w, r); ok {
			body, _ := ioutil.ReadAll(r.Body)
			if len(body) == 0 {
				glog.Errorf(APIlogString(fmt.Sprintf("No input found.")))
				writeResponse(w, msgPrinter.Sprintf("No input found."), http.StatusBadRequest)
			} else if input, err := a.decodeUserInputCheckBody(body, msgPrinter); err != nil {
				writeResponse(w, err.Error(), http.StatusBadRequest)
			} else {
				// if checkAll is set, then check all the services defined in the business policy for compatibility.
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

// @Title deploy_compatible
// @Description Check deployment compatibility. This API does compatibility check for the given business policy (or a pattern), service definition, node policy and node user input. It does both policy compatibility check and user input compatibility check. If the result is compatible, it means that, when deployed, the node will form an agreement with the agbot and the service will be running on the node.
// @Accept  json
// @Produce json
// @Param   checkAll     		query    bool     false        "Return the compatibility check result for all the service versions referenced in the business policy or pattern."
// @Param   long         		query    bool     false        "Show the input which was used to come up with the result."
// @Param   node_id      		body     string   false        "The exchange id of the node. Mutually exclusive with node_policy and node_user_input."
// @Param   node_arch    		body     string   false        "The architecture of the node."
// @Param   node_policy  		body     externalpolicy.ExternalPolicy 	false        "The node policy that will be put in the exchange. Mutually exclusive with node_id."
// @Param   node_user_input  	body     policy.UserInput       		false        "The user input that will be put in the exchange for the services. Mutually exclusive with node_id."
// @Param   business_policy_id  body     string   false        "The exchange id of the business policy. Mutually exclusive with business_policy. Mutually exclusive with pattern_id and pattern."
// @Param   business_policy  	body     businesspolicy.BusinessPolicy  false        "The defintion of the business policy that will be put in the exchange. Mutually exclusive with business_policy_id. Mutually exclusive with pattern_id and pattern."
// @Param   pattern_id      	body     string   false        "The exchange id of the pattern. Mutually exclusive with pattern. Mutually exclusive with business_policy_id and business_policy."
// @Param   pattern  			body     common.PatternFile      		false        "The pattern that will be put in the exchange. Mutually exclusive with pattern_id. Mutually exclusive with business_policy_id and business_policy."
// @Param   service_policy  	body     externalpolicy.ExternalPolicy 	false        "The service policy that will be put in the exchange. They are for the top level service referenced in the business policy. If omitted, the service policy will be retrieved from the exchange. The service policy has the same format as the node policy."
// @Param   service  			body     common.ServiceFile     		false        "An array of the top level services that will be put in the exchange. They are refrenced in the business policy or pattern. If omitted, the services will be retrieved from the exchange."
// @Success 200 {object}  compcheck.CompCheckOutput
// @Failure 400 {object}  string      "No input found"
// @Failure 401 {object}  string      "Failed to authenticate"
// @Failure 500 {object}  string      "Error"
// @Resource /deploycheck
// @Router /deploycheck/deploycompatible [get]
// This function does policy and userinput compatibility check.
func (a *SecureAPI) deploy_compatible(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		glog.V(5).Infof(APIlogString(fmt.Sprintf("/deploycheck/deploycompatible called.")))

		if user_ec, msgPrinter, ok := a.processUserCred("/deploycheck/deploycompatible", w, r); ok {
			body, _ := ioutil.ReadAll(r.Body)
			if len(body) == 0 {
				glog.Errorf(APIlogString(fmt.Sprintf("No input found.")))
				writeResponse(w, msgPrinter.Sprintf("No input found."), http.StatusBadRequest)
			} else if input, err := a.decodeCompCheckBody(body, msgPrinter); err != nil {
				writeResponse(w, err.Error(), http.StatusBadRequest)
			} else {
				// if checkAll is set, then check all the services defined in the business policy for compatibility.
				checkAll := r.URL.Query().Get("checkAll")

				// do user input compatibility check
				output, err := compcheck.DeployCompatible(user_ec, input, (checkAll != ""), msgPrinter)

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

// This function checks user cred and writes corrsponding response. It also creates a message printer with given language from the http request.
func (a *SecureAPI) processUserCred(resource string, w http.ResponseWriter, r *http.Request) (exchange.ExchangeContext, *message.Printer, bool) {
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
		return nil, nil, false
	} else if user_ec, err := a.authenticateWithExchange(userId, userPasswd, msgPrinter); err != nil {
		glog.Errorf(APIlogString(fmt.Sprintf("Failed to authenticate user %v with the Exchange. %v", userId, err)))
		writeResponse(w, msgPrinter.Sprintf("Failed to authenticate the user with the Exchange. %v", err), http.StatusUnauthorized)
		return nil, nil, false
	} else {
		return user_ec, msgPrinter, true
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
func (a *SecureAPI) authenticateWithExchange(user string, userPasswd string, msgPrinter *message.Printer) (exchange.ExchangeContext, error) {
	glog.V(5).Infof(APIlogString(fmt.Sprintf("authenticateWithExchange called with user %v", user)))

	orgId, userId := cutil.SplitOrgSpecUrl(user)
	if userId == "" {
		return nil, fmt.Errorf(msgPrinter.Sprintf("No exchange user id is supplied."))
	} else if orgId == "" {
		return nil, fmt.Errorf(msgPrinter.Sprintf("No exchange user organization id is supplied."))
	} else if userPasswd == "" {
		return nil, fmt.Errorf(msgPrinter.Sprintf("No exchange user password or api key is supplied."))
	}

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
				return nil, fmt.Errorf(msgPrinter.Sprintf("Wrong organization id, user id or password."))
			} else {
				return nil, err
			}
		} else if tpErr != nil {
			glog.Warningf(APIlogString(tpErr.Error()))

			if retryCount <= 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", user_ec.GetHTTPFactory().RetryCount, tpErr)
			}
			time.Sleep(time.Duration(retryInterval) * time.Second)
			continue
		} else {
			return user_ec, nil
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

// Handles secret fetch, updates and delete from the vault API
// org - This url sub-path dictates where the secret exists within the vault
// vault-secret-name - The actual secret name used in the secret bindings
func (a *SecureAPI) secrets(w http.ResponseWriter, r *http.Request) {
	// get message printer with the language passed in from the header
	lan := r.Header.Get("Accept-Language")
	if lan == "" {
		lan = i18n.DEFAULT_LANGUAGE
	}
	msgPrinter := i18n.GetMessagePrinterWithLocale(lan)

	// Check if vault is configured in the management hub
	if a.Config.AgreementBot.Vault.VaultURL == "" {
		glog.Errorf(APIlogString("There is no vault component in the management hub."))
		writeResponse(w, msgPrinter.Sprintf("There is no vault component in the management hub. %v", a.Config.IsVaultConfigured()), http.StatusServiceUnavailable)
		return
	}

	pathVars := mux.Vars(r)
	org := pathVars["org"]
	vaultSecretName := pathVars["vault-secret-name"]
	glog.V(5).Infof(APIlogString(fmt.Sprintf("/org/%v/secrets/%v called.", org, vaultSecretName)))

	// handle secret API options
	switch r.Method {
	case "GET":
		if vaultToken, ok := a.processVaultUserCred(fmt.Sprintf("/org/%v/secrets/%v", org, vaultSecretName), msgPrinter, w, r); ok {
			targetURL := fmt.Sprintf("%v/org/%v/secrets/%v", a.Config.GetAgbotVaultURL(), org, vaultSecretName)
			// Replace with a call to invoke the vault API at targetURL with the vaultToken generated from authentication. Returns response body & code
			if _, respCode, err := fmt.Sprintf("Dummy response - Token:%v, url:%v", vaultToken, targetURL), http.StatusOK, error(nil); err != nil {
				glog.Errorf(APIlogString(fmt.Sprintf("Vault invocation failure. The caller should retry this API call a small number of times with a short delay between calls to ensure that the vault is really not there. %v.", err)))
				writeResponse(w, msgPrinter.Sprintf("Vault invovation failure. The caller should retry this API call a small number of times with a short delay between calls to ensure that the vault is really not there. %v.", err), http.StatusServiceUnavailable)
			} else if respCode == http.StatusNotFound {
				glog.Infof(APIlogString("Secret does not exist."))
				writeResponse(w, msgPrinter.Sprintf("Secret does not exist."), http.StatusNotFound)
			} else if respCode == http.StatusOK {
				glog.Infof(APIlogString("Secret exists."))
				writeResponse(w, map[string]bool{"exists": true}, http.StatusOK)
			}
		}
	case "PUT":
		fallthrough
	case "POST":
		if vaultToken, ok := a.processVaultUserCred(fmt.Sprintf("/org/%v/secrets/%v", org, vaultSecretName), msgPrinter, w, r); ok {
			targetURL := fmt.Sprintf("%v/org/%v/secrets/%v", a.Config.GetAgbotVaultURL(), org, vaultSecretName)
			// Replace with a call to invoke the vault API at targetURL with the vault token generated from authentication
			if _, respCode, err := fmt.Sprintf("Token:%v, url:%v", vaultToken, targetURL), http.StatusCreated, error(nil); err != nil {
				glog.Errorf(APIlogString(fmt.Sprintf("Vault invocation failure. The caller should retry this API call a small number of times with a short delay between calls to ensure that the vault is really not there. %v.", err)))
				writeResponse(w, msgPrinter.Sprintf("Vault invovation failure. The caller should retry this API call a small number of times with a short delay between calls to ensure that the vault is really not there. %v.", err), http.StatusServiceUnavailable)
			} else if respCode == http.StatusCreated {
				// POST application logic goes here
				glog.Infof(APIlogString("Secret created/updated."))
				writeResponse(w, map[string]string{"name": "secret-name", "secret": "secret"}, http.StatusCreated)
			}
		}
	case "DELETE":
		if vaultToken, ok := a.processVaultUserCred(fmt.Sprintf("/org/%v/secrets/%v", org, vaultSecretName), msgPrinter, w, r); ok {
			targetURL := fmt.Sprintf("%v/org/%v/secrets/%v", a.Config.GetAgbotVaultURL(), org, vaultSecretName)
			// Replace with a call to invoke the vault API at targetURL with the vaultToken generated from authentication
			if _, respCode, err := fmt.Sprintf("Token:%v, url:%v", vaultToken, targetURL), http.StatusNoContent, error(nil); err != nil {
				glog.Errorf(APIlogString(fmt.Sprintf("Vault invocation failure. The caller should retry this API call a small number of times with a short delay between calls to ensure that the vault is really not there. %v.", err)))
				writeResponse(w, msgPrinter.Sprintf("Vault invovation failure. The caller should retry this API call a small number of times with a short delay between calls to ensure that the vault is really not there. %v.", err), http.StatusServiceUnavailable)
			} else if respCode == http.StatusNotFound {
				glog.Infof(APIlogString("Secret does not exist."))
				writeResponse(w, msgPrinter.Sprintf("Secret does not exist."), http.StatusNotFound)
			} else if respCode == http.StatusNoContent {
				// DELETE application logic goes here
				glog.Infof(APIlogString("Secret is deleted."))
				writeResponse(w, "Secret is deleted.", http.StatusNoContent)
			}
		}
	case "OPTIONS":
		w.Header().Set("Allow", "GET, PUT, POST, DELETE, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *SecureAPI) processVaultUserCred(resource string, msgPrinter *message.Printer, w http.ResponseWriter, r *http.Request) (string, bool) {
	// check caller user credentials
	userID, userPasswd, ok := r.BasicAuth()

	// extract user org and secret org from API path
	userOrg, _ := cutil.SplitOrgSpecUrl(userID)
	pathVars := mux.Vars(r)
	secretOrg := pathVars["org"]

	if !ok {
		glog.Errorf(APIlogString(fmt.Sprintf("%v is called without vault authentication.", resource)))
		writeResponse(w, msgPrinter.Sprintf("Unauthorized. No vault user id is supplied."), http.StatusUnauthorized)
		return "", false
	} else if _, err := a.authenticateWithExchange(userID, userPasswd, msgPrinter); err != nil {
		glog.Errorf(APIlogString(fmt.Sprintf("Failed to authenticate user %v with the Exchange. %v", userID, err)))
		writeResponse(w, msgPrinter.Sprintf("Failed to authenticate the user with the Exchange. %v", err), http.StatusUnauthorized)
		return "", false
	} else if tokenID, err := a.authenticateWithVault(userID, userPasswd, msgPrinter); err != nil && userOrg != secretOrg {
		// The vault api returns same respCode for invalid credentials and forbidden access to resources
		// Hence upon failed authentication unmatching user orgs and secret orgs is a possible casue for failure
		glog.Errorf(APIlogString(fmt.Sprintf("Failed to authenticate user %v with the vault. User not in org specified on secret API path.", userID)))
		writeResponse(w, msgPrinter.Sprintf("Failed to authenticate the user with the vault. User not in org specified on secret API path."), http.StatusForbidden)
		return "", false
	} else if err != nil {
		glog.Errorf(APIlogString(fmt.Sprintf("Failed to authenticate user %v with the vault. %v", userID, err)))
		writeResponse(w, msgPrinter.Sprintf("Failed to authenticate the user with the vault. %v", err), http.StatusUnauthorized)
		return "", false
	} else {
		return tokenID, true
	}
}

// Possibly replace token returned as a part of a vault context
func (a *SecureAPI) authenticateWithVault(user string, userPasswd string, msgPrinter *message.Printer) (string, error) {
	glog.V(5).Infof(APIlogString(fmt.Sprintf("authenticateWithVault called with user %v", user)))

	orgID, userID := cutil.SplitOrgSpecUrl(user)
	if userID == "" {
		return "", fmt.Errorf(msgPrinter.Sprintf("No vault user id is supplied."))
	} else if orgID == "" {
		return "", fmt.Errorf(msgPrinter.Sprintf("No vault user org id is supplied."))
	} else if userPasswd == "" {
		return "", fmt.Errorf(msgPrinter.Sprintf("No vault user password is supplied."))
	}

	// Make the API call to authentication plugin for vault
	targetURL := fmt.Sprintf("%v/auth/openhorizon/login", a.Config.AgreementBot.Vault.VaultURL)
	if _, respCode, err := fmt.Sprintf("URL:%v", targetURL), http.StatusOK, error(nil); err != nil {
		return "", fmt.Errorf(msgPrinter.Sprintf("Vault invocation failure. %v"), err)
	} else if respCode == http.StatusOK {
		// extract token from response body
		return "tokenID", nil
	} else {
		return "", nil
	}
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
