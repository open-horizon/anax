package agreementbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	gwhisper "github.com/open-horizon/go-whisper"
	"net/http"
	"time"
)

func (w *AgreementBotWorker) GovernAgreements() {

	glog.Info(logString(fmt.Sprintf("started agreement governance")))

	sendMessage := func(mt interface{}, pay []byte) error {
		// The mt parameter is an abstract message target object that is passed to this routine
		// by the agreement protocol. It's an interface{} type so that we can avoid the protocol knowing
		// about non protocol types.

		var messageTarget *exchange.ExchangeMessageTarget
		switch mt.(type) {
		case *exchange.ExchangeMessageTarget:
			messageTarget = mt.(*exchange.ExchangeMessageTarget)
		default:
			return errors.New(fmt.Sprintf("input message target is %T, expecting exchange.MessageTarget", mt))
		}

		// If the message target is using whisper, then send via whisper
		if len(messageTarget.ReceiverMsgEndPoint) != 0 {
			to := messageTarget.ReceiverMsgEndPoint
			glog.V(3).Infof("Sending whisper message to: %v at whisper %v, message %v", messageTarget.ReceiverExchangeId, to, string(pay))

			if from, err := gwhisper.AccountId(w.Config.AgreementBot.GethURL); err != nil {
				return errors.New(fmt.Sprintf("Error obtaining whisper id: %v", err))
			} else {
				// this is to last long enough to be read by even an overloaded governor but still expire before a new worker might try to pick up the contract
				msg, err := gwhisper.TopicMsgParams(from, to, []string{citizenscientist.PROTOCOL_NAME}, string(pay), 180, 50)
				if err != nil {
					return errors.New(fmt.Sprintf("Error creating whisper message topic parameters: %v", err))
				}

				_, err = gwhisper.WhisperSend(w.httpClient, w.Config.AgreementBot.GethURL, gwhisper.POST, msg, 3)
				if err != nil {
					return errors.New(fmt.Sprintf("Error sending whisper message: %v, error: %v", msg, err))
				}
			}

		// The message target is using the exchange message queue, so use it
		} else {

			// Grab the exchange ID of the message receiver
			glog.V(3).Infof("Sending exchange message to: %v, message %v", messageTarget.ReceiverExchangeId, string(pay))

			// Get my own keys
			myPubKey, myPrivKey, _ := exchange.GetKeys(w.Config.AgreementBot.MessageKeyPath)

			// Demarshal the receiver's public key if we need to
			if messageTarget.ReceiverPublicKeyObj == nil {
				if mtpk, err := exchange.DemarshalPublicKey(messageTarget.ReceiverPublicKeyBytes); err != nil {
					return errors.New(fmt.Sprintf("Unable to demarshal device's public key %x, error %v", messageTarget.ReceiverPublicKeyBytes, err))
				} else {
					messageTarget.ReceiverPublicKeyObj = mtpk
				}
			}

			// Create an encrypted message
			if encryptedMsg, err := exchange.ConstructExchangeMessage(pay, myPubKey, myPrivKey, messageTarget.ReceiverPublicKeyObj); err != nil {
				return errors.New(fmt.Sprintf("Unable to construct encrypted message from %v, error %v", pay, err))
			// Marshal it into a byte array
			} else if msgBody, err := json.Marshal(encryptedMsg); err != nil {
				return errors.New(fmt.Sprintf("Unable to marshal exchange message %v, error %v", encryptedMsg, err))
			// Send it to the device's message queue
			} else {
				pm := exchange.CreatePostMessage(msgBody, w.Config.AgreementBot.ExchangeMessageTTL)
				var resp interface{}
				resp = new(exchange.PostDeviceResponse)
				targetURL := w.Config.AgreementBot.ExchangeURL + "devices/" + messageTarget.ReceiverExchangeId + "/msgs"
				for {
					if err, tpErr := exchange.InvokeExchange(w.httpClient, "POST", targetURL, w.agbotId, w.token, pm, &resp); err != nil {
						return err
					} else if tpErr != nil {
						glog.V(5).Infof(tpErr.Error())
						time.Sleep(10 * time.Second)
						continue
					} else {
						glog.V(5).Infof("Sent message for %v to exchange.", messageTarget.ReceiverExchangeId)
						return nil
					}
				}
			}
		}
		return nil
	}

	protocolHandler := citizenscientist.NewProtocolHandler(w.Config.AgreementBot.GethURL, w.pm)

	for {

		notYetFinalFilter := func() AFilter {
			return func(a Agreement) bool { return a.AgreementCreationTime != 0 && a.AgreementTimedout == 0 }
		}

		// Find all agreements that are in progress. They might be waiting for a reply or not yet finalized on blockchain.
		if agreements, err := FindAgreements(w.db, []AFilter{notYetFinalFilter()}, citizenscientist.PROTOCOL_NAME); err == nil {
			activeDataVerification := true
			allActiveAgreements := make(map[string][]string)
			for _, ag := range agreements {

				// Govern agreements that have seen a reply from the device
				if ag.CounterPartyAddress != "" {

					// For agreements that havent seen a blockchain write yet, check again.
					if ag.AgreementFinalizedTime == 0 {

						// We are waiting for the write to the blockchain. The counterparty address comes in the reply
						glog.V(5).Infof("AgreementBot Governance checking agreement %v for finalization.", ag.CurrentAgreementId)
						if recorded, err := protocolHandler.VerifyAgreementRecorded(ag.CurrentAgreementId, ag.CounterPartyAddress, ag.ProposalSig, w.bc.Agreements); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to verify agreement %v on blockchain, error: %v", ag.CurrentAgreementId, err)))
						} else if recorded {
							// Update state in the database
							if _, err := AgreementFinalized(w.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
								glog.Errorf(logString(fmt.Sprintf("error persisting agreement %v finalized: %v", ag.CurrentAgreementId, err)))
							}
							// Update state in exchange
							if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
								glog.Errorf(logString(fmt.Sprintf("error demarshalling policy from agreement %v, error: %v", ag.CurrentAgreementId, err)))
							} else if err := recordConsumerAgreementState(w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token, ag.CurrentAgreementId, pol.APISpecs[0].SpecRef, "Finalized Agreement"); err != nil {
								glog.Errorf(logString(fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", ag.CurrentAgreementId, err)))
							}
						} else {
							glog.V(5).Infof("AgreementBot Governance detected agreement %v not yet final.", ag.CurrentAgreementId)
							now := uint64(time.Now().Unix())
							if ag.AgreementCreationTime+w.Worker.Manager.Config.AgreementBot.AgreementTimeoutS < now {
								// Start timing out the agreement
								w.TerminateAgreement(&ag, citizenscientist.AB_CANCEL_NOT_FINALIZED_TIMEOUT)
							}
						}
					// For agreements that are known to be on the blockchain, make sure they are still there
					} else {

						if recorded, err := protocolHandler.VerifyAgreementRecorded(ag.CurrentAgreementId, ag.CounterPartyAddress, ag.ProposalSig, w.bc.Agreements); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to verify finalized agreement %v on blockchain, error: %v", ag.CurrentAgreementId, err)))
						} else if !recorded {
							// The agreement was in the blockchain but isnt there any more, we need to cancel on this side and update the exchange
							glog.V(3).Infof(logString(fmt.Sprintf("discovered terminated agreement %v, cleaning up.", ag.CurrentAgreementId)))
							w.TerminateAgreement(&ag, citizenscientist.AB_CANCEL_DISCOVERED)
						}
					}

					// Check for the receipt of data in the data ingest system (if necessary)
					now := uint64(time.Now().Unix())
					if now - ag.DataVerifiedTime >= w.Worker.Manager.Config.AgreementBot.NoDataIntervalS {
						// No data is being received, terminate the agreement
						glog.V(3).Infof(logString(fmt.Sprintf("cancelling agreement %v due to lack of data", ag.CurrentAgreementId)))
						w.TerminateAgreement(&ag, citizenscientist.AB_CANCEL_NO_DATA_RECEIVED)
					} else if activeDataVerification {
						// And make sure the device is still sending data
						if activeAgreements, err := GetActiveAgreements(allActiveAgreements, ag, &w.Worker.Manager.Config.AgreementBot); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to retrieve active agreement list. Terminating data verification loop early, error: %v", err)))
							activeDataVerification = false
						} else if ActiveAgreementsContains(activeAgreements, ag, w.Config.AgreementBot.DVPrefix) {
							if _, err := DataVerified(w.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
								glog.Errorf(logString(fmt.Sprintf("unable to record data verification, error: %v", err)))
							}
							if ag.DataNotificationSent == 0 {
								// Get whisper address of the device from the exchange. The device ensures that the exchange is kept current.
								// If the address happens to be invalid, that should be a temporary condition. We will keep sending until
								// we get an ack to our verification message.
								if whisperTo, pubkeyTo, err := getDeviceMessageEndpoint(ag.DeviceId, w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token); err != nil {
									glog.Errorf(logString(fmt.Sprintf("Error obtaining whisper id for data notification: %v", err)))
								} else if mt, err := exchange.CreateMessageTarget(ag.DeviceId, nil, pubkeyTo, whisperTo); err != nil {
									glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
								} else if err := protocolHandler.NotifyDataReceipt(ag.CurrentAgreementId, mt, sendMessage); err != nil {
									glog.Errorf(logString(fmt.Sprintf("unable to send data notification, error: %v", err)))
								}
							}
						}
					}

				// Govern agreements that havent seen a proposal reply yet
				} else {
					// We are waiting for a reply
					glog.V(5).Infof("AgreementBot Governance waiting for reply to %v.", ag.CurrentAgreementId)
					now := uint64(time.Now().Unix())
					if ag.AgreementCreationTime + w.Worker.Manager.Config.AgreementBot.ProtocolTimeoutS < now {
						w.TerminateAgreement(&ag, citizenscientist.AB_CANCEL_NO_REPLY)
					}
				}
			}
		} else {
			glog.Errorf(logString(fmt.Sprintf("unable to read agreements from database, error: %v", err)))
		}

		time.Sleep(time.Duration(w.Worker.Manager.Config.AgreementBot.ProcessGovernanceIntervalS) * time.Second)
	}

	glog.Info(logString(fmt.Sprintf("terminated agreement governance")))

}

func (w *AgreementBotWorker) TerminateAgreement(ag *Agreement, reason uint) {
	// Start timing out the agreement
	glog.V(3).Infof(logString(fmt.Sprintf("detected agreement %v needs to terminate.", ag.CurrentAgreementId)))

	// Update the database
	if _, err := AgreementTimedout(w.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error marking agreement %v terminate: %v", ag.CurrentAgreementId, err)))
	}
	// Update state in exchange
	if err := DeleteConsumerAgreement(w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token, ag.CurrentAgreementId); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error deleting agreement %v in exchange: %v", ag.CurrentAgreementId, err)))
	}
	// Queue up a command for an agreement worker to do the blockchain work
	w.pwcommands <- NewAgreementTimeoutCommand(ag.CurrentAgreementId, ag.AgreementProtocol, reason)
}

func recordConsumerAgreementState(url string, agbotId string, token string, agreementId string, workloadID string, state string) error {

	logString := func(v interface{}) string {
		return fmt.Sprintf("AgreementBot Governance: %v", v)
	}

	glog.V(5).Infof(logString(fmt.Sprintf("setting agreement %v state to %v", agreementId, state)))

	as := new(exchange.PutAgbotAgreementState)
	as.Workload = workloadID
	as.State = state
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "agbots/" + agbotId + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(&http.Client{}, "PUT", targetURL, agbotId, token, &as, &resp); err != nil {
			glog.Errorf(logString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("set agreement %v to state %v", agreementId, state)))
			return nil
		}
	}

}

func DeleteConsumerAgreement(url string, agbotId string, token string, agreementId string) error {

	logString := func(v interface{}) string {
		return fmt.Sprintf("AgreementBot Governance: %v", v)
	}

	glog.V(5).Infof(logString(fmt.Sprintf("deleting agreement %v in exchange", agreementId)))

	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "agbots/" + agbotId + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(&http.Client{}, "DELETE", targetURL, agbotId, token, nil, &resp); err != nil {
			glog.Errorf(logString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("deleted agreement %v from exchange", agreementId)))
			return nil
		}
	}

}

func getDeviceMessageEndpoint(deviceId string, url string, agbotId string, token string) (string, []byte, error) {

	glog.V(5).Infof(logString(fmt.Sprintf("retrieving device %v msg endpoint from exchange", deviceId)))

	var resp interface{}
	resp = new(exchange.GetDevicesResponse)
	targetURL := url + "devices/" + deviceId
	for {
		if err, tpErr := exchange.InvokeExchange(&http.Client{}, "GET", targetURL, agbotId, token, nil, &resp); err != nil {
			glog.Errorf(logString(fmt.Sprintf(err.Error())))
			return "", nil, err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			devs := resp.(*exchange.GetDevicesResponse).Devices
			if dev, there := devs[deviceId]; !there {
				return "", nil, errors.New(fmt.Sprintf("device %v not in GET response %v as expected", deviceId, devs))
			} else {
				glog.V(5).Infof(logString(fmt.Sprintf("retrieved device %v msg endpoint from exchange %v", deviceId, dev.MsgEndPoint)))
				return dev.MsgEndPoint, dev.PublicKey, nil
			}
		}
	}

}

var logString = func(v interface{}) string {
	return fmt.Sprintf("AgreementBot Governance: %v", v)
}
