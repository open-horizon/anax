// +build unit

package agreementbot

import (
	"encoding/json"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/policy"
	"sync"
	"testing"
)

func Test_agreement_success1(t *testing.T) {

	expectedType := policy.Ethereum_bc
	expectedName := policy.Default_Blockchain_name
	expectedOrg := policy.Default_Blockchain_org

	testProposal := `{"address":"123456","producerPolicy":"policy","consumerId":"ag12345","type":"proposal","protocol":"Citizen Scientist","version":1,"agreementId":"deadbeef"}`
	testPolicy := `{"header":{"name":"testpolicy","version":"1.0"},"agreementProtocols":[{"name":"Citizen Scientist"}]}`

	if ag, err := createAgreement(testProposal, testPolicy, 0, expectedType, expectedName, expectedOrg); err != nil {
		t.Errorf("Error creating mock agreement, %v", err)
	} else if bcType, bcName, bcOrg := createEmptyPH().GetKnownBlockchain(ag); bcType != expectedType || bcName != expectedName || bcOrg != expectedOrg {
		t.Errorf("Wrong BC type %v and name %v and org %v returned, expecting %v %v %v", bcType, bcName, bcOrg, expectedType, expectedName, expectedOrg)
	}

}

// Utility to help create the testing context
func createEmptyPH() *CSProtocolHandler {
	return &CSProtocolHandler{
		BaseConsumerProtocolHandler: &BaseConsumerProtocolHandler{
			name:             "test",
			pm:               nil,
			db:               nil,
			config:           nil,
			httpClient:       nil,
			agbotId:          "ag12345",
			token:            "abcdefg",
			deferredCommands: nil,
			messages:         nil,
		},
		genericAgreementPH: nil,
		Work:               nil,
		bcState:            nil,
		bcStateLock:        sync.Mutex{},
	}
}

func createAgreement(proposal string, pol string, agpVersion int, bcType string, bcName string, bcOrg string) (*Agreement, error) {
	if ag, err := agreement("testagid", "testorg", "deviceid", "testpolicy", bcType, bcName, bcOrg, "Citizen Scientist", "apattern", policy.NodeHealth{}); err != nil {
		return nil, err
	} else {
		prop := new(citizenscientist.CSProposal)
		if err := json.Unmarshal([]byte(proposal), prop); err != nil {
			return nil, err
		} else {
			prop.TsandCs = pol
			if propString, err := json.Marshal(prop); err != nil {
				return nil, err
			} else {
				ag.Proposal = string(propString)
				ag.AgreementProtocolVersion = agpVersion
				return ag, nil
			}
		}
	}
}
