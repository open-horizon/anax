package exchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"strconv"
	"time"
)

type ExchangeMessageWorker struct {
	worker.BaseWorker // embedded field
	db                *bolt.DB
	httpClient        *http.Client
	pattern           string // device pattern
	config            *config.HorizonConfig
	pollInterval      int  // The current message polling interval
	noMsgCount        int  // How many consecutive message polls have returned no messages
	agreementReached  bool // True when ths node has seen at least one agreement
}

func NewExchangeMessageWorker(name string, cfg *config.HorizonConfig, db *bolt.DB) *ExchangeMessageWorker {

	var ec *worker.BaseExchangeContext
	pattern := ""
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		ec = worker.NewExchangeContext(fmt.Sprintf("%v/%v", dev.Org, dev.Id), dev.Token, cfg.Edge.ExchangeURL, cfg.GetCSSURL(), cfg.Collaborators.HTTPClientFactory)
		pattern = dev.Pattern
	}

	worker := &ExchangeMessageWorker{
		BaseWorker:       worker.NewBaseWorker(name, cfg, ec),
		db:               db,
		httpClient:       cfg.Collaborators.HTTPClientFactory.NewHTTPClient(nil),
		pattern:          pattern,
		config:           cfg,
		pollInterval:     cfg.Edge.ExchangeMessagePollInterval,
		noMsgCount:       0,
		agreementReached: false,
	}

	worker.Start(worker, cfg.Edge.ExchangeMessagePollInterval)
	return worker
}

func (w *ExchangeMessageWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *ExchangeMessageWorker) NewEvent(incoming events.Message) {
	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		w.EC = worker.NewExchangeContext(fmt.Sprintf("%v/%v", msg.Org(), msg.DeviceId()), msg.Token(), w.Config.Edge.ExchangeURL, w.Config.GetCSSURL(), w.Config.Collaborators.HTTPClientFactory)
		w.pattern = msg.Pattern()

	case *events.AgreementReachedMessage:
		w.Commands <- NewAgreementCommand()

	case *events.GovernanceWorkloadCancelationMessage:
		msg, _ := incoming.(*events.GovernanceWorkloadCancelationMessage)
		switch msg.Event().Id {
		case events.AGREEMENT_ENDED:
			w.Commands <- NewResetIntervalCommand()
		}

	case *events.ApiAgreementCancelationMessage:
		msg, _ := incoming.(*events.ApiAgreementCancelationMessage)
		switch msg.Event().Id {
		case events.AGREEMENT_ENDED:
			w.Commands <- NewResetIntervalCommand()
		}

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- worker.NewTerminateCommand("shutdown")
		}

	default: //nothing

	}
}

func (w *ExchangeMessageWorker) Initialize() bool {

	// Dont pull messages until the device is registered
	for {
		if w.GetExchangeToken() != "" {
			break
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("waiting for exchange registration")))
			time.Sleep(5 * time.Second)
		}
	}

	// If there are already agreements, then we can allow the message polling interval to grow. If not, the first agreement
	// that gets made will allow the poller interval to grow.
	if agreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error searching for agreements, error %v", err)))
	} else if len(agreements) != 0 {
		w.agreementReached = true
	}

	return true
}

func (w *ExchangeMessageWorker) CommandHandler(command worker.Command) bool {

	switch command.(type) {
	case *AgreementCommand:
		w.agreementReached = true

	case *ResetIntervalCommand:
		w.resetPollingInterval()

	default:
		return false
	}
	return true
}

func (w *ExchangeMessageWorker) NoWorkHandler() {

	receivedMsg := false

	// Pull messages from the exchange and send them out as individual events.
	glog.V(5).Infof(logString(fmt.Sprintf("retrieving messages from the exchange")))

	if msgs, err := w.getMessages(); err != nil {
		glog.Errorf(logString(fmt.Sprintf("unable to retrieve exchange messages, error: %v", err)))
	} else {

		if len(msgs) > 0 {
			receivedMsg = true
		}

		// Loop through all the returned messages and process them
		for _, msg := range msgs {

			glog.V(3).Infof(logString(fmt.Sprintf("reading message %v from the exchange", msg.MsgId)))

			// First get my own keys
			_, myPrivKey, _ := GetKeys("")

			// Deconstruct and decrypt the message. If there is a problem with the message, it will be deleted.
			deleteMessage := true
			if protocolMessage, receivedPubKey, err := DeconstructExchangeMessage(msg.Message, myPrivKey); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to deconstruct exchange message %v, error %v", msg, err)))
			} else if serializedPubKey, err := MarshalPublicKey(receivedPubKey); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to marshal the key from the encrypted message %v, error %v", receivedPubKey, err)))
			} else if bytes.Compare(msg.AgbotPubKey, serializedPubKey) != 0 {
				glog.Errorf(logString(fmt.Sprintf("sender public key from exchange %v is not the same as the sender public key in the encrypted message %v", msg.AgbotPubKey, serializedPubKey)))
			} else if mBytes, err := json.Marshal(msg); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error marshalling message %v, error: %v", msg, err)))
			} else {
				// The message seems to be good, so don't delete it yet, the worker that handles the message will delete it.
				deleteMessage = false

				// Send the message to all workers.
				em := events.NewExchangeDeviceMessage(events.RECEIVED_EXCHANGE_DEV_MSG, msg.AgbotId, mBytes, string(protocolMessage))
				w.Messages() <- em
			}

			// If anything went wrong trying to decrypt the message or verify its origin, etc, just delete it. These errors aren't
			// expected to be retryable.
			if deleteMessage {
				w.deleteMessage(&msg)
			}

		}
	}

	// Update the polling interval if necessary
	if w.config.Edge.ExchangeMessageDynamicPoll {
		w.updatePollingInterval(receivedMsg)
	}

}

func (w *ExchangeMessageWorker) getMessages() ([]DeviceMessage, error) {
	var resp interface{}
	resp = new(GetDeviceMessageResponse)
	targetURL := w.Manager.Config.Edge.ExchangeURL + "orgs/" + GetOrg(w.GetExchangeId()) + "/nodes/" + GetId(w.GetExchangeId()) + "/msgs"
	for {
		if err, tpErr := InvokeExchange(w.httpClient, "GET", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(logString(err.Error()))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(logString(tpErr.Error()))
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("retrieved %v messages", len(resp.(*GetDeviceMessageResponse).Messages))))
			msgs := resp.(*GetDeviceMessageResponse).Messages
			return msgs, nil
		}
	}
}

func (w *ExchangeMessageWorker) deleteMessage(msg *DeviceMessage) error {
	var resp interface{}
	resp = new(PostDeviceResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + GetOrg(w.GetExchangeId()) + "/nodes/" + GetId(w.GetExchangeId()) + "/msgs/" + strconv.Itoa(msg.MsgId)
	for {
		if err, tpErr := InvokeExchange(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), "DELETE", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(logString(err.Error()))
			return err
		} else if tpErr != nil {
			glog.Warningf(logString(tpErr.Error()))
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("deleted message %v because it was not usable.", msg.MsgId)))
			return nil
		}
	}
}

// A stepping function for slowly increasing the time interval between polls to the node's message queue.
// If there are no agreements, leave the polling interval alone.
// If a msg was received in the last interval, reduce the interval to the starting configured interval.
// Otherwise, increase the polling interval if enough polls have passed. The algorithm is a simple function that
// slowly increases the polling interval at first and then increases it's length more quickly as more and more
// polls come back with no messages.
func (w *ExchangeMessageWorker) updatePollingInterval(msgReceived bool) {

	if msgReceived {
		w.resetPollingInterval()
		return
	}

	if !w.agreementReached || (w.pollInterval >= w.config.Edge.ExchangeMessagePollMaxInterval) {
		return
	}

	w.noMsgCount += 1

	if w.noMsgCount >= (w.config.Edge.ExchangeMessagePollMaxInterval / w.pollInterval) {
		w.pollInterval += w.config.Edge.ExchangeMessagePollIncrement
		if w.pollInterval > w.config.Edge.ExchangeMessagePollMaxInterval {
			w.pollInterval = w.config.Edge.ExchangeMessagePollMaxInterval
		}
		w.noMsgCount = 0
		w.SetNoWorkInterval(w.pollInterval)
		glog.V(3).Infof(logString(fmt.Sprintf("increasing message poll interval to %v, increment is %v", w.pollInterval, w.config.Edge.ExchangeMessagePollIncrement)))
	}

	return

}

func (w *ExchangeMessageWorker) resetPollingInterval() {
	if w.pollInterval != w.config.Edge.ExchangeMessagePollInterval {
		w.pollInterval = w.config.Edge.ExchangeMessagePollInterval
		w.SetNoWorkInterval(w.pollInterval)
		glog.V(3).Infof(logString(fmt.Sprintf("resetting message poll interval to %v, increment is %v", w.pollInterval, w.config.Edge.ExchangeMessagePollIncrement)))
	}
	w.noMsgCount = 0
	return
}

type AgreementCommand struct {
}

func (c AgreementCommand) ShortString() string {
	return fmt.Sprintf("AgreementCommand")
}

func NewAgreementCommand() *AgreementCommand {
	return &AgreementCommand{}
}

type ResetIntervalCommand struct {
}

func (c ResetIntervalCommand) ShortString() string {
	return fmt.Sprintf("ResetIntervalCommand")
}

func NewResetIntervalCommand() *ResetIntervalCommand {
	return &ResetIntervalCommand{}
}

var logString = func(v interface{}) string {
	return fmt.Sprintf("ExchangeMessageWorker %v", v)
}
