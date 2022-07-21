package exchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	"strconv"
	"time"
)

type DeviceMessage struct {
	MsgId       int    `json:"msgId"`
	AgbotId     string `json:"agbotId"`
	AgbotPubKey []byte `json:"agbotPubKey"`
	Message     []byte `json:"message"`
	TimeSent    string `json:"timeSent"`
}

func (d DeviceMessage) String() string {
	return fmt.Sprintf("MsgId: %v, AgbotId: %v, AgbotPubKey %v, Message %v, TimeSent %v", d.MsgId, d.AgbotId, d.AgbotPubKey, cutil.TruncateDisplayString(string(d.Message), 32), d.TimeSent)
}

type GetDeviceMessageResponse struct {
	Messages  []DeviceMessage `json:"messages"`
	LastIndex int             `json:"lastIndex"`
}

type ExchangeMessageWorker struct {
	worker.BaseWorker // embedded field
	db                *bolt.DB
	config            *config.HorizonConfig
}

func NewExchangeMessageWorker(name string, cfg *config.HorizonConfig, db *bolt.DB) *ExchangeMessageWorker {

	var ec *worker.BaseExchangeContext
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		ec = worker.NewExchangeContext(fmt.Sprintf("%v/%v", dev.Org, dev.Id), dev.Token, cfg.Edge.ExchangeURL, cfg.GetCSSURL(), newLimitedRetryHTTPFactory(cfg.Collaborators.HTTPClientFactory))
	}

	worker := &ExchangeMessageWorker{
		BaseWorker: worker.NewBaseWorker(name, cfg, ec),
		db:         db,
		config:     cfg,
	}

	// If there are temporary errors trying to retrieve messages, the command handler will requeue the command to handle messages.
	// Therefore the NoWorkHandler needs to wake up periodically so the worker framework can perform the requeue. Thus, even though
	// this worker doesnt do anything in the NoWorkHandler, there is still a short interval set.
	worker.Start(worker, 10)
	return worker
}

// Customized HTTPFactory for limiting retries.
func newLimitedRetryHTTPFactory(base *config.HTTPClientFactory) *config.HTTPClientFactory {
	limitedRetryHTTPFactory := &config.HTTPClientFactory{
		NewHTTPClient: base.NewHTTPClient,
		RetryCount:    1,
		RetryInterval: 5,
	}
	return limitedRetryHTTPFactory
}

func (w *ExchangeMessageWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *ExchangeMessageWorker) NewEvent(incoming events.Message) {
	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		w.EC = worker.NewExchangeContext(fmt.Sprintf("%v/%v", msg.Org(), msg.DeviceId()), msg.Token(), w.Config.Edge.ExchangeURL, w.Config.GetCSSURL(), newLimitedRetryHTTPFactory(w.Config.Collaborators.HTTPClientFactory))

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- worker.NewTerminateCommand("shutdown")
		}

	case *events.ExchangeChangeMessage:
		msg, _ := incoming.(*events.ExchangeChangeMessage)
		switch msg.Event().Id {
		case events.CHANGE_MESSAGE_TYPE:
			w.Commands <- NewMessageCommand()
		}

	case *events.NodeHeartbeatStateChangeMessage:
		msg, _ := incoming.(*events.NodeHeartbeatStateChangeMessage)
		switch msg.Event().Id {
		case events.NODE_HEARTBEAT_RESTORED:

			// Now that heartbeating is restored, fire the functions to check on exchange state changes. If the node
			// was offline long enough, the exchange might have pruned changes we needed to see, which means we will
			// never see them now. So, assume there were some changes we care about.
			w.Commands <- NewMessageCommand()
		}

	default: //nothing

	}
}

func (w *ExchangeMessageWorker) Initialize() bool {
	return true
}

func (w *ExchangeMessageWorker) CommandHandler(command worker.Command) bool {

	switch command.(type) {
	case *MessageCommand:
		if !w.handleMessages() {
			w.AddDeferredCommand(command)
		}

	default:
		return false
	}
	return true
}

func (w *ExchangeMessageWorker) NoWorkHandler() {
}

func (w *ExchangeMessageWorker) handleMessages() bool {

	// Pull messages from the exchange and send them out as individual events.
	glog.V(5).Infof(logString(fmt.Sprintf("retrieving messages from the exchange")))

	var msgs []DeviceMessage
	var err error

	msgs, err = w.getMessages()
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("unable to retrieve exchange messages, error: %v", err)))
		return false
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
	return true

}

func (w *ExchangeMessageWorker) getMessages() ([]DeviceMessage, error) {
	var resp interface{}
	resp = new(GetDeviceMessageResponse)

	retryCount := w.GetHTTPFactory().RetryCount
	retryInterval := w.GetHTTPFactory().GetRetryInterval()

	targetURL := w.GetExchangeURL() + "orgs/" + GetOrg(w.GetExchangeId()) + "/nodes/" + GetId(w.GetExchangeId()) + "/msgs"
	for {
		if err, tpErr := InvokeExchange(w.GetHTTPFactory().NewHTTPClient(nil), "GET", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(logString(err.Error()))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(logString(tpErr.Error()))
			if w.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", w.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
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

	retryCount := w.GetHTTPFactory().RetryCount
	retryInterval := w.GetHTTPFactory().GetRetryInterval()

	targetURL := w.GetExchangeURL() + "orgs/" + GetOrg(w.GetExchangeId()) + "/nodes/" + GetId(w.GetExchangeId()) + "/msgs/" + strconv.Itoa(msg.MsgId)
	for {
		if err, tpErr := InvokeExchange(w.GetHTTPFactory().NewHTTPClient(nil), "DELETE", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(logString(err.Error()))
			return err
		} else if tpErr != nil {
			glog.Warningf(logString(tpErr.Error()))
			if w.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return fmt.Errorf("Exceeded %v retries for error: %v", w.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("deleted message %v because it was not usable.", msg.MsgId)))
			return nil
		}
	}
}

// Indicates that there is a message for this node.
type MessageCommand struct {
}

func (c MessageCommand) ShortString() string {
	return fmt.Sprintf("MessageCommand")
}

func NewMessageCommand() *MessageCommand {
	return &MessageCommand{}
}

var logString = func(v interface{}) string {
	return fmt.Sprintf("ExchangeMessageWorker %v", v)
}
