package citizenscientist

import (
    "fmt"
    "github.com/open-horizon/anax/abstractprotocol"
    "testing"
)

// Validation tests
func Test_Proposal_validation(t *testing.T) {

    proposal1 := `{"address":"123456","tsandcs":"abc","producerPolicy":"policy","consumerId":"ag12345","type":"proposal","protocol":"Citizen Scientist","version":1,"agreementId":"deadbeef"}`

    ph := new(ProtocolHandler)
    if _, err := ph.ValidateProposal(proposal1); err != nil {
        t.Errorf("Error validating %v, error %v", proposal1, err)
    }

}