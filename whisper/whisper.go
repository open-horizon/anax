package whisper

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"net/http"
	"repo.hovitos.engineering/MTN/anax/blockchain"
	"repo.hovitos.engineering/MTN/anax/config"
	"repo.hovitos.engineering/MTN/anax/events"
	"repo.hovitos.engineering/MTN/anax/persistence"
	"repo.hovitos.engineering/MTN/anax/worker"
	"repo.hovitos.engineering/MTN/go-solidity/contract_api"
	gwhisper "repo.hovitos.engineering/MTN/go-whisper"
	"runtime"
	"time"
)

type WhisperWorker struct {
	worker.Worker            // embedded field
	db                       *bolt.DB
	whisperDirectoryContract *contract_api.SolidityContract
}

func NewWhisperWorker(config *config.Config, db *bolt.DB) *WhisperWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 200)

	worker := &WhisperWorker{
		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},

		db: db,
		whisperDirectoryContract: nil, // to be overwritten in load process
	}

	worker.start()
	return worker
}

// TODO: consolidate with verification in image download impl
func verifyDeploymentSignature(pubKeyFile string, configure *gwhisper.Configure) (bool, error) {
	pubKeyData, err := ioutil.ReadFile(pubKeyFile)
	if err != nil {
		return false, err
	}

	block, _ := pem.Decode(pubKeyData)
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false, err
	}

	glog.Infof("Using RSA pubkey: %v", publicKey)

	glog.V(5).Infof("Checking signature of deployment string: %v", configure.Deployment)

	if decoded, err := base64.StdEncoding.DecodeString(configure.DeploymentSignature); err != nil {
		return false, fmt.Errorf("Error decoding base64 signature: %v, Error: %v", configure.DeploymentSignature, err)
	} else {

		hasher := sha256.New()
		if _, err := io.WriteString(hasher, configure.Deployment); err != nil {
			return false, err
		} else {
			if err := rsa.VerifyPSS(publicKey.(*rsa.PublicKey), crypto.SHA256, hasher.Sum(nil), decoded, nil); err != nil {
				return false, err
			} else {
				return true, nil
			}
		}
	}
}

func (w *WhisperWorker) generateDirectConfigure(configure *gwhisper.Configure, configureRaw []byte) (*WhisperConfigMessage, error) {

	nonceFilter := func(nonce string) persistence.ECFilter {
		return func(e persistence.EstablishedContract) bool { return e.ConfigureNonce == nonce }
	}

	if contracts, err := persistence.FindEstablishedContracts(w.db, []persistence.ECFilter{persistence.UnarchivedECFilter(), nonceFilter(configure.ConfigureNonce)}); err != nil {
		return nil, err
	} else if len(contracts) > 1 {
		return nil, fmt.Errorf("Expected only one record with nonce: %v. Found: %v", configure.ConfigureNonce, contracts)
	} else if len(contracts) == 0 {
		return nil, fmt.Errorf("No matching db contract record for ConfigureNonce: %v, ignoring.", configure.ConfigureNonce)
	} else {
		match := contracts[0]
		glog.Infof("Found matching nonce for established contract record: %v", match)

		verified, err := verifyDeploymentSignature(w.Config.PublicKeyPath, configure)
		if !verified {
			glog.Errorf("DeploymentSignature did not match deployment, refusing to process. Encapsulated error: %v", err)
			return NewWhisperConfigMessage(events.CONFIGURE_ERROR, match.ContractAddress, match.CurrentAgreementId, configure, configureRaw, match.EnvironmentAdditions), nil // signals failure that should cause contract cancelation
		} else {
			glog.Infof("Deployment description matched provided signature: %v", configure.DeploymentSignature)

			if _, err := persistence.ContractStateConfigured(w.db, match.ContractAddress); err != nil {
				return nil, err
			} else {
				glog.Infof("Generating direct configure message for contract %v, agreement %v", match.ContractAddress, match.CurrentAgreementId)
				return NewWhisperConfigMessage(events.DIRECT_CONFIGURE, match.ContractAddress, match.CurrentAgreementId, configure, configureRaw, match.EnvironmentAdditions), nil
			}
		}
	}
}

func (w *WhisperWorker) pollIncoming() {
	go func() {
		// block worker until discovered values in config are populated
		loaded := false
		now := func() int64 { return time.Now().Unix() }
		printed := now() - 31

		for !loaded {
			e := now()
			if e-printed > 30 {
				glog.Infof("Waiting for directory values from blockchain reads to be set on config object. Config: %v", w.Config)
				printed = now()
			}

			loaded = (w.Config.BlockchainAccountId != "" && w.Config.BlockchainDirectoryAddress != "")
			time.Sleep(1 * time.Second)
		}

		// load whisper directory contract; TODO: consolidate w/ blockchain contract loading
		if directoryContract, err := blockchain.DirectoryContract(w.Config.GethURL, w.Config.BlockchainAccountId, w.Config.BlockchainDirectoryAddress); err != nil {
			glog.Errorf("Unable to read directory contract: %v", err)
		} else if c, err := blockchain.ContractInDirectory(w.Config.GethURL, "whisper_directory", w.Config.BlockchainAccountId, directoryContract); err != nil {
			glog.Error(err)
		} else {
			w.whisperDirectoryContract = c
		}

		// on call to read(), reader will block polling for proper topic messages; will return a list of them if any arrive
		// N.B. logical OR on topics in nested array, logical ANDs between topics in top-level array

		read := gwhisper.WhisperReader(w.Config.GethURL, [][]string{[]string{"micropayment", "configure"}})

		for {
			glog.V(4).Infof("Reading from whisper")

			// poll really often and do so until we get a message
			if results, err := read(time.Duration(45)*time.Second, 600); err != nil {
				glog.Errorf("Error reading messages from whisper: %v", err)
			} else {
				glog.V(4).Info("Whisper read returned results or timed-out waiting")
				for _, r := range results {
					glog.V(3).Infof("Saw result w/ hash: %v", r.Hash)

					// attempt deserialization of Provider message from Result payload
					var whisperProviderMsg gwhisper.WhisperProviderMsg

					if err := json.Unmarshal([]byte(r.Payload), &whisperProviderMsg); err != nil {
						if glog.V(5) {
							glog.Errorf("Error deserializing whisperprovidermsg. Data: %s, Error: %v", r.Payload, err)
						} else {
							glog.Errorf("Error deserializing whisperprovidermsg. Error: %v", err)
						}
					} else {
						switch whisperProviderMsg.Type {

						case gwhisper.T_CONFIGURE:
							var configure gwhisper.Configure
							configureRaw := []byte(r.Payload)

							if err := json.Unmarshal(configureRaw, &configure); err != nil {
								if glog.V(5) {
									glog.Errorf("Error deserializing provider message %v. Data: %s, Error: %v", whisperProviderMsg.Type, r.Payload, err)
								} else {
									glog.Errorf("Error deserializing provider message %v. Error: %v", whisperProviderMsg.Type, err)
								}
							} else {
								glog.Infof("Received provider Configure msg: %v", configure)

								// security check! must make sure "configure" message has nonce matching an established contract *and* expire that nonce before sending DIRECT_CONFIGURE message

								msg, err := w.generateDirectConfigure(&configure, configureRaw)
								if err != nil {
									glog.Errorf("No direct configure message generated: %v", err)
									continue // so we don't enqueue a possibly nil message
								} else {
									// msg well-formed even if it failed validation, send an ack msg to provider
									// flip the params from the message just received
									if ack, err := gwhisper.TopicMsgParams(r.To, r.From, []string{"configure"}, msg.AgreementLaunchContext.AgreementId, 240, 100); err != nil {
										glog.Errorf("Unable to prep Ack: %v. Error: %v", ack, err)
									} else if _, err := gwhisper.WhisperSend(nil, w.Config.GethURL, gwhisper.POST, ack, 3); err != nil {
										glog.Errorf("Unable to send Ack: %v. Error: %v", ack, err)
									}
								}
								// enqueue message; could be the result of an error

								w.Messages <- msg
							}

						case gwhisper.T_MICROPAYMENT:
							var micropayment gwhisper.Micropayment

							if err := json.Unmarshal([]byte(r.Payload), &micropayment); err != nil {
								if glog.V(5) {
									glog.Errorf("Error deserializing provider message %v. Data: %s, Error: %v", whisperProviderMsg.Type, r.Payload, err)
								} else {
									glog.Errorf("Error deserializing provider message %v. Error: %v", whisperProviderMsg.Type, err)
								}
							} else {
								glog.V(3).Infof("Received provider micropayment msg: %v in original payload: %v", micropayment, r)

								if _, _, realPayer, err := persistence.LastMicropayment(w.db, micropayment.AgreementId); err != nil {
									glog.Error(err)
								} else if realPayer == "" {
									glog.Infof("Unable to locate payment record for agreement, skipping recording micropayment: %v", micropayment)
								} else if realPayerWhisperId, err := w.whisperIdByPayer(realPayer); err != nil {
									glog.Error(err)
								} else if realPayerWhisperId != r.From {
									glog.Errorf("Refusing to record payment from whisper account %v: real payer account id %v registered with another whisper account id: %v", r.From, realPayer, realPayerWhisperId)

								} else if err := persistence.RecordMicropayment(w.db, micropayment.AgreementId, micropayment.AmountToDate, micropayment.Recorded); err != nil {
									glog.Errorf("Failed to write micropayment %v. Error: %v", micropayment, err)
								} else {

									glog.Infof("Recorded micropayment %v", micropayment)
								}
							}
						}
					}
				}
			}

			time.Sleep(1 * time.Second)
		}
	}()
}

func (w *WhisperWorker) whisperIdByPayer(payerAccountAddress string) (string, error) {
	glog.V(3).Infof("Looking up whisper ID by given payer account address: %v", payerAccountAddress)
	if bkId, err := w.whisperDirectoryContract.Invoke_method("get_entry", blockchain.ContractParam(payerAccountAddress)); err != nil {
		return "", err
	} else {
		return bkId.(string), nil
	}
}

func DoAnnounce(client *http.Client, url string, from string, to string, announce *gwhisper.Announce) error {
	pay, err := json.Marshal(announce)
	if err != nil {
		glog.Errorf("Unable to serialize announce message: %v", err)
	}

	glog.V(3).Infof("Sending whisper announce message: %v to: %v", string(pay), to)

	// this is to last long enough to be read by even an overloaded governor but still expire before a new worker might try to pick up the contract
	announceMsg, err := gwhisper.TopicMsgParams(from, to, []string{"announce"}, string(pay), 900, 50)
	if err != nil {
		return err
	}

	_, err = gwhisper.WhisperSend(client, url, gwhisper.POST, announceMsg, 3)
	if err != nil {
		return err
	}
	return nil
}

func (w *WhisperWorker) start() {
	client := &http.Client{
		Timeout: time.Second * 240,
	}

	go func() {
		for {
			glog.V(4).Infof("WhisperWorker command processor blocking waiting to receive incoming commands")
			command := <-w.Commands

			switch command.(type) {
			case *AnnounceCommand:
				cmd := command.(*AnnounceCommand)
				glog.V(2).Infof("AnnounceCommand: ContractId: %s, AgreementId: %s, WhisperId: %s, and ConfigureNonce: %s", cmd.ContractId, cmd.AgreementId, cmd.WhisperId, cmd.ConfigureNonce)

				if whisperId, err := gwhisper.AccountId(w.Config.GethURL); err != nil {
					glog.Error(err)
				} else {
					err = DoAnnounce(client, w.Config.GethURL, whisperId, cmd.WhisperId, &gwhisper.Announce{AgreementId: cmd.AgreementId, ConfigureNonce: cmd.ConfigureNonce})
					if err != nil {
						glog.Error(err)
					}

					glog.Infof("Sent announce-topic message to provider with whisper ID %v on agreement %v", cmd.WhisperId, cmd.AgreementId)
				}

			default:
				glog.Errorf("Unsupported message(%T): %v", command, command)

			}

			runtime.Gosched()
		}
	}()

	// start polling for incoming messages
	w.pollIncoming()
}

type AnnounceCommand struct {
	ContractId     string
	AgreementId    string
	WhisperId      string
	ConfigureNonce string
}

func (w *WhisperWorker) NewAnnounceCommand(contractId string, agreementId string, whisperId string, configureNonce string) *AnnounceCommand {
	return &AnnounceCommand{
		ContractId:     contractId,
		AgreementId:    agreementId,
		WhisperId:      whisperId,
		ConfigureNonce: configureNonce,
	}
}

type WhisperConfigMessage struct {
	event                  events.Event
	AgreementLaunchContext *events.AgreementLaunchContext
}

func (w *WhisperConfigMessage) Event() events.Event {
	return w.event
}

func NewWhisperConfigMessage(id events.EventId, contractId string, agreementId string, configure *gwhisper.Configure, configureRaw []byte, environmentAdditions map[string]string) *WhisperConfigMessage {
	return &WhisperConfigMessage{
		event: events.Event{
			Id: id,
		},
		AgreementLaunchContext: &events.AgreementLaunchContext{
			ContractId:           contractId,
			AgreementId:          agreementId,
			Configure:            configure,
			ConfigureRaw:         configureRaw,
			EnvironmentAdditions: &environmentAdditions,
		},
	}
}
