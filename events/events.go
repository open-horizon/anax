package events

import (
	"fmt"
	"github.com/open-horizon/anax/persistence"
	"net/url"
	"time"
)

type Event struct {
	Id EventId
}

func (e Event) String() string {
	return fmt.Sprintf("%v", e.Id)
}

type EventId string

// event constants are declared here for all workers to ensure uniqueness of constant values
const (
	// blockchain-related
	NOOP                  EventId = "NOOP"
	AGREEMENT_ACCEPTED    EventId = "AGREEMENT_ACCEPTED"
	AGREEMENT_ENDED       EventId = "AGREEMENT_ENDED"
	AGREEMENT_CREATED     EventId = "AGREEMENT_CREATED"
	AGREEMENT_REGISTERED  EventId = "AGREEMENT_REGISTERED"
	ACCOUNT_FUNDED        EventId = "ACCOUNT_FUNDED"
	BC_CLIENT_INITIALIZED EventId = "BC_CLIENT_INITIALIZED"
	BC_EVENT              EventId = "BC_EVENT"

	// whisper related
	RECEIVED_MSG EventId = "RECEIVED_MSG"
	SUBSCRIBE_TO EventId = "SUBSCRIBE_TO"

	// exchange related
	RECEIVED_EXCHANGE_DEV_MSG EventId = "RECEIVED_EXCHANGE_DEV_MSG"

	// torrent-related
	TORRENT_FAILURE EventId = "TORRENT_FAILURE"
	TORRENT_FETCHED EventId = "TORRENT_FETCHED"

	// container-related
	EXECUTION_FAILED   EventId = "EXECUTION_FAILED"
	EXECUTION_BEGUN    EventId = "EXECUTION_BEGUN"
	WORKLOAD_DESTROYED EventId = "WORKLOAD_DESTROYED"
	CONTAINER_MAINTAIN EventId = "CONTAINER_MAINTAIN"
	LOAD_CONTAINER     EventId = "LOAD_CONTAINER"
	NEW_ETH_CLIENT     EventId = "NEW_ETH_CONTAINER"

	// policy-related
	NEW_POLICY EventId     = "NEW_POLICY"
	CHANGED_POLICY EventId = "CHANGED_POLICY"
	DELETED_POLICY EventId = "DELETED_POLICY"

	// exchange-related
	NEW_DEVICE_REG EventId = "NEW_DEVICE_REG"
	NEW_AGBOT_REG  EventId = "NEW_AGBOT_REG"

	// agreement-related
	AGREEMENT_REACHED        EventId = "AGREEMENT_REACHED"
	DEVICE_AGREEMENTS_SYNCED EventId = "DEVICE_AGREEMENTS_SYNCED"
	DEVICE_CONTAINERS_SYNCED EventId = "DEVICE_CONTAINERS_SYNCED"
)

type EndContractCause string

const (
	AG_TERMINATED EndContractCause = "AG_TERMINATED"
	AG_ERROR      EndContractCause = "AG_ERROR"
	AG_FULFILLED  EndContractCause = "AG_FULFILLED"
)

type Message interface {
	Event() Event
	ShortString() string
}

type LaunchContext interface {
	URL()         url.URL
	Hashes()      map[string]string
	Signatures()  map[string]string
	ShortString() string
}

type AgreementLaunchContext struct {
	AgreementProtocol    string
	AgreementId          string
	Configure            ContainerConfig
	ConfigureRaw         []byte
	EnvironmentAdditions *map[string]string // provided by platform, not but user
}

func (c AgreementLaunchContext) String() string {
	return fmt.Sprintf("AgreementProtocol: %v, AgreementId: %v, Configure: %v, EnvironmentAdditions: %v", c.AgreementProtocol, c.AgreementId, c.Configure, c.EnvironmentAdditions)
}

func (c AgreementLaunchContext) ShortString() string {
	return fmt.Sprintf("AgreementProtocol: %v, AgreementId: %v", c.AgreementProtocol, c.AgreementId)
}

func (c AgreementLaunchContext) URL() url.URL {
	return c.Configure.TorrentURL
}

func (c AgreementLaunchContext) Hashes() map[string]string {
	return c.Configure.ImageHashes
}

func (c AgreementLaunchContext) Signatures() map[string]string {
	return c.Configure.ImageSignatures
}

type ContainerConfig struct {
	TorrentURL          url.URL           `json:"torrent_url"`
	ImageHashes         map[string]string `json:"image_hashes"`
	ImageSignatures     map[string]string `json:"image_signatures"` // cryptographic signatures per-image
	Deployment          string            `json:"deployment"`       // JSON docker-compose like
	DeploymentSignature string            `json:"deployment_signature"`
	DeploymentUserInfo  string            `json:"deployment_user_info"`
}

func (c ContainerConfig) String() string {
	return fmt.Sprintf("TorrentURL: %v, ImageHashes: %v, ImageSignatures: %v, Deployment: %v, DeploymentSignature: %v, DeploymentUserInfo: %v", c.TorrentURL.String(), c.ImageHashes, c.ImageSignatures, c.Deployment, c.DeploymentSignature, c.DeploymentUserInfo)
}

func NewContainerConfig(torrentURL url.URL, imageHashes map[string]string, imageSignatures map[string]string, deployment string, deploymentSignature string, deploymentUserInfo string) *ContainerConfig {
	return &ContainerConfig{
		TorrentURL:          torrentURL,
		ImageHashes:         imageHashes,
		ImageSignatures:     imageSignatures,
		Deployment:          deployment,
		DeploymentSignature: deploymentSignature,
		DeploymentUserInfo:  deploymentUserInfo,
	}
}

type BlockchainConfig struct {
	Type string
	Name string
}

type ContainerLaunchContext struct {
	Configure            ContainerConfig
	EnvironmentAdditions *map[string]string
	Blockchain           BlockchainConfig
}

func (c ContainerLaunchContext) String() string {
	return fmt.Sprintf("ContainerConfig: %v, EnvironmentAdditions: %v, Blockchain: %v", c.Configure, c.EnvironmentAdditions, c.Blockchain)
}

func (c ContainerLaunchContext) ShortString() string {
	return c.String()
}

func (c ContainerLaunchContext) URL() url.URL {
	return c.Configure.TorrentURL
}

func (c ContainerLaunchContext) Hashes() map[string]string {
	return c.Configure.ImageHashes
}

func (c ContainerLaunchContext) Signatures() map[string]string {
	return c.Configure.ImageSignatures
}

func NewContainerLaunchContext(config *ContainerConfig, envAdds *map[string]string, bc BlockchainConfig) *ContainerLaunchContext {
	return &ContainerLaunchContext{
		Configure:            *config,
		EnvironmentAdditions: envAdds,
		Blockchain:           bc,
	}
}

// Anax device side fires this event when it needs to download and load a container.
type LoadContainerMessage struct {
	event         Event
	launchContext *ContainerLaunchContext
}

func (e LoadContainerMessage) String() string {
	return fmt.Sprintf("event: %v, launch context: %v", e.event, e.launchContext)
}

func (e LoadContainerMessage) ShortString() string {
	return e.String()
}

func (e *LoadContainerMessage) Event() Event {
	return e.event
}

func (e *LoadContainerMessage) LaunchContext() *ContainerLaunchContext {
	return e.launchContext
}

func NewLoadContainerMessage(id EventId, lc *ContainerLaunchContext) *LoadContainerMessage {

	return &LoadContainerMessage{
		event: Event{
			Id: id,
		},
		launchContext: lc,
	}
}

// This event indicates that a new microservice has been created in the form of a policy file
type PolicyCreatedMessage struct {
	event    Event
	fileName string
}

func (e PolicyCreatedMessage) String() string {
	return fmt.Sprintf("event: %v, file: %v", e.event, e.fileName)
}

func (e PolicyCreatedMessage) ShortString() string {
	return e.String()
}

func (e *PolicyCreatedMessage) Event() Event {
	return e.event
}

func (e *PolicyCreatedMessage) PolicyFile() string {
	return e.fileName
}

func NewPolicyCreatedMessage(id EventId, policyFileName string) *PolicyCreatedMessage {

	return &PolicyCreatedMessage{
		event: Event{
			Id: id,
		},
		fileName: policyFileName,
	}
}

// This event indicates that a policy file has changed. It might also be a new policy file in the agbot.
type PolicyChangedMessage struct {
	event    Event
	fileName string
	name     string
	policy   string
}

func (e PolicyChangedMessage) String() string {
	return fmt.Sprintf("event: %v, file: %v, name: %v, policy: %v", e.event, e.fileName, e.name, e.policy)
}

func (e PolicyChangedMessage) ShortString() string {
	return e.String()
}

func (e *PolicyChangedMessage) Event() Event {
	return e.event
}

func (e *PolicyChangedMessage) PolicyFile() string {
	return e.fileName
}

func (e *PolicyChangedMessage) PolicyName() string {
	return e.name
}

func (e *PolicyChangedMessage) PolicyString() string {
	return e.policy
}

func NewPolicyChangedMessage(id EventId, policyFileName string, policyName string, policy string) *PolicyChangedMessage {

	return &PolicyChangedMessage{
		event: Event{
			Id: id,
		},
		fileName: policyFileName,
		name:     policyName,
		policy:   policy,
	}
}

// This event indicates that a policy file was deleted.
type PolicyDeletedMessage struct {
	event    Event
	fileName string
	name     string
	policy   string
}

func (e PolicyDeletedMessage) String() string {
	return fmt.Sprintf("event: %v, file: %v, name: %v, policy: %v", e.event, e.fileName, e.name, e.policy)
}

func (e PolicyDeletedMessage) ShortString() string {
	return e.String()
}

func (e *PolicyDeletedMessage) Event() Event {
	return e.event
}

func (e *PolicyDeletedMessage) PolicyFile() string {
	return e.fileName
}

func (e *PolicyDeletedMessage) PolicyName() string {
	return e.name
}

func (e *PolicyDeletedMessage) PolicyString() string {
	return e.policy
}

func NewPolicyDeletedMessage(id EventId, policyFileName string, policyName string, policy string) *PolicyDeletedMessage {

	return &PolicyDeletedMessage{
		event: Event{
			Id: id,
		},
		fileName: policyFileName,
		name:     policyName,
		policy:   policy,
	}
}

// This event indicates that the edge device has been registered in the exchange
type EdgeRegisteredExchangeMessage struct {
	event Event
	token string
}

func (e EdgeRegisteredExchangeMessage) String() string {
	return fmt.Sprintf("event: %v, token: %v", e.event, e.token)
}

func (e EdgeRegisteredExchangeMessage) ShortString() string {
	return e.String()
}

func (e *EdgeRegisteredExchangeMessage) Event() Event {
	return e.event
}

func (e *EdgeRegisteredExchangeMessage) Token() string {
	return e.token
}

func NewEdgeRegisteredExchangeMessage(evId EventId, token string) *EdgeRegisteredExchangeMessage {

	return &EdgeRegisteredExchangeMessage{
		event: Event{
			Id: evId,
		},
		token: token,
	}
}

// Anax device side fires this event when an agreement is reached so that it can begin
// downloading containers. The Agreement is not final until it is seen in the blockchain.
type AgreementReachedMessage struct {
	event         Event
	launchContext *AgreementLaunchContext
}

func (e AgreementReachedMessage) String() string {
	return fmt.Sprintf("event: %v, launch context: %v", e.event, e.launchContext)
}

func (e AgreementReachedMessage) ShortString() string {
	return fmt.Sprintf("event: %v, launch context: %v", e.event, e.launchContext.ShortString())
}

func (e *AgreementReachedMessage) Event() Event {
	return e.event
}

func (e *AgreementReachedMessage) LaunchContext() *AgreementLaunchContext {
	return e.launchContext
}

func NewAgreementMessage(id EventId, lc *AgreementLaunchContext) *AgreementReachedMessage {

	return &AgreementReachedMessage{
		event: Event{
			Id: id,
		},
		launchContext: lc,
	}
}

type WhisperSubscribeToMessage struct {
	event Event
	topic string
}

func (e WhisperSubscribeToMessage) String() string {
	return fmt.Sprintf("event: %v, topic: %v", e.event, e.topic)
}

func (e WhisperSubscribeToMessage) ShortString() string {
	return e.String()
}

func (e *WhisperSubscribeToMessage) Event() Event {
	return e.event
}

func (e *WhisperSubscribeToMessage) Topic() string {
	return e.topic
}

func NewWhisperSubscribeToMessage(id EventId, topic string) *WhisperSubscribeToMessage {

	return &WhisperSubscribeToMessage{
		event: Event{
			Id: id,
		},
		topic: topic,
	}
}

type WhisperReceivedMessage struct {
	event   Event
	payload string
	topics  []string
	from    string
	to      string
}

func (e WhisperReceivedMessage) ShortString() string {
	return fmt.Sprintf("Event: %v, From: %v, To: %v, Topics: %v, Payload: %v", e.event, e.from, e.to, e.topics, e.payload[:40])
}

func (e WhisperReceivedMessage) String() string {
	return fmt.Sprintf("event: %v, payload: %v", e.event, e.payload)
}

func (e *WhisperReceivedMessage) Event() Event {
	return e.event
}

func (e *WhisperReceivedMessage) Payload() string {
	return e.payload
}

func (e *WhisperReceivedMessage) Topics() []string {
	return e.topics
}

func (e *WhisperReceivedMessage) From() string {
	return e.from
}

func (e *WhisperReceivedMessage) To() string {
	return e.to
}

func NewWhisperReceivedMessage(id EventId, payload string, from string, to string) *WhisperReceivedMessage {

	return &WhisperReceivedMessage{
		event: Event{
			Id: id,
		},
		payload: payload,
		from:    from,
		to:      to,
	}
}

type TorrentMessage struct {
	event         Event
	ImageFiles    []string
	LaunchContext interface{}
}

// fulfill interface of events.Message
func (b *TorrentMessage) Event() Event {
	return b.event
}

func (b *TorrentMessage) String() string {
	return fmt.Sprintf("event: %v, imageFiles: %v, launchContext: %v", b.event, b.ImageFiles, b.LaunchContext)
}

func (b *TorrentMessage) ShortString() string {
	return fmt.Sprintf("event: %v, imageFiles: %v, launchContext: %v", b.event, b.ImageFiles, b.LaunchContext)
}

func NewTorrentMessage(id EventId, imageFiles []string, launchContext interface{}) *TorrentMessage {

	return &TorrentMessage{
		event: Event{
			Id: id,
		},
		ImageFiles:    imageFiles,
		LaunchContext: launchContext,
	}
}

// Governance messages
type GovernanceMaintenanceMessage struct {
	event             Event
	AgreementProtocol string
	AgreementId       string
	Deployment        map[string]persistence.ServiceConfig
}

func (m *GovernanceMaintenanceMessage) Event() Event {
	return m.event
}

func (m GovernanceMaintenanceMessage) String() string {
	return fmt.Sprintf("Event: %v, AgreementProtocol: %v, AgreementId: %v, Deployment: %v", m.event, m.AgreementProtocol, m.AgreementId, m.Deployment)
}

func (m GovernanceMaintenanceMessage) ShortString() string {
    depStr := ""
    for key, _ := range m.Deployment {
        depStr = depStr + key + ","
    }
	return fmt.Sprintf("Event: %v, AgreementProtocol: %v, AgreementId: %v, Deployment Services: %v", m.event, m.AgreementProtocol, m.AgreementId, depStr)
}

type GovernanceCancelationMessage struct {
	GovernanceMaintenanceMessage
	Message
	Cause EndContractCause
}

func (m *GovernanceCancelationMessage) Event() Event {
	return m.event
}

func (m GovernanceCancelationMessage) String() string {
	return fmt.Sprintf("Event: %v, AgreementProtocol: %v, AgreementId: %v, Deployment: %v, Cause: %v", m.event, m.AgreementProtocol, m.AgreementId, persistence.ServiceConfigNames(&m.Deployment), m.Cause)
}

func (m GovernanceCancelationMessage) ShortString() string {
	return m.String()
}

func NewGovernanceMaintenanceMessage(id EventId, protocol string, agreementId string, deployment map[string]persistence.ServiceConfig) *GovernanceMaintenanceMessage {
	return &GovernanceMaintenanceMessage{
		event: Event{
			Id: id,
		},
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		Deployment:        deployment,
	}
}

func NewGovernanceCancelationMessage(id EventId, cause EndContractCause, protocol string, agreementId string, deployment map[string]persistence.ServiceConfig) *GovernanceCancelationMessage {

	govMaint := NewGovernanceMaintenanceMessage(id, protocol, agreementId, deployment)

	return &GovernanceCancelationMessage{
		GovernanceMaintenanceMessage: *govMaint,
		Cause: cause,
	}
}

//Workload messages
type WorkloadMessage struct {
	event             Event
	AgreementProtocol string
	AgreementId       string
	Deployment        map[string]persistence.ServiceConfig
}

func (m WorkloadMessage) String() string {
	return fmt.Sprintf("event: %v, AgreementProtocol: %v, AgreementId: %v, Deployment: %v", m.event.Id, m.AgreementProtocol, m.AgreementId, persistence.ServiceConfigNames(&m.Deployment))
}

func (m WorkloadMessage) ShortString() string {
	return m.String()
}

func (b WorkloadMessage) Event() Event {
	return b.event
}

func NewWorkloadMessage(id EventId, protocol string, agreementId string, deployment map[string]persistence.ServiceConfig) *WorkloadMessage {

	return &WorkloadMessage{
		event: Event{
			Id: id,
		},
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		Deployment:        deployment,
	}
}

//Container messages
type ContainerMessage struct {
	event             Event
	LaunchContext     ContainerLaunchContext
}

func (m ContainerMessage) String() string {
	return fmt.Sprintf("event: %v, LaunchContext: %v", m.event.Id, m.LaunchContext)
}

func (m ContainerMessage) ShortString() string {
	return m.String()
}

func (b ContainerMessage) Event() Event {
	return b.event
}

func NewContainerMessage(id EventId, lc ContainerLaunchContext) *ContainerMessage {

	return &ContainerMessage{
		event: Event{
			Id: id,
		},
		LaunchContext: lc,
	}
}

// Api messages
type ApiAgreementCancelationMessage struct {
	event             Event
	AgreementProtocol string
	AgreementId       string
	Deployment        map[string]persistence.ServiceConfig
	Cause             EndContractCause
}

func (m *ApiAgreementCancelationMessage) Event() Event {
	return m.event
}

func (m ApiAgreementCancelationMessage) String() string {
	return fmt.Sprintf("Event: %v, AgreementProtocol: %v, AgreementId: %v, Deployment: %v, Cause: %v", m.event, m.AgreementProtocol, m.AgreementId, persistence.ServiceConfigNames(&m.Deployment), m.Cause)
}

func (m ApiAgreementCancelationMessage) ShortString() string {
	return m.String()
}

func NewApiAgreementCancelationMessage(id EventId, cause EndContractCause, protocol string, agreementId string, deployment map[string]persistence.ServiceConfig) *ApiAgreementCancelationMessage {
	return &ApiAgreementCancelationMessage{
		event: Event{
			Id: id,
		},
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		Deployment:        deployment,
		Cause:             cause,
	}
}

// Agbot Api messages
type ABApiAgreementCancelationMessage struct {
	event             Event
	AgreementProtocol string
	AgreementId       string
	Reason            uint64
}

func (m *ABApiAgreementCancelationMessage) Event() Event {
	return m.event
}

func (m ABApiAgreementCancelationMessage) String() string {
	return fmt.Sprintf("Event: %v, AgreementProtocol: %v, AgreementId: %v, Reason: %v", m.event, m.AgreementProtocol, m.AgreementId, m.Reason)
}

func (m ABApiAgreementCancelationMessage) ShortString() string {
	return m.String()
}

func NewABApiAgreementCancelationMessage(id EventId, reason uint64, protocol string, agreementId string) *ABApiAgreementCancelationMessage {
	return &ABApiAgreementCancelationMessage{
		event: Event{
			Id: id,
		},
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		Reason:            reason,
	}
}

// Initialization and restart messages
type InitAgreementCancelationMessage struct {
	event             Event
	AgreementProtocol string
	AgreementId       string
	Deployment        map[string]persistence.ServiceConfig
	Reason            uint
}

func (m *InitAgreementCancelationMessage) Event() Event {
	return m.event
}

func (m InitAgreementCancelationMessage) String() string {
	return fmt.Sprintf("Event: %v, AgreementProtocol: %v, AgreementId: %v, Deployment: %v, Reason: %v", m.event, m.AgreementProtocol, m.AgreementId, persistence.ServiceConfigNames(&m.Deployment), m.Reason)
}

func (m InitAgreementCancelationMessage) ShortString() string {
	return m.String()
}

func NewInitAgreementCancelationMessage(id EventId, reason uint, protocol string, agreementId string, deployment map[string]persistence.ServiceConfig) *InitAgreementCancelationMessage {
	return &InitAgreementCancelationMessage{
		event: Event{
			Id: id,
		},
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		Deployment:        deployment,
		Reason:            reason,
	}
}

// Account funded message
type AccountFundedMessage struct {
	event   Event
	Account string
	Time    uint64
}

func (m *AccountFundedMessage) Event() Event {
	return m.event
}

func (m AccountFundedMessage) String() string {
	return fmt.Sprintf("Event: %v, Account: %v, Time: %v", m.event.Id, m.Account, m.Time)
}

func (m AccountFundedMessage) ShortString() string {
	return m.String()
}

func NewAccountFundedMessage(id EventId, acct string) *AccountFundedMessage {
	return &AccountFundedMessage{
		event: Event{
			Id: id,
		},
		Account: acct,
		Time:    uint64(time.Now().Unix()),
	}
}

// Blockchain client initialized message
type BlockchainClientInitializedMessage struct {
	event Event
	Time  uint64
}

func (m *BlockchainClientInitializedMessage) Event() Event {
	return m.event
}

func (m BlockchainClientInitializedMessage) String() string {
	return fmt.Sprintf("Event: %v, Time: %v", m.event, m.Time)
}

func (m BlockchainClientInitializedMessage) ShortString() string {
	return m.String()
}

func NewBlockchainClientInitializedMessage(id EventId) *BlockchainClientInitializedMessage {
	return &BlockchainClientInitializedMessage{
		event: Event{
			Id: id,
		},
		Time: uint64(time.Now().Unix()),
	}
}

// Blockchain event occurred
type EthBlockchainEventMessage struct {
	event    Event
	rawEvent string
	protocol string
	Time     uint64
}

func (m *EthBlockchainEventMessage) Event() Event {
	return m.event
}

func (m *EthBlockchainEventMessage) RawEvent() string {
	return m.rawEvent
}

func (m EthBlockchainEventMessage) String() string {
	return fmt.Sprintf("Event: %v, Protocol: %v, Raw Event: %v, Time: %v", m.event, m.protocol, m.rawEvent, m.Time)
}

func (m EthBlockchainEventMessage) ShortString() string {
	return fmt.Sprintf("Event: %v, Protocol: %v, Time: %v", m.event, m.protocol, m.Time)
}

func NewEthBlockchainEventMessage(id EventId, ev string, protocol string) *EthBlockchainEventMessage {
	return &EthBlockchainEventMessage{
		event: Event{
			Id: id,
		},
		rawEvent: ev,
		protocol: protocol,
		Time:     uint64(time.Now().Unix()),
	}
}

// Exchange message received event occurred
type ExchangeDeviceMessage struct {
	event           Event
	exchangeMessage []byte
	protocolMessage string
	Time            uint64
}

func (m *ExchangeDeviceMessage) Event() Event {
	return m.event
}

func (m *ExchangeDeviceMessage) ExchangeMessage() []byte {
	return m.exchangeMessage
}

func (m *ExchangeDeviceMessage) ProtocolMessage() string {
	return m.protocolMessage
}

func (m *ExchangeDeviceMessage) ShortProtocolMessage() string {
	end := 200
	if len(m.protocolMessage) < end {
		end = len(m.protocolMessage)
	}
	return m.protocolMessage[:end]
}

func (m ExchangeDeviceMessage) String() string {
	return fmt.Sprintf("Event: %v, ProtocolMessage: %v, Time: %v, ExchangeMessage: %s", m.event, m.protocolMessage, m.Time, m.exchangeMessage)
}

func (m ExchangeDeviceMessage) ShortString() string {
	return fmt.Sprintf("Event: %v, ProtocolMessage: %v, Time: %v", m.event, m.ShortProtocolMessage(), m.Time)
}

func NewExchangeDeviceMessage(id EventId, exMsg []byte, pMsg string) *ExchangeDeviceMessage {
	return &ExchangeDeviceMessage{
		event: Event{
			Id: id,
		},
		exchangeMessage: exMsg,
		protocolMessage: pMsg,
		Time:            uint64(time.Now().Unix()),
	}
}

// Make sure eth container is up and running
type NewEthContainerMessage struct {
	event             Event
	exchangeURL       string
	exchangeId        string
	exchangeToken     string
	Time              uint64
}

func (m *NewEthContainerMessage) Event() Event {
	return m.event
}

func (m *NewEthContainerMessage) ExchangeURL() string {
	return m.exchangeURL
}

func (m *NewEthContainerMessage) ExchangeId() string {
	return m.exchangeId
}
func (m *NewEthContainerMessage) ExchangeToken() string {
	return m.exchangeToken
}

func (m NewEthContainerMessage) String() string {
	return fmt.Sprintf("Event: %v, Time: %v, ExchangeURL: %v, ExchangeId: %v, ExchangeToken: %v", m.event, m.Time, m.exchangeURL, m.exchangeId, m.exchangeToken)
}

func (m NewEthContainerMessage) ShortString() string {
	return m.String()
}

func NewNewEthContainerMessage(id EventId, exchangeURL string, exchangeId string, exchangeToken string) *NewEthContainerMessage {
	return &NewEthContainerMessage{
		event: Event{
			Id: id,
		},
		exchangeURL:      exchangeURL,
		exchangeId:       exchangeId,
		exchangeToken:    exchangeToken,
		Time:             uint64(time.Now().Unix()),
	}
}

// Tell everyone that the device side of anax has synced up it's agreements wiht the exchange and blockchain
type DeviceAgreementsSyncedMessage struct {
	event     Event
	Completed bool
	Time      uint64
}

func (m *DeviceAgreementsSyncedMessage) Event() Event {
	return m.event
}

func (m *DeviceAgreementsSyncedMessage) IsCompleted() bool {
	return m.Completed
}

func (m DeviceAgreementsSyncedMessage) String() string {
	return fmt.Sprintf("Event: %v, Completed: %v, Time: %v", m.event, m.Completed, m.Time)
}

func (m DeviceAgreementsSyncedMessage) ShortString() string {
	return m.String()
}

func NewDeviceAgreementsSyncedMessage(id EventId, completed bool) *DeviceAgreementsSyncedMessage {
	return &DeviceAgreementsSyncedMessage{
		event: Event{
			Id: id,
		},
		Completed: completed,
		Time:      uint64(time.Now().Unix()),
	}
}

// Tell everyone that the device side of anax has synced up it's containers with the local DB
type DeviceContainersSyncedMessage struct {
	event     Event
	Completed bool
	Time      uint64
}

func (m *DeviceContainersSyncedMessage) Event() Event {
	return m.event
}

func (m *DeviceContainersSyncedMessage) IsCompleted() bool {
	return m.Completed
}

func (m DeviceContainersSyncedMessage) String() string {
	return fmt.Sprintf("Event: %v, Completed: %v, Time: %v", m.event, m.Completed, m.Time)
}

func (m DeviceContainersSyncedMessage) ShortString() string {
	return m.String()
}

func NewDeviceContainersSyncedMessage(id EventId, completed bool) *DeviceContainersSyncedMessage {
	return &DeviceContainersSyncedMessage{
		event: Event{
			Id: id,
		},
		Completed: completed,
		Time:      uint64(time.Now().Unix()),
	}
}
