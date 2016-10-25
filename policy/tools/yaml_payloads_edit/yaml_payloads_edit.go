/*
Replaces 1 torrent payload in the governor.sls file.  See usage below.
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v2"
	// "github.com/davecgh/go-spew/spew"

	"repo.hovitos.engineering/MTN/go-policy/payload"
)

type Config struct {
	Governor struct {
		Prod string `yaml:"prod,flow"`
		Stg  string `yaml:"stg,flow"`
	}
}

const FOOTER = "# vim: set ts=4 sw=4 expandtab:"

func usage(exitCode int) {
	usageStr := "" +
		"Usage:\n" +
		"  cat <torrent-payload> | yaml_payloads_edit inserttorrent <path-governor.sls> <prod-or-stg> <match-attrs>\n"+
		"  yaml_payloads_edit promotetorrent <path-governor.sls> <match-attrs>\n"+
		// "  yaml_payloads_edit expandjson <path-governor.sls> <prod-or-stg>\n"+
		"  yaml_payloads_edit expandfile <path-governor.sls>\n"+
		// "  cat <json> | yaml_payloads_edit insertjson <path-governor.sls> <prod-or-stg>\n"+
		"  yaml_payloads_edit checksyntax <path-governor.sls>\n"+
		"\n"+
		"Update the governor.sls salt file, which produces the payloads.json file for the governor in the prod or stg environment.\n"+
		"\n"+
		"Commands:\n"+
		"  inserttorrent    Insert the torrent info from stdin to the specified section.\n"+
		"  promotetorrent   Copy the specified torrent section from the stg section to the prod section.\n"+
		// "  expandjson       Write the json from prod or stg to stdout in expanded form for easier editing.\n"+
		"  expandfile       Write the file to stdout in expanded form for easier editing.\n"+
		// "  insertjson       Insert the json from stdin to prod or stg (after editing).\n"+
		"  checksyntax      Check the yaml and json syntax of the governor salt file and report errors.\n"+
		"\n"+
		"Arguments:\n"+
		"  <path-governor.sls>   The relative or full path to the current governor.sls file. The new\n"+
		"                        governor.sls will be written to stdin.\n"+
		"  <prod-or-stg>         Which YAML section of the governor.sls file: 'prod' or 'stg'. \n"+
		"  <match-attrs>         The match attributes to identify the specific torrent section, e.g.\n"+
		"                        'sdr:true,arch:arm' (with no spaces).\n"+
		"\n"+
		"Examples:\n"+
		"  cat /tmp/crypto_payload | yaml_payloads_edit inserttorrent /tmp/governor.sls stg sdr:true,arch:arm > /tmp/governor.sls.new\n"+
		"  yaml_payloads_edit promotetorrent /tmp/governor.sls sdr:true,arch:arm > /tmp/governor.sls.new\n"+
		// "  yaml_payloads_edit expandjson /tmp/governor.sls prod > prod.json\n"+
		"  yaml_payloads_edit expandfile /tmp/governor.sls > governor.sls.new\n"+
		// "  cat prod.json | yaml_payloads_edit insertjson /tmp/governor.sls stg > /tmp/governor.sls.new\n"+
		"  yaml_payloads_edit checksyntax /tmp/governor.sls\n"+
		""
	if exitCode > 0 {
		log.Printf(usageStr)		// send it to stderr
	} else {
		fmt.Printf(usageStr)		// send it to stdout
	}
	os.Exit(exitCode)
}

/*
Return the json that is in the yaml field specified.
The json is returned as an array Payload structs, each Payload is a struct that has: MatchGroups, Deployment, Torrent
*/
func getJson(govYamlConfig *Config, yamlField string) []payload.Payload {
	// Get the json that is in the yaml field specified
	// panics if something goes wrong and that's cool
	field := reflect.ValueOf(govYamlConfig.Governor).FieldByName(yamlField)
	// fmt.Printf("field: %#v\n", field)
	// spew.Dump(field)
	val := field.String()
	if val == "" {		// reflect returns the zero value if the field is not there
		log.Fatalf("error: YAML field governor.%v does not exist in the governor file.", strings.ToLower(yamlField))
	}

	jsonHand, err := payload.NewPayloadHander([]byte(val))
	if err != nil {
		log.Fatalf("json error: %v", err)
	}
	return jsonHand.Payloads
}

/*
Find the correct payload and return the index of it.
*/
func getPayloadIndex(jsonPayloads []payload.Payload, attributes map[string]interface{}) int {
	// N.B. a cheat, really. the payload package does the matching but it
	// isn't intended to work like this so we're doing some of the matching logic ourselves
	for i, p := range jsonPayloads {
		// The FindPayload method name is misleading, it determines if the specified payload is a match to the attributes
		if match, err := payload.FindPayload(attributes, p); err != nil {
			log.Fatalf("error: %v", err)
		} else if match != nil {
			// fmt.Printf("found match: %d, %#v\n", i, p.MatchGroups)
			// spew.Dump(p.MatchGroups)
			return i
		}
	}
	return -1
}

/*
Find the correct payload and insert the new torrent info into it.
*/
func insertTorrentInJson(jsonPayloads []payload.Payload, inputTorrent payload.Torrent, attributes map[string]interface{}) {
	index := getPayloadIndex(jsonPayloads, attributes)
	if index >= 0 {
		jsonPayloads[index].Torrent = inputTorrent
	} else {
		log.Fatalf("error: did not find a matching payload for these attributes:  %#v", attributes)
	}
}

/*
Put the updated json into the stg or prod section of the yaml and marshal it
*/
func insertJsonInYaml(jsonPayloads []payload.Payload, govYamlConfig *Config, yamlField string) []byte {
	// newJson, err := json.Marshal(jsonPayloads)
	newJson, err := json.MarshalIndent(jsonPayloads, "", "    ")
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// the mutable version
	reflect.ValueOf(&govYamlConfig.Governor).Elem().FieldByName(yamlField).SetString(string(newJson))

	// serialize the whole yaml file
	newYaml, err := yaml.Marshal(govYamlConfig)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	return newYaml
}

/*
Get the torrent seed info from stdin and insert it into the section in govYamlConfig specified by yamField and attributes.
*/
func inserttorrent(govYamlConfig *Config, yamlField string, attributes map[string]interface{}) {
	// deserialize the new torrent info from stdin
	buf := bytes.NewBuffer([]byte{})
	buf.ReadFrom(os.Stdin)

	var inputTorrent payload.Torrent

	err := json.Unmarshal(buf.Bytes(), &inputTorrent)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Get the json payloads for stg or prod
	jsonPayloads := getJson(govYamlConfig, yamlField)

	// Find the correct payload and insert the new torrent info into it 
	insertTorrentInJson(jsonPayloads, inputTorrent, attributes)

	// Put the updated json into the stg or prod section of the yaml and unmarshal it
	newYaml := insertJsonInYaml(jsonPayloads, govYamlConfig, yamlField)

	// send the new governor.sls to stdout
	fmt.Printf("%s", string(newYaml))
	fmt.Printf("\n%v\n", FOOTER)
}

/*
Note:  this function is not correct, because when retagging a container from volcanostaging to volcano, it needs
		to be reseeded in bittorrent.
Get the torrent seed info specified by attributes from the stg section of govYamlConfig and copy it to the prod section.
*/
func promotetorrent(govYamlConfig *Config, attributes map[string]interface{}) {
	fromYamlField := "Stg"
	toYamlField := "Prod"

	// Get the json payloads from stg
	jsonStgPayloads := getJson(govYamlConfig, fromYamlField)

	// Get the payload we want to promote
	index := getPayloadIndex(jsonStgPayloads, attributes)
	if index < 0 {
		log.Fatalf("error: did not find a matching payload in %v for these attributes:  %#v", fromYamlField, attributes)
	}
	promotedTorrent := jsonStgPayloads[index].Torrent

	// Get the json payloads from prod
	jsonProdPayloads := getJson(govYamlConfig, toYamlField)

	// Find the corresponding payload in prod and replace it (or add it if not found)
	index = getPayloadIndex(jsonProdPayloads, attributes)
	if index >= 0 {
		// found it, so replace it
		jsonProdPayloads[index].Torrent = promotedTorrent
	} else {
		// did not find it
		log.Fatalf("error: did not find a matching payload in %v for these attributes:  %#v", toYamlField, attributes)
	}

	// Put the updated json into the prod section of the yaml and unmarshal it
	newYaml := insertJsonInYaml(jsonProdPayloads, govYamlConfig, toYamlField)

	// send the new governor.sls to stdout
	fmt.Printf("%s", string(newYaml))
	fmt.Printf("\n%v\n", FOOTER)
}


/*
Get the json from the govYamlConfig section specified by yamlField, expand it and write to stdout.
*/
func expandjson(govYamlConfig *Config, yamlField string) {
	// Get the json payloads from stg or prod
	jsonPayloads := getJson(govYamlConfig, yamlField)

	newJson, err := json.MarshalIndent(jsonPayloads, "", "    ")
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	fmt.Printf("%s\n", string(newJson))
}


/*
Get the json (either expanded or conpressed) from stdin and insert it into the section in govYamlConfig specified by yamField.
*/
func insertjson(govYamlConfig *Config, yamlField string) {
	// deserialize the new json from stdin
	buf := bytes.NewBuffer([]byte{})
	buf.ReadFrom(os.Stdin)

	var inputJson []payload.Payload

	err := json.Unmarshal(buf.Bytes(), &inputJson)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Put the updated json into the stg or prod section of the yaml and unmarshal it
	newYaml := insertJsonInYaml(inputJson, govYamlConfig, yamlField)

	// send the new governor.sls to stdout
	fmt.Printf("%s", string(newYaml))
	fmt.Printf("\n%v\n", FOOTER)
}


/*
Check the syntax of the yaml and json and report any errors.
*/
func checksyntax(govYamlConfig *Config) {
	// When main() unmarshalled the yaml, it checked its syntax, but not for the specific field names
	// Get the json payloads from stg or prod
	for _, yamlField := range []string{"Prod", "Stg"} {
		// The getJason() function checks the yaml Prod and Stg fields
		jsonPayloads := getJson(govYamlConfig, yamlField)
		// Check all levels of the json, see types in repo.hovitos.engineering/MTN/provider-tremor/payload/payload.go
		for i, payload := range jsonPayloads {
			// It seems that json.Unmarshal() creates fields in our go data structure even if they do not
			// exit in the json, so we just check for zero values.

			// Check the MatchGroups section
			if len(payload.MatchGroups) == 0 {
				log.Fatalf("error: no matchgroups specified for payload %v in section '%v'.\n", i, strings.ToLower(yamlField))
			}
			for j, matchgroup := range payload.MatchGroups {
				if len(matchgroup) == 0 {
					log.Fatalf("error: matchgroup %v in payload %v in section '%v' has no entries.\n", j, i, strings.ToLower(yamlField))
				}
				for k, match := range matchgroup {
					if match.Attr == "" || match.Value == "" {
						log.Fatalf("error: match %v in matchgroup %v in payload %v in section '%v' is missing 'attr' or 'value' field.\n", k, j, i, strings.ToLower(yamlField))
					} else {
						fmt.Printf("found 'attr' and 'value' fields in match %v in matchgroup %v in payload %v in section '%v'\n", k, j, i, strings.ToLower(yamlField))
					}
				}
			}

			// Check the Deployment section
			if payload.Deployment == "" {
				log.Fatalf("error: no 'deployment' field specified for payload %v in section '%v'.\n", i, strings.ToLower(yamlField))
			} else {
				fmt.Println("found 'deployment' field")
			}

			// Check the DeploymentSignature section
			if payload.DeploymentSignature == "" {
				log.Fatalf("error: no 'deployment_signature' field specified for payload %v in section '%v'.\n", i, strings.ToLower(yamlField))
			} else {
				fmt.Println("found 'deployment_signature' field")
			}

			// Check the Torrent section
			if payload.Torrent.Url == "" {
				log.Fatalf("error: no 'torrent.url' field specified for payload %v in section '%v'.\n", i, strings.ToLower(yamlField))
			} else {
				fmt.Printf("found 'torrent.url' field for payload %v in section '%v'\n", i, strings.ToLower(yamlField))
			}
			if len(payload.Torrent.Images) == 0 {
				log.Fatalf("error: no 'torrent.images' field specified for payload %v in section '%v'.\n", i, strings.ToLower(yamlField))
			} else {
				fmt.Printf("found 'torrent.images' field for payload %v in section '%v'\n", i, strings.ToLower(yamlField))
			}
			for n, image := range payload.Torrent.Images {
				if image.File == "" || image.Signature == "" {
					log.Fatalf("error: image %v in 'torrent' field in payload %v in section '%v' is missing 'file' or 'signature' field.\n", n, i, strings.ToLower(yamlField))
				} else {
					fmt.Printf("found 'file' and 'signature' fields in image %v for payload %v in section '%v'\n", n, i, strings.ToLower(yamlField))
				}
			}

			// typeStr := reflect.TypeOf(payload.MatchGroups)
			// field := reflect.ValueOf(payload).FieldByName("MatchGroups")
			// spew.Dump(field)
		}
	}

	// If none of unmarshalling, reflection, or other checks exited with an error, then we are good
	fmt.Println("Syntax is good.")
}


/*
Expand the json string for each yaml key so that it is easier to hand-edit the file.
*/
func expandYamlFile(govYamlConfig *Config) {
	// Get the json payloads from stg or prod
	for _, yamlField := range []string{"Prod", "Stg"} {
		// Get the json payloads from stg or prod
		jsonPayloads := getJson(govYamlConfig, yamlField)

		// Put the json into the stg or prod section of the yaml and unmarshal it
		// newYaml := insertJsonInYaml(jsonPayloads, govYamlConfig, yamlField)

		// newJson, err := json.Marshal(jsonPayloads)
		newJson, err := json.MarshalIndent(jsonPayloads, "", "    ")
		if err != nil {
			log.Fatalf("error: %v", err)
		}

		// the mutable version
		reflect.ValueOf(&govYamlConfig.Governor).Elem().FieldByName(yamlField).SetString(string(newJson))
	}

	// serialize the whole yaml file
	newYaml, err := yaml.Marshal(govYamlConfig)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// send the new governor.sls to stdout
	fmt.Printf("%s", string(newYaml))
	fmt.Printf("\n%v\n", FOOTER)
}


func main() {
	// Get and check args for each command
	// spew.Dump(os.Args)
	// log.Printf("this should be going to stderr")
	// log.Fatalf("this should be going to stderr")
	if len(os.Args) <= 1 {
		usage(0)
	}
	var filename, yamlField, matchAttrs string
	command := os.Args[1]
	switch command {
	case "inserttorrent":
		if len(os.Args) <= 4 { usage(1) }
		filename = os.Args[2]
		yamlField = os.Args[3]
		matchAttrs = os.Args[4]
	case "promotetorrent":
		if len(os.Args) <= 3 { usage(1) }
		filename = os.Args[2]
		matchAttrs = os.Args[3]
	/*
	case "expandjson":
		if len(os.Args) <= 3 { usage(1) }
		filename = os.Args[2]
		yamlField = os.Args[3]
	case "insertjson":
		if len(os.Args) <= 3 { usage(1) }
		filename = os.Args[2]
		yamlField = os.Args[3]
	*/
	case "expandfile":
		if len(os.Args) <= 2 { usage(1) }
		filename = os.Args[2]
	case "checksyntax":
		if len(os.Args) <= 2 { usage(1) }
		filename = os.Args[2]
	default:
		log.Printf("error: unrecognized command given to yaml_payloads_edit: %v\n\n", command)
		usage(1)
	}

	var govFile []byte
	var err error
	if filename != "" {
		govFile, err = ioutil.ReadFile(filename) 		// the governor salt file (in yaml format)
		if err != nil {
			panic(err)
		}
	}

	if yamlField != "" {
		switch yamlField {
		case "prod":
			yamlField = "Prod"
		case "stg", "staging":
			yamlField = "Stg"
		case "Prod", "Stg":
			// these are the values we use in the code
		default:
			log.Printf("error: unrecognized prod or stg section specified: %v\n\n", yamlField)
			usage(1)
		}
	}

	// the attributes in the matchgroups entry we want to change the torrent info for
	attributes := make(map[string]interface{})
	if matchAttrs != "" {
		// attributes := make([]map[string]interface{}, 0, 5)
		for _, s := range strings.Split(matchAttrs, ",") {
			f := strings.Split(s, ":")
			attributes[f[0]] = f[1]
		}

		// attributes := []map[string]interface{}{
		// 	{
		// 		"sdr": "true",			
		// 	},
		// 	{
		// 		"arch": "arm",			
		// 	},
		// }
	}
	// fmt.Printf("args: %+v, %+v, %+v, %+v\n", command, filename, yamlField, matchAttrs)
	// os.Exit(0)

	// Deserialize the yaml aspect of the entire governor.sls config file content.  All commands need this.
	var govYamlConfig Config
	err = yaml.Unmarshal(govFile, &govYamlConfig)
	if err != nil {
		log.Fatalf("yaml error: %v", err)
	}

	switch command {
	case "inserttorrent":
		inserttorrent(&govYamlConfig, yamlField, attributes)
	case "promotetorrent":
		promotetorrent(&govYamlConfig, attributes)
	case "expandjson":
		expandjson(&govYamlConfig, yamlField)
	case "insertjson":
		insertjson(&govYamlConfig, yamlField)
	case "expandfile":
		expandYamlFile(&govYamlConfig)
	case "checksyntax":
		checksyntax(&govYamlConfig)
	}

}
