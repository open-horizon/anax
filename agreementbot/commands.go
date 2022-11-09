package agreementbot

import (
	"fmt"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/policy"
)

// ==============================================================================================================
type AgreementTimeoutCommand struct {
	AgreementId string
	Protocol    string
	Reason      uint
}

func (a AgreementTimeoutCommand) ShortString() string {
	return fmt.Sprintf("%v", a)
}

func NewAgreementTimeoutCommand(agreementId string, protocol string, reason uint) *AgreementTimeoutCommand {
	return &AgreementTimeoutCommand{
		AgreementId: agreementId,
		Protocol:    protocol,
		Reason:      reason,
	}
}

// ==============================================================================================================
type PolicyChangedCommand struct {
	Msg events.PolicyChangedMessage
}

func (p PolicyChangedCommand) ShortString() string {
	return fmt.Sprintf("%v", p)
}

func NewPolicyChangedCommand(msg events.PolicyChangedMessage) *PolicyChangedCommand {
	return &PolicyChangedCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type PolicyDeletedCommand struct {
	Msg events.PolicyDeletedMessage
}

func (p PolicyDeletedCommand) ShortString() string {
	return fmt.Sprintf("%v", p)
}

func NewPolicyDeletedCommand(msg events.PolicyDeletedMessage) *PolicyDeletedCommand {
	return &PolicyDeletedCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type NewProtocolMessageCommand struct {
	Message   []byte
	MessageId int
	From      string
	PubKey    []byte
}

func (p NewProtocolMessageCommand) ShortString() string {
	return fmt.Sprintf("%v", p)
}

func NewNewProtocolMessageCommand(msg []byte, msgId int, deviceId string, pubkey []byte) *NewProtocolMessageCommand {
	return &NewProtocolMessageCommand{
		Message:   msg,
		MessageId: msgId,
		From:      deviceId,
		PubKey:    pubkey,
	}
}

// ==============================================================================================================
type BlockchainEventCommand struct {
	Msg events.EthBlockchainEventMessage
}

func (e BlockchainEventCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewBlockchainEventCommand(msg events.EthBlockchainEventMessage) *BlockchainEventCommand {
	return &BlockchainEventCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type WorkloadUpgradeCommand struct {
	Msg events.ABApiWorkloadUpgradeMessage
}

func (e WorkloadUpgradeCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewWorkloadUpgradeCommand(msg events.ABApiWorkloadUpgradeMessage) *WorkloadUpgradeCommand {
	return &WorkloadUpgradeCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type MakeAgreementCommand struct {
	ProducerPolicy     policy.Policy                            // the producer policy received from the exchange
	ConsumerPolicy     policy.Policy                            // the consumer policy we're matched up with
	Org                string                                   // the org of the consumer
	ConsumerPolicyName string                                   // the name of the consumer policy in the exchange
	Device             exchange.SearchResultDevice              // the device entry in the exchange
	ServicePolicies    map[string]externalpolicy.ExternalPolicy // cached service polices, keyed by service id. it is a subset of the service versions in the consumer policy file
}

func (e MakeAgreementCommand) ShortString() string {
	keys := []string{}
	if e.ServicePolicies != nil {
		for k, _ := range e.ServicePolicies {
			keys = append(keys, k)
		}
	}
	return fmt.Sprintf("Produder Policy: %v, ConsumerPolicy: %v, Org: %v, ConsumerPolicyName %v, Device: %v, ServicePolicies: %v", e.ProducerPolicy.Header.Name, e.ConsumerPolicy.Header.Name, e.Org, e.ConsumerPolicyName, e.Device, keys)
}

func NewMakeAgreementCommand(pPol policy.Policy, cPol policy.Policy, org string, polname string, dev exchange.SearchResultDevice, cachedServicePolicies map[string]externalpolicy.ExternalPolicy) *MakeAgreementCommand {

	copiedConsumerPolicy := cPol.DeepCopy()

	return &MakeAgreementCommand{
		ProducerPolicy:     pPol,
		ConsumerPolicy:     *copiedConsumerPolicy,
		Org:                org,
		ConsumerPolicyName: polname,
		Device:             dev,
		ServicePolicies:    cachedServicePolicies,
	}
}

// ==============================================================================================================
type ClientInitializedCommand struct {
	Msg events.BlockchainClientInitializedMessage
}

func (e ClientInitializedCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewClientInitializedCommand(msg *events.BlockchainClientInitializedMessage) *ClientInitializedCommand {
	return &ClientInitializedCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type ClientStoppingCommand struct {
	Msg events.BlockchainClientStoppingMessage
}

func (e ClientStoppingCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewClientStoppingCommand(msg *events.BlockchainClientStoppingMessage) *ClientStoppingCommand {
	return &ClientStoppingCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type AccountFundedCommand struct {
	Msg events.AccountFundedMessage
}

func (e AccountFundedCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewAccountFundedCommand(msg *events.AccountFundedMessage) *AccountFundedCommand {
	return &AccountFundedCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type AgbotShutdownCommand struct {
	Msg events.NodeShutdownMessage
}

func (e AgbotShutdownCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewAgbotShutdownCommand(msg *events.NodeShutdownMessage) *AgbotShutdownCommand {
	return &AgbotShutdownCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type CacheServicePolicyCommand struct {
	Msg events.CacheServicePolicyMessage
}

func (e CacheServicePolicyCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewCacheServicePolicyCommand(msg *events.CacheServicePolicyMessage) *CacheServicePolicyCommand {
	return &CacheServicePolicyCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type ServicePolicyChangedCommand struct {
	Msg events.ServicePolicyChangedMessage
}

func (e ServicePolicyChangedCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewServicePolicyChangedCommand(msg *events.ServicePolicyChangedMessage) *ServicePolicyChangedCommand {
	return &ServicePolicyChangedCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type ServicePolicyDeletedCommand struct {
	Msg events.ServicePolicyDeletedMessage
}

func (e ServicePolicyDeletedCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewServicePolicyDeletedCommand(msg *events.ServicePolicyDeletedMessage) *ServicePolicyDeletedCommand {
	return &ServicePolicyDeletedCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type NodePolicyChangedCommand struct {
	Msg events.NodePolicyChangedMessage
}

func (e NodePolicyChangedCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewNodePolicyChangedCommand(msg *events.NodePolicyChangedMessage) *NodePolicyChangedCommand {
	return &NodePolicyChangedCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type MMSObjectPolicyEventCommand struct {
	Msg events.MMSObjectPolicyMessage
}

func (e MMSObjectPolicyEventCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewMMSObjectPolicyEventCommand(msg *events.MMSObjectPolicyMessage) *MMSObjectPolicyEventCommand {
	return &MMSObjectPolicyEventCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type MessageCommand struct {
	Msg events.ExchangeChangeMessage
}

func (e MessageCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewMessageCommand(msg *events.ExchangeChangeMessage) *MessageCommand {
	return &MessageCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type PatternChangeCommand struct {
	Msg events.ExchangeChangeMessage
}

func (e PatternChangeCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewPatternChangeCommand(msg *events.ExchangeChangeMessage) *PatternChangeCommand {
	return &PatternChangeCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type PolicyChangeCommand struct {
	Msg events.ExchangeChangeMessage
}

func (e PolicyChangeCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewPolicyChangeCommand(msg *events.ExchangeChangeMessage) *PolicyChangeCommand {
	return &PolicyChangeCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type ServicePolicyChangeCommand struct {
	Msg events.ExchangeChangeMessage
}

func (e ServicePolicyChangeCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewServicePolicyChangeCommand(msg *events.ExchangeChangeMessage) *ServicePolicyChangeCommand {
	return &ServicePolicyChangeCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type ObjectPoliciesChangeCommand struct {
	Msg events.MMSObjectPoliciesMessage
}

func (e ObjectPoliciesChangeCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewObjectPoliciesChangeCommand(msg *events.MMSObjectPoliciesMessage) *ObjectPoliciesChangeCommand {
	return &ObjectPoliciesChangeCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type ServedPatternCommand struct {
}

func (e ServedPatternCommand) ShortString() string {
	return "ServedPatternCommand"
}

func NewServedPatternCommand() *ServedPatternCommand {
	return &ServedPatternCommand{}
}

// ==============================================================================================================
type ServedPolicyCommand struct {
}

func (e ServedPolicyCommand) ShortString() string {
	return "ServedPolicyCommand"
}

func NewServedPolicyCommand() *ServedPolicyCommand {
	return &ServedPolicyCommand{}
}

// ==============================================================================================================
type HAGroupChangedCommand struct {
	Msg events.ExchangeChangeMessage
}

func (p HAGroupChangedCommand) ShortString() string {
	return fmt.Sprintf("%v", p)
}

func NewHAGroupChangedCommand(msg *events.ExchangeChangeMessage) *HAGroupChangedCommand {
	return &HAGroupChangedCommand{
		Msg: *msg,
	}
}
