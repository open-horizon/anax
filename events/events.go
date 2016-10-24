package events

import (
	"fmt"
	gwhisper "github.com/open-horizon/go-whisper"
)

type Event struct {
	Id EventId
}

type EventId string

// event constants are declared here for all workers to ensure uniqueness of constant values
const (
	// blockchain-related
	NOOP                EventId = "NOOP"
	CONTRACT_ACCEPTED   EventId = "CONTRACT_ACCEPTED"
	CONTRACT_ENDED      EventId = "CONTRACT_ENDED"
	CONTRACT_CREATED    EventId = "CONTRACT_CREATED"
	CONTRACT_REGISTERED EventId = "CONTRACT_REGISTERED"

	PREVIOUS_AGREEMENT_REAP EventId = "PREVIOUS_AGREEMENT_REAP"

	// whisper related
    RECEIVED_MSG        EventId = "RECEIVED_MSG"
    SUBSCRIBE_TO        EventId = "SUBSCRIBE_TO"

	// torrent-related
	TORRENT_FAILURE     EventId = "TORRENT_FAILURE"
	TORRENT_FETCHED     EventId = "TORRENT_FETCHED"

	// container-related
	EXECUTION_FAILED    EventId = "EXECUTION_FAILED"
	EXECUTION_BEGUN     EventId = "EXECUTION_BEGUN"
	PATTERN_DESTROYED   EventId = "PATTERN_DESTROYED"
	CONTAINER_MAINTAIN  EventId = "CONTAINER_MAINTAIN"

	// policy-related
	NEW_POLICY          EventId = "NEW_POLICY"
    NEW_AB_POLICY       EventId = "NEW_AB_POLICY"

	// exchange-related
	NEW_DEVICE_REG      EventId = "NEW_DEVICE_REG"
	NEW_AGBOT_REG       EventId = "NEW_AGBOT_REG"

	// agreement-related
	AGREEMENT_REACHED   EventId = "AGREEMENT_REACHED"

)

type EndContractCause string

const (
	CT_TERMINATED EndContractCause = "CT_TERMINATED"
	CT_ERROR      EndContractCause = "CT_ERROR"
	CT_FULFILLED  EndContractCause = "CT_FULFILLED"
)

type Message interface {
	Event() Event
}

type AgreementLaunchContext struct {
	ContractId           string
	AgreementId          string
	Configure            *gwhisper.Configure
	ConfigureRaw         []byte
	EnvironmentAdditions *map[string]string // provided by platform, not but user
}

func (c AgreementLaunchContext) String() string {
	return fmt.Sprintf("ContractId: %v, AgreementId: %v, Configure: %v, EnvironmentAdditions: %v", c.ContractId, c.AgreementId, c.Configure, c.EnvironmentAdditions)
}

// This event indicates that a new microservice has been created in the form of a policy file
type PolicyCreatedMessage struct {
    event    Event
    fileName string
}

func (e PolicyCreatedMessage) String() string {
    return fmt.Sprintf("event: %v, file: %v", e.event, e.fileName)
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
    event    Event
    id       string
    token    string
}

func (e EdgeRegisteredExchangeMessage) String() string {
    return fmt.Sprintf("event: %v, id: %v, token: %v", e.event, e.id, e.token)
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
        id: id,
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
    event         Event
    topic         string
}

func (e WhisperSubscribeToMessage) String() string {
    return fmt.Sprintf("event: %v, topic: %v", e.event, e.topic)
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
    event         Event
    payload       string
    topics        []string
    from          string
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

func NewWhisperReceivedMessage(id EventId, payload string, from string) *WhisperReceivedMessage {

    return &WhisperReceivedMessage{
        event: Event{
            Id: id,
        },
        payload: payload,
        from: from,
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

func NewTorrentMessage(id EventId, imageFiles []string, agreementLaunchContext *AgreementLaunchContext) *TorrentMessage {

    return &TorrentMessage{
        event: Event{
            Id: id,
        },
        ImageFiles:             imageFiles,
        AgreementLaunchContext: agreementLaunchContext,
    }
}
