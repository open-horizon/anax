package metering

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
)

type MeteringNotification struct {
	Amount                 uint64 `json:"amount"`                       // The number of tokens granted by this notification, rounded to the nearest minute
	StartTime              string `json:"start_time"`                   // The time when the agreement started, in seconds since 1970.
	CurrentTime            string `json:"current_time"`                 // The time when the notification was sent, in seconds since 1970.
	MissedTime             uint64 `json:"missed_time"`                  // The amount of time in seconds that the consumer detected missing data
	ConsumerMeterSignature string `json:"consumer_meter_signature"`     // The consumer's signature of the meter (amount, current time, agreement Id)
	AgreementHash          string `json:"agreement_hash"`               // The 32 byte SHA3 FIPS 202 hash of the proposal for the agreement.
	ConsumerSignature      string `json:"consumer_agreement_signature"` // The consumer's signature of the agreement hash.
	ConsumerAddress        string `json:"consumer_address"`             // The consumer's blockchain account/address.
	ProducerSignature      string `json:"producer_agreement_signature"` // The producer's signature of the agreement
	BlockchainType         string `json:"blockchain_type"`              // The type of the blockchain that this notification is intended to work with
}

func (m *MeteringNotification) CopyMeteringInto(metering persistence.MeteringNotification) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	m.Amount = metering.Amount
	m.StartTime = cliutils.ConvertTime(metering.StartTime)
	m.CurrentTime = cliutils.ConvertTime(metering.CurrentTime)
	m.MissedTime = metering.MissedTime
	m.ConsumerMeterSignature = metering.ConsumerMeterSignature
	m.AgreementHash = metering.AgreementHash
	m.ConsumerSignature = metering.ConsumerSignature
	m.ConsumerAddress = metering.ConsumerAddress
	m.ProducerSignature = metering.ProducerSignature
	m.BlockchainType = metering.BlockchainType
}

type ActiveMetering struct {
	Name                      string               `json:"name"`
	CurrentAgreementId        string               `json:"current_agreement_id"`
	ConsumerId                string               `json:"consumer_id"`
	AgreementFinalizedTime    string               `json:"agreement_finalized_time"`
	AgreementDataReceivedTime string               `json:"agreement_data_received_time"`
	AgreementProtocol         string               `json:"agreement_protocol"`              // the agreement protocol being used. It is also in the proposal.
	MeteringNotificationMsg   MeteringNotification `json:"metering_notification,omitempty"` // the most recent metering notification received
}

// CopyAgreementInto copies the agreement info into our output struct
func (a *ActiveMetering) CopyAgreementInto(agreement persistence.EstablishedAgreement) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	a.Name = agreement.Name
	a.CurrentAgreementId = agreement.CurrentAgreementId
	a.ConsumerId = agreement.ConsumerId
	a.AgreementFinalizedTime = cliutils.ConvertTime(agreement.AgreementFinalizedTime)
	a.AgreementDataReceivedTime = cliutils.ConvertTime(agreement.AgreementDataReceivedTime)
	a.AgreementProtocol = agreement.AgreementProtocol
	a.MeteringNotificationMsg.CopyMeteringInto(agreement.MeteringNotificationMsg)
}

type ArchivedMetering struct {
	Name                      string `json:"name"`
	CurrentAgreementId        string `json:"current_agreement_id"`
	ConsumerId                string `json:"consumer_id"`
	AgreementFinalizedTime    string `json:"agreement_finalized_time"`
	AgreementDataReceivedTime string `json:"agreement_data_received_time"`
	AgreementProtocol         string `json:"agreement_protocol"` // the agreement protocol being used. It is also in the proposal.

	AgreementTerminatedTime string `json:"agreement_terminated_time"`
	TerminatedReason        uint64 `json:"terminated_reason"`      // the reason that the agreement was terminated
	TerminatedDescription   string `json:"terminated_description"` // a string form of the reason that the agreement was terminated

	MeteringNotificationMsg MeteringNotification `json:"metering_notification,omitempty"` // the most recent metering notification received
}

// CopyAgreementInto copies the agreement info into our output struct
func (a *ArchivedMetering) CopyAgreementInto(agreement persistence.EstablishedAgreement) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	a.Name = agreement.Name
	a.CurrentAgreementId = agreement.CurrentAgreementId
	a.ConsumerId = agreement.ConsumerId
	a.AgreementFinalizedTime = cliutils.ConvertTime(agreement.AgreementFinalizedTime)
	a.AgreementDataReceivedTime = cliutils.ConvertTime(agreement.AgreementDataReceivedTime)
	a.AgreementProtocol = agreement.AgreementProtocol

	a.AgreementTerminatedTime = cliutils.ConvertTime(agreement.AgreementTerminatedTime)
	a.TerminatedReason = agreement.TerminatedReason
	a.TerminatedDescription = agreement.TerminatedDescription

	a.MeteringNotificationMsg.CopyMeteringInto(agreement.MeteringNotificationMsg)
}

func List(archivedMetering bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	apiOutput := make(map[string]map[string][]persistence.EstablishedAgreement, 0)
	cliutils.HorizonGet("agreement", []int{200}, &apiOutput, false)
	var ok bool
	if _, ok = apiOutput["agreements"]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("horizon api agreement output did not include 'agreements' key"))
	}
	whichAgreements := "active"
	if archivedMetering {
		whichAgreements = "archived"
	}
	var apiAgreements []persistence.EstablishedAgreement
	if apiAgreements, ok = apiOutput["agreements"][whichAgreements]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("horizon api agreement output did not include '%s' key", whichAgreements))
	}

	// Go thru the apiAgreements and convert into our output struct and then print
	if !archivedMetering {
		metering := make([]ActiveMetering, len(apiAgreements))
		for i := range apiAgreements {
			metering[i].CopyAgreementInto(apiAgreements[i])
		}
		jsonBytes, err := json.MarshalIndent(metering, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn metering list' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		metering := make([]ArchivedMetering, len(apiAgreements))
		for i := range apiAgreements {
			metering[i].CopyAgreementInto(apiAgreements[i])
		}
		jsonBytes, err := json.MarshalIndent(metering, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn metering list' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	}
}
