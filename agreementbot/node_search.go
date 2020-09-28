package agreementbot

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/policy"
	"sync"
	"time"
)

// The node search object searches the exchange for nodes that can run services referred to in patterns and deployment
// policies. The exchange search API for patterns will return all nodes eligible for a given pattern. For deployment
// policy based searches, the exchange search API returns pages of the entire result set, because the result set can be
// quite large. This object maintains the search session information that tells the exchange how far the agbot's have
// progressed through the pages of the result set. The session state is persisted in the agbot's DB.
type NodeSearch struct {
	db                   persistence.AgbotDatabase
	pm                   *policy.PolicyManager
	ph                   *ConsumerPHMgr
	ec                   exchange.ExchangeContext
	msgs                 chan events.Message // Outgoing internal event messages are placed here.
	nextScanIntervalS    uint64              // The interval between scans when there are changes in the system. It allows the system to process existing work before injecting new agreements.
	fullRescanIntervalS  uint64              // The interval between scans when there are NOT changes in the system. This is a safety net in case changes are missed.
	lastSearchComplete   bool
	lastSearchTime       uint64
	searchThread         chan bool
	rescanLock           sync.Mutex // The lock that protects the rescanNeeded flag. The rescanNeeded flag can be checked/changed on different threads.
	rescanNeeded         bool       // A broad indicator that something policy or pattern related changed, and therefore the agbot needs to rescan all nodes.
	batchSize            uint64     // The max number of nodes that this object will process in a deployment policy search result.
	activeDeviceTimeoutS int        // The amount of time a device can go without heartbeating and still be considered active for the purposes of search.
	retryLookBack        uint64     // The amount of time to look backward for node changes when node retries are happening.
	policyOrder          bool       // When true, order policies most recently changed to least recently changed.
}

func NewNodeSearch() *NodeSearch {
	ns := &NodeSearch{
		nextScanIntervalS:   0,
		fullRescanIntervalS: 0,
		lastSearchComplete:  true,
		lastSearchTime:      0,
		searchThread:        make(chan bool, 10),
		rescanNeeded:        false,
	}
	return ns
}

// Give the object a chance to initialize itself.
func (n *NodeSearch) Init(db persistence.AgbotDatabase, pm *policy.PolicyManager, ph *ConsumerPHMgr, msgs chan events.Message, ec exchange.ExchangeContext, cfg *config.HorizonConfig) {

	n.db = db
	n.pm = pm
	n.ph = ph
	n.msgs = msgs
	n.ec = ec
	n.nextScanIntervalS = cfg.AgreementBot.NewContractIntervalS
	n.fullRescanIntervalS = cfg.GetAgbotFullRescan()
	n.batchSize = cfg.GetAgbotAgreementBatchSize()
	n.activeDeviceTimeoutS = cfg.AgreementBot.ActiveDeviceTimeoutS
	n.retryLookBack = cfg.GetAgbotRetryLookBackWindow()
	n.policyOrder = cfg.GetAgbotPolicyOrder()

	// Set the time of the worker restart to 1 minute ago. This time is used to indicate that the node searches need to go backward in time
	// because this agbot just restarted, and therefore could have lost search results that were in memory but the database was
	// already updated with a new changedSince.

	// Now move the changedSince for all ended sessions backward in time to the restart time. In flight sessions are also updated with
	// the new time which will get picked up the next time a new session is started.
	now := uint64(time.Now().Unix() - 60)
	err := n.db.ResetAllChangedSince(now)
	if err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("unable to reset changed since on restart, error: %v", err)))
	} else {
		glog.V(3).Infof(AWlogString(fmt.Sprintf("reset all search sessions changed since to: %v, %v", now, time.Unix(int64(now), 0).Format(cutil.ExchangeTimeFormat))))
	}

	if err := n.db.DumpSearchSessions(); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("unable to dump search session records, error: %v", err)))
	}

}

// Indicate that a rescan of all nodes is needed. This function is thread safe.
func (n *NodeSearch) SetRescanNeeded() {
	n.rescanLock.Lock()
	defer n.rescanLock.Unlock()
	n.rescanNeeded = true
}

// Indicate that a rescan of all nodes is no longer needed. This function is thread safe.
func (n *NodeSearch) UnsetRescanNeeded() {
	n.rescanLock.Lock()
	defer n.rescanLock.Unlock()
	n.rescanNeeded = false
}

// Check if a node rescan is needed. This function is thread safe.
func (n *NodeSearch) IsRescanNeeded() bool {
	n.rescanLock.Lock()
	defer n.rescanLock.Unlock()
	return n.rescanNeeded
}

// This is the main driving function in this object. It will initiate a node scan if needed, using an exiting search session or obtain a new one if needed.
// The actual processing of a node scan for all policies and patterns is actually performed on a sub-thread. This function also also handles updating
// itself if a previous scan has completed since the last time this method was called.
func (n *NodeSearch) Scan() {

	// If a previous scan has completed, remember it and log the completion.
	select {
	case n.lastSearchComplete = <-n.searchThread:
		glog.V(3).Infof(AWlogString("Done Polling Exchange"))
	default:
		if !n.lastSearchComplete {
			glog.V(5).Infof(AWlogString("waiting for search results."))
		}
	}

	// Now check to see if a new scan is needed. This function will periodically scan all nodes, to ensure that missed change events are eventually acted on.
	// If there is no rescan needed but it's been a while since the last full scan, then do a full scan anyway.
	// A full rescan uses its own changedSince time so that the full rescans overlap each other.
	if n.lastSearchComplete && !n.IsRescanNeeded() && (n.fullRescanIntervalS != 0 && (uint64(time.Now().Unix())-n.lastSearchTime) >= n.fullRescanIntervalS) {
		n.lastSearchTime = uint64(time.Now().Unix())
		glog.V(3).Infof(AWlogString("Polling Exchange (full rescan)"))
		n.lastSearchComplete = false
		go n.findAndMakeAgreements()
	}

	// If changes in the system have occurred such that a rescan is needed, start a scan now.
	if n.lastSearchComplete && n.IsRescanNeeded() && ((uint64(time.Now().Unix()) - n.lastSearchTime) >= uint64(n.nextScanIntervalS)) {
		n.lastSearchTime = uint64(time.Now().Unix())
		glog.V(3).Infof(AWlogString("Polling Exchange"))
		n.lastSearchComplete = false
		n.UnsetRescanNeeded()
		go n.findAndMakeAgreements()
	}

}

// Go through all the patterns and deployment polices and make agreements. This function runs on a sub-thread of the agbot
// main thread so that the main thread can continue handling inflight agreements and changes.
func (n *NodeSearch) findAndMakeAgreements() {

	if err := n.db.DumpSearchSessions(); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("unable to dump search session records, error: %v", err)))
	}

	// Errors encountered during the search will cause the next set of searches to be performed with the same changedSince
	// time and the same search session.
	searchError := false

	// Get a list of all the orgs this agbot is serving.
	allOrgs := n.pm.GetAllPolicyOrgs()
	for _, org := range allOrgs {

		// The policies in the policy manager are generated from patterns and deployment policies. Order the policies
		// by importance, with the most recently changed deployment policies first and the patterns at the end. This ordering
		// will help to ensure that the agbots are processing policies in the same order, thereby enabling pagination to
		// have its desired effect.
		availablePolicies := n.getOrderedPolicies(org)

		for _, consumerPolicy := range availablePolicies {

			// Search for nodes based on the current changedSince timestamp to pick up any newly changed nodes.
			if consumerPolicy.PatternId != "" {
				if _, err := n.searchNodesAndMakeAgreements(&consumerPolicy, org, "", 0); err != nil {
					// Dont move the changed since time forward since there was an error.
					searchError = true
					break
				}
			} else if pBE := businessPolManager.GetBusinessPolicyEntry(org, &consumerPolicy); pBE != nil {
				_, polName := cutil.SplitOrgSpecUrl(consumerPolicy.Header.Name)
				if lastPage, err := n.searchNodesAndMakeAgreements(&consumerPolicy, org, polName, pBE.Updated); err != nil {
					// Dont move the changed since time forward since there was an error.
					searchError = true
					break
				} else if !lastPage {
					// The search returned a large number of results that need to be processed. Let the system work on them
					// and then we'll come back and try again.
					n.SetRescanNeeded()
					break
				}
			}

		}
		if searchError {
			break
		}
	}

	// Done scanning all nodes across all policies, and no errors were encountered.
	if searchError {
		n.SetRescanNeeded()
	}

	// Dump search tables to the log.
	if err := n.db.DumpSearchSessions(); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("unable to dump search session records, error: %v", err)))
	}

	n.searchThread <- true

}

// Order the input policies for processing based on most recently changed processed first.
// The returned list of policies contains a mix of pattern based generated policy and deployment policy converted
// to this internal format.
func (n *NodeSearch) getOrderedPolicies(org string) []policy.Policy {

	// First, get the deployment policies ordered as configured.
	res := businessPolManager.GetAllPoliciesOrderedForOrg(org, n.policyOrder)

	// Second, append the pattern based policies to the end of the list.
	for _, oldPol := range n.pm.GetAllAvailablePolicies(org) {
		if oldPol.PatternId != "" {
			res = append(res, oldPol)
		}
	}

	return res
}

// Search the exchange and make agreements with any device that is eligible based on the policies we have and
// agreement protocols that we support. If the search did not process all the possible node matches, return false
// to indicate that there are more nodes to be processed.
func (n *NodeSearch) searchNodesAndMakeAgreements(consumerPolicy *policy.Policy, org string, polName string, polLastUpdateTime uint64) (bool, error) {

	endOfResults := true

	if devices, err := n.searchExchange(consumerPolicy, org, polName, polLastUpdateTime); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("received error searching for %v, error: %v", consumerPolicy, err)))
		return endOfResults, err

	} else {

		// Remember whether or not this search returned all the possible nodes.
		if uint64(len(*devices)) == n.batchSize {
			endOfResults = false
		}

		// Get all the agreements for this policy that are still active.
		pendingAgreementFilter := func() persistence.AFilter {
			return func(a persistence.Agreement) bool {
				return a.PolicyName == consumerPolicy.Header.Name && a.AgreementTimedout == 0
			}
		}

		ags := make(map[string][]persistence.Agreement)

		// The agreements with this policy could be part of any supported agreement protocol.
		for _, agp := range policy.AllAgreementProtocols() {
			// Find all agreements that are in progress. They might be waiting for a reply or not yet finalized.
			// TODO: To support more than 1 agreement (maxagreements > 1) with this device for this policy, we need to adjust this logic.
			if agreements, err := n.db.FindAgreements([]persistence.AFilter{persistence.UnarchivedAFilter(), pendingAgreementFilter()}, agp); err != nil {
				glog.Errorf(AWlogString(fmt.Sprintf("received error trying to find pending agreements for protocol %v: %v", agp, err)))
			} else {
				ags[agp] = agreements
			}
		}

		for _, dev := range *devices {

			glog.V(3).Infof(AWlogString(fmt.Sprintf("picked up %v for policy %v.", dev.ShortString(), consumerPolicy.Header.Name)))
			glog.V(5).Infof(AWlogString(fmt.Sprintf("picked up %v", dev)))

			// Check for agreements already in progress with this device
			if found := n.alreadyMakingAgreementWith(&dev, consumerPolicy, ags); found {
				glog.V(5).Infof(AWlogString(fmt.Sprintf("skipping device id %v, agreement attempt already in progress with %v", dev.Id, consumerPolicy.Header.Name)))
				continue
			}

			// If the device is not ready to make agreements yet, then skip it.
			if dev.PublicKey == "" {
				glog.V(5).Infof(AWlogString(fmt.Sprintf("skipping device id %v, node is not ready to exchange messages", dev.Id)))
				continue
			}

			producerPolicy := policy.Policy_Factory(consumerPolicy.Header.Name)

			// Get the cached service policies from the business policy manager. The returned value
			// is a map keyed by the service id.
			// There could be many service versions defined in a business policy.
			// The policy manager only caches the ones that are used by an old agreement for this business policy.
			// The cached ones may not be what the new agreement will use. If the new agreement chooses a
			// new service version, then the new service policy will be put into the cache.
			svcPolicies := make(map[string]externalpolicy.ExternalPolicy, 0)
			if consumerPolicy.PatternId == "" {
				svcPolicies = businessPolManager.GetServicePoliciesForPolicy(org, polName)
			}

			// Select a worker pool based on the agreement protocol that will be used. This is decided by the
			// consumer policy.
			protocol := policy.Select_Protocol(producerPolicy, consumerPolicy)
			cmd := NewMakeAgreementCommand(*producerPolicy, *consumerPolicy, org, polName, dev, svcPolicies)

			bcType, bcName, bcOrg := producerPolicy.RequiresKnownBC(protocol)

			if !n.ph.Has(protocol) {
				glog.Errorf(AWlogString(fmt.Sprintf("unable to find protocol handler for %v.", protocol)))
			} else if bcType != "" && !n.ph.Get(protocol).IsBlockchainWritable(bcType, bcName, bcOrg) {
				// Get that blockchain running if it isn't up.
				glog.V(5).Infof(AWlogString(fmt.Sprintf("skipping device id %v, requires blockchain %v %v %v that isnt ready yet.", dev.Id, bcType, bcName, bcOrg)))
				n.msgs <- events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, bcType, bcName, bcOrg, n.ec.GetExchangeURL(), n.ec.GetExchangeId(), n.ec.GetExchangeToken())
				continue
			} else if !n.ph.Get(protocol).AcceptCommand(cmd) {
				glog.Errorf(AWlogString(fmt.Sprintf("protocol handler for %v not accepting new agreement commands.", protocol)))
			} else {
				n.ph.Get(protocol).HandleMakeAgreement(cmd, n.ph.Get(protocol))
				glog.V(5).Infof(AWlogString(fmt.Sprintf("queued agreement attempt for policy %v and node %v using protocol %v", consumerPolicy.Header.Name, dev.Id, protocol)))
			}
		}

	}

	return endOfResults, nil

}

// Check all agreement protocol buckets to see if there are any agreements with this device.
// Return true if there is already an agreement for this node and policy.
func (n *NodeSearch) alreadyMakingAgreementWith(dev *exchange.SearchResultDevice, consumerPolicy *policy.Policy, allAgreements map[string][]persistence.Agreement) bool {

	// Check to see if we're already doing something with this device.
	for _, ags := range allAgreements {
		// Look for any agreements with the current node.
		for _, ag := range ags {
			if ag.DeviceId == dev.Id {
				if ag.AgreementFinalizedTime != 0 {
					glog.V(5).Infof(AWlogString(fmt.Sprintf("sending agreement verify for %v", ag.CurrentAgreementId)))
					n.ph.Get(ag.AgreementProtocol).VerifyAgreement(&ag, n.ph.Get(ag.AgreementProtocol))
					n.AddRetry(consumerPolicy.Header.Name, ag.AgreementFinalizedTime-n.retryLookBack)
				}
				return true
			}
		}
	}
	return false

}

// Search the exchange for devices to make agreements with. The system should be operating such that devices are
// not returned from the exchange (for any given set of search criteria) once an agreement which includes those
// criteria has been reached. This prevents the agbot from continually sending proposals to devices that are
// already in an agreement.
//
// There are 2 ways to search the exchange; (a) by pattern and service or workload URL, or (b) by business policy.
// If the agbot is working with a policy file that was generated from a pattern, then it will do searches
// by pattern. If the agbot is working with a business policy, then it will do searches by the business policy.
func (n *NodeSearch) searchExchange(pol *policy.Policy, polOrg string, polName string, polLastUpdateTime uint64) (*[]exchange.SearchResultDevice, error) {

	// If it is a pattern based policy, search by workload URL and pattern.
	if pol.PatternId != "" {
		// Get a list of node orgs that the agbot is serving for this pattern.
		nodeOrgs := patternManager.GetServedNodeOrgs(polOrg, exchange.GetId(pol.PatternId))
		if len(nodeOrgs) == 0 {
			glog.V(3).Infof(AWlogString(fmt.Sprintf("Policy file for pattern %v exists but currently the agbot is not serving this policy for any organizations.", pol.PatternId)))
			empty := make([]exchange.SearchResultDevice, 0, 0)
			return &empty, nil
		}

		// Setup the search request body
		ser := exchange.CreateSearchPatternRequest()
		ser.SecondsStale = n.activeDeviceTimeoutS
		ser.NodeOrgIds = nodeOrgs
		ser.ServiceURL = cutil.FormOrgSpecUrl(pol.Workloads[0].WorkloadURL, pol.Workloads[0].Org)

		glog.V(3).Infof(AWlogString(fmt.Sprintf("searching %v with %v", pol.PatternId, ser)))

		// Invoke the exchange
		devs, err := exchange.GetHTTPAgbotPatternNodeSearchHandler(n.ec)(ser, polOrg, pol.PatternId)
		if err == nil {
			glog.V(3).Infof(AWlogString(fmt.Sprintf("found %v devices in exchange.", len(*devs))))
		}
		return devs, err

	} else {

		// Current timestamp to be saved as the next agreement making cycle start time. This time is used to ensure that no changes are
		// missed. This will cause the next search to look for changes nodes that overlap in time with the search that is about to be
		// initiated. That's one way to ensure that changes aren't missed.
		currentSearchStart := uint64(time.Now().Unix()) - 1

		// Get the current changedSince time from the DB. The changedSince time is coordinated across all agbot instances.
		// It indicates to the exchange that it should only return nodes that have changed since the given time. This time
		// could be updated in the DB immediately after this point, which will result in the current searches using a
		// changedSince value that has already been used. While this will result in extra work for the agbot, it should
		// not cause errors in the system as a whole.

		// Begin or continue a node search session. The exchange will return nodes in pages, i.e. a subset of all possible results to be
		// processed by this Agbot. The Exchange uses the search session number as a key to know how much of the total result
		// set has already been returned. This allows the exchange to return alternating pages of the result set to different
		// Agbot instances.
		searchSession, changedSince, err := n.db.ObtainSearchSession(pol.Header.Name)
		if err != nil {
			glog.Errorf(AWlogString(fmt.Sprintf("unable to start a new search session for %v, error: %v", pol.Header.Name, err)))
			return nil, err
		}

		// Get a list of node orgs that the agbot is serving for this business policy.
		nodeOrgs := businessPolManager.GetServedNodeOrgs(polOrg, polName)
		if len(nodeOrgs) == 0 {
			glog.V(3).Infof(AWlogString(fmt.Sprintf("Business policy %v exists but currently the agbot is not serving this policy for any organizations.", pol.Header.Name)))
			empty := make([]exchange.SearchResultDevice, 0, 0)
			return &empty, nil
		}

		// To make the search more efficient, the exchange only searches the nodes that have been changed since bp_check_time.
		// If the business policy has changed since the last search cycle, then set changedSince to zero so that all nodes
		// will be checked again. One or more of them might have become compatible when the policy changed.
		bp_check_time := changedSince
		if polLastUpdateTime > changedSince {
			bp_check_time = 0
		}

		// Setup the search request body
		ser := exchange.SearchExchBusinessPolRequest{
			NodeOrgIds:   nodeOrgs,
			ChangedSince: bp_check_time,
			Session:      searchSession,
			NumEntries:   n.batchSize,
		}

		// The search for nodes exploits pagination on the exchange, which means that the search API returns a "page" of results
		// on each call, not the entire result set. To manage the page state, the agbot uses a coordinated session token
		// to indicate that it wants the next page of results for a given session. If that session gets out of sync between
		// the agbot and the exchange, the safest way to recover is for the agbot to use the session that the exchange
		// is using and retry the search. This will happen until the exchange returns the last page of results, at which
		// point the agbot can resume using the session that it wants to use. It is important to understand that the agbot
		// keeps a single session token for all searches using a given policy and so does the exchange, so in theory
		// it is possible that the exchange could have different sessions for different policies, but that should never get out
		// of sync with the agbot. The error handling in this loop is intended to compensate if the agbot session ever gets out
		// of sync with the exchange session.
		devs := make([]exchange.SearchResultDevice, 0, 0)
		for {

			glog.V(3).Infof(AWlogString(fmt.Sprintf("searching %v with %v", pol.Header.Name, ser)))

			// Invoke the exchange and return the device list or any hard errors that occur.
			resp, err := exchange.GetHTTPAgbotPolicyNodeSearchHandler(n.ec)(&ser, polOrg, polName)
			if err != nil {
				return nil, err
			} else if resp.Session != "" {
				glog.Errorf(AWlogString(fmt.Sprintf("for %v search session is out of sync: %v", pol.Header.Name, resp)))
				// To get the agbot back in sync, we will need to use the exchange session until it is exhausted.
				ser.Session = resp.Session
				searchSession = resp.Session
				n.SetRescanNeeded()
				continue
			} else {
				// The call was successful, update the DB if we got the last page.  If the exchange returns the number of nodes
				// requested, then assume there are more nodes in the search result set that weren't returned.
				if uint64(len(resp.Devices)) != n.batchSize {
					// Update the DB with the new changedSince value, indicating that the scan is complete. This update also
					// ends the current search session for this policy.
					glog.V(3).Infof(AWlogString(fmt.Sprintf("for %v ending Session: %v", pol.Header.Name, searchSession)))
					if sessionEnded, err := n.db.UpdateSearchSessionChangedSince(changedSince, currentSearchStart, pol.Header.Name); err != nil {
						glog.Errorf(AWlogString(fmt.Sprintf("unable to update search session changed since, error: %v", err)))
					} else {
						if sessionEnded {
							glog.V(3).Infof(AWlogString(fmt.Sprintf("for %v search Session: %v was already ended.", pol.Header.Name, searchSession)))
						}
					}
				} else {
					// There are more nodes to process, log it.
					glog.V(3).Infof(AWlogString(fmt.Sprintf("for %v Session: %v scan not complete", pol.Header.Name, searchSession)))
					n.SetRescanNeeded()
				}
				devs = resp.Devices
			}
			glog.V(3).Infof(AWlogString(fmt.Sprintf("found %v devices in exchange.", len(devs))))
			return &devs, nil
		}
	}
}

func (n *NodeSearch) AddRetry(policyName string, changedSince uint64) {
	n.SetRescanNeeded()
	if err := n.db.ResetPolicyChangedSince(policyName, changedSince); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("unable to update %v search session changed since, error: %v", policyName, err)))
	}
}
