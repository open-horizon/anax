package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"net/http"
	"strings"
)

type ExchangeOrgs struct {
	Orgs      map[string]interface{} `json:"orgs"`
	LastIndex int                    `json:"lastIndex"`
}

// use this structure instead of exchange.Organization for updating the org tags
// so that an empty map can be passed into the exchange api without get lost by json.Mashal()
type PatchOrgTags struct {
	Tags map[string]string `json:"tags"`
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

func OrgCreate(org, userPwCreds, theOrg string, label string, desc string, tags []string, min int, max int, adjust int, maxNodes int, agbot string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get credentials
	cliutils.SetWhetherUsingApiKey(userPwCreds)

	// check if the agbot specified by -a exist or not
	CheckAgbot(org, userPwCreds, agbot)

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
		negatives := strings.Join(negFlags, ",")
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid input for %v. Negative integer is not allowed.", negatives))
	}

	// if min is not less than or equal to max, throw error
	if min != 0 && max != 0 && min > max {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The value for --heartbeatmin must be less than or equal to the value for --heartbeatmax."))
	}

	if label == "" {
		label = theOrg
	}

	// handle tags. the input is -t mytag1=myvalue1 -t mytag2=mytag2
	orgTags := convertTags(tags, false)

	// validate maxNodes
	if maxNodes < 0 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid input for --max-nodes. Negative integer is not allowed."))
	}

	// add org to exchange
	orgHb := exchange.HeartbeatIntervals{MinInterval: min, MaxInterval: max, IntervalAdjustment: adjust}
	limits := exchange.OrgLimits{MaxNodes: maxNodes}
	postOrgReq := exchange.Organization{Label: label, Description: desc, HeartbeatIntv: &orgHb, Tags: orgTags, Limits: &limits}
	cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, postOrgReq, nil)

	msgPrinter.Printf("Organization %v is successfully added to the Exchange.", theOrg)
	msgPrinter.Println()

	// adding the org to the agbot's served list
	if agbot == "" {
		// if -a is not specified, it go get the first agbot it can find
		ag := GetDefaultAgbot(org, userPwCreds)
		if ag == "" {
			msgPrinter.Printf("No agbot found in the Exchange.")
			msgPrinter.Println()
			return

		}
		agbot = ag
	}
	AddOrgToAgbotServingList(org, userPwCreds, theOrg, agbot)
	msgPrinter.Printf("Agbot %v is responsible for deploying services in org %v", agbot, theOrg)
	msgPrinter.Println()
}

func OrgUpdate(org, userPwCreds, theOrg string, label string, desc string, tags []string, min int, max int, adjust int, maxNodes int) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// check existance and also get current attribute values for comparizon later
	var orgs exchange.GetOrganizationResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{200, 404}, &orgs)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("org '%s' not found.", theOrg))
	}

	// if --label is specified, update it
	if label != "" {
		newOrgLabel := exchange.Organization{Label: label}
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, newOrgLabel, nil)
	}

	// if --description is specified, update it
	if desc != "" {
		newOrgDesc := exchange.Organization{Description: desc}
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, newOrgDesc, nil)
	}

	// convert the input tags into map[string]string
	orgTags := convertTags(tags, true)
	if orgTags != nil {
		newTags := PatchOrgTags{Tags: orgTags}
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, newTags, nil)
	}

	// do nothing if they are -1
	if min != -1 || max != -1 || adjust != -1 {
		newMin, newMax, newAdjust := getNewHeartbeatAttributes(min, max, adjust, orgs.Orgs[theOrg].HeartbeatIntv)
		orgHb := exchange.HeartbeatIntervals{MinInterval: newMin, MaxInterval: newMax, IntervalAdjustment: newAdjust}
		newOrgHeartbeaat := exchange.Organization{HeartbeatIntv: &orgHb}
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, newOrgHeartbeaat, nil)
	}

	// do nothing if maxNodes is -1
	if maxNodes < -1 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid input for --max-nodes. Only -1, 0 and positive integers are allowed."))
	} else if maxNodes > -1 {
		limits := exchange.OrgLimits{MaxNodes: maxNodes}
		newOrgLimits := exchange.Organization{Limits: &limits}
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, newOrgLimits, nil)
	}

	msgPrinter.Printf("Organization %v is successfully updated.", theOrg)
	msgPrinter.Println()
}

func OrgDel(org, userPwCreds, theOrg, agbot string, force bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Search exchange for org, throw error if not found.
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{200, 404}, nil)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("org '%s' not found.", theOrg))
	}

	// check if the agbot specified by -a exist or not
	CheckAgbot(org, userPwCreds, agbot)

	// "Are you sure?" prompt
	cliutils.SetWhetherUsingApiKey(userPwCreds)
	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Warning: this will also delete all Exchange resources owned by this org (nodes, services, patterns, etc). Are you sure you want to remove user %v from the Horizon Exchange?", theOrg))
	}

	if agbot == "" {
		// if -a is not specified, it go get the first agbot it can find
		ag := GetDefaultAgbot(org, userPwCreds)
		if ag == "" {
			msgPrinter.Printf("No agbot found in the Exchange.")
			msgPrinter.Println()
		} else {
			agbot = ag
			cliutils.Verbose(msgPrinter.Sprintf("Using agbot %v", agbot))
		}
	}

	if agbot != "" {
		var agbotOrg string
		agbotOrg, agbot = cliutils.TrimOrg(org, agbot)

		// Load and remove all agbot served patterns associated with this org
		var patternsResp ExchangeAgbotPatterns
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+agbotOrg+"/agbots/"+agbot+"/patterns", cliutils.OrgAndCreds(org, userPwCreds), []int{200, 404}, &patternsResp)

		for patternId, p := range patternsResp.Patterns {
			// Convert pattern's value into JSON and unmarshal it
			var servedPattern ServedPattern
			patternJson := cliutils.MarshalIndent(p, "Cannot convert pattern to JSON")
			cliutils.Unmarshal([]byte(patternJson), &servedPattern, msgPrinter.Sprintf("Cannot unmarshal served pattern"))

			if servedPattern.PatternOrg == theOrg || servedPattern.NodeOrg == theOrg {
				cliutils.Verbose(msgPrinter.Sprintf("Removing pattern %s from agbot %s", patternId, agbot))
				cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+agbotOrg+"/agbots/"+agbot+"/patterns/"+patternId, cliutils.OrgAndCreds(org, userPwCreds), []int{204})
			}
		}

		// Load and remove all agbot served business policies associated with this org
		polResp := new(exchange.GetAgbotsBusinessPolsResponse)
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+agbotOrg+"/agbots/"+agbot+"/businesspols", cliutils.OrgAndCreds(org, userPwCreds), []int{200, 404}, polResp)

		for polId, p := range polResp.BusinessPols {
			if p.BusinessPolOrg == theOrg { // the nodeOrg and polOrg are the same
				cliutils.Verbose(msgPrinter.Sprintf("Removing policy %s from agbot %s", polId, agbot))
				cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+agbotOrg+"/agbots/"+agbot+"/businesspols/"+polId, cliutils.OrgAndCreds(org, userPwCreds), []int{204})
			}
		}
	}

	// Search exchange for org and delete it.
	cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+theOrg, cliutils.OrgAndCreds(org, userPwCreds), []int{204})

	msgPrinter.Printf("Organization %v is successfully removed.", theOrg)
	msgPrinter.Println()

}

// validate the heartbeat input values and return the new values to update
func getNewHeartbeatAttributes(min, max, adjust int, existingInterv *exchange.HeartbeatIntervals) (int, int, int) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// check constraints
	// if any invalid values for heartbeat, throw error
	negFlags := []string{}
	if min < -1 {
		negFlags = append(negFlags, "--heartbeatmin")
	}
	if max < -1 {
		negFlags = append(negFlags, "--heartbeatmax")
	}
	if adjust < -1 {
		negFlags = append(negFlags, "--heartbeatadjust")
	}
	if len(negFlags) > 0 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid input for %v. Only -1, 0 and positive integers are allowed.", strings.Join(negFlags, ",")))
	}

	// if min is not less than or equal to max from pure user input, throw error
	if min > 0 && max > 0 && min > max {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The value for --heartbeatmin must be less than or equal to the value for --heartbeatmax."))
	}

	// use the existing value if it is -1
	var old_heartbeats exchange.HeartbeatIntervals
	if existingInterv != nil {
		old_heartbeats = *existingInterv
	}
	if min == -1 {
		min = old_heartbeats.MinInterval
	}
	if max == -1 {
		max = old_heartbeats.MaxInterval
	}
	if adjust == -1 {
		adjust = old_heartbeats.IntervalAdjustment
	}

	// if min is not less than or equal to max from the final combined values, throw error
	if min > 0 && max > 0 && min > max {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The value for heartbeat minInterval (%v) must be less than or equal to the value for heartbeat maxInterval (%v).", min, max))
	}

	return min, max, adjust
}

// convert the tags from an array of mytag=myvalue into a map.
// return nil if the input array is empty
func convertTags(tags []string, update bool) map[string]string {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if tags == nil || len(tags) == 0 {
		return nil
	}

	orgTags := map[string]string{}
	for _, tag := range tags {
		if update && tag == "" {
			msgPrinter.Printf("Empty string is specified with -t, all tags will be removed.")
			msgPrinter.Println()
			return orgTags
		}

		t := strings.SplitN(tag, "=", 2)
		if len(t) != 2 {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid input for -t or --tag flag: %v. The valid format is '-t mytag=myvalue' or '--tag mytag=myvalue'.", tag))
		} else {
			orgTags[t[0]] = t[1]
		}
	}
	return orgTags
}

// This function checks if the given agbot exists or not
func CheckAgbot(org string, userPw string, agbot string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()
	if agbot == "" {
		return
	}

	agOrg, ag1 := cliutils.TrimOrg(org, agbot)
	var agbots ExchangeAgbots
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+agOrg+"/agbots"+cliutils.AddSlash(ag1), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &agbots)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Agbot '%v/%v' specified by -a cannot be not found in the Exchange", agOrg, ag1))
	}
}

// This function goes through all the orgs and get the agbots for that org.
// It returns the first agbot it found.
func GetDefaultAgbot(org, userPwCreds string) string {
	exchUrlBase := cliutils.GetExchangeUrl()

	// get all the orgs
	var orgs ExchangeOrgs
	cliutils.ExchangeGet("Exchange", exchUrlBase, "orgs", cliutils.OrgAndCreds(org, userPwCreds), []int{200}, &orgs)

	// for each org find agbots
	for o := range orgs.Orgs {
		var resp ExchangeAgbots
		httpCode := cliutils.ExchangeGet("Exchange", exchUrlBase, "orgs/"+o+"/agbots", cliutils.OrgAndCreds(org, userPwCreds), []int{200, 404}, &resp)
		if httpCode == 200 {
			for a := range resp.Agbots {
				return a
			}
		}
	}

	return ""
}

// Add the given org to the given agbot's served pattern and served policy list.
func AddOrgToAgbotServingList(org, userPw, theOrg, agbot string) {
	var agbotOrg string
	agbotOrg, agbot = cliutils.TrimOrg(org, agbot)

	// http code 201 means success, 409 means it already exists
	inputPat := ServedPattern{PatternOrg: theOrg, Pattern: "*", NodeOrg: theOrg}
	cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+agbotOrg+"/agbots/"+agbot+"/patterns", cliutils.OrgAndCreds(org, userPw), []int{201, 409}, inputPat, nil)

	inputPol := exchange.ServedBusinessPolicy{BusinessPolOrg: theOrg, BusinessPol: "*", NodeOrg: theOrg}
	cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+agbotOrg+"/agbots/"+agbot+"/businesspols", cliutils.OrgAndCreds(org, userPw), []int{201, 409}, inputPol, nil)
}
