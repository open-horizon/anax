package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"net/http"
	"strings"
)

func HAGroupList(org, credToUse, haGroupName string, namesOnly bool) {

	cliutils.SetWhetherUsingApiKey(credToUse)

	var haGroupOrg string
	haGroupOrg, haGroupName = cliutils.TrimOrg(org, haGroupName)

	if haGroupName == "*" {
		haGroupName = ""
	}

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var haGroups exchangecommon.GetHAGroupResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+haGroupOrg+"/hagroups"+cliutils.AddSlash(haGroupName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &haGroups)
	if httpCode == 404 && haGroupName != "" {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("HA group %s not found in org %s", haGroupName, haGroupOrg))
	} else if httpCode == 404 {
		tmpList := []string{}
		fmt.Println(tmpList)
	} else if namesOnly && haGroupName == "" {
		nameList := []string{}
		for _, hagr := range haGroups.NodeGroups {
			hagroupEntry := fmt.Sprintf("%v/%v", haGroupOrg, hagr.Name)
			nameList = append(nameList, hagroupEntry)
		}
		jsonBytes, err := json.MarshalIndent(nameList, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange hagroup list' output: %v", err))
		}
		fmt.Println(string(jsonBytes))
	} else {
		hagrMap := make(map[string]exchangecommon.HAGroup)
		for _, hagr := range haGroups.NodeGroups {
			hagroupKey := fmt.Sprintf("%v/%v", haGroupOrg, hagr.Name)
			hagrMap[hagroupKey] = hagr
		}

		output := cliutils.MarshalIndent(hagrMap, "exchange hagroup list")
		fmt.Println(output)
	}
}

func HAGroupNew() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var hagroup_template = []string{
		`{`,
		`  "name": "",             /* ` + msgPrinter.Sprintf("Optional. The name of the HA group.") + ` */`,
		`  "description": "",      /* ` + msgPrinter.Sprintf("A description of the HA group.") + ` */`,
		`  "members": [            /* ` + msgPrinter.Sprintf("A list of node names that are members of this group.") + ` */`,
		`    "node1",`,
		`    "node2"`,
		`  ]`,
		`}`,
	}

	for _, s := range hagroup_template {
		fmt.Println(s)
	}
}

func HAGroupAdd(org, credToUse, haGroupName, jsonFilePath string) {
	// check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(credToUse)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// read in the new ha group from file
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(jsonFilePath)
	var haGroupFile exchangecommon.HAGroup
	err := json.Unmarshal(newBytes, &haGroupFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", jsonFilePath, err))
	}

	if haGroupName == "" {
		haGroupName = haGroupFile.Name
	}

	var haGroupOrg string
	if haGroupName != "" {
		// get the group name from the input file if the name is not given by the cli argument
		haGroupOrg, haGroupName = cliutils.TrimOrg(org, haGroupName)
	} else {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("HA group name is not specified."))
	}

	if haGroupFile.Description == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("HA group description cannot be empty."))
	}

	haGroupRequest := exchangecommon.HAGroupPutPostRequest{
		Description: haGroupFile.Description,
		Members:     haGroupFile.Members,
	}

	// make sure that the nodes added are of "device" type.
	clusterNode := CheckNodesForClusterType(org, credToUse, haGroupFile.Members)
	if clusterNode != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot create HA group %v/%v because node %v is 'cluster' type. HA group does not support 'cluster' type members.", haGroupOrg, haGroupName, clusterNode))
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	// add or overwrite hagroup file
	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrl, "orgs/"+haGroupOrg+"/hagroups"+cliutils.AddSlash(haGroupName), cliutils.OrgAndCreds(org, credToUse), []int{201, 409}, haGroupRequest, &resp)
	if httpCode == 409 {
		//try to update the existing HA group
		httpCode = cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+haGroupOrg+"/hagroups"+cliutils.AddSlash(haGroupName), cliutils.OrgAndCreds(org, credToUse), []int{201, 404}, haGroupRequest, nil)
		if httpCode == 201 {
			msgPrinter.Printf("HA group %v/%v updated in the Horizon Exchange", haGroupOrg, haGroupName)
			msgPrinter.Println()
		} else if httpCode == 404 {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot create HA group %v/%v: %v", haGroupOrg, haGroupName, resp.Msg))
		}
	} else {
		msgPrinter.Printf("HA group %v/%v added in the Horizon Exchange", haGroupOrg, haGroupName)
		msgPrinter.Println()
	}
}

func HAGroupRemove(org, credToUse, haGroupName string, force bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)

	var haGroupOrg string
	haGroupOrg, haGroupName = cliutils.TrimOrg(org, haGroupName)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove HA group %v for org %v from the Horizon Exchange?", haGroupName, haGroupOrg))
	}

	//remove HA group
	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+haGroupOrg+"/hagroups"+cliutils.AddSlash(haGroupName), cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("HA group %s is not found in org %s", haGroupName, haGroupOrg))
	} else if httpCode == 204 {
		msgPrinter.Printf("HA group %v/%v removed from the Horizon Exchange.", haGroupOrg, haGroupName)
		msgPrinter.Println()
	}
}

func HAGroupMemberAdd(org, credToUse, haGroupName string, nodeNames []string) {
	// check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(credToUse)

	var haGroupOrg string
	haGroupOrg, haGroupName = cliutils.TrimOrg(org, haGroupName)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+haGroupOrg+"/hagroups"+cliutils.AddSlash(haGroupName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, nil)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("HA group %s is not found in org %s", haGroupName, haGroupOrg))
	}

	// make sure that the nodes added are of "device" type.
	clusterNode := CheckNodesForClusterType(org, credToUse, nodeNames)
	if clusterNode != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot add node %v to HA group %v/%v because it has 'cluster' type. HA group does not support 'cluster' type members.", clusterNode, haGroupOrg, haGroupName))
	}

	addedNodes := []string{}
	failedNodes := []string{}

	// make sure the given node names exist in the exchange and the node is not already in the group
	for _, nodeName := range nodeNames {

		// make sure node org is in the same or as the ha group org
		var nodeOrg string
		nodeOrg, nodeName = cliutils.TrimOrg(haGroupOrg, nodeName)
		if nodeOrg != haGroupOrg {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("node org is different from the group org %v for node '%s/%s'", haGroupOrg, nodeOrg, nodeName))
		}

		var nodes ExchangeNodes
		httpCode = cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+nodeOrg+"/nodes"+cliutils.AddSlash(nodeName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodes)
		if httpCode == 404 {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node '%s' not found in org %s", nodeName, nodeOrg))
		}

		key := cliutils.AddOrg(org, nodeName)
		currentHAGroup := nodes.Nodes[key].HAGroup

		if currentHAGroup == haGroupName {
			msgPrinter.Printf("Node %s is already in HA group %s/%s. Skipping the node.", nodeName, haGroupOrg, haGroupName)
			msgPrinter.Println()
		} else if currentHAGroup != "" && currentHAGroup != haGroupName {
			failedNodes = append(failedNodes, nodeName)
		} else {
			cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/hagroups/"+haGroupName+"/nodes/"+nodeName, cliutils.OrgAndCreds(org, credToUse), []int{201}, nil, nil)
			addedNodes = append(addedNodes, nodeName)
		}
	}

	if len(addedNodes) > 0 {
		msgPrinter.Printf("The following nodes are added to HA group %v/%v: \"%v\"", haGroupOrg, haGroupName, strings.Join(addedNodes, ","))
		msgPrinter.Println()
	}

	if len(failedNodes) > 0 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("the following nodes cannot be added to the group because they are in different groups: \"%v\"", strings.Join(failedNodes, ",")))
	}
}

func HAGroupMemberRemove(org, credToUse, haGroupName string, nodeNames []string, force bool) {
	// check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(credToUse)

	var haGroupOrg string
	haGroupOrg, haGroupName = cliutils.TrimOrg(org, haGroupName)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove nodes %v from HA group %v for org %v in the Horizon Exchange?", nodeNames, haGroupName, haGroupOrg))
	}

	// get the current members for the group
	var haGroupResp exchangecommon.GetHAGroupResponse
	httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+haGroupOrg+"/hagroups"+cliutils.AddSlash(haGroupName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &haGroupResp)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("HA group %s is not found in org %s", haGroupName, haGroupOrg))
	}
	members := haGroupResp.NodeGroups[0].Members

	// check if the node is in the group or not
	nodesToRemove := []string{}
	for _, nodeName := range nodeNames {

		// make sure node org is in the same or as the ha group org
		var nodeOrg string
		nodeOrg, nodeName = cliutils.TrimOrg(haGroupOrg, nodeName)
		if nodeOrg != haGroupOrg {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("node org is different from the group org %v for node '%s/%s'", haGroupOrg, nodeOrg, nodeName))
		}

		found := false
		for i, oldNode := range members {
			_, oldNode = cliutils.TrimOrg(haGroupOrg, oldNode)
			if oldNode == nodeName {
				members = append(members[:i], members[i+1:]...)
				found = true
				break
			}
		}

		if !found {
			msgPrinter.Printf("Node %v is not in HA group %v/%v.", nodeName, haGroupOrg, haGroupName)
			msgPrinter.Println()
		} else {
			nodesToRemove = append(nodesToRemove, nodeName)
			cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/hagroups/"+haGroupName+"/nodes/"+nodeName, cliutils.OrgAndCreds(org, credToUse), []int{204})
		}

	}
	if len(nodesToRemove) > 1 {
		msgPrinter.Printf("Nodes \"%v\" are removed from HA group %v/%v in the Horizon Exchange", strings.Join(nodesToRemove, ","), haGroupOrg, haGroupName)
		msgPrinter.Println()
	} else if len(nodesToRemove) == 1 {
		msgPrinter.Printf("Node \"%v\" is removed from HA group %v/%v in the Horizon Exchange", strings.Join(nodesToRemove, ","), haGroupOrg, haGroupName)
		msgPrinter.Println()
	}
}

// This function makes sure that all the given nodes are of "device" types.
// It returns the first node that has "cluster" type.
func CheckNodesForClusterType(org string, credToUse string, nodes []string) string {
	if nodes == nil {
		return ""
	}

	for _, node := range nodes {
		// get node org
		nodeOrg, nodeName := cliutils.TrimOrg(org, node)

		if node == "" {
			continue
		}

		var nodeResponse ExchangeNodes
		httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes"+cliutils.AddSlash(nodeName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodeResponse)
		if httpCode == 404 {
			cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("node '%s' not found in org %s", nodeName, nodeOrg))
		}

		if nodeResponse.Nodes != nil {
			exNode := nodeResponse.Nodes[fmt.Sprintf("%s/%s", nodeOrg, nodeName)]
			if exNode.NodeType == persistence.DEVICE_TYPE_CLUSTER {
				return node
			}
		}
	}

	return ""
}
