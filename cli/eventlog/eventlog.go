package eventlog

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/persistence"
	"regexp"
	"strings"
	"time"
)

type EventLog struct {
	Id         string           `json:"record_id"` // unique primary key for records
	Timestamp  string           `json:"timestamp"` // converted to "yyyy-mm-dd hh:mm:ss" format
	Severity   string           `json:"severity"`  // info, warning or error
	Message    string           `json:"message"`
	EventCode  string           `json:"event_code"`
	SourceType string           `json:"source_type"`  // the type of the source. It can be agreement, service, image, workload etc.
	Source     *json.RawMessage `json:"event_source"` // source involved for this event.
}

// This function takes a list of selection strings. validate them and
// convert them to the format that the the anax api can take.
func getSelectionString(selections []string) (string, error) {
	valid_sel := regexp.MustCompile(`^([^~=><]+)([~=><])(.*)$`)

	sels := []string{}
	for _, v := range selections {
		match := valid_sel.FindStringSubmatch(v)

		if len(match) > 2 {
			attrib := match[1]
			op := match[2]
			val := ""
			if len(match) > 3 {
				val = match[3]
			}

			if len(op) > 1 {
				return "", fmt.Errorf("The selection string %v is not valid.", v)
			}

			real_op := ""
			switch op {
			case "=":
				real_op = "="
			case ">":
				real_op = "=>"
			case "<":
				real_op = "=<"
			case "~":
				real_op = "=~"
			default:
				real_op = "="
			}
			sels = append(sels, fmt.Sprintf("%v%v%v", attrib, real_op, val))
		} else {
			return "", fmt.Errorf("The selection string %v is not valid.", v)
		}
	}
	return strings.Join(sels, "&"), nil
}

func List(all bool, detail bool, selections []string) {

	// format the eventlog api string
	url_s := "eventlog"
	if all {
		url_s = fmt.Sprintf("%v/all", url_s)
	}
	if len(selections) > 0 {
		if s, err := getSelectionString(selections); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "%v", err)
		} else {
			url_s = fmt.Sprintf("%v?%v", url_s, s)
		}
	}

	// get the eventlog from anax
	apiOutput := make([]persistence.EventLogRaw, 0)
	cliutils.HorizonGet(url_s, []int{200}, &apiOutput)

	//output
	if detail {
		long_output := make([]EventLog, len(apiOutput))
		for i, v := range apiOutput {
			long_output[i].Id = v.Id
			long_output[i].Timestamp = cliutils.ConvertTime(v.Timestamp)
			long_output[i].Severity = v.Severity
			long_output[i].Message = v.Message
			long_output[i].EventCode = v.EventCode
			long_output[i].SourceType = v.SourceType
			long_output[i].Source = v.Source
		}

		jsonBytes, err := json.MarshalIndent(long_output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn eventlog list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		short_output := make([]string, len(apiOutput))
		for i, v := range apiOutput {
			t := time.Unix(int64(v.Timestamp), 0)
			short_output[i] = fmt.Sprintf("%v:   %v", t.Format("2006-01-02 15:04:05"), v.Message)
		}
		jsonBytes, err := json.MarshalIndent(short_output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn eventlog list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	}
}
