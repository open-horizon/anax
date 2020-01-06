package agreement

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
)

type ActiveAgreement struct {
	Name                        string                   `json:"name"`
	CurrentAgreementId          string                   `json:"current_agreement_id"`
	ConsumerId                  string                   `json:"consumer_id"`
	AgreementCreationTime       string                   `json:"agreement_creation_time"`
	AgreementAcceptedTime       string                   `json:"agreement_accepted_time"`
	AgreementFinalizedTime      string                   `json:"agreement_finalized_time"`
	AgreementExecutionStartTime string                   `json:"agreement_execution_start_time"`
	AgreementDataReceivedTime   string                   `json:"agreement_data_received_time"`
	AgreementProtocol           string                   `json:"agreement_protocol"` // the agreement protocol being used. It is also in the proposal.
	Workload                    persistence.WorkloadInfo `json:"workload_to_run"`
}

// CopyAgreementInto copies the agreement info into our output struct
func (a *ActiveAgreement) CopyAgreementInto(agreement persistence.EstablishedAgreement) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	a.Name = agreement.Name
	a.CurrentAgreementId = agreement.CurrentAgreementId
	a.ConsumerId = agreement.ConsumerId

	a.AgreementCreationTime = cliutils.ConvertTime(agreement.AgreementCreationTime)
	a.AgreementAcceptedTime = cliutils.ConvertTime(agreement.AgreementAcceptedTime)
	a.AgreementFinalizedTime = cliutils.ConvertTime(agreement.AgreementFinalizedTime)
	a.AgreementExecutionStartTime = cliutils.ConvertTime(agreement.AgreementExecutionStartTime)
	a.AgreementDataReceivedTime = cliutils.ConvertTime(agreement.AgreementDataReceivedTime)

	a.AgreementProtocol = agreement.AgreementProtocol

	a.Workload = agreement.RunningWorkload
}

type ArchivedAgreement struct {
	Name                        string                   `json:"name"`
	CurrentAgreementId          string                   `json:"current_agreement_id"`
	ConsumerId                  string                   `json:"consumer_id"`
	AgreementCreationTime       string                   `json:"agreement_creation_time"`
	AgreementAcceptedTime       string                   `json:"agreement_accepted_time"`
	AgreementFinalizedTime      string                   `json:"agreement_finalized_time"`
	AgreementExecutionStartTime string                   `json:"agreement_execution_start_time"`
	AgreementDataReceivedTime   string                   `json:"agreement_data_received_time"`
	AgreementProtocol           string                   `json:"agreement_protocol"` // the agreement protocol being used. It is also in the proposal.
	Workload                    persistence.WorkloadInfo `json:"workload_to_run"`

	AgreementTerminatedTime string `json:"agreement_terminated_time"`
	TerminatedReason        uint64 `json:"terminated_reason"`      // the reason that the agreement was terminated
	TerminatedDescription   string `json:"terminated_description"` // a string form of the reason that the agreement was terminated
}

// CopyAgreementInto copies the agreement info into our output struct
func (a *ArchivedAgreement) CopyAgreementInto(agreement persistence.EstablishedAgreement) {
	//todo: what's the best way to make this part common with the active agreement copy? Interface? Anonymous struct?
	a.Name = agreement.Name
	a.CurrentAgreementId = agreement.CurrentAgreementId
	a.ConsumerId = agreement.ConsumerId
	a.AgreementCreationTime = cliutils.ConvertTime(agreement.AgreementCreationTime)
	a.AgreementAcceptedTime = cliutils.ConvertTime(agreement.AgreementAcceptedTime)
	a.AgreementFinalizedTime = cliutils.ConvertTime(agreement.AgreementFinalizedTime)
	a.AgreementExecutionStartTime = cliutils.ConvertTime(agreement.AgreementExecutionStartTime)
	a.AgreementDataReceivedTime = cliutils.ConvertTime(agreement.AgreementDataReceivedTime)
	a.AgreementProtocol = agreement.AgreementProtocol
	a.Workload = agreement.RunningWorkload

	a.AgreementTerminatedTime = cliutils.ConvertTime(agreement.AgreementTerminatedTime)
	a.TerminatedReason = agreement.TerminatedReason
	a.TerminatedDescription = agreement.TerminatedDescription
}

func GetAgreements(archivedAgreements bool) (apiAgreements []persistence.EstablishedAgreement) {
	// Get horizon api agreement output and drill down to the category we want
	apiOutput := make(map[string]map[string][]persistence.EstablishedAgreement, 0)
	cliutils.HorizonGet("agreement", []int{200}, &apiOutput, false)
	var ok bool
	if _, ok = apiOutput["agreements"]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, i18n.GetMessagePrinter().Sprintf("horizon api agreement output did not include 'agreements' key"))
	}
	whichAgreements := "active"
	if archivedAgreements {
		whichAgreements = "archived"
	}
	if apiAgreements, ok = apiOutput["agreements"][whichAgreements]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, i18n.GetMessagePrinter().Sprintf("horizon api agreement output did not include '%s' key", whichAgreements))
	}
	return
}

func List(archivedAgreements bool, agreementId string) {
	apiAgreements := GetAgreements(archivedAgreements)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if agreementId != "" {
		// Look for our agreement id. This works for either active or archived
		for i := range apiAgreements {
			if agreementId == apiAgreements[i].CurrentAgreementId {
				// Found it
				jsonBytes, err := json.MarshalIndent(apiAgreements[i], "", cliutils.JSON_INDENT)
				if err != nil {
					cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal agreement with index %d: %v", i, err))
				}
				fmt.Printf("%s\n", jsonBytes)
				return
			}
		}
		// Did not find it
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("agreement id %s not found", agreementId))
	} else {
		// Listing all active or archived agreements. Go thru apiAgreements and convert into our output struct and then print
		if !archivedAgreements {
			agreements := make([]ActiveAgreement, len(apiAgreements))
			for i := range apiAgreements {
				agreements[i].CopyAgreementInto(apiAgreements[i])
			}
			jsonBytes, err := json.MarshalIndent(agreements, "", cliutils.JSON_INDENT)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn agreement list' output: %v", err))
			}
			fmt.Printf("%s\n", jsonBytes)
		} else {
			// Archived agreements
			agreements := make([]ArchivedAgreement, len(apiAgreements))
			for i := range apiAgreements {
				agreements[i].CopyAgreementInto(apiAgreements[i])
			}
			jsonBytes, err := json.MarshalIndent(agreements, "", cliutils.JSON_INDENT)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn agreement list' output: %v", err))
			}
			fmt.Printf("%s\n", jsonBytes)
		}
	}
}

func Cancel(agreementId string, allAgreements bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Put the agreement ids in a slice
	var agrIds []string
	if allAgreements {
		apiAgreements := GetAgreements(false)
		for _, a := range apiAgreements {
			agrIds = append(agrIds, a.CurrentAgreementId)
		}
		if len(agrIds) == 0 {
			msgPrinter.Printf("No active agreements to cancel.")
			msgPrinter.Println()
		}
	} else {
		if agreementId == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("either an agreement ID or -a must be specified."))
		}
		agrIds = append(agrIds, agreementId)
	}

	// Cancel the agreements
	for _, id := range agrIds {
		msgPrinter.Printf("Canceling agreement %s ...", id)
		msgPrinter.Println()
		cliutils.HorizonDelete("agreement/"+id, []int{200, 204}, []int{}, false)
	}
}
