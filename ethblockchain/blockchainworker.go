package ethblockchain

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// must be safely-constructed!!
type EthBlockchainWorker struct {
	worker.Worker       // embedded field
	httpClient          *http.Client
	gethURL             string
	bc                  *BaseContracts
	el                  *Event_Log
	ethContainerStarted bool
	ethContainerLoaded  bool
	exchangeURL         string
	exchangeId          string
	exchangeToken       string
	horizonPubKeyFile   string
}

func NewEthBlockchainWorker(config *config.HorizonConfig, gethURL string) *EthBlockchainWorker {
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

		httpClient:          &http.Client{},
		gethURL:             gethURL,
		ethContainerStarted: false,
		ethContainerLoaded:  false,
		horizonPubKeyFile:   config.Edge.PublicKeyPath,
	}

	glog.Info(logString("starting worker"))
	worker.start()
	return worker
}

func (w *EthBlockchainWorker) Messages() chan events.Message {
	return w.Worker.Manager.Messages
}

func (w *EthBlockchainWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.NewEthContainerMessage:
		msg, _ := incoming.(*events.NewEthContainerMessage)
		cmd := NewNewClientCommand(*msg)
		w.Commands <- cmd

	case *events.ContainerMessage:
		msg, _ := incoming.(*events.ContainerMessage)
		switch msg.Event().Id {
		case events.EXECUTION_FAILED:
			noBCCOnfig := events.BlockchainConfig{}
			if msg.LaunchContext.Blockchain != noBCCOnfig && msg.LaunchContext.Blockchain.Type == "ethereum" {
				w.ethContainerStarted = false
				// fake up a new eth container message to restart the process of loading the eth container
				newMsg := events.NewNewEthContainerMessage(events.NEW_ETH_CLIENT, w.exchangeURL, w.exchangeId, w.exchangeToken)
				cmd := NewNewClientCommand(*newMsg)
				w.Commands <- cmd
			}
		}

	case *events.TorrentMessage:
		msg, _ := incoming.(*events.TorrentMessage)
		switch msg.Event().Id {
		case events.TORRENT_FAILURE:
			noBCCOnfig := events.BlockchainConfig{}

			switch msg.LaunchContext.(type) {
			case *events.ContainerLaunchContext:
				lc := msg.LaunchContext.(*events.ContainerLaunchContext)
				if lc.Blockchain != noBCCOnfig && lc.Blockchain.Type == "ethereum" {
					w.ethContainerStarted = false
					// fake up a new eth container message to restart the process of loading the eth container
					newMsg := events.NewNewEthContainerMessage(events.NEW_ETH_CLIENT, w.exchangeURL, w.exchangeId, w.exchangeToken)
					cmd := NewNewClientCommand(*newMsg)
					w.Commands <- cmd
				}
			default:
				glog.Errorf(logString(fmt.Sprintf("unknown LaunchContext type: %T", msg.LaunchContext)))
			}
		}
	default: //nothing
	}

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
				case *NewClientCommand:
					cmd := command.(*NewClientCommand)
					w.handleNewClient(cmd)

				default:
					glog.Errorf(logString(fmt.Sprintf("unknown command (%T): %v", command, command)))
				}
				glog.V(5).Infof(logString(fmt.Sprintf("handled command")))

			case <-time.After(time.Duration(nonBlockDuration) * time.Second):

				// Make sure we are trying to start the container
				if !w.ethContainerStarted {
					w.ethContainerStarted = true
					if err := w.getEthContainer(); err != nil {
						w.ethContainerStarted = false
						glog.Errorf(logString(fmt.Sprintf("unable to start Eth container, error %v", err)))
					}
				}

				// Check status of blockchain
				if dirAddr, err := DirectoryAddress(); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to obtain directory address, error %v", err)))
				} else if acct, err := AccountId(); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to obtain account, error %v", err)))
				} else if funded, err := AccountFunded(w.gethURL); err != nil {
					glog.V(3).Infof(logString(fmt.Sprintf("error checking for account funding: %v", err)))
				} else {
					glog.V(3).Infof(logString(fmt.Sprintf("using directory address: %v", dirAddr)))
					if !notifiedBCReady {
						// geth initilzed
						notifiedBCReady = true
						glog.V(3).Infof(logString(fmt.Sprintf("sending blockchain client initialized event")))
						w.initBlockchainEventListener()
						w.Messages() <- events.NewBlockchainClientInitializedMessage(events.BC_CLIENT_INITIALIZED)
					}

					if !funded {
						glog.V(3).Infof(logString(fmt.Sprintf("account %v not funded yet", acct)))
					} else if funded && !notifiedFunded {
						notifiedFunded = true
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

func (w *EthBlockchainWorker) handleNewClient(cmd *NewClientCommand) {
	// Start the eth container if necessary
	if !w.ethContainerStarted {
		w.ethContainerStarted = true
		w.exchangeURL = cmd.Msg.ExchangeURL()
		w.exchangeId = cmd.Msg.ExchangeId()
		w.exchangeToken = cmd.Msg.ExchangeToken()

		if err := w.getEthContainer(); err != nil {
			w.ethContainerStarted = false
			glog.Errorf(logString(fmt.Sprintf("unable to start Eth container, error %v", err)))
		}
	} else {
		glog.V(3).Infof(logString(fmt.Sprintf("ignoring duplicate request to start eth container")))
	}
}

// This function is used to start the process of starting the ethereum container
func (w *EthBlockchainWorker) getEthContainer() error {

	// Get blockchain metadata from the exchange and tell the eth worker to start the ethereum client container
	chainName := "bluehorizon"
	if overrideName := os.Getenv("CMTN_BLOCKCHAIN"); overrideName != "" {
		chainName = overrideName
	}

	if bcMetadata, err := exchange.GetEthereumClient(w.exchangeURL, chainName, w.exchangeId, w.exchangeToken); err != nil {
		return errors.New(logString(fmt.Sprintf("unable to get eth client metadata, error: %v", err)))
	} else {

		// Convert the metadata into a container config object so that the Torrent worker can download the container.
		detailsObj := new(exchange.BlockchainDetails)
		if err := json.Unmarshal([]byte(bcMetadata), detailsObj); err != nil {
			return errors.New(logString(fmt.Sprintf("could not unmarshal blockchain metadata, error %v, metadata %v", err, bcMetadata)))
		} else {
			// Search for the architecture we're running on
			fired := false
			for _, chain := range detailsObj.Chains {
				if chain.Arch == runtime.GOARCH {
					if err := w.fireStartEvent(&chain, chainName); err != nil {
						return err
					}
					fired = true
					break
				}
			}
			if !fired {
				return errors.New(logString(fmt.Sprintf("could not locate eth metadata for %v", runtime.GOARCH)))
			}
		}
		return nil
	}
}

func (w *EthBlockchainWorker) fireStartEvent(details *exchange.ChainDetails, chainName string) error {
	if url, err := url.Parse(details.DeploymentDesc.Torrent.Url); err != nil {
		return errors.New(logString(fmt.Sprintf("ill-formed URL: %v, error %v", details.DeploymentDesc.Torrent.Url, err)))
	} else {
		hashes := make(map[string]string, 0)
		signatures := make(map[string]string, 0)

		for _, image := range details.DeploymentDesc.Torrent.Images {
			bits := strings.Split(image.File, ".")
			if len(bits) < 2 {
				return errors.New(logString(fmt.Sprintf("found ill-formed image filename: %v, no file suffix found", bits)))
			} else {
				hashes[image.File] = bits[0]
			}
			signatures[image.File] = image.Signature
		}

		// Verify the deployment signature
		if err := details.DeploymentDesc.HasValidSignature(w.horizonPubKeyFile); err != nil {
			return errors.New(logString(fmt.Sprintf("eth container has invalid deployment signature %v for %v", details.DeploymentDesc.DeploymentSignature, details.DeploymentDesc.Deployment)))
		}

		// Fire an event to the torrent worker so that it will download the container
		cc := events.NewContainerConfig(*url, hashes, signatures, details.DeploymentDesc.Deployment, details.DeploymentDesc.DeploymentSignature, details.DeploymentDesc.DeploymentUserInfo)
		envAdds := make(map[string]string)
		envAdds["COLONUS_DIR"] = "/root/eth"
		// envAdds["ETHEREUM_DIR"] = "/root/.ethereum"
		envAdds["HZN_RAM"] = "2048"
		lc := events.NewContainerLaunchContext(cc, &envAdds, events.BlockchainConfig{Type:"ethereum", Name:chainName})
		w.Worker.Manager.Messages <- events.NewLoadContainerMessage(events.LOAD_CONTAINER, lc)

		return nil
	}
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
			glog.V(3).Info(logString(fmt.Sprintf("found event: %v", rawEvent)))
			w.Messages() <- events.NewEthBlockchainEventMessage(events.BC_EVENT, rawEvent, policy.CitizenScientist)
		}
	}
}

func getFilter() []interface{} {
	filter := []interface{}{}
	return filter
}

type NewClientCommand struct {
	Msg events.NewEthContainerMessage
}

func NewNewClientCommand(msg events.NewEthContainerMessage) *NewClientCommand {
	return &NewClientCommand{
		Msg: msg,
	}
}



// ==========================================================================================================
// Utility functions

var logString = func(v interface{}) string {
	return fmt.Sprintf("EthBlockchainWorker %v", v)
}
