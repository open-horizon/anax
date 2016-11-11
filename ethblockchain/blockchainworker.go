package ethblockchain

import (
    "fmt"
    "github.com/golang/glog"
    "github.com/open-horizon/anax/config"
    "github.com/open-horizon/anax/events"
    "github.com/open-horizon/anax/policy"
    "github.com/open-horizon/anax/worker"
    "net/http"
    "runtime"
    "time"
)


// must be safely-constructed!!
type EthBlockchainWorker struct {
    worker.Worker // embedded field
    httpClient    *http.Client
    gethURL       string
    bc            *policy.EthereumBlockchain
}

func NewEthBlockchainWorker(config *config.HorizonConfig, gethURL string, bc *policy.EthereumBlockchain) *EthBlockchainWorker {
    messages := make(chan events.Message)        // The channel for outbound messages to the anax wide bus
    commands := make(chan worker.Command, 100)   // The channel for commands into the agreement bot worker

    worker := &EthBlockchainWorker{
        Worker: worker.Worker{
            Manager: worker.Manager{
                Config:   config,
                Messages: messages,
            },

            Commands: commands,
        },

        httpClient: &http.Client{},
        gethURL: gethURL,
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
                } else if !funded {
                    glog.V(3).Infof(logString(fmt.Sprintf("account %v not funded yet", acct)))
                } else if funded && !notifiedFunded {
                    notifiedFunded = true
                    nonBlockDuration = 300
                    glog.V(3).Infof(logString(fmt.Sprintf("sending acct %v funded event", acct)))
                    w.Messages() <- events.NewAccountFundedMessage(events.ACCOUNT_FUNDED, acct)
                } else if funded {
                    glog.V(3).Infof(logString(fmt.Sprintf("%v still funded", acct)))
                }

            }
            
            runtime.Gosched()
        }

    }()

    glog.Info(logString("ready for commands."))
}

// ==========================================================================================================
// Utility functions

var logString = func(v interface{}) string {
    return fmt.Sprintf("EthBlockchainWorker %v", v)
}
