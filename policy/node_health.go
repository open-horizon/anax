package policy

import ()

type NodeHealth struct {
	MissingHBInterval    int `json:"missing_heartbeat_interval,omitempty"` // How long a heartbeat can be missing until it is considered missing (in seconds)
	CheckAgreementStatus int `json:"check_agreement_status,omitempty"`     // How often to check that the node agreement entry still exists in the exchange (in seconds)
}

func (h NodeHealth) IsSame(compare NodeHealth) bool {
	return h.MissingHBInterval == compare.MissingHBInterval && h.CheckAgreementStatus == compare.CheckAgreementStatus
}

func NodeHealth_Factory(hbInterval int, checkRate int) *NodeHealth {
	nh := new(NodeHealth)
	if hbInterval != 0 {
		nh.MissingHBInterval = hbInterval
	}
	if checkRate != 0 {
		nh.CheckAgreementStatus = checkRate
	}
	return nh
}
