package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"time"
)

type Agbot struct {
	Token         string `json:"token"`
	Name          string `json:"name"`
	Owner         string `json:"owner"`
	MsgEndPoint   string `json:"msgEndPoint"`
	LastHeartbeat string `json:"lastHeartbeat"`
	PublicKey     []byte `json:"publicKey"`
}

func (a Agbot) String() string {
	return fmt.Sprintf("Name: %v, Owner: %v, LastHeartbeat: %v, PublicKey: %x", a.Name, a.Owner, a.LastHeartbeat, a.PublicKey)
}

func (a Agbot) ShortString() string {
	return fmt.Sprintf("Name: %v, Owner: %v, LastHeartbeat: %v", a.Name, a.Owner, a.LastHeartbeat)
}

type GetAgbotsResponse struct {
	Agbots    map[string]Agbot `json:"agbots"`
	LastIndex int              `json:"lastIndex"`
}

type GetAgbotsPatternsResponse struct {
	Patterns map[string]ServedPattern `json:"patterns"`
}

// TODO-New check with exchange
type GetAgbotsBusinessPolsResponse struct {
	BusinessPols map[string]ServedBusinessPolicy `json:"businessPols"`
}

type AgbotAgreement struct {
	Service     WorkloadAgreement `json:"service,omitempty"`
	State       string            `json:"state"`
	LastUpdated string            `json:"lastUpdated,omitempty"`
}

func (a AgbotAgreement) String() string {
	return fmt.Sprintf("Service: %v, State: %v, LastUpdated: %v", a.Service, a.State, a.LastUpdated)
}

type AllAgbotAgreementsResponse struct {
	Agreements map[string]AgbotAgreement `json:"agreements"`
	LastIndex  int                       `json:"lastIndex"`
}

func (a AllAgbotAgreementsResponse) String() string {
	return fmt.Sprintf("Agreements: %v, LastIndex: %v", a.Agreements, a.LastIndex)
}

type AgbotMessage struct {
	MsgId        int    `json:"msgId"`
	DeviceId     string `json:"nodeId"`
	DevicePubKey []byte `json:"nodePubKey"`
	Message      []byte `json:"message"`
	TimeSent     string `json:"timeSent"`
	TimeExpires  string `json:"timeExpires"`
}

func (a AgbotMessage) String() string {
	return fmt.Sprintf("MsgId: %v, DeviceId: %v, TimeSent %v, TimeExpires %v, DevicePubKey %v, Message %v", a.MsgId, a.DeviceId, a.TimeSent, a.TimeExpires, a.DevicePubKey, cutil.TruncateDisplayString(string(a.Message), 32))
}

type GetAgbotMessageResponse struct {
	Messages  []AgbotMessage `json:"messages"`
	LastIndex int            `json:"lastIndex"`
}

type PutAgbotAgreementState struct {
	Service WorkloadAgreement `json:"service,omitempty"`
	State   string            `json:"state"`
}

// patterns served by an agbot that are allowed to be put on the nodes of an org.
type ServedPattern struct {
	PatternOrg  string `json:"patternOrgid"` // defaults to NodeOrg
	Pattern     string `json:"pattern"`      // '*' means all
	NodeOrg     string `json:"nodeOrgid"`
	LastUpdated string `json:"lastUpdated,omitempty"`
}

// business policies served by an agbot that are allowed to be put on the nodes of an org.
type ServedBusinessPolicy struct {
	BusinessPolOrg string `json:"businessPolOrgid"` // defaults to nodeOrgid
	BusinessPol    string `json:"businessPol"`      // '*' means all
	NodeOrg        string `json:"nodeOrgid"`
	LastUpdated    string `json:"lastUpdated,omitempty"`
}

type PatchAgbotPublicKey struct {
	PublicKey []byte `json:"publicKey"`
}

func (p PatchAgbotPublicKey) String() string {
	return fmt.Sprintf("PublicKey: %v", p.PublicKey)
}

func (p PatchAgbotPublicKey) ShortString() string {
	if p.PublicKey == nil {
		return fmt.Sprintf("PublicKey: nil")
	} else {
		pk := cutil.TruncateDisplayString(string(p.PublicKey), 30)
		return fmt.Sprintf("PublicKey: %v", []byte(pk))
	}
}

// This function creates the device registration message body.
func CreateAgbotPublicKeyPatch(keyPath string) *PatchAgbotPublicKey {

	keyBytes := func() []byte {
		if pubKey, _, err := GetKeys(keyPath); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf("Error getting keys %v", err)))
			return []byte(`none`)
		} else if b, err := MarshalPublicKey(pubKey); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf("Error marshalling agbot public key %v, error %v", pubKey, err)))
			return []byte(`none`)
		} else {
			return b
		}
	}

	pdr := &PatchAgbotPublicKey{
		PublicKey: keyBytes(),
	}

	return pdr
}

func GetAgbotDeploymentPols(ec ExchangeContext) (map[string]ServedBusinessPolicy, error) {

	var resp interface{}
	resp = new(GetAgbotsBusinessPolsResponse)
	targetURL := ec.GetExchangeURL() + "orgs/" + GetOrg(ec.GetExchangeId()) + "/agbots/" + GetId(ec.GetExchangeId()) + "/businesspols"
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(err.Error()))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(tpErr.Error()))
			time.Sleep(10 * time.Second)
			continue
		} else {
			pols := resp.(*GetAgbotsBusinessPolsResponse).BusinessPols
			glog.V(5).Infof(rpclogString(fmt.Sprintf("retrieved agbot serviced deployment policy names from exchange %v", pols)))
			return pols, nil
		}
	}
}

func GetAgbotPatterns(ec ExchangeContext) (map[string]ServedPattern, error) {

	var resp interface{}
	resp = new(GetAgbotsPatternsResponse)
	targetURL := ec.GetExchangeURL() + "orgs/" + GetOrg(ec.GetExchangeId()) + "/agbots/" + GetId(ec.GetExchangeId()) + "/patterns"
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(err.Error()))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(tpErr.Error()))
			time.Sleep(10 * time.Second)
			continue
		} else {
			pats := resp.(*GetAgbotsPatternsResponse).Patterns
			glog.V(5).Infof(rpclogString(fmt.Sprintf("retrieved agbot served patterns from exchange %v", pats)))
			return pats, nil
		}
	}

}
