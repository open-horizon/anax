package ethblockchain

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"
)

// must be safely-constructed!!
type EthBlockchainWorker struct {
	worker.Worker // embedded field
	httpClient    *http.Client
	gethURL       string
	bcMetadata    *policy.EthereumBlockchain
	bc            *BaseContracts
	el            *Event_Log
}

func NewEthBlockchainWorker(config *config.HorizonConfig, gethURL string, bcMetadata *policy.EthereumBlockchain) *EthBlockchainWorker {
	messages := make(chan events.Message)      // The channel for outbound messages to the anax wide bus
	commands := make(chan worker.Command, 100) // The channel for commands into the agreement bot worker

	worker := &EthBlockchainWorker{
		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},

		httpClient: &http.Client{},
		gethURL:    gethURL,
		bcMetadata: bcMetadata,
	}

	glog.Info(logString("starting worker"))
	worker.start()
	return worker
}

func (w *EthBlockchainWorker) Messages() chan events.Message {
	return w.Worker.Manager.Messages
}

func (w *EthBlockchainWorker) NewEvent(incoming events.Message) {
	return
}

func (w *EthBlockchainWorker) start() {
	glog.Info(logString("worker started"))

	go func() {

		notifiedBCReady := false
		notifiedFunded := false
		nonBlockDuration := 15

		for {
			glog.V(5).Infof(logString(fmt.Sprintf("about to select command (non-blocking)")))

			select {
			case command := <-w.Commands:
				switch command.(type) {
				default:
					glog.Errorf(logString(fmt.Sprintf("unknown command (%T): %v", command, command)))
				}
				glog.V(5).Infof(logString(fmt.Sprintf("handled command")))

			case <-time.After(time.Duration(nonBlockDuration) * time.Second):
				// Check status of blockchain
				if acct, err := AccountId(); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to obtain account, error %v", err)))
				} else if funded, err := AccountFunded(w.gethURL); err != nil {
					glog.V(3).Infof(logString(fmt.Sprintf("error checking for account funding: %v", err)))
				} else {
					if !notifiedBCReady {
						// geth initilzed
						notifiedBCReady = true
						glog.V(3).Infof(logString(fmt.Sprintf("sending blockchian client initialized event")))
						w.initBlockchainEventListener()
						w.Messages() <- events.NewBlockchainClientInitializedMessage(events.BC_CLIENT_INITIALIZED)
					}

					if !funded {
						glog.V(3).Infof(logString(fmt.Sprintf("account %v not funded yet", acct)))
					} else if funded && !notifiedFunded {
						notifiedFunded = true
						nonBlockDuration = 60
						glog.V(3).Infof(logString(fmt.Sprintf("sending acct %v funded event", acct)))
						w.Messages() <- events.NewAccountFundedMessage(events.ACCOUNT_FUNDED, acct)
					} else if funded {
						glog.V(3).Infof(logString(fmt.Sprintf("%v still funded", acct)))
					}
				}

				// Get new blockchain events and publish them to the rest of anax.
				if w.el != nil {
					if events, _, err := w.el.Get_Next_Raw_Event_Batch(getFilter(), 0); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to get event batch, error %v", err)))
						return
					} else {
						w.handleEvents(events)
					}
				}

			}

			runtime.Gosched()
		}

	}()

	glog.Info(logString("ready for commands."))
}

// This function sets up the blockchain event listener
func (w *EthBlockchainWorker) initBlockchainEventListener() {

	// Establish the go objects that are used to interact with the ethereum blockchain.
	acct, _ := AccountId()
	dir, _ := DirectoryAddress()
	gethURL := w.Worker.Manager.Config.Edge.GethURL
	if gethURL == "" {
		gethURL = w.Worker.Manager.Config.AgreementBot.GethURL
	}

	if bc, err := InitBaseContracts(acct, gethURL, dir); err != nil {
		glog.Errorf(logString(fmt.Sprintf("unable to initialize platform contracts, error: %v", err)))
		return
	} else {
		w.bc = bc
	}

	// Establish the event logger that will be used to listen for blockchain events
	if conn := RPC_Connection_Factory("", 0, gethURL); conn == nil {
		glog.Errorf(logString(fmt.Sprintf("unable to create connection")))
		return
	} else if rpc := RPC_Client_Factory(conn); rpc == nil {
		glog.Errorf(logString(fmt.Sprintf("unable to create RPC client")))
		return
	} else if el := Event_Log_Factory(rpc, w.bc.Agreements.Get_contract_address()); el == nil {
		glog.Errorf(logString(fmt.Sprintf("unable to create blockchain event log")))
		return
	} else {
		w.el = el

		// Set the starting block for the event logger. We will ignore events before this block.
		// Assume that anax will sync it's state with the blockchain by calling methods on the
		// relevant smart contracts, not depending on this logger to publish events from the past.
		block_read_delay := 0
		if rd, err := strconv.Atoi(os.Getenv("mtn_soliditycontract_block_read_delay")); err == nil {
			block_read_delay = rd
		}
		if block, err := rpc.Get_block_number(); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to get current block, error %v", err)))
			return
		} else if err := os.Setenv("bh_event_log_start", strconv.FormatUint(block - uint64(block_read_delay), 10)); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to set starting block, error %v", err)))
			return
		}

		// Grab the first bunch of events and process them. Put no limit on the batch size.
		if events, err := w.el.Get_Raw_Event_Batch(getFilter(), 0); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to get initial event batch, error %v", err)))
			return
		} else {
			w.handleEvents(events)
		}

	}
}

// Process each event in the list
func (w *EthBlockchainWorker) handleEvents(newEvents []Raw_Event) {
	for _, ev := range newEvents {
		if evBytes, err := json.Marshal(ev); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to marshal event %v, error %v", ev, err)))
		} else {
			rawEvent := string(evBytes)
			glog.V(5).Info(logString(fmt.Sprintf("found event: %v", rawEvent)))
			w.Messages() <- events.NewEthBlockchainEventMessage(events.BC_EVENT, rawEvent, policy.CitizenScientist)
		}
	}
}

func getFilter() []interface{} {
	filter := []interface{}{}
	return filter
}

// ==========================================================================================================
// Utility functions

var logString = func(v interface{}) string {
	return fmt.Sprintf("EthBlockchainWorker %v", v)
}
