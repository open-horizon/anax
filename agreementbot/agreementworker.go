package agreementbot

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/ethblockchain"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	gwhisper "github.com/open-horizon/go-whisper"
	"github.com/satori/go.uuid"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

type CSAgreementWorker struct {
	pm         *policy.PolicyManager
	db         *bolt.DB
	config     *config.HorizonConfig
	httpClient *http.Client
	agbotId    string
	token      string
	protocol   string
}

func NewCSAgreementWorker(policyManager *policy.PolicyManager, config *config.HorizonConfig, db *bolt.DB) *CSAgreementWorker {

	p := &CSAgreementWorker{
		pm:         policyManager,
		db:         db,
		config:     config,
		httpClient: &http.Client{},
		agbotId:    config.AgreementBot.ExchangeId,
		token:      config.AgreementBot.ExchangeToken,
	}

	return p
}

// These structs are the event bodies that flows from the processor to the agreement workers
const INITIATE = "INITIATE_AGREEMENT"
const REPLY = "AGREEMENT_REPLY"
const CANCEL = "AGREEMENT_CANCEL"
const DATARECEIVEDACK = "AGREEMENT_DATARECEIVED_ACK"

type CSAgreementWork interface {
	Type() string
}

type CSInitiateAgreement struct {
	workType       string
	ProducerPolicy policy.Policy   // the producer policy received from the exchange
	ConsumerPolicy policy.Policy   // the consumer policy we're matched up with
	Device         exchange.SearchResultDevice // the device entry in the exchange
}

func (c CSInitiateAgreement) String() string {
	res := ""
	res += fmt.Sprintf("Workitem: %v\n", c.workType)
	res += fmt.Sprintf("Producer Policy: %v\n", c.ProducerPolicy)
	res += fmt.Sprintf("Consumer Policy: %v\n", c.ConsumerPolicy)
	res += fmt.Sprintf("Device: %v", c.Device)
	return res
}

func (c CSInitiateAgreement) Type() string {
	return c.workType
}

type CSHandleReply struct {
	workType     string
	Reply        string
	From         string        // deprecated whisper address
	SenderId     string        // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func (c CSHandleReply) String() string {
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Reply: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Reply, c.SenderPubKey)
}

func (c CSHandleReply) Type() string {
	return c.workType
}

type CSHandleDataReceivedAck struct {
	workType     string
	Ack          string
	From         string        // deprecated whisper address
	SenderId     string        // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func (c CSHandleDataReceivedAck) String() string {
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Ack: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Ack, c.SenderPubKey)
}

func (c CSHandleDataReceivedAck) Type() string {
	return c.workType
}

type CSCancelAgreement struct {
	workType    string
	AgreementId string
	Protocol    string
	Reason      uint
}

func (c CSCancelAgreement) Type() string {
	return c.workType
}


// This function receives an event to "make a new agreement" from the Process function, and then synchronously calls a function
// to actually work through the agreement protocol.
func (a *CSAgreementWorker) start(work chan CSAgreementWork, random *rand.Rand, bc *ethblockchain.BaseContracts) {
	workerID := uuid.NewV4().String()

	logString := func(v interface{}) string {
		return fmt.Sprintf("CSAgreementWorker (%v): %v", workerID, v)
	}

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

			if from, err := gwhisper.AccountId(a.config.AgreementBot.GethURL); err != nil {
				return errors.New(fmt.Sprintf("Error obtaining whisper id: %v", err))
			} else {
				// this is to last long enough to be read by even an overloaded governor but still expire before a new worker might try to pick up the contract
				msg, err := gwhisper.TopicMsgParams(from, to, []string{citizenscientist.PROTOCOL_NAME}, string(pay), 180, 50)
				if err != nil {
					return errors.New(fmt.Sprintf("Error creating whisper message topic parameters: %v", err))
				}

				_, err = gwhisper.WhisperSend(a.httpClient, a.config.AgreementBot.GethURL, gwhisper.POST, msg, 3)
				if err != nil {
					return errors.New(fmt.Sprintf("Error sending whisper message: %v, error: %v", msg, err))
				}
			}

		// The message target is using the exchange message queue, so use it
		} else {

			// Grab the exchange ID of the message receiver
			glog.V(3).Infof("Sending exchange message to: %v, message %v", messageTarget.ReceiverExchangeId, string(pay))

			// Get my own keys
			myPubKey, myPrivKey, _ := exchange.GetKeys(a.config.AgreementBot.MessageKeyPath)

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
				return errors.New(fmt.Sprintf("Unable to construct encrypted message, error %v for message %s", err, pay))
			// Marshal it into a byte array
			} else if msgBody, err := json.Marshal(encryptedMsg); err != nil {
				return errors.New(fmt.Sprintf("Unable to marshal exchange message, error %v for message %v", err, encryptedMsg))
			// Send it to the device's message queue
			} else {
				pm := exchange.CreatePostMessage(msgBody, a.config.AgreementBot.ExchangeMessageTTL)
				var resp interface{}
				resp = new(exchange.PostDeviceResponse)
				targetURL := a.config.AgreementBot.ExchangeURL + "devices/" + messageTarget.ReceiverExchangeId + "/msgs"
				for {
					if err, tpErr := exchange.InvokeExchange(a.httpClient, "POST", targetURL, a.agbotId, a.token, pm, &resp); err != nil {
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

	protocolHandler := citizenscientist.NewProtocolHandler(a.config.AgreementBot.GethURL, a.pm)

	for {
		glog.V(5).Infof(logString(fmt.Sprintf("blocking for work")))
		workItem := <-work // block waiting for work
		glog.V(2).Infof(logString(fmt.Sprintf("received work: %v", workItem)))

		if workItem.Type() == INITIATE {
			wi := workItem.(CSInitiateAgreement)
			// Generate an agreement ID
			agreementId := generateAgreementId(random)
			myAddress, _ := ethblockchain.AccountId()
			agreementIdString := hex.EncodeToString(agreementId)
			glog.V(5).Infof(logString(fmt.Sprintf("using AgreementId %v", agreementIdString)))

			// Create pending agreement in database
			if err := AgreementAttempt(a.db, agreementIdString, wi.Device.Id, wi.ConsumerPolicy.Header.Name, "Citizen Scientist"); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error persisting agreement attempt: %v", err)))

			// Create message target for protocol message
			} else if mt, err := exchange.CreateMessageTarget(wi.Device.Id, nil, wi.Device.PublicKey, wi.Device.MsgEndPoint); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))

			// Initiate the protocol
			} else if proposal, err := protocolHandler.InitiateAgreement(agreementIdString, &wi.ProducerPolicy, &wi.ConsumerPolicy, myAddress, a.agbotId, mt, a.config.AgreementBot.DefaultWorkloadPW, sendMessage); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error initiating agreement: %v", err)))

				// Remove pending agreement from database
				if err := DeleteAgreement(a.db, agreementIdString, citizenscientist.PROTOCOL_NAME); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error deleting pending agreement: %v, error %v", agreementIdString, err)))
				}

				// TODO: Publish error on the message bus

			// Update the agreement in the DB with the proposal and policy
			} else if polBytes, err := json.Marshal(wi.ConsumerPolicy); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error marshalling policy for storage %v, error: %v", wi.ConsumerPolicy, err)))
			} else if pBytes, err := json.Marshal(proposal); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error marshalling proposal for storage %v, error: %v", *proposal, err)))
			} else if _, err := AgreementUpdate(a.db, agreementIdString, string(pBytes), string(polBytes), wi.ConsumerPolicy.DataVerify.URL, wi.ConsumerPolicy.DataVerify.URLUser, wi.ConsumerPolicy.DataVerify.URLPassword, !wi.ConsumerPolicy.Get_DataVerification_enabled(), citizenscientist.PROTOCOL_NAME); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error updating agreement with proposal %v in DB, error: %v", *proposal, err)))

			// Record that the agreement was initiated, in the exchange
			} else if err := a.recordConsumerAgreementState(agreementIdString, wi.ConsumerPolicy.APISpecs[0].SpecRef, "Formed Proposal"); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error setting agreement state for %v", agreementIdString)))
			}

		} else if workItem.Type() == REPLY {
			wi := workItem.(CSHandleReply)

			// The reply message is usually deleted before recording on the blockchain. For now assume it will be deleted at the end.
			deletedMessage := false


			if reply, err := protocolHandler.ValidateReply(wi.Reply); err != nil {
				glog.Warningf(logString(fmt.Sprintf("discarding message: %v", wi.Reply)))
			} else if reply.ProposalAccepted() {
				// The producer is happy with the proposal. Assume we will ack negatively unless we find out that everything is ok.
				ackReplyAsValid := false

				// Find the saved agreement in the database
				if agreement, err := FindSingleAgreementByAgreementId(a.db, reply.AgreementId(), citizenscientist.PROTOCOL_NAME); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error querying pending agreement %v, error: %v", reply.AgreementId(), err)))
				} else if agreement == nil {
					glog.V(5).Infof(logString(fmt.Sprintf("discarding reply, agreement id %v not in our database", reply.AgreementId())))
				} else if agreement.CounterPartyAddress != "" {
					glog.V(5).Infof(logString(fmt.Sprintf("discarding reply, agreement id %v already received a reply", agreement.CurrentAgreementId)))
					// this will cause us to not send a reply ack, which is what we want in this case
					ackReplyAsValid = true

				// Now we need to write the info to the exchange and the database
				} else if proposal, err := protocolHandler.DemarshalProposal(agreement.Proposal); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error validating proposal from pending agreement %v, error: %v", reply.AgreementId(), err)))
				} else if pol, err := policy.DemarshalPolicy(proposal.TsAndCs); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error demarshalling tsandcs policy from pending agreement %v, error: %v", reply.AgreementId(), err)))

				} else if _, err := AgreementMade(a.db, reply.AgreementId(), reply.Address, reply.Signature, citizenscientist.PROTOCOL_NAME); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error updating agreement with proposal %v in DB, error: %v", *proposal, err)))

				} else if err := a.recordConsumerAgreementState(reply.AgreementId(), pol.APISpecs[0].SpecRef, "Producer agreed"); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error setting agreement state for %v", reply.AgreementId())))

				// We need to send a reply ack and write the info to the blockchain
				} else if consumerPolicy, err := policy.DemarshalPolicy(agreement.Policy); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", reply.AgreementId(), err)))
				} else {
					// Done handling the response successfully
					ackReplyAsValid = true

					// Send the reply Ack
					if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, wi.From); err != nil {
						glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
					} else if err := protocolHandler.Confirm(ackReplyAsValid, reply.AgreementId(), mt, sendMessage); err != nil {
						glog.Errorf(logString(fmt.Sprintf("error trying to send reply ack for %v to %v, error: %v", reply.AgreementId(), mt, err)))
					}

					// Delete the original reply message
					if wi.MessageId != 0 {
						if err := a.deleteMessage(wi.MessageId); err != nil {
							glog.Errorf(logString(fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, a.agbotId)))
						}
					}
					deletedMessage = true

					// Recording the agreement on the blockchain could take a long time, so it needs to be the last thing we do.
					if err := protocolHandler.RecordAgreement(proposal, reply, consumerPolicy, bc.Agreements); err != nil {
						glog.Errorf(logString(fmt.Sprintf("error trying to record agreement in blockchain, %v", err)))
					}

					glog.V(3).Infof(logString(fmt.Sprintf("recorded agreement %v", reply.AgreementId())))
				}

				// Always send an ack for a reply with a positive decision in it
				if !ackReplyAsValid {
					if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, wi.From); err != nil {
						glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
					} else if err := protocolHandler.Confirm(ackReplyAsValid, reply.AgreementId(), mt, sendMessage); err != nil {
						glog.Errorf(logString(fmt.Sprintf("error trying to send reply ack for %v to %v, error: %v", reply.AgreementId(), wi.From, err)))
					}
				}

			} else {
				// Delete the agreement from the exchange
				if err := DeleteConsumerAgreement(a.config.AgreementBot.ExchangeURL, a.agbotId, a.token, reply.AgreementId()); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error deleting agreement %v in exchange: %v", reply.AgreementId(), err)))
				}

				// Allow the protocol to perform any cleanup
				if ag, err := FindSingleAgreementByAgreementId(a.db, reply.AgreementId(), citizenscientist.PROTOCOL_NAME); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error querying pending agreement %v, error: %v", reply.AgreementId(), err)))
				} else if ag == nil {
					glog.Errorf(logString(fmt.Sprintf("no database entry for agreement %v, can't cleanup after rejection.", reply.AgreementId())))
				} else if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to demarshal policy for agreement %v while trying to cleanup after rejection, error %v", reply.AgreementId(), err)))
				} else if err := protocolHandler.TerminateAgreement(pol, ag.CounterPartyAddress, ag.CurrentAgreementId, citizenscientist.AB_CANCEL_NEGATIVE_REPLY, bc.Agreements); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error terminating agreement %v on the blockchain: %v", reply.AgreementId(), err)))
				}

				// Delete from the database
				if err := DeleteAgreement(a.db, reply.AgreementId(), citizenscientist.PROTOCOL_NAME); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error deleting rejected agreement: %v, error: %v", reply.AgreementId(), err)))
				}

				glog.Errorf(logString(fmt.Sprintf("received rejection from producer %v", *reply)))
			}

			// Get rid of the exchange message if there is one
			if wi.MessageId != 0 && !deletedMessage {
				if err := a.deleteMessage(wi.MessageId); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, a.agbotId)))
				}
			}

		} else if workItem.Type() == DATARECEIVEDACK {
			wi := workItem.(CSHandleDataReceivedAck)
			if drAck, err := protocolHandler.ValidateDataReceivedAck(wi.Ack); err != nil {
				glog.Warningf(logString(fmt.Sprintf("discarding message: %v", wi.Ack)))
			} else if ag, err := FindSingleAgreementByAgreementId(a.db, drAck.AgreementId(), citizenscientist.PROTOCOL_NAME); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error querying timed out agreement %v, error: %v", drAck.AgreementId(), err)))
			} else if ag == nil {
				glog.V(3).Infof(logString(fmt.Sprintf("nothing to terminate for agreement %v, no database record.", drAck.AgreementId())))
			} else if _, err := DataNotification(a.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to record data notification, error: %v", err)))
			}

			// Get rid of the exchange message if there is one
			if wi.MessageId != 0 {
				if err := a.deleteMessage(wi.MessageId); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, a.agbotId)))
				}
			}

		} else if workItem.Type() == CANCEL {
			wi := workItem.(CSCancelAgreement)

			if ag, err := FindSingleAgreementByAgreementId(a.db, wi.AgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error querying timed out agreement %v, error: %v", wi.AgreementId, err)))
			} else if ag == nil {
				glog.V(3).Infof(logString(fmt.Sprintf("nothing to terminate for agreement %v, no database record.", wi.AgreementId)))
			} else if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to demarshal policy while trying to cancel %v, error %v", wi.AgreementId, err)))
			} else if err := protocolHandler.TerminateAgreement(pol, ag.CounterPartyAddress, ag.CurrentAgreementId, wi.Reason, bc.Agreements); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error terminating agreement %v on the blockchain: %v", wi.AgreementId, err)))
			}

			if err := DeleteAgreement(a.db, wi.AgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error deleting terminated agreement: %v, error: %v", wi.AgreementId, err)))
			}

		} else {
			glog.Errorf(logString(fmt.Sprintf("received unknown work request: %v", workItem)))
		}

		glog.V(5).Infof(logString(fmt.Sprintf("handled work: %v", workItem)))
		runtime.Gosched()

	}
}

// This function is used to generate a random 32 byte bitstring that is used as the agreement id.
func generateAgreementId(random *rand.Rand) []byte {

	b := make([]byte, 32, 32)
	for i := range b {
		b[i] = byte(random.Intn(256))
	}
	return b
}

func (a *CSAgreementWorker) recordConsumerAgreementState(agreementId string, workloadID string, state string) error {

	glog.V(5).Infof("CSAgreementWorker setting agreement %v state to %v", agreementId, state)

	as := new(exchange.PutAgbotAgreementState)
	as.Workload = workloadID
	as.State = state
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := a.config.AgreementBot.ExchangeURL + "agbots/" + a.agbotId + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(a.httpClient, "PUT", targetURL, a.agbotId, a.token, &as, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof("CSAgreementWorker set agreement %v to state %v", agreementId, state)
			return nil
		}
	}

}

func (a *CSAgreementWorker) deleteMessage(msgId int) error {
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := a.config.AgreementBot.ExchangeURL + "agbots/" + a.agbotId + "/msgs/" + strconv.Itoa(msgId)
	for {
		if err, tpErr := exchange.InvokeExchange(a.httpClient, "DELETE", targetURL, a.agbotId, a.token, nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("deleted message %v", msgId)))
			return nil
		}
	}
}
