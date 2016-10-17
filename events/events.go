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

	// whisper (direct msg) related
	DIRECT_CONFIGURE EventId = "DIRECT_CONFIGURE"
	CONFIGURE_ERROR  EventId = "CONFIGURE_ERROR"

	// torrent-related
	TORRENT_FAILURE EventId = "TORRENT_FAILURE"
	TORRENT_FETCHED EventId = "TORRENT_FETCHED"

	// container-related
	EXECUTION_FAILED   EventId = "EXECUTION_FAILED"
	EXECUTION_BEGUN    EventId = "EXECUTION_BEGUN"
	PATTERN_DESTROYED  EventId = "PATTERN_DESTROYED"
	CONTAINER_MAINTAIN EventId = "CONTAINER_MAINTAIN"
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
