package governance

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/exchangesync"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/producer"
	"time"
)

type ChangePattern struct {
	NewPattern string
	LastError  string
}

func (c ChangePattern) String() string {
	return fmt.Sprintf("NewPattern: %v, LastError: %v", c.NewPattern, c.LastError)
}

func (c ChangePattern) Reset() {
	c.NewPattern = ""
	c.LastError = ""
}

// This is called after the node heartneat is restored. For the basic protocol, it will contact the agbot to check if the current agreements are
// still needed by the agbot.
func (w *GovernanceWorker) handleNodeHeartbeatRestored(checkAll bool) error {
	glog.V(5).Infof(logString(fmt.Sprintf("handling agreements after node heartbeat restored.")))

	if ags, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter()}); err != nil {
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_UNARCHIVED_AG_FROM_DB, err.Error()),
			persistence.EC_DATABASE_ERROR)
		return fmt.Errorf(logString(fmt.Sprintf("Unable to retrieve unarchived agreements from database. %v", err)))
	} else {
		veryfication_failed := false
		for _, ag := range ags {
			timeSinceVer := uint64(time.Now().Unix()) - ag.LastVerAttemptUpdateTime
			if ag.AgreementTerminatedTime == 0 && (checkAll || (ag.FailedVerAttempts != 0 && timeSinceVer > 60)) {
				bcType, bcName, bcOrg := w.producerPH[ag.AgreementProtocol].GetKnownBlockchain(&ag)

				// Check to see if the agreement is valid. For agreement on the blockchain, we check the blockchain directly. This call to the blockchain
				// should be very fast if the client is up and running. For other agreements, send a message to the agbot to get the agbot's opinion
				// on the agreement.
				// Remember, the device might have been down for some time and/or restarted, causing it to miss events on the blockchain.
				if w.producerPH[ag.AgreementProtocol].IsBlockchainClientAvailable(bcType, bcName, bcOrg) && w.producerPH[ag.AgreementProtocol].IsAgreementVerifiable(&ag) {

					if _, err := w.producerPH[ag.AgreementProtocol].VerifyAgreement(&ag); err != nil {
						glog.Errorf(logString(fmt.Sprintf("encountered error verifying agreement %v, error %v", ag.CurrentAgreementId, err)))
						eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_GOV_ERR_AG_VERIFICATION, ag.RunningWorkload.URL, err.Error()),
							persistence.EC_ERROR_AGREEMENT_VERIFICATION,
							ag)
						veryfication_failed = true
						if ag.FailedVerAttempts > 5 {
							reason := w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_FAILED_AGREEMENT_VERIFY)
							w.cancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, reason, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason))
						}
					} else {
						_, err := persistence.SetFailedVerAttempts(w.db, ag.CurrentAgreementId, ag.AgreementProtocol, ag.FailedVerAttempts+1)
						if err != nil {
							glog.Errorf(logString(fmt.Sprintf("encountered error updating agreement %v, error %v", ag.CurrentAgreementId, err)))
						}
					}
				}
			}
		}

		// put it to the deferred queue and retry
		if veryfication_failed {
			w.AddDeferredCommand(w.NewNodeHeartbeatRestoredCommand(true))
		}
	}
	return nil
}

// User input has been changes. Go through all the agreement to see if it has dependencies on the given
// services. If it does, cancel it.
func (w *GovernanceWorker) handleNodeUserInputUpdated(svcSpecs persistence.ServiceSpecs) {

	glog.V(5).Infof(logString(fmt.Sprintf("handling node user input changes")))

	if svcSpecs == nil || len(svcSpecs) == 0 {
		return
	}

	// get all the unarchived agreements
	agreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter()})
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to retrieve all the  from the database, error %v", err)))
		return
	}

	// cancel the agreement if needed
	for _, ag := range agreements {
		agreementId := ag.CurrentAgreementId
		if ag.AgreementTerminatedTime != 0 && ag.AgreementForceTerminatedTime == 0 {
			glog.V(3).Infof(logString(fmt.Sprintf("skip agreement %v, it is already terminating", agreementId)))
		} else {
			bCancel, err := w.agreementRequiresService(ag, svcSpecs)
			if err != nil {
				glog.Errorf(fmt.Sprintf("%v", err))
			}

			if bCancel {
				glog.V(3).Infof(logString(fmt.Sprintf("ending the agreement: %v", agreementId)))

				reason := w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_NODE_USERINPUT_CHANGED)

				eventlog.LogAgreementEvent(
					w.db,
					persistence.SEVERITY_INFO,
					persistence.NewMessageMeta(EL_GOV_START_TERM_AG_WITH_REASON, ag.RunningWorkload.URL, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason)),
					persistence.EC_CANCEL_AGREEMENT,
					ag)

				w.cancelAgreement(agreementId, ag.AgreementProtocol, reason, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason))

				// send the event to the container in case it has started the workloads.
				w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, agreementId, ag.GetDeploymentConfig())
				// clean up microservice instances if needed
				w.handleMicroserviceInstForAgEnded(agreementId, false)
			}
		}
	}
}

// Node pattern has been changes. Go unregister and re-register.
func (w *GovernanceWorker) handleNodeExchPatternChanged(shutdown bool, new_pattern string) {
	glog.V(5).Infof(logString(fmt.Sprintf("handling node pattern changes")))

	pDevice, err := persistence.FindExchangeDevice(w.db)
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("error getting device from the local database. %v", err)))
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_DEVICE_FROM_DB, err.Error()),
			persistence.EC_DATABASE_ERROR)
		return
	}

	if shutdown {
		// only mention it once
		if w.patternChange.LastError == "" {
			eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_INFO,
				persistence.NewMessageMeta(EL_GOV_EXCH_NODE_PATTERN_CHANGED, w.devicePattern, new_pattern),
				persistence.EC_NODE_PATTERN_CHANGED)
		}

		// check if the node has enough user input for this pattern
		getPatterns := exchange.GetHTTPExchangePatternHandler(w)
		serviceResolver := exchange.GetHTTPServiceResolverHandler(w)
		getService := exchange.GetHTTPServiceHandler(w)

		// only log the same error once
		if err := ValidateNewPattern(pDevice.GetNodeType(), new_pattern, getPatterns, serviceResolver, getService, w.db, w.Config); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error validating new node pattern %v: %v", new_pattern, err)))

			if w.patternChange.NewPattern != new_pattern || w.patternChange.LastError != err.Error() {
				w.patternChange.NewPattern = new_pattern
				w.patternChange.LastError = err.Error()

				eventlog.LogNodeEvent(w.db,
					persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_ERR_VALIDATE_NEW_PATTERN, new_pattern, err.Error()),
					persistence.EC_ERROR_VALIDATE_NEW_PATTERN,
					pDevice.Id, pDevice.Org, new_pattern, pDevice.Config.State)

				glog.V(3).Infof(logString(fmt.Sprintf("The node will keep using the old pattern %v.", w.devicePattern)))
				eventlog.LogNodeEvent(w.db,
					persistence.SEVERITY_INFO,
					persistence.NewMessageMeta(EL_GOV_NODE_KEEP_OLD_PATTERN, w.devicePattern),
					persistence.EC_NODE_KEEP_OLD_PATTERN,
					pDevice.Id, pDevice.Org, new_pattern, pDevice.Config.State)
			}

			// remove the pattern change flag from the local database so that the pattern can be tried again because
			// user input may change.
			if err := persistence.DeleteNodeExchPattern(w.db); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error deleting node exchange pattern from the local database. %v", err)))
				eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_DEL_NODE_EXCH_PATTERN_FROM_DB, err.Error()),
					persistence.EC_DATABASE_ERROR)
			}
			return
		}

		w.patternChange.Reset()
		glog.V(3).Infof(logString(fmt.Sprintf("New pattern %v verified.  Will cancel agreements and re-register the node with the new pattern.", new_pattern)))
		eventlog.LogNodeEvent(w.db,
			persistence.SEVERITY_INFO,
			persistence.NewMessageMeta(EL_GOV_NEW_PATTERN_VERIFIED, new_pattern),
			persistence.EC_NEW_PATTERN_VERIFIED,
			pDevice.Id, pDevice.Org, new_pattern, pDevice.Config.State)

		// unconfigure the node only if it is not already unconfiguring
		if pDevice.Config.State != persistence.CONFIGSTATE_UNCONFIGURING {
			// set the device config stat to unconfiguring
			_, err = pDevice.SetConfigstate(w.db, pDevice.Id, persistence.CONFIGSTATE_UNCONFIGURING)
			if err != nil {
				glog.Errorf(logString(fmt.Sprintf("error persisting unconfiguring on node object: %v", err)))
				eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_ERR_SAVE_NODE_CONFIGSTATE_TO_DB, persistence.CONFIGSTATE_UNCONFIGURING, err.Error()),
					persistence.EC_DATABASE_ERROR)
				return
			}
			// set the node shutdown message
			w.Messages() <- events.NewNodeShutdownMessage(events.START_UNCONFIGURE, false, false)
		}
	} else {
		// the device is up again and rereg the device with the new pattern
		if err := w.changeNodePattern(pDevice, new_pattern); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error while re-registering node with new pattern %v. %v", new_pattern, err)))
			eventlog.LogNodeEvent(w.db,
				persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_GOV_ERR_REG_NODE_WITH_NEW_PATTERN, new_pattern, err.Error()),
				persistence.EC_ERROR_REG_NODE_WITH_NEW_PATTERN,
				pDevice.Id, pDevice.Org, new_pattern, pDevice.Config.State)
		}
	}
}

// This function handles node pattern change. It will reregister the node with the new pattern.
func (w *GovernanceWorker) changeNodePattern(dev *persistence.ExchangeDevice, new_pattern string) error {

	glog.V(3).Infof(logString(fmt.Sprintf("start node re-registration after pattern changed to %v", new_pattern)))
	eventlog.LogNodeEvent(w.db,
		persistence.SEVERITY_INFO,
		persistence.NewMessageMeta(EL_GOV_START_REREG_NODE_PATTERN_CHANGE, new_pattern),
		persistence.EC_START_REREG_NODE_PATTERN_CHANGE,
		dev.Id, dev.Org, new_pattern, dev.Config.State)

	// make sure the exchange node pattern is not changed since last shutdown
	if exchNode, err := exchangesync.GetExchangeNode(); err != nil {
		return errors.New(logString(fmt.Sprintf("error getting node from the exchange. %v", err)))
	} else if exchNode.Pattern == "" {
		return errors.New(logString(fmt.Sprintf("the node was shutdown for pattern change, but no pattern is set for node %v/%v on the exchange now. %v", dev.Org, dev.Id, err)))
	} else if exchNode.Pattern != new_pattern {
		new_pattern = exchNode.Pattern

		glog.V(3).Infof(logString(fmt.Sprintf("node pattern changed again on the exchange. Will register the node with the new pattern %v", new_pattern)))
		eventlog.LogNodeEvent(w.db,
			persistence.SEVERITY_INFO,
			persistence.NewMessageMeta(EL_GOV_PATTERN_CHANGED_AGAIN, new_pattern),
			persistence.EC_NODE_PATTERN_CHANGED_AGAIN,
			dev.Id, dev.Org, new_pattern, dev.Config.State)
	}

	// now set the node with the new pattern
	if _, err := dev.SetPattern(w.db, dev.Id, new_pattern); err != nil {
		return errors.New(logString(fmt.Sprintf("unable to update the pattern to %v for horizon device, error: %v", new_pattern, err)))
	} else {
		dev.Pattern = new_pattern
	}

	// set the node config state to CONFIGSTATE_UNCONFIGURED so that the node can start re-register with the new pattern
	if _, err := dev.SetConfigstate(w.db, dev.Id, persistence.CONFIGSTATE_CONFIGURING); err != nil {
		return errors.New(logString(fmt.Sprintf("unable to update the config state to CONFIGSTATE_UNCONFIGURED for horizon device, error: %v", err)))
	} else {
		dev.Config.State = persistence.CONFIGSTATE_CONFIGURING
	}

	// send out a node registered message
	// w.Messages() <- events.NewEdgeRegisteredExchangeMessage(events.NEW_DEVICE_REG, dev.Id, dev.Token, dev.Org, new_pattern)

	//set node config state to
	patternHandler := exchange.GetHTTPExchangePatternHandler(w)
	serviceResolver := exchange.GetHTTPServiceDefResolverHandler(w)
	getService := exchange.GetHTTPServiceHandler(w)
	getDevice := exchange.GetHTTPDeviceHandler(w)
	patchDevice := exchange.GetHTTPPatchDeviceHandler(w)

	error_handler := func(err error) bool {
		glog.Errorf(logString(fmt.Sprintf("encountered error while re-registering node with new pattern %v. %v", new_pattern, err)))
		eventlog.LogNodeEvent(w.db,
			persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_GOV_ERR_REG_NODE_WITH_NEW_PATTERN, new_pattern, err.Error()),
			persistence.EC_ERROR_REG_NODE_WITH_NEW_PATTERN,
			dev.Id, dev.Org, new_pattern, dev.Config.State)
		return true
	}

	state := persistence.CONFIGSTATE_CONFIGURED
	configState := api.Configstate{State: &state}

	// Validate and update the config state.
	_, _, msgs := api.UpdateConfigstate(&configState, error_handler, patternHandler, serviceResolver, getService, getDevice, patchDevice, w.db, w.Config)

	// Send out all messages
	for _, msg := range msgs {
		w.Messages() <- msg
	}

	// remove the pattern change flag from the local database
	if err := persistence.DeleteNodeExchPattern(w.db); err != nil {
		return errors.New(logString(fmt.Sprintf("error deleting node exchange pattern from the local database. %v", err)))
	}

	eventlog.LogNodeEvent(w.db,
		persistence.SEVERITY_INFO,
		persistence.NewMessageMeta(EL_GOV_END_REREG_NODE_PATTERN_CHANGE, new_pattern),
		persistence.EC_END_REREG_NODE_PATTERN_CHANGE,
		dev.Id, dev.Org, new_pattern, persistence.CONFIGSTATE_CONFIGURED)

	// Send out the config complete message that enables the device for agreements
	w.Messages() <- events.NewEdgeConfigCompleteMessage(events.NEW_DEVICE_CONFIG_COMPLETE)

	glog.V(3).Infof(logString(fmt.Sprintf("Complete node re-registration after pattern changed to %v", new_pattern)))

	return nil
}

// This function makes sure that the pattern exits on the exchange and
// the local device has needed userinputs for services
func ValidateNewPattern(nodeType string, new_pattern string,
	getPatterns exchange.PatternHandler,
	serviceResolver exchange.ServiceResolverHandler,
	getService exchange.ServiceHandler,
	db *bolt.DB,
	config *config.HorizonConfig) error {

	glog.V(3).Infof(logString(fmt.Sprintf("Validating new pattern %v", new_pattern)))

	if new_pattern == "" {
		return fmt.Errorf("The input pattern must not be empty.")
	}

	// get node user input
	nodeUserInput, err := persistence.FindNodeUserInput(db)
	if err != nil {
		return fmt.Errorf("Failed get user input from local db. %v", err)
	}

	// parse the new pattern name
	pattern_org, pattern_name, pat := persistence.GetFormatedPatternString(new_pattern, "")

	pattern, err := getPatterns(pattern_org, pattern_name)
	if err != nil {
		return fmt.Errorf("Unable to read pattern object %v from exchange, error %v", pat, err)
	} else if len(pattern) != 1 {
		return fmt.Errorf("Expected only 1 pattern from exchange, received %v", len(pattern))
	}

	// Get the pattern definition that we need to analyze.
	patternDef, ok := pattern[pat]
	if !ok {
		return fmt.Errorf("Expected pattern id not found in GET pattern response: %v", pattern)
	}

	// merge node user input it with pattern user input
	mergedUserInput := policy.MergeUserInputArrays(patternDef.UserInput, nodeUserInput, true)
	if mergedUserInput == nil {
		mergedUserInput = []policy.UserInput{}
	}

	// For each top-level service in the pattern, resolve it to a list of required services.
	completeAPISpecList := new(policy.APISpecList)
	thisArch := cutil.ArchString()

	for _, service := range patternDef.Services {

		// Ignore top-level services that don't match this node's hardware architecture.
		if service.ServiceArch != thisArch && config.ArchSynonyms.GetCanonicalArch(service.ServiceArch) != thisArch {
			glog.Infof(logString(fmt.Sprintf("skipping service because it is for a different hardware architecture, this node is %v. Skipped service is: %v", thisArch, service.ServiceArch)))
			continue
		}

		// Each top-level service in the pattern can specify rollback versions, so to get a fully qualified top-level service URL,
		// we need to iterate each "workloadChoice" to grab the version.
		for _, serviceChoice := range service.ServiceVersions {

			apiSpecList, serviceDef, _, err := serviceResolver(service.ServiceURL, service.ServiceOrg, serviceChoice.Version, service.ServiceArch)
			if err != nil {
				return fmt.Errorf("Error resolving service %v/%v %v %v, error %v", service.ServiceOrg, service.ServiceURL, serviceChoice.Version, thisArch, err)
			}

			// ignore the services that do not match the node type
			serviceType := serviceDef.GetServiceType()
			if serviceType != exchangecommon.SERVICE_TYPE_BOTH && nodeType != serviceType {
				break
			}

			// check if we have needed user input for this service
			if err := ValidateUserInput(serviceDef, service.ServiceOrg, mergedUserInput, db); err != nil {
				return err
			}

			// Look for inconsistencies in the hardware architecture of the list of dependencies.
			if apiSpecList != nil {
				for _, apiSpec := range *apiSpecList {
					if apiSpec.Arch != thisArch && config.ArchSynonyms.GetCanonicalArch(apiSpec.Arch) != thisArch {
						return fmt.Errorf("The referenced service %v by service %v/%v has a hardware architecture that is not supported by this node: %v.", apiSpec, service.ServiceOrg, service.ServiceURL, thisArch)
					}
				}

				// MergeWith will omit exact duplicates when merging the 2 lists.
				(*completeAPISpecList) = completeAPISpecList.MergeWith(apiSpecList)
			}

		}

	}

	// The pattern search doesnt find any depencent services
	if len(*completeAPISpecList) != 0 {
		// for now, anax only allow one service version, so we need to get the common version range for each service.
		common_apispec_list, err := completeAPISpecList.GetCommonVersionRanges()
		if err != nil {
			return fmt.Errorf("Error resolving the common version ranges for the referenced services for %v %v. %v", pat, thisArch, err)
		}

		// Checking user input for dependent services
		for _, apiSpec := range *common_apispec_list {
			serviceDef, _, err := getService(apiSpec.SpecRef, apiSpec.Org, apiSpec.Version, apiSpec.Arch)
			if err != nil {
				return fmt.Errorf("Error resolving service %v/%v %v %v, error %v", apiSpec.Org, apiSpec.SpecRef, apiSpec.Version, apiSpec.Arch, err)
			}

			// check if we have needed user input for this service
			if err := ValidateUserInput(serviceDef, apiSpec.Org, mergedUserInput, db); err != nil {
				return err
			}
		}
	}

	glog.V(3).Infof(logString(fmt.Sprintf("Complete validating new pattern %v", new_pattern)))

	return nil
}

// This function validats if there is enough user input for the given service.
// The given mergedUserInput is the merged user input from node and pattern.
// The user input from attribute will be added to it before checking.
func ValidateUserInput(sdef *exchange.ServiceDefinition, serviceOrg string, mergedUserInput []policy.UserInput, db *bolt.DB) error {
	glog.V(5).Infof(logString(fmt.Sprintf("Start validating userinput for service %v/%v", serviceOrg, sdef.URL)))

	if !sdef.NeedsUserInput() {
		return nil
	}

	// get the merged user input for this service, it is from node userinput and pattern user input
	mergedSvcUI, _, err := policy.FindUserInput(sdef.URL, serviceOrg, "", sdef.Arch, mergedUserInput)
	if err != nil {
		return fmt.Errorf("Failed to find user input for service %v/%v from the merged user input, error: %v", serviceOrg, sdef.URL, err)
	}

	// get attributes related to this service
	var userInput *policy.UserInput
	attrs, err := persistence.FindApplicableAttributes(db, sdef.URL, serviceOrg)
	if err != nil {
		return fmt.Errorf("Unable to fetch service %v/%v attributes, error: %v", serviceOrg, sdef.URL, err)
	} else {
		for _, attr := range attrs {
			switch attr.(type) {
			case persistence.UserInputAttributes:
				uiAttr := attr.(persistence.UserInputAttributes)
				userInput = ConvertAttributeToUserInput(sdef.URL, serviceOrg, sdef.Arch, &uiAttr)
			}
		}
	}

	merged_ui := mergedSvcUI
	if userInput != nil {
		if mergedSvcUI == nil {
			merged_ui = userInput
		} else {
			// there should be only one for this service
			merged_ui, _ = policy.MergeUserInput(*mergedSvcUI, *userInput, false)
		}
	}

	if merged_ui == nil || merged_ui.Inputs == nil || len(merged_ui.Inputs) == 0 {
		for _, ui := range sdef.UserInputs {
			if ui.DefaultValue == "" {
				return fmt.Errorf("Userinput for %v is required for service %v/%v.", ui.Name, serviceOrg, sdef.URL)
			}
		}
	} else {
		// check if the user input has all the necessary values
		for _, ui := range sdef.UserInputs {
			if ui.DefaultValue != "" {
				continue
			} else if merged_ui.FindInput(ui.Name) == nil {
				return fmt.Errorf("Userinput for %v is required for service %v/%v.", ui.Name, serviceOrg, sdef.URL)
			}
		}
	}

	glog.V(5).Infof(logString(fmt.Sprintf("Complete validating userinput for service %v/%v", serviceOrg, sdef.URL)))
	return nil
}

// Convert the UserInputAttributes to policy.UserInput. It returns nil if the atts is nil or the mappings in the attr has no contents.
func ConvertAttributeToUserInput(serviceName string, serviceOrg string, serviceArch string, attr *persistence.UserInputAttributes) *policy.UserInput {
	if attr == nil {
		return nil
	}

	userInput := new(policy.UserInput)
	if attr.ServiceSpecs != nil && len(*attr.ServiceSpecs) > 0 {
		userInput.ServiceUrl = (*attr.ServiceSpecs)[0].Url
		userInput.ServiceOrgid = (*attr.ServiceSpecs)[0].Org
	} else {
		userInput.ServiceUrl = serviceName
		userInput.ServiceOrgid = serviceOrg
	}
	userInput.ServiceArch = serviceArch

	userInput.ServiceVersionRange = "[0.0.0,INFINITY)"

	if len(attr.Mappings) == 0 {
		return nil
	} else {
		ui := []policy.Input{}
		for k, v := range attr.Mappings {
			ui = append(ui, policy.Input{Name: k, Value: v})
		}
		userInput.Inputs = ui
	}
	return userInput
}
