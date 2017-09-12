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
	"runtime"
	"time"
)

type ExchangeMessageWorker struct {
	worker.Worker // embedded field
	db            *bolt.DB
	httpClient    *http.Client
	id            string // device id
	token         string // device token
}

func NewExchangeMessageWorker(cfg *config.HorizonConfig, db *bolt.DB) *ExchangeMessageWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 200)

	id := ""
	token := ""
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		token = dev.Token
		id = dev.Id
	}

	worker := &ExchangeMessageWorker{
		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   cfg,
				Messages: messages,
			},

			Commands: commands,
		},
		db:         db,
		httpClient: cfg.Collaborators.HTTPClientFactory.NewHTTPClient(nil),
		id:         id,
		token:      token,
	}

	worker.start()
	return worker
}

func (w *ExchangeMessageWorker) Messages() chan events.Message {
	return w.Worker.Manager.Messages
}

func (w *ExchangeMessageWorker) NewEvent(incoming events.Message) {
	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		w.id = msg.DeviceId()
		w.token = msg.Token()

	default: //nothing

	}
}

func (w *ExchangeMessageWorker) pollIncoming() {

	// Dont pull message until the device si registered
	for {
		if w.token != "" {
			break
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("waiting for exchange registration")))
			time.Sleep(5 * time.Second)
		}
	}

	// Pull messages from the exchange and send them out as individual events.
	for {
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
		time.Sleep(10 * time.Second)
	}

}

func (w *ExchangeMessageWorker) getMessages() ([]DeviceMessage, error) {
	var resp interface{}
	resp = new(GetDeviceMessageResponse)
	targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.id + "/msgs"
	for {
		if err, tpErr := InvokeExchange(w.httpClient, "GET", targetURL, w.id, w.token, nil, &resp); err != nil {
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

func (w *ExchangeMessageWorker) start() {

	go func() {
		for {
			glog.V(2).Infof(logString("command processor blocking for commands"))
			command := <-w.Commands
			glog.V(2).Infof(logString(fmt.Sprintf("received command: %v", command)))

			glog.V(5).Infof(logString("handled command"))
			runtime.Gosched()
		}
	}()

	// start polling for incoming messages
	go w.pollIncoming()
}

var logString = func(v interface{}) string {
	return fmt.Sprintf("ExchangeMessageWorker %v", v)
}
