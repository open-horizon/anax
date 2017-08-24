package ethblockchain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"golang.org/x/crypto/sha3"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const CHAIN_TYPE = "ethereum"

// This object holds the state of all BC instances that this worker is managing. It manages
// all blockchain instances of type 'ethereum'. Each of the fields in this object are
// specific to a given instance of a blockchain.
type BCInstanceState struct {
	bc             *BaseContracts
	el             *Event_Log
	started        bool // remains true when needsRestart is true so that messages to start the container are ignored until we are ready to start it
	needsRestart   bool
	notifiedReady  bool
	notifiedFunded bool
	name           string
	serviceName    string
	servicePort    string
	colonusDir     string
	metadataHash   []byte
}

// The worker is single threaded so there are no multi-thread concerns. Events that cause changes to instance state
// need to be dispatched to the worker thread as commands.
type EthBlockchainWorker struct {
	worker.Worker     // embedded field
	httpClient        *http.Client
	exchangeURL       string
	exchangeId        string
	exchangeToken     string
	horizonPubKeyFile string
	instances         map[string]*BCInstanceState
	neededBCs         map[string]uint64 // time stamp last time this BC was reported as needed
}

func NewEthBlockchainWorker(cfg *config.HorizonConfig) *EthBlockchainWorker {
	messages := make(chan events.Message)      // The channel for outbound messages to the anax wide bus
	commands := make(chan worker.Command, 100) // The channel for commands into the agreement bot worker

	worker := &EthBlockchainWorker{
		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   cfg,
				Messages: messages,
			},

			Commands: commands,
		},

		httpClient:        &http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT * time.Millisecond)},
		horizonPubKeyFile: cfg.Edge.PublicKeyPath,
		instances:         make(map[string]*BCInstanceState),
		neededBCs:         make(map[string]uint64),
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
	case *events.NewBCContainerMessage:
		msg, _ := incoming.(*events.NewBCContainerMessage)
		if msg.TypeName() == policy.Ethereum_bc {
			cmd := NewNewClientCommand(*msg)
			w.Commands <- cmd
		}

	case *events.ReportNeededBlockchainsMessage:
		msg, _ := incoming.(*events.ReportNeededBlockchainsMessage)
		if msg.BlockchainType() == policy.Ethereum_bc {
			cmd := NewReportNeededBlockchainsCommand(msg)
			w.Commands <- cmd
		}

	case *events.ContainerMessage:
		msg, _ := incoming.(*events.ContainerMessage)
		switch msg.Event().Id {
		case events.EXECUTION_FAILED:
			noBCConfig := events.BlockchainConfig{}
			if msg.LaunchContext.Blockchain != noBCConfig && msg.LaunchContext.Blockchain.Type == CHAIN_TYPE {
				cmd := NewContainerNotExecutingCommand(*msg)
				w.Commands <- cmd
			}

		case events.EXECUTION_BEGUN:
			noBCConfig := events.BlockchainConfig{}
			if msg.LaunchContext.Blockchain != noBCConfig && msg.LaunchContext.Blockchain.Type == CHAIN_TYPE {
				cmd := NewContainerExecutingCommand(*msg)
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
				if lc.Blockchain != noBCCOnfig && lc.Blockchain.Type == CHAIN_TYPE {
					cmd := NewTorrentFailureCommand(*msg)
					w.Commands <- cmd
				}
			default:
				glog.Warningf(logString(fmt.Sprintf("unknown LaunchContext type: %T", msg.LaunchContext)))
			}
		}

	case *events.ContainerShutdownMessage:
		msg, _ := incoming.(*events.ContainerShutdownMessage)
		switch msg.Event().Id {
		case events.CONTAINER_DESTROYED:
			cmd := NewContainerShutdownCommand(msg)
			w.Commands <- cmd
		}

	default: //nothing
	}

	return
}

func (w *EthBlockchainWorker) NewBCInstanceState(name string) *BCInstanceState {

	if _, ok := w.instances[name]; ok {
		return nil
	} else {
		i := new(BCInstanceState)
		i.name = name
		w.instances[name] = i
		return i
	}

}

func (w *EthBlockchainWorker) SetInstanceNotStarted(name string) {
	if _, ok := w.instances[name]; ok {
		w.instances[name].started = false
		w.instances[name].needsRestart = false
	}
}

func (w *EthBlockchainWorker) SetServiceStarted(name string, serviceName string, servicePort string) {
	if _, ok := w.instances[name]; ok {
		w.instances[name].serviceName = serviceName
		w.instances[name].servicePort = servicePort
	}
}

func (w *EthBlockchainWorker) SetColonusDir(name string, dir string) {
	if _, ok := w.instances[name]; ok {
		w.instances[name].colonusDir = dir
	}
}

func (w *EthBlockchainWorker) DeleteBCInstance(name string) {
	if _, ok := w.instances[name]; ok {
		delete(w.instances, name)
	}
}

func (w *EthBlockchainWorker) NeedContainer(name string) bool {
	if ts, ok := w.neededBCs[name]; ok {
		if ts == 0 || (uint64(time.Now().Unix()) <= (ts + uint64(300))) {
			return true
		} else {
			return false
		}
	}
	return true
}

func (w *EthBlockchainWorker) RestartContainer(cmd *ContainerShutdownCommand) {

	if !w.NeedContainer(cmd.Msg.ContainerName) {
		return
	}

	glog.V(5).Infof(logString(fmt.Sprintf("restarting %v", cmd.Msg.ContainerName)))

	if _, ok := w.instances[cmd.Msg.ContainerName]; ok {
		// Remove the old state from the last instance of the container
		i := new(BCInstanceState)
		i.name = cmd.Msg.ContainerName
		w.instances[cmd.Msg.ContainerName] = i

		// Create a new eth container message to begin the process of loading the eth container
		newMsg := events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, policy.Ethereum_bc, cmd.Msg.ContainerName, w.exchangeURL, w.exchangeId, w.exchangeToken)
		ncmd := NewNewClientCommand(*newMsg)
		w.Commands <- ncmd
	}
}

func (w *EthBlockchainWorker) UpdatedNeededBlockchains(cmd *ReportNeededBlockchainsCommand) {

	for name, _ := range cmd.Msg.NeededBlockchains() {
		w.neededBCs[name] = uint64(time.Now().Unix())
		glog.V(5).Infof(logString(fmt.Sprintf("blockchain %v is still needed", name)))
	}

}

func (w *EthBlockchainWorker) start() {
	glog.Info(logString("worker started"))

	go func() {

		nonBlockDuration := 15

		for {
			glog.V(5).Infof(logString(fmt.Sprintf("about to select command (non-blocking)")))

			select {
			case command := <-w.Commands:
				switch command.(type) {
				case *NewClientCommand:
					cmd := command.(*NewClientCommand)
					w.handleNewClient(cmd)

				case *ContainerExecutingCommand:
					cmd := command.(*ContainerExecutingCommand)
					w.SetServiceStarted(cmd.Msg.LaunchContext.Blockchain.Name, cmd.Msg.ServiceName, cmd.Msg.ServicePort)
					glog.V(3).Infof(logString(fmt.Sprintf("started service %v %v %v", cmd.Msg.LaunchContext.Blockchain.Name, cmd.Msg.ServiceName, cmd.Msg.ServicePort)))

				case *ContainerNotExecutingCommand:
					cmd := command.(*ContainerNotExecutingCommand)
					w.SetInstanceNotStarted(cmd.Msg.LaunchContext.Blockchain.Name)

					// fake up a new eth container message to restart the process of loading the eth container
					newMsg := events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, policy.Ethereum_bc, cmd.Msg.LaunchContext.Blockchain.Name, w.exchangeURL, w.exchangeId, w.exchangeToken)
					ncmd := NewNewClientCommand(*newMsg)
					w.Commands <- ncmd

				case *TorrentFailureCommand:
					cmd := command.(*TorrentFailureCommand)
					lc := cmd.Msg.LaunchContext.(*events.ContainerLaunchContext)
					w.SetInstanceNotStarted(lc.Blockchain.Name)

					// fake up a new eth container message to restart the process of loading the eth container
					newMsg := events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, policy.Ethereum_bc, lc.Blockchain.Name, w.exchangeURL, w.exchangeId, w.exchangeToken)
					ncmd := NewNewClientCommand(*newMsg)
					w.Commands <- ncmd

				case *ContainerShutdownCommand:
					cmd := command.(*ContainerShutdownCommand)
					w.RestartContainer(cmd)

				case *ReportNeededBlockchainsCommand:
					cmd := command.(*ReportNeededBlockchainsCommand)
					w.UpdatedNeededBlockchains(cmd)

				default:
					glog.Errorf(logString(fmt.Sprintf("unknown command (%T): %v", command, command)))
				}
				glog.V(5).Infof(logString(fmt.Sprintf("handled command %v", command)))

				// If all commands have been handled, give ther status check function a chance to run.
				if len(w.Commands) == 0 {
					w.CheckStatus()
				}

			case <-time.After(time.Duration(nonBlockDuration) * time.Second):
				w.CheckStatus()

			}

			runtime.Gosched()
		}

	}()

	glog.Info(logString("ready for commands."))
}

func (w *EthBlockchainWorker) CheckStatus() {

	glog.V(3).Infof(logString(fmt.Sprintf("checking blockchain status")))

	for name, bcState := range w.instances {

		// Check status of blockchain. If there is an anax filesystem for the BC client, then it means we have
		// gotten far enough to obtain the metadata for the chain and have attempted to start it. Now we can monitor
		// the progress of the container as it starts up.

		if !bcState.needsRestart {
			if bcState.colonusDir == "" {
				glog.V(5).Infof(logString(fmt.Sprintf("no %v eth client filesystem to read from yet", name)))
			} else if dirAddr, err := DirectoryAddress(bcState.colonusDir); err != nil {
				glog.Warningf(logString(fmt.Sprintf("unable to obtain directory address for %v, error %v", name, err)))
			} else if acct, err := AccountId(bcState.colonusDir); err != nil {
				glog.Warningf(logString(fmt.Sprintf("unable to obtain account for %v, error %v", name, err)))
			} else if bcState.serviceName == "" {
				glog.Warningf(logString(fmt.Sprintf("eth service not started yet for %v", name)))
			} else if funded, err := AccountFunded(bcState.colonusDir, fmt.Sprintf("http://%v:%v", bcState.serviceName, bcState.servicePort)); err != nil {
				// If the blockchain has been up before but this API is now failing, then we need to restart the container.
				if bcState.notifiedReady {

					glog.V(3).Infof(logString(fmt.Sprintf("detected %v API is down. Error was %v", name, err)))
					i := new(BCInstanceState)
					i.name = name
					w.instances[name] = i
					w.Messages() <- events.NewBlockchainClientStoppingMessage(events.BC_CLIENT_STOPPING, policy.Ethereum_bc, name)
					// If we dont need this container any more then dont restart it.
					if w.NeedContainer(name) {
						newMsg := events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, policy.Ethereum_bc, name, w.exchangeURL, w.exchangeId, w.exchangeToken)
						ncmd := NewNewClientCommand(*newMsg)
						w.Commands <- ncmd
					} else {
						glog.V(3).Infof(logString(fmt.Sprintf("not restarting, container %v is not needed any more", name)))
					}

				} else {
					glog.V(3).Infof(logString(fmt.Sprintf("error checking %v for account funding: %v", name, err)))
				}
			} else {
				glog.V(3).Infof(logString(fmt.Sprintf("%v using directory address: %v", name, dirAddr)))
				if !bcState.notifiedReady {
					// geth initialzed
					bcState.notifiedReady = true
					glog.V(3).Infof(logString(fmt.Sprintf("sending blockchain %v client initialized event", name)))
					w.Messages() <- events.NewBlockchainClientInitializedMessage(events.BC_CLIENT_INITIALIZED, policy.Ethereum_bc, name, bcState.serviceName, bcState.servicePort, bcState.colonusDir)
				}

				if !funded {
					glog.V(3).Infof(logString(fmt.Sprintf("account %v for %v not funded yet", acct, name)))
				} else if funded && !bcState.notifiedFunded {
					bcState.notifiedFunded = true
					glog.V(3).Infof(logString(fmt.Sprintf("sending acct %v funded event for %v", acct, name)))
					w.initBlockchainEventListener(name)
					w.Messages() <- events.NewAccountFundedMessage(events.ACCOUNT_FUNDED, acct, policy.Ethereum_bc, name, bcState.serviceName, bcState.servicePort, bcState.colonusDir)
				} else if funded {
					glog.V(3).Infof(logString(fmt.Sprintf("%v still funded for %v", acct, name)))
				}
			}
		}

		// Check to see if the blockchain def in the exchange has changed
		if !w.instances[name].needsRestart && w.instances[name].started && len(w.instances[name].metadataHash) != 0 {
			if bcMetadata, _, err := w.getBCMetadata(name); err == nil {
				hash := sha3.Sum256([]byte(bcMetadata))
				if !bytes.Equal(w.instances[name].metadataHash, hash[:]) {
					// BC metadata has changed, restart the container
					glog.V(3).Infof(logString(fmt.Sprintf("exchange metadata for %v has changed, restarting eth.", name)))

					w.instances[name].needsRestart = true
					w.Messages() <- events.NewBlockchainClientStoppingMessage(events.BC_CLIENT_STOPPING, policy.Ethereum_bc, name)
					w.Messages() <- events.NewContainerStopMessage(events.CONTAINER_STOPPING, name)

					// The next phase in the restart occurs after the shutdown message arrives back at this worker

				}
			}
		}

		// Get new blockchain events and publish them to the rest of anax.
		if w.instances[name].el != nil {
			if events, _, err := bcState.el.Get_Next_Raw_Event_Batch(getFilter(), 0); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to get event batch for %v, error %v", err, name)))
			} else {
				w.handleEvents(events, name)
			}
		}
	}
}

func (w *EthBlockchainWorker) handleNewClient(cmd *NewClientCommand) {

	// Grab the exchange metadata we need for all blockchain client requests.
	if w.exchangeURL == "" {
		w.exchangeURL = cmd.Msg.ExchangeURL()
		w.exchangeId = cmd.Msg.ExchangeId()
		w.exchangeToken = cmd.Msg.ExchangeToken()
	}

	// Make sure we are tracking this new instance.
	w.NewBCInstanceState(cmd.Msg.Instance())

	bcState := w.instances[cmd.Msg.Instance()]

	// Start the eth container if necessary. If it's already started then ignore the duplicate request.
	if !bcState.started {
		bcState.started = true

		if err := w.getEthContainer(cmd.Msg.Instance()); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to start Eth container %v, error %v", cmd.Msg.Instance(), err)))
			w.DeleteBCInstance(cmd.Msg.Instance())
		}

	} else {
		glog.V(3).Infof(logString(fmt.Sprintf("ignoring duplicate request to start eth container %v", cmd.Msg.Instance())))
	}

}

// This function is used to start the process of starting the ethereum container
func (w *EthBlockchainWorker) getEthContainer(name string) error {

	if bcMetadata, detailsObj, err := w.getBCMetadata(name); err != nil {
		return err
	} else {
		// Search for the architecture we're running on
		fired := false

		arch := runtime.GOARCH
		if strings.Contains(arch, "arm") {
			arch = "armhf"
		}

		for _, chain := range detailsObj.Chains {
			if chain.Arch == arch {
				if err := w.fireStartEvent(&chain, name); err != nil {
					return err
				}
				fired = true
				break
			}
		}
		if !fired {
			return errors.New(logString(fmt.Sprintf("could not locate eth metadata for %v", runtime.GOARCH)))
		} else {
			// Hash the metadata and save it.
			hash := sha3.Sum256([]byte(bcMetadata))
			w.instances[name].metadataHash = hash[:]
		}
	}
	return nil

}

func (w *EthBlockchainWorker) getBCMetadata(name string) (string, *exchange.BlockchainDetails, error) {

	// Get blockchain metadata from the exchange
	if bcMetadata, err := exchange.GetEthereumClient(w.exchangeURL, name, CHAIN_TYPE, w.exchangeId, w.exchangeToken); err != nil {
		return "", nil, errors.New(logString(fmt.Sprintf("unable to get eth client metadata, error: %v", err)))
	} else if len(bcMetadata) == 0 {
		glog.Errorf(logString(fmt.Sprintf("no metadata for container %v, giving up on it.", name)))
		return "", nil, errors.New(logString(fmt.Sprintf("blockchain not found")))
	} else {

		// Convert the metadata into a container config object so that the Torrent worker can download the container.
		detailsObj := new(exchange.BlockchainDetails)
		if err := json.Unmarshal([]byte(bcMetadata), detailsObj); err != nil {
			return "", nil, errors.New(logString(fmt.Sprintf("could not unmarshal blockchain metadata, error %v, metadata %v", err, bcMetadata)))
		} else {
			return bcMetadata, detailsObj, nil
		}
	}
}

func (w *EthBlockchainWorker) fireStartEvent(details *exchange.ChainDetails, name string) error {
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
		if err := details.DeploymentDesc.HasValidSignature(w.horizonPubKeyFile, w.Config.UserPublicKeyPath()); err != nil {
			return errors.New(logString(fmt.Sprintf("eth container has invalid deployment signature %v for %v", details.DeploymentDesc.DeploymentSignature, details.DeploymentDesc.Deployment)))
		}

		// Fire an event to the torrent worker so that it will download the container
		cc := events.NewContainerConfig(*url, hashes, signatures, details.DeploymentDesc.Deployment, details.DeploymentDesc.DeploymentSignature, details.DeploymentDesc.DeploymentUserInfo)
		envAdds := w.computeEnvVarsForContainer(details)
		w.SetColonusDir(name, envAdds["COLONUS_DIR"])
		lc := events.NewContainerLaunchContext(cc, &envAdds, events.BlockchainConfig{Type: CHAIN_TYPE, Name: name}, name)
		w.Worker.Manager.Messages <- events.NewLoadContainerMessage(events.LOAD_CONTAINER, lc)

		return nil
	}
}

func (w *EthBlockchainWorker) computeEnvVarsForContainer(details *exchange.ChainDetails) map[string]string {
	envAdds := make(map[string]string)

	// Make sure the vars that MUST be set are set.
	if ram := os.Getenv("CMTN_GETH_RAM_OVERRIDE"); ram == "" {
		envAdds["HZN_RAM"] = "192"
	} else {
		envAdds["HZN_RAM"] = ram
	}

	envAdds["COLONUS_DIR"] = getInstanceValue("COLONUS_DIR", details.Instance.ColonusDir)

	// If there are no instance details, then dont set any of these envvars.
	if details.Instance == (exchange.ChainInstance{}) {
		return envAdds
	}

	// Set env vars from the blockchain metadata details
	envAdds["BLOCKS_URLS"] = details.Instance.BlocksURLs
	envAdds["CHAINDATA_DIR"] = details.Instance.ChainDataDir
	envAdds["DISCOVERY_URLS"] = details.Instance.DiscoveryURLs
	envAdds["PORT"] = getInstanceValue("PORT", details.Instance.Port)
	envAdds["HOSTNAME"] = getInstanceValue("HOSTNAME", details.Instance.HostName)
	envAdds["IDENTITY"] = getInstanceValue("IDENTITY", details.Instance.Identity) + "-" + envAdds["HOSTNAME"]
	envAdds["KDF"] = getInstanceValue("KDF", details.Instance.KDF)
	envAdds["PING_HOST"] = details.Instance.PingHost
	envAdds["ETHEREUM_DIR"] = getInstanceValue("ETHEREUM_DIR", details.Instance.EthDir)
	envAdds["MAXPEERS"] = getInstanceValue("MAXPEERS", details.Instance.MaxPeers)
	envAdds["GETH_LOG"] = getInstanceValue("GETH_LOG", details.Instance.GethLog)

	return envAdds
}

func getInstanceValue(name string, value string) string {
	if value != "" {
		return value
	}

	res := ""
	switch name {
	case "PORT":
		res = "33303"
	case "HOSTNAME":
		hName, _ := os.Hostname()
		res = strings.Split(hName, ".")[0]
	case "IDENTITY":
		res = runtime.GOARCH
	case "KDF":
		res = "--lightkdf"
	case "COLONUS_DIR":
		res = "/root/eth"
	case "ETHEREUM_DIR":
		res = os.Getenv("HOME") + "/.ethereum"
	case "MAXPEERS":
		res = "12"
	case "GETH_LOG":
		res = "/tmp/geth.log"
	}
	return res
}

// This function sets up the blockchain event listener
func (w *EthBlockchainWorker) initBlockchainEventListener(name string) {

	bcState := w.instances[name]

	// Establish the go objects that are used to interact with the ethereum blockchain.
	acct, _ := AccountId(bcState.colonusDir)
	dir, _ := DirectoryAddress(bcState.colonusDir)
	gethURL := fmt.Sprintf("http://%v:%v", bcState.serviceName, bcState.servicePort)

	if bc, err := InitBaseContracts(acct, gethURL, dir); err != nil {
		glog.Errorf(logString(fmt.Sprintf("unable to initialize platform contracts, error: %v", err)))
		return
	} else {
		bcState.bc = bc
	}

	// Establish the event logger that will be used to listen for blockchain events
	if conn := RPC_Connection_Factory("", 0, gethURL); conn == nil {
		glog.Errorf(logString(fmt.Sprintf("unable to create connection")))
		return
	} else if rpc := RPC_Client_Factory(conn); rpc == nil {
		glog.Errorf(logString(fmt.Sprintf("unable to create RPC client")))
		return
	} else if el := Event_Log_Factory(rpc, bcState.bc.Agreements.Get_contract_address()); el == nil {
		glog.Errorf(logString(fmt.Sprintf("unable to create blockchain event log")))
		return
	} else {
		bcState.el = el

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
		} else if err := os.Setenv("bh_event_log_start", strconv.FormatUint(block-uint64(block_read_delay), 10)); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to set starting block, error %v", err)))
			return
		}

		// Grab the first bunch of events and process them. Put no limit on the batch size.
		if events, err := bcState.el.Get_Raw_Event_Batch(getFilter(), 0); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to get initial event batch, error %v", err)))
			return
		} else {
			w.handleEvents(events, name)
		}

	}
}

// Process each event in the list
func (w *EthBlockchainWorker) handleEvents(newEvents []Raw_Event, name string) {
	for _, ev := range newEvents {
		if evBytes, err := json.Marshal(ev); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to marshal event %v, error %v", ev, err)))
		} else {
			rawEvent := string(evBytes)
			glog.V(3).Info(logString(fmt.Sprintf("found event: %v", rawEvent)))
			w.Messages() <- events.NewEthBlockchainEventMessage(events.BC_EVENT, rawEvent, name, policy.CitizenScientist)
		}
	}
}

func getFilter() []interface{} {
	filter := []interface{}{}
	return filter
}

// ==========================================================================================================
type NewClientCommand struct {
	Msg events.NewBCContainerMessage
}

func (c NewClientCommand) ShortString() string {
	return c.Msg.ShortString()
}

func NewNewClientCommand(msg events.NewBCContainerMessage) *NewClientCommand {
	return &NewClientCommand{
		Msg: msg,
	}
}

type ContainerExecutingCommand struct {
	Msg events.ContainerMessage
}

func (c ContainerExecutingCommand) ShortString() string {
	return c.Msg.ShortString()
}

func NewContainerExecutingCommand(msg events.ContainerMessage) *ContainerExecutingCommand {
	return &ContainerExecutingCommand{
		Msg: msg,
	}
}

type ContainerNotExecutingCommand struct {
	Msg events.ContainerMessage
}

func (c ContainerNotExecutingCommand) ShortString() string {
	return c.Msg.ShortString()
}

func NewContainerNotExecutingCommand(msg events.ContainerMessage) *ContainerNotExecutingCommand {
	return &ContainerNotExecutingCommand{
		Msg: msg,
	}
}

type TorrentFailureCommand struct {
	Msg events.TorrentMessage
}

func (c TorrentFailureCommand) ShortString() string {
	return c.Msg.ShortString()
}

func NewTorrentFailureCommand(msg events.TorrentMessage) *TorrentFailureCommand {
	return &TorrentFailureCommand{
		Msg: msg,
	}
}

type ContainerShutdownCommand struct {
	Msg events.ContainerShutdownMessage
}

func (c ContainerShutdownCommand) ShortString() string {
	return c.Msg.ShortString()
}

func NewContainerShutdownCommand(msg *events.ContainerShutdownMessage) *ContainerShutdownCommand {
	return &ContainerShutdownCommand{
		Msg: *msg,
	}
}

type ReportNeededBlockchainsCommand struct {
	Msg events.ReportNeededBlockchainsMessage
}

func (c ReportNeededBlockchainsCommand) ShortString() string {
	return c.Msg.ShortString()
}

func NewReportNeededBlockchainsCommand(msg *events.ReportNeededBlockchainsMessage) *ReportNeededBlockchainsCommand {
	return &ReportNeededBlockchainsCommand{
		Msg: *msg,
	}
}

// ==========================================================================================================
// Utility functions

var logString = func(v interface{}) string {
	return fmt.Sprintf("EthBlockchainWorker %v", v)
}
