package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"net/http"
)

type ExchangeOrgs struct {
	Orgs      map[string]interface{} `json:"orgs"`
	LastIndex int                    `json:"lastIndex"`
}

func OrgList(org, userPwCreds, theOrg string, long bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()
	exchUrlBase := cliutils.GetExchangeUrl()

	// if org name is empty, list name of all orgs
	var orgurl string
	nameOnly := false
	if theOrg == "" {
		orgurl = "orgs"
		nameOnly = true
	} else {
		orgurl = "orgs/" + theOrg
	}

	// get orgs
	var orgs ExchangeOrgs
	httpCode := cliutils.ExchangeGet("Exchange", exchUrlBase, orgurl, cliutils.OrgAndCreds(org, userPwCreds), []int{200, 404}, &orgs)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("org '%s' not found.", theOrg))
	}

	// Print only the names if listing all orgs AND -l was not inputted
	// in the case of -l being true, print all details regardless
	if !long && nameOnly {
		organizations := []string{}
		for o := range orgs.Orgs {
			organizations = append(organizations, o)
		}

		jsonBytes, err := json.MarshalIndent(organizations, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'exchange org list' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		output := cliutils.MarshalIndent(orgs.Orgs, "exchange orgs list")
		fmt.Println(output)
	}
}

func OrgCreate(org, userPwCreds, theOrg string, label string, desc string, min int, max int, adjust int) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get credentials
	cliutils.SetWhetherUsingApiKey(userPwCreds)

	// check constraints on min, max, amnd adjust
	// if any negative values for heartbeat, throw error
	negFlags := []string{}
	if min < 0 {
		negFlags = append(negFlags, "--heartbeatmin")
	}
	if max < 0 {
		negFlags = append(negFlags, "--heartbeatmax")
	}
	if adjust < 0 {
		negFlags = append(negFlags, "--heartbeatadjust")
	}

	// indicate which flags are negative
	if len(negFlags) > 0 {
		negatives := ""
		for _, n := range negFlags {
			negatives = msgPrinter.Sprintf("%v, %v", negatives, n)
		}

		negatives = negatives[1:len(negatives)]
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, i18n.GetMessagePrinter().Sprintf("Invalid input for %v. Only positive integers are allowed.", negatives))
	}

	// if min is not less than or equal to max, throw error
	if min > max {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, i18n.GetMessagePrinter().Sprintf("The value for --heartbeatmin must be less than the value for --heartbeatmax."))
	}

	if label == "" {
		label = theOrg
	}

	// add org to exchange
	orgHb := exchange.HeartbeatIntervals{MinInterval: min, MaxInterval: max, IntervalAdjustment: adjust}
	postOrgReq := exchange.Organization{Label: label, Description: desc, HeartbeatIntv: &orgHb}
	cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, postOrgReq, nil)

	msgPrinter.Printf("Organization %v is successfully added to the Exchange.", theOrg)
	msgPrinter.Println()
}

func OrgUpdate(org, userPwCreds, theOrg string, label string, desc string, min int, max int, adjust int) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// if --label is specified, update it
	if label != "" {
		newOrgLabel := exchange.Organization{Label: label}
		httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, newOrgLabel, nil)
		if httpCode == 404 {
			cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("org %s not found.", theOrg))
		}
	}

	// if --description is specified, update it
	if desc != "" {
		newOrgDesc := exchange.Organization{Description: desc}
		httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, newOrgDesc, nil)
		if httpCode == 404 {
			cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("org %s not found.", theOrg))
		}
	}

	// if --heartbeatmin, --heartbeatmax, and --heartbeatadjust are all specified, update them all
	// do nothing if they are all zero
	// otherwise, throw error
	if min != 0 && max != 0 && adjust != 0 {

		// check constraints
		// if any negative values for heartbeat, throw error
		negFlags := []string{}
		if min < 0 {
			negFlags = append(negFlags, "--heartbeatmin")
		}
		if max < 0 {
			negFlags = append(negFlags, "--heartbeatmax")
		}
		if adjust < 0 {
			negFlags = append(negFlags, "--heartbeatadjust")
		}

		// indicate which flags are negative
		if len(negFlags) > 0 {
			negatives := ""
			for _, n := range negFlags {
				negatives = msgPrinter.Sprintf("%v, %v", negatives, n)
			}

			negatives = negatives[1:len(negatives)]
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, i18n.GetMessagePrinter().Sprintf("Invalid input for %v. Only positive integers are allowed.", negatives))
		}

		// if min is not less than or equal to max, throw error
		if min > max {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, i18n.GetMessagePrinter().Sprintf("The value for --heartbeatmin must be less than the value for --heartbeatmax."))
		}

		orgHb := exchange.HeartbeatIntervals{MinInterval: min, MaxInterval: max, IntervalAdjustment: adjust}
		newOrgHeartbeaat := exchange.Organization{HeartbeatIntv: &orgHb}
		httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, newOrgHeartbeaat, nil)
		if httpCode == 404 {
			cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("org %s not found.", theOrg))
		}
	} else if !(min == 0 && max == 0 && adjust == 0) {
		cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("--heartbeatmin, --heartbeatmax, and --heartbeatadjust must all be non-zero in order to update."))
	}

	msgPrinter.Printf("Organization %v is successfully updated.", theOrg)
	msgPrinter.Println()
}

func OrgDel(org, userPwCreds, theOrg string, force bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// "Are you sure?" prompt
	cliutils.SetWhetherUsingApiKey(userPwCreds)
	if !force {
		cliutils.ConfirmRemove(i18n.GetMessagePrinter().Sprintf("Warning: this will also delete all Exchange resources owned by this org (nodes, services, patterns, etc). Are you sure you want to remove user %v from the Horizon Exchange?", theOrg))
	}

	// Search exchange for org and delete it, throw error if not found.
	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("org %s not found.", theOrg))
	} else {
		msgPrinter.Printf("Organization %v is successfully removed.", theOrg)
		msgPrinter.Println()
	}

}
