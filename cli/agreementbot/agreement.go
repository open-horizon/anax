package agreementbot

import (
	"encoding/json"
	"fmt"
	agbot "github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"os"
)

type ActiveAgreement struct {
	CurrentAgreementId     string `json:"current_agreement_id"`     // unique
	Org                    string `json:"org"`                      // the org in which the policy exists that was used to make this agreement
	EdgeNodeId             string `json:"edge_node_id"`             // the edge node id we are working with, immutable after construction
	AgreementProtocol      string `json:"agreement_protocol"`       // immutable after construction - name of protocol in use
	AgreementInceptionTime string `json:"agreement_inception_time"` // immutable after construction
	AgreementCreationTime  string `json:"agreement_creation_time"`  // device responds affirmatively to proposal
	AgreementFinalizedTime string `json:"agreement_finalized_time"` // agreement is seen in the blockchain
	DataVerifiedTime       string `json:"data_verification_time"`   // The last time that data verification was successful
	DataNotificationSent   string `json:"data_notification_sent"`   // The timestamp for when data notification was sent to the device
	PolicyName             string `json:"policy_name"`              // The name of the policy for this agreement, policy names are unique
	Pattern                string `json:"pattern"`                  // The pattern used to make the agreement
}

// create an ActiveAgreement object
func NewActiveAgreement(agreement agbot.Agreement) *ActiveAgreement {
	var a ActiveAgreement
	a.CurrentAgreementId = agreement.CurrentAgreementId
	a.Org = agreement.Org
	a.EdgeNodeId = agreement.DeviceId
	a.AgreementProtocol = agreement.AgreementProtocol

	a.AgreementInceptionTime = cliutils.ConvertTime(agreement.AgreementInceptionTime)
	a.AgreementCreationTime = cliutils.ConvertTime(agreement.AgreementCreationTime)
	a.AgreementFinalizedTime = cliutils.ConvertTime(agreement.AgreementFinalizedTime)
	a.DataVerifiedTime = cliutils.ConvertTime(agreement.DataVerifiedTime)
	a.DataNotificationSent = cliutils.ConvertTime(agreement.DataNotificationSent)

	a.PolicyName = agreement.PolicyName
	a.Pattern = agreement.Pattern

	return &a
}

type ArchivedAgreement struct {
	ActiveAgreement              // inheritance
	AgreementTimedout     string `json:"agreement_timeout"`      // agreement was not finalized before it timed out
	TerminatedReason      uint   `json:"terminated_reason"`      // The reason the agreement was terminated
	TerminatedDescription string `json:"terminated_description"` // The description of why the agreement was terminated

}

// create an ArchivedAgreement object
func NewArchivedAgreement(agreement agbot.Agreement) *ArchivedAgreement {
	activeAg := NewActiveAgreement(agreement)
	a := ArchivedAgreement{
		*activeAg,
		cliutils.ConvertTime(agreement.AgreementTimedout),
		agreement.TerminatedReason,
		agreement.TerminatedDescription,
	}

	return &a
}

func getAgreements(archivedAgreements bool) (apiAgreements []agbot.Agreement) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// set env to call agbot url
	if err := os.Setenv("HORIZON_URL", cliutils.GetAgbotUrlBase()); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("unable to set env var 'HORIZON_URL', error %v", err))
	}

	// Get horizon api agreement output and drill down to the category we want
	apiOutput := make(map[string]map[string][]agbot.Agreement, 0)
	cliutils.HorizonGet("agreement", []int{200}, &apiOutput, false)

	var ok bool
	if _, ok = apiOutput["agreements"]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("horizon api agreement output did not include 'agreements' key"))
	}
	whichAgreements := "active"
	if archivedAgreements {
		whichAgreements = "archived"
	}
	if apiAgreements, ok = apiOutput["agreements"][whichAgreements]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("horizon api agreement output did not include '%s' key", whichAgreements))
	}
	return
}

func AgreementList(archivedAgreements bool, agreement string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	apiAgreements := getAgreements(archivedAgreements)

	// Go thru the apiAgreements and convert into our output struct and then print
	if !archivedAgreements {
		agreements := make([]ActiveAgreement, len(apiAgreements))
		for i := range apiAgreements {
			agreements[i] = *NewActiveAgreement(apiAgreements[i])
		}
		jsonBytes, err := json.MarshalIndent(agreements, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'agreement list' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		agreements := make([]ArchivedAgreement, len(apiAgreements))
		for i := range apiAgreements {
			agreements[i] = *NewArchivedAgreement(apiAgreements[i])
		}
		jsonBytes, err := json.MarshalIndent(agreements, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'agreement list' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	}
}

func AgreementCancel(agreementId string, allAgreements bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Put the agreement ids in a slice
	var agrIds []string
	if allAgreements {
		apiAgreements := getAgreements(false)
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
	if err := os.Setenv("HORIZON_URL", cliutils.GetAgbotUrlBase()); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("unable to set env var 'HORIZON_URL', error %v", err))
	}

	for _, id := range agrIds {
		msgPrinter.Printf("Canceling agreement %s ...", id)
		msgPrinter.Println()
		cliutils.HorizonDelete("agreement/"+id, []int{200, 204}, []int{}, false)
	}
}
