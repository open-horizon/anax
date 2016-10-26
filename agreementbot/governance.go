package agreementbot

import (
    "fmt"
    "github.com/golang/glog"
    "github.com/open-horizon/anax/citizenscientist"
    "github.com/open-horizon/anax/exchange"
    "net/http"
    "runtime"
    "time"
)

func (w *AgreementBotWorker) GovernAgreements() {

    glog.Info(logString(fmt.Sprintf("started")))

    protocolHandler := citizenscientist.NewProtocolHandler(w.Config.AgreementBot.GethURL, w.pm)

    for {

        notYetFinalFilter := func () AFilter {
            return func(a Agreement) bool { return a.AgreementInceptionTime != 0 && a.AgreementTimedout == 0 }
        }

        // Find all agreements that are in progress. They might be waiting for a reply or not yet finalized on blockchain.
        if agreements, err := FindAgreements(w.db, []AFilter{notYetFinalFilter()}); err == nil {
            for _, ag := range agreements {

                // Govern agreements that havent been finalized yet, we are waiting for confirmation in the blockchain
                if ag.CounterPartyAddress != "" && ag.AgreementFinalizedTime == 0  {

                    // We are waiting for the write to the blockchain. The counterparty address comes in the reply
                    glog.V(5).Infof("AgreementBot Governance checking agreement %v for finalization.", ag.CurrentAgreementId)
                    if recorded, err := protocolHandler.VerifyAgreementRecorded(ag.CurrentAgreementId, ag.CounterPartyAddress, ag.ProposalSig, w.bc.Agreements); err != nil {
                        glog.Errorf(logString(fmt.Sprintf("unable to verify agreement %v on blockchain, error: %v", ag.CurrentAgreementId, err)))
                    } else if recorded {
                        // Update state in the database
                        if _, err := AgreementFinalized(w.db, ag.CurrentAgreementId); err != nil {
                            glog.Errorf(logString(fmt.Sprintf("error persisting agreement %v finalized: %v", ag.CurrentAgreementId, err)))
                        }
                        // Update state in exchange
                        if err := recordConsumerAgreementState(w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token, ag.CurrentAgreementId, "", "Finalized Agreement"); err != nil {
                            glog.Errorf(logString(fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", ag.CurrentAgreementId, err)))
                        }
                    } else {
                        glog.V(5).Infof("AgreementBot Governance detected agreement %v not yet final.", ag.CurrentAgreementId)
                        now := uint64(time.Now().Unix())
                        if ag.AgreementCreationTime + w.Worker.Manager.Config.AgreementBot.AgreementTimeoutS < now {
                            // Start timing out the agreement
                            w.TerminateAgreement(&ag, CANCEL_NOT_FINALIZED_TIMEOUT)
                        }
                    }

                // Govern agreements that we think are finalized in the blockchain, make sure they are still there
                } else if ag.CounterPartyAddress != "" && ag.AgreementFinalizedTime != 0 {
                    if recorded, err := protocolHandler.VerifyAgreementRecorded(ag.CurrentAgreementId, ag.CounterPartyAddress, ag.ProposalSig, w.bc.Agreements); err != nil {
                        glog.Errorf(logString(fmt.Sprintf("unable to verify finalized agreement %v on blockchain, error: %v", ag.CurrentAgreementId, err)))
                    } else if !recorded {
                        // The agreement is not on the blockchain, update state in exchange
                        glog.V(3).Infof(logString(fmt.Sprintf("discovered terminated agreement %v, cleaning up.", ag.CurrentAgreementId)))
                        w.TerminateAgreement(&ag, CANCEL_DISCOVERED)
                    }

                // Govern agreements that havent seen a proposal reply yet
                } else {
                    // We are waiting for a reply
                    glog.V(5).Infof("AgreementBot Governance waiting for reply to %v.", ag.CurrentAgreementId)
                    now := uint64(time.Now().Unix())
                    if ag.AgreementCreationTime + w.Worker.Manager.Config.AgreementBot.AgreementTimeoutS < now {
                        w.TerminateAgreement(&ag, CANCEL_NO_REPLY)
                    }
                }
            }
        } else {
            glog.Errorf(logString(fmt.Sprintf("unable to read agreements from database, error: %v", err)))
        }

        time.Sleep(time.Duration(w.Worker.Manager.Config.AgreementBot.ProcessGovernanceIntervalS) * time.Second)
        runtime.Gosched()
    }

    glog.Info(logString(fmt.Sprintf("terminated")))

}

func (w *AgreementBotWorker) TerminateAgreement(ag *Agreement, reason uint) {
    // Start timing out the agreement
    glog.V(3).Infof(logString(fmt.Sprintf("detected agreement %v needs to terminate.", ag.CurrentAgreementId)))

    // Update the database
    if _, err := AgreementTimedout(w.db, ag.CurrentAgreementId); err != nil {
        glog.Errorf(logString(fmt.Sprintf("error marking agreement %v terminate: %v", ag.CurrentAgreementId, err)))
    }
    // Update state in exchange
    if err := DeleteConsumerAgreement(w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token, ag.CurrentAgreementId); err != nil {
        glog.Errorf(logString(fmt.Sprintf("error deleting agreement %v in exchange: %v", ag.CurrentAgreementId, err)))
    }
    // Queue up a command for an agreement worker to do the blockchain work
    w.pwcommands <- NewAgreementTimeoutCommand(ag.CurrentAgreementId, ag.AgreementProtocol, reason)
}


func recordConsumerAgreementState(url string, agbotId string, token string, agreementId string, workloadID string, state string) error {

    logString := func(v interface{}) string {
        return fmt.Sprintf("AgreementBot Governance: %v", v)
    }

    glog.V(5).Infof(logString(fmt.Sprintf("setting agreement %v state to %v", agreementId, state)))

    as := new(exchange.PutAgbotAgreementState)
    as.Workload = workloadID
    as.State = state
    var resp interface{}
    resp = new(exchange.PostDeviceResponse)
    targetURL := url + "agbots/" + agbotId + "/agreements/" + agreementId + "?token=" + token
    for {
        if err, tpErr := exchange.InvokeExchange(&http.Client{}, "PUT", targetURL, &as, &resp); err != nil {
            glog.Errorf(logString(fmt.Sprintf(err.Error())))
            return err
        } else if tpErr != nil {
            glog.Warningf(err.Error())
            time.Sleep(10 * time.Second)
            continue
        } else {
            glog.V(5).Infof(logString(fmt.Sprintf("set agreement %v to state %v", agreementId, state)))
            return nil
        }
    }

}

func DeleteConsumerAgreement(url string, agbotId string, token string, agreementId string) error {

    logString := func(v interface{}) string {
        return fmt.Sprintf("AgreementBot Governance: %v", v)
    }

    glog.V(5).Infof(logString(fmt.Sprintf("deleting agreement %v in exchange", agreementId)))

    var resp interface{}
    resp = new(exchange.PostDeviceResponse)
    targetURL := url + "agbots/" + agbotId + "/agreements/" + agreementId + "?token=" + token
    for {
        if err, tpErr := exchange.InvokeExchange(&http.Client{}, "DELETE", targetURL, nil, &resp); err != nil {
            glog.Errorf(logString(fmt.Sprintf(err.Error())))
            return err
        } else if tpErr != nil {
            glog.Warningf(err.Error())
            time.Sleep(10 * time.Second)
            continue
        } else {
            glog.V(5).Infof(logString(fmt.Sprintf("deleted agreement %v from exchange", agreementId)))
            return nil
        }
    }

}

var logString = func(v interface{}) string {
    return fmt.Sprintf("AgreementBot Governance: %v", v)
}
