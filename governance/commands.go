package governance

import (
    "fmt"
    "github.com/open-horizon/anax/events"
    "github.com/open-horizon/anax/persistence"
)

type StartGovernExecutionCommand struct {
    AgreementId       string
    AgreementProtocol string
    Deployment        map[string]persistence.ServiceConfig
}

func (g StartGovernExecutionCommand) ShortString() string {
    depStr := ""
    for key, _ := range g.Deployment {
        depStr = depStr + key + ","
    }

    return fmt.Sprintf("AgreementId %v, AgreementProtocol %v, Deployed Services %v", g.AgreementId, g.AgreementProtocol, depStr)
}

func (w *GovernanceWorker) NewStartGovernExecutionCommand(deployment map[string]persistence.ServiceConfig, protocol string, agreementId string) *StartGovernExecutionCommand {
    return &StartGovernExecutionCommand{
        AgreementId:       agreementId,
        AgreementProtocol: protocol,
        Deployment:        deployment,
    }
}

// ==============================================================================================================
type CleanupExecutionCommand struct {
    AgreementProtocol string
    AgreementId       string
    Reason            uint
    Deployment        map[string]persistence.ServiceConfig
}

func (c CleanupExecutionCommand) ShortString() string {
    depStr := ""
    for key, _ := range c.Deployment {
        depStr = depStr + key + ","
    }

    return fmt.Sprintf("AgreementId %v, AgreementProtocol %v, Reason %v, Deployed Services %v", c.AgreementId, c.AgreementProtocol, c.Reason, depStr)
}

func (w *GovernanceWorker) NewCleanupExecutionCommand(protocol string, agreementId string, reason uint, deployment map[string]persistence.ServiceConfig) *CleanupExecutionCommand {
    return &CleanupExecutionCommand{
        AgreementProtocol: protocol,
        AgreementId:       agreementId,
        Reason:            reason,
        Deployment:        deployment,
    }
}

// ==============================================================================================================
type CleanupStatusCommand struct {
    AgreementProtocol string
    AgreementId       string
    Status            uint
}

func (c CleanupStatusCommand) ShortString() string {

    return fmt.Sprintf("AgreementId %v, AgreementProtocol %v, Status %v", c.AgreementId, c.AgreementProtocol, c.Status)
}

func (w *GovernanceWorker) NewCleanupStatusCommand(protocol string, agreementId string, status uint) *CleanupStatusCommand {
    return &CleanupStatusCommand{
        AgreementProtocol: protocol,
        AgreementId:       agreementId,
        Status:            status,
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
type ExchangeMessageCommand struct {
    Msg events.ExchangeDeviceMessage
}

func (e ExchangeMessageCommand) ShortString() string {
    return e.Msg.ShortString()
}

func NewExchangeMessageCommand(msg events.ExchangeDeviceMessage) *ExchangeMessageCommand {
    return &ExchangeMessageCommand{
        Msg: msg,
    }
}