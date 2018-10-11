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
	"github.com/open-horizon/anax/worker"
	"net/http"
	"time"
)

type ExchangeMessageWorker struct {
	worker.BaseWorker // embedded field
	db                *bolt.DB
	httpClient        *http.Client
	pattern           string // device pattern
}

func NewExchangeMessageWorker(name string, cfg *config.HorizonConfig, db *bolt.DB) *ExchangeMessageWorker {

	var ec *worker.BaseExchangeContext
	pattern := ""
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		ec = worker.NewExchangeContext(fmt.Sprintf("%v/%v", dev.Org, dev.Id), dev.Token, cfg.Edge.ExchangeURL, cfg.Collaborators.HTTPClientFactory)
		pattern = dev.Pattern
	}

	worker := &ExchangeMessageWorker{
		BaseWorker: worker.NewBaseWorker(name, cfg, ec),
		db:         db,
		httpClient: cfg.Collaborators.HTTPClientFactory.NewHTTPClient(nil),
		pattern:    pattern,
	}

	worker.Start(worker, 10)
	return worker
}

func (w *ExchangeMessageWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *ExchangeMessageWorker) NewEvent(incoming events.Message) {
	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		w.EC = worker.NewExchangeContext(fmt.Sprintf("%v/%v", msg.Org(), msg.DeviceId()), msg.Token(), w.Config.Edge.ExchangeURL, w.Config.Collaborators.HTTPClientFactory)
		w.pattern = msg.Pattern()

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
	return true
}

func (w *ExchangeMessageWorker) NoWorkHandler() {
	// Pull messages from the exchange and send them out as individual events.
	glog.V(5).Infof(logString(fmt.Sprintf("retrieving messages from the exchange")))

	if msgs, err := w.getMessages(); err != nil {
		glog.Errorf(logString(fmt.Sprintf("unable to retrieve exchange messages, error: %v", err)))
	} else {
		// Loop through all the returned messages and process them
		for _, msg := range msgs {

			glog.V(3).Infof(logString(fmt.Sprintf("reading message %v from the exchange", msg.MsgId)))

			// First get my own keys
			_, myPrivKey, _ := GetKeys("")

			// Deconstruct and decrypt the message.
			if protocolMessage, receivedPubKey, err := DeconstructExchangeMessage(msg.Message, myPrivKey); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to deconstruct exchange message %v, error %v", msg, err)))
			} else if serializedPubKey, err := MarshalPublicKey(receivedPubKey); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to marshal the key from the encrypted message %v, error %v", receivedPubKey, err)))
			} else if bytes.Compare(msg.AgbotPubKey, serializedPubKey) != 0 {
				glog.Errorf(logString(fmt.Sprintf("sender public key from exchange %v is not the same as the sender public key in the encrypted message %v", msg.AgbotPubKey, serializedPubKey)))
			} else if mBytes, err := json.Marshal(msg); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error marshalling message %v, error: %v", msg, err)))
			} else {
				em := events.NewExchangeDeviceMessage(events.RECEIVED_EXCHANGE_DEV_MSG, mBytes, string(protocolMessage))
				w.Messages() <- em
			}
		}
	}

}

func (w *ExchangeMessageWorker) getMessages() ([]DeviceMessage, error) {
	var resp interface{}
	resp = new(GetDeviceMessageResponse)
	targetURL := w.Manager.Config.Edge.ExchangeURL + "orgs/" + GetOrg(w.GetExchangeId()) + "/nodes/" + GetId(w.GetExchangeId()) + "/msgs"
	for {
		if err, tpErr := InvokeExchange(w.httpClient, "GET", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("retrieved %v messages", len(resp.(*GetDeviceMessageResponse).Messages))))
			msgs := resp.(*GetDeviceMessageResponse).Messages
			return msgs, nil
		}
	}
}

var logString = func(v interface{}) string {
	return fmt.Sprintf("ExchangeMessageWorker %v", v)
}
