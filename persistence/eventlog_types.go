package persistence

import (
	"fmt"
)

type AgreementEventSource struct {
	AgreementId       string       `json:"agreement_id"`
	RunningWorkload   WorkloadInfo `json:"workload_to_run"`
	ServiceUrl        []string     `json:"service_url"`
	ConsumerId        string       `json:"consumer_id"`
	AgreementProtocol string       `json:"agreement_protocol"`
}

func (w AgreementEventSource) String() string {
	return fmt.Sprintf(
		"AgreementId: %v, "+
			"RunningWorkload: %v, "+
			"ServiceUrl: %v, "+
			"ConsumerId: %v, "+
			"AgreementProtocol: %v",
		w.AgreementId, w.RunningWorkload, w.ServiceUrl, w.ConsumerId, w.AgreementProtocol)
}

func (w AgreementEventSource) ShortString() string {
	return w.String()
}

func NewAgreementEventSourceFromAg(ag EstablishedAgreement) *AgreementEventSource {
	source := AgreementEventSource{
		AgreementId:       ag.CurrentAgreementId,
		RunningWorkload:   ag.RunningWorkload,
		ServiceUrl:        ag.SensorUrl,
		ConsumerId:        ag.ConsumerId,
		AgreementProtocol: ag.AgreementProtocol,
	}

	return &source
}

func NewAgreementEventSource(agreement_id string, workload WorkloadInfo, service_url []string, consumer_id string, protocol string) *AgreementEventSource {
	source := AgreementEventSource{
		RunningWorkload:   workload,
		AgreementId:       agreement_id,
		ConsumerId:        consumer_id,
		AgreementProtocol: protocol,
		ServiceUrl:        service_url,
	}
	return &source
}

func (w AgreementEventSource) Matches(selectors map[string][]Selector) bool {
	for s_attr, s_vals := range selectors {
		handle := true
		var attr interface{}
		switch s_attr {
		case "workload_to_run":
			attr = fmt.Sprintf("%v", w.RunningWorkload)
		case "workload_to_run.url":
			attr = w.RunningWorkload.URL
		case "workload_to_run.org":
			attr = w.RunningWorkload.Org
		case "workload_to_run.arch":
			attr = w.RunningWorkload.Arch
		case "workload_to_run.version":
			attr = w.RunningWorkload.Version
		case "agreement_id":
			attr = w.AgreementId
		case "consumer_id":
			attr = w.ConsumerId
		case "service_url":
			matches := false
			for _, url1 := range w.ServiceUrl {
				if m, _, _ := MatchAttributeValue(url1, s_vals); m {
					matches = true
					break
				}
			}
			if !matches {
				return false
			}
			handle = false
		case "agreement_protocol":
			attr = w.AgreementProtocol
		default:
			return false // not tolerate wrong attribute name in the selector
		}

		if handle {
			m, _, _ := MatchAttributeValue(attr, s_vals)
			if !m {
				return false
			}
		}
	}
	return true
}

// purposly made some of the attribute names the same as the AgreementEventSource for easy search.
type ServiceEventSource struct {
	InstanceId           string   `json:"instance_id"`
	ServiceUrl           string   `json:"service_url"`
	Org                  string   `json:"organization"`
	Version              string   `json:"version"`
	Arch                 string   `json:"arch"`
	AssociatedAgreements []string `json:"agreement_id"`
}

func (w ServiceEventSource) String() string {
	return fmt.Sprintf("InstanceId: %v, "+
		"ServiceUrl: %v, "+
		"Org: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"AssociatedAgreements: %v",
		w.InstanceId, w.ServiceUrl, w.Org, w.Version, w.Arch, w.AssociatedAgreements)
}

func (w ServiceEventSource) ShortString() string {
	return w.String()
}

func NewServiceEventSourceFromServiceInstance(msi MicroserviceInstance) *ServiceEventSource {
	source := ServiceEventSource{
		InstanceId:           msi.InstanceId,
		ServiceUrl:           msi.SpecRef,
		Org:                  "", // msi does not contain the org
		Version:              msi.Version,
		Arch:                 msi.Arch,
		AssociatedAgreements: msi.AssociatedAgreements,
	}

	return &source
}

func NewServiceEventSourceFromServiceDef(msdef MicroserviceDefinition) *ServiceEventSource {
	source := ServiceEventSource{
		InstanceId:           "", // msdef does not contain the instance id
		ServiceUrl:           msdef.SpecRef,
		Org:                  msdef.Org,
		Version:              msdef.Version,
		Arch:                 msdef.Arch,
		AssociatedAgreements: []string{}, // msdef does not contain the agreement ids
	}
	return &source
}

func NewServiceEventSource(instance_id string, service_url string, org string, version string, arch string, agreement_ids []string) *ServiceEventSource {
	source := ServiceEventSource{
		InstanceId:           instance_id,
		ServiceUrl:           service_url,
		Org:                  org,
		Version:              version,
		Arch:                 arch,
		AssociatedAgreements: agreement_ids,
	}
	return &source
}

//Check if this event log matches the input selection.
func (w ServiceEventSource) Matches(selectors map[string][]Selector) bool {
	for s_attr, s_vals := range selectors {
		handle := true
		var attr interface{}
		switch s_attr {
		case "instance_id":
			attr = w.InstanceId
		case "service_url":
			attr = w.ServiceUrl
		case "organization":
			attr = w.Org
		case "version":
			attr = w.Version
		case "arch":
			attr = w.Arch
		case "agreement_id":
			matches := false
			for _, id1 := range w.AssociatedAgreements {
				if m, _, _ := MatchAttributeValue(id1, s_vals); m {
					matches = true
					break
				}
			}
			if !matches {
				return false
			}
			handle = false
		default:
			return false // not tolerate wrong attribute name in the selector
		}

		if handle {
			m, _, _ := MatchAttributeValue(attr, s_vals)
			if !m {
				return false
			}
		}
	}
	return true
}

type NodeEventSource struct {
	Id          string `json:"node_id"`
	Org         string `json:"node_org"`
	Pattern     string `json:"pattern"` // fprmat: pattern_org/pattern
	ConfigState string `json:"config_state"`
}

func (w NodeEventSource) String() string {
	return fmt.Sprintf("Id: %v, "+
		"Org: %v, "+
		"Pattern: %v, "+
		"ConfigState: %v",
		w.Id, w.Org, w.Pattern, w.ConfigState)
}

func (w NodeEventSource) ShortString() string {
	return w.String()
}

func NewNodeEventSource(id string, org string, pattern string, state string) *NodeEventSource {
	source := NodeEventSource{
		Id:          id,
		Org:         org,
		Pattern:     pattern,
		ConfigState: state,
	}
	return &source
}

func (w NodeEventSource) Matches(selectors map[string][]Selector) bool {
	for s_attr, s_vals := range selectors {
		handle := true
		var attr interface{}
		switch s_attr {
		case "node_id":
			attr = w.Id
		case "node_org":
			attr = w.Org
		case "pattern":
			attr = w.Pattern
		case "config_state":
			attr = w.ConfigState
		default:
			return false // not tolerate wrong attribute name in the selector
		}

		if handle {
			m, _, _ := MatchAttributeValue(attr, s_vals)
			if !m {
				return false
			}
		}
	}
	return true
}

type DatabaseEventSource struct {
}

func NewDatabaseEventSource() *DatabaseEventSource {
	source := DatabaseEventSource{}
	return &source
}

func (w DatabaseEventSource) String() string {
	return ""
}

func (w DatabaseEventSource) ShortString() string {
	return w.String()
}

func (w DatabaseEventSource) Matches(selectors map[string][]Selector) bool {
	return true
}

type ExchangeEventSource struct {
	ExchangeUrl string `json:"exchange_url"`
}

func (w ExchangeEventSource) String() string {
	return fmt.Sprintf("ExchangeUrl: %v", w.ExchangeUrl)
}

func (w ExchangeEventSource) ShortString() string {
	return w.String()
}

func NewExchangeEventSource(exchange_url string) *ExchangeEventSource {
	source := ExchangeEventSource{
		ExchangeUrl: exchange_url,
	}
	return &source
}

func (w ExchangeEventSource) Matches(selectors map[string][]Selector) bool {
	for s_attr, s_vals := range selectors {
		handle := true
		var attr interface{}
		switch s_attr {
		case "exchange_url":
			attr = w.ExchangeUrl
		default:
			return false // not tolerate wrong attribute name in the selector
		}

		if handle {
			m, _, _ := MatchAttributeValue(attr, s_vals)
			if !m {
				return false
			}
		}
	}
	return true
}
