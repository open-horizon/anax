package events

import (
	"fmt"
	"github.com/open-horizon/anax/persistence"
	gwhisper "github.com/open-horizon/go-whisper"
	"time"
)

type Event struct {
	Id EventId
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

	// whisper related
	RECEIVED_MSG EventId = "RECEIVED_MSG"
	SUBSCRIBE_TO EventId = "SUBSCRIBE_TO"

	// torrent-related
	TORRENT_FAILURE EventId = "TORRENT_FAILURE"
	TORRENT_FETCHED EventId = "TORRENT_FETCHED"

	// container-related
	EXECUTION_FAILED   EventId = "EXECUTION_FAILED"
	EXECUTION_BEGUN    EventId = "EXECUTION_BEGUN"
	PATTERN_DESTROYED  EventId = "PATTERN_DESTROYED"
	CONTAINER_MAINTAIN EventId = "CONTAINER_MAINTAIN"

	// policy-related
	NEW_POLICY    EventId = "NEW_POLICY"
	NEW_AB_POLICY EventId = "NEW_AB_POLICY"

	// exchange-related
	NEW_DEVICE_REG EventId = "NEW_DEVICE_REG"
	NEW_AGBOT_REG  EventId = "NEW_AGBOT_REG"

	// agreement-related
	AGREEMENT_REACHED EventId = "AGREEMENT_REACHED"
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

type AgreementLaunchContext struct {
	AgreementProtocol    string
	AgreementId          string
	Configure            *gwhisper.Configure
	ConfigureRaw         []byte
	EnvironmentAdditions *map[string]string // provided by platform, not but user
}

func (c AgreementLaunchContext) String() string {
	return fmt.Sprintf("AgreementProtocol: %v, AgreementId: %v, Configure: %v, EnvironmentAdditions: %v", c.AgreementProtocol, c.AgreementId, c.Configure, c.EnvironmentAdditions)
}

func (c AgreementLaunchContext) ShortString() string {
	return fmt.Sprintf("AgreementProtocol: %v, AgreementId: %v", c.AgreementProtocol, c.AgreementId)
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

// This event indicates that a new agbot policy has been created
type ABPolicyCreatedMessage struct {
	event    Event
	fileName string
}

func (e ABPolicyCreatedMessage) String() string {
	return fmt.Sprintf("event: %v, file: %v", e.event, e.fileName)
}

func (e ABPolicyCreatedMessage) ShortString() string {
	return e.String()
}

func (e *ABPolicyCreatedMessage) Event() Event {
	return e.event
}

func (e *ABPolicyCreatedMessage) PolicyFile() string {
	return e.fileName
}

func NewABPolicyCreatedMessage(id EventId, policyFileName string) *ABPolicyCreatedMessage {

	return &ABPolicyCreatedMessage{
		event: Event{
			Id: id,
		},
		fileName: policyFileName,
	}
}

// This event indicates that the edge device has been registered in the exchange
type EdgeRegisteredExchangeMessage struct {
	event Event
	id    string
	token string
}

func (e EdgeRegisteredExchangeMessage) String() string {
	return fmt.Sprintf("event: %v, id: %v, token: %v", e.event, e.id, e.token)
}

func (e EdgeRegisteredExchangeMessage) ShortString() string {
	return e.String()
}

func (e *EdgeRegisteredExchangeMessage) Event() Event {
	return e.event
}

func (e *EdgeRegisteredExchangeMessage) ID() string {
	return e.id
}

func (e *EdgeRegisteredExchangeMessage) Token() string {
	return e.token
}

func NewEdgeRegisteredExchangeMessage(evId EventId, id string, token string) *EdgeRegisteredExchangeMessage {

	return &EdgeRegisteredExchangeMessage{
		event: Event{
			Id: evId,
		},
		id:    id,
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
	event                  Event
	ImageFiles             []string
	AgreementLaunchContext *AgreementLaunchContext
}

// fulfill interface of events.Message
func (b *TorrentMessage) Event() Event {
	return b.event
}

func (b *TorrentMessage) String() string {
	return fmt.Sprintf("event: %v, imageFiles: %v, agreementLaunchContext: %v", b.event, b.ImageFiles, b.AgreementLaunchContext)
}

func (b *TorrentMessage) ShortString() string {
	return fmt.Sprintf("event: %v, imageFiles: %v, agreementLaunchContext: %v", b.event, b.ImageFiles, b.AgreementLaunchContext.ShortString())
}

func NewTorrentMessage(id EventId, imageFiles []string, agreementLaunchContext *AgreementLaunchContext) *TorrentMessage {

	return &TorrentMessage{
		event: Event{
			Id: id,
		},
		ImageFiles:             imageFiles,
		AgreementLaunchContext: agreementLaunchContext,
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
	return m.String()
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
	return fmt.Sprintf("Event: %v, AgreementProtocol: %v, AgreementId: %v, Deployment: %v, Cause: %v", m.Event, m.AgreementProtocol, m.AgreementId, persistence.ServiceConfigNames(&m.Deployment), m.Cause)
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

//Container messages
type ContainerMessage struct {
	event             Event
	AgreementProtocol string
	AgreementId       string
	Deployment        map[string]persistence.ServiceConfig
}

func (m ContainerMessage) String() string {
	return fmt.Sprintf("event: %v, AgreementProtocol: %v, AgreementId: %v, Deployment: %v", m.event, m.AgreementProtocol, m.AgreementId, persistence.ServiceConfigNames(&m.Deployment))
}

func (m ContainerMessage) ShortString() string {
	return m.String()
}

func (b ContainerMessage) Event() Event {
	return b.event
}

func NewContainerMessage(id EventId, protocol string, agreementId string, deployment map[string]persistence.ServiceConfig) *ContainerMessage {

	return &ContainerMessage{
		event: Event{
			Id: id,
		},
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		Deployment:        deployment,
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
	return fmt.Sprintf("Event: %v, AgreementProtocol: %v, AgreementId: %v, Deployment: %v, Cause: %v", m.Event, m.AgreementProtocol, m.AgreementId, persistence.ServiceConfigNames(&m.Deployment), m.Cause)
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
	return fmt.Sprintf("Event: %v, AgreementProtocol: %v, AgreementId: %v, Deployment: %v, Reason: %v", m.Event, m.AgreementProtocol, m.AgreementId, persistence.ServiceConfigNames(&m.Deployment), m.Reason)
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
	return fmt.Sprintf("Event: %v, Account: %v, Time: %v", m.Event, m.Account, m.Time)
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
type BlockchainClientInitilizedMessage struct {
	event Event
	Time  uint64
}

func (m *BlockchainClientInitilizedMessage) Event() Event {
	return m.event
}

func (m BlockchainClientInitilizedMessage) String() string {
	return fmt.Sprintf("Event: %v, Time: %v", m.Event, m.Time)
}

func (m BlockchainClientInitilizedMessage) ShortString() string {
	return m.String()
}

func NewBlockchainClientInitilizedMessage(id EventId) *BlockchainClientInitilizedMessage {
	return &BlockchainClientInitilizedMessage{
		event: Event{
			Id: id,
		},
		Time: uint64(time.Now().Unix()),
	}
}
