// +build unit

package exchange

import (
	"encoding/json"
	"testing"
)

func Test_Blockchain_Demarshal(t *testing.T) {
	// Simulate the detail string that we get from the exchange on a call to get details for a given blockchain type and name
	details := `{"chains":[{"arch":"amd64","deployment_description":{"deployment":"{\"services\":{\"geth\":{\"environment\":[\"CHAIN=bluehorizon\"],\"image\":\"summit.hovitos.engineering/x86_64/geth:1.5.7\",\"command\":[\"start.sh\"]}}}","deployment_signature":"abcdefg","deployment_user_info":"","torrent":{"url":"https://images.bluehorizon.network/f27f762cef632af1a19cd8a761ac4c3da4f9ef7d.torrent","images":[{"file":"f27f762cef632af1a19cd8a761ac4c3da4f9ef7d.tar.gz","signature":"123456"}]}}}]}`

	detailsObj := new(BlockchainDetails)
	if err := json.Unmarshal([]byte(details), detailsObj); err != nil {
		t.Errorf("Could not unmarshal details, error %v\n", err)
	} else {
		for _, chain := range detailsObj.Chains {
			if chain.Arch != "amd64" {
				t.Errorf("Could not find amd64 arch in the array.\n")
			} else if chain.DeploymentDesc.Deployment[:6] != `{"serv` {
				t.Errorf("Could not find deployment string %v in %v.\n", `{"serv`, chain.DeploymentDesc.Deployment[:6])
			}
		}
	}

}
