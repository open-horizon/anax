package agreementbot

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"strings"
)

func AssignObjectToNode(ec exchange.ExchangeContext, objPolicies *exchange.ObjectDestinationPolicies, nodeId string, nodePolicy *policy.Policy) error {

	if len(*objPolicies) == 0 {
		return nil
	}

	updateDestHandler := exchange.GetHTTPUpdateObjectDestinationHandler(ec)

	// For each object policy received, evaluate it against the node policy.
	for _, objPol := range (*objPolicies) {

		glog.V(5).Infof(opLogstring(fmt.Sprintf("evaluating policy for object %v of type %v", objPol.ObjectID, objPol.ObjectType)))

		// Evaluate the object policy against the edge node policy. If the object policy is compatible, then place the object
		// on the node for the current agreement.

		// Convert the object's policy into an internal policy so that we can do the compatibility check.
		internalObjPol := policy.Policy_Factory(fmt.Sprintf("object policy for %v type %v", objPol.ObjectID, objPol.ObjectType))
		internalObjPol.Properties = objPol.DestinationPolicy.Properties
		internalObjPol.Constraints = objPol.DestinationPolicy.Constraints
		glog.V(5).Infof(opLogstring(fmt.Sprintf("converted object policy to: %v", internalObjPol)))

		// Check if node and model polices are compatible. Incompatible policies are not necessarily an error so just log a warning and return.
		if err := policy.Are_Compatible(nodePolicy, internalObjPol); err != nil {
			glog.Warningf(opLogstring(fmt.Sprintf("error matching node policy %v and object policy %v, error: %v", nodePolicy, internalObjPol, err)))
			return nil
		} else {
			glog.V(5).Infof(opLogstring(fmt.Sprintf("node %v is compatible with object %v with type %v", nodeId, objPol.ObjectID, objPol.ObjectType)))
		}

		// Policies are compatible so place this object on the node. If the node we just made an agreement with is not in
		// the destination list of the object, add it.
		pdlr := new(exchange.PutDestinationListRequest)
		found := false
		for _, destStatus := range objPol.Destinations {
			if destStatus.DestID == exchange.GetId(nodeId) {
				found = true
				break
			} else {
				// The destination list update is a full replace so we have to capture all the current destinations as
				// we iterate the current list.
				(*pdlr) = append((*pdlr), destStatus.DestType + ":" + destStatus.DestID)
			}
		}

		if !found {
			(*pdlr) = append((*pdlr), "openhorizon.edgenode:" + exchange.GetId(nodeId))

			if err := updateDestHandler(objPol.OrgID, &objPol, pdlr); err != nil {
				glog.Errorf(opLogstring(fmt.Sprintf("%v", err)))
			} else {
				glog.V(3).Infof(opLogstring(fmt.Sprintf("updated destination list for object %v of type %v with node %v", objPol.ObjectID, objPol.ObjectType, nodeId)))
			}
		} else {
			glog.V(5).Infof(opLogstring(fmt.Sprintf("node %v is already a destination for object %v with type %v", nodeId, objPol.ObjectID, objPol.ObjectType)))
		}
	}
	return nil
}

func UnassignObjectFromNode(ec exchange.ExchangeContext, objPol *exchange.ObjectDestinationPolicy, nodeId string) error {

	updateDestHandler := exchange.GetHTTPUpdateObjectDestinationHandler(ec)
	pdlr := new(exchange.PutDestinationListRequest)
	found := false
	for _, destStatus := range objPol.Destinations {
		if destStatus.DestID == exchange.GetId(nodeId) {
			found = true
		} else {
			// The destination list update is a full replace so we have to capture all the current destinations as
			// we iterate the current list.
			(*pdlr) = append((*pdlr), destStatus.DestType + ":" + destStatus.DestID)
		}
	}

	if found {
		if err := updateDestHandler(objPol.OrgID, objPol, pdlr); err != nil {
			glog.Errorf(opLogstring(fmt.Sprintf("%v", err)))
		} else {
			glog.V(3).Infof(opLogstring(fmt.Sprintf("updated destination list for object %v of type %v to remove node %v", objPol.ObjectID, objPol.ObjectType, nodeId)))
		}
	}
	return nil
}

// MMS object policy changes can cause a significant impact to where objects are placed through the entire system.
// When an MMS object policy changes, it might mean one of the following:
// 1. Nothing changes.
//   a. A brand new policy is not eligible for any node on which there is already an agreement.
//   b. A policy change is still not sufficent to make the object eligible for nodes that are already in an agreement.
// 2. There are nodes on which the object should be removed.
// 3. There are nodes on which the object should be placed, where there is already an agreement.
//   a. A new object/policy is placed on the node long after the agreement is in place.
//   b. A policy change makes the object eligible for the node long after the agreement is in place.
//
// Objects are not placed on nodes without an agreement, so we can find all the relevant nodes by looking through
// all of our agreements. The actions we can take are to either remove a node from the destination list of a
// policy or add it to the object's destination list.
//
func (w *AgreementBotWorker) HandleMMSObjectPolicy(cmd *MMSObjectPolicyEventCommand) {

	glog.V(5).Infof(opLogstring(fmt.Sprintf("received MMS Object Policy event command: %v", cmd)))

	// Convert the object policies in the message to their real types.
	var oldPolicy exchange.ObjectDestinationPolicy
	var newPolicy exchange.ObjectDestinationPolicy
	var ok bool

	if cmd.Msg.OldPolicy != nil {
		if oldPolicy, ok = cmd.Msg.OldPolicy.(exchange.ObjectDestinationPolicy); !ok {
			glog.Errorf(opLogstring(fmt.Sprintf("MMS object policy event contains incorrect old policy type (%T)", cmd.Msg.OldPolicy)))
		}
	}

	if cmd.Msg.NewPolicy == nil {
		glog.Errorf(opLogstring(fmt.Sprintf("MMS object policy event missing new policy")))
	} else if newPolicy, ok = cmd.Msg.NewPolicy.(exchange.ObjectDestinationPolicy); !ok {
		glog.Errorf(opLogstring(fmt.Sprintf("MMS object policy event contains incorrect new policy type (%T)", cmd.Msg.NewPolicy)))
	}

	glog.V(5).Infof(opLogstring(fmt.Sprintf("OldPolicy: %v", oldPolicy)))
	glog.V(5).Infof(opLogstring(fmt.Sprintf("NewPolicy: %v", newPolicy)))

	// Construct a list of service ids in the new policy.
	newPolicyServiceKeys := make([]string, 0 ,5)
	for _, serviceID := range newPolicy.DestinationPolicy.Services {
		newPolicyServiceKeys = append(newPolicyServiceKeys, cutil.FormOrgSpecUrl(serviceID.ServiceName, serviceID.OrgID))
	}

	glog.V(5).Infof(opLogstring(fmt.Sprintf("NewPolicy service keys: %v", newPolicyServiceKeys)))

	// Construct a list of destinations from the old policy.
	oldPolicyDestNodes := make([]string, 0 ,5)
	for _, dest := range oldPolicy.Destinations {
		oldPolicyDestNodes = append(oldPolicyDestNodes, dest.DestID)
	}

	glog.V(5).Infof(opLogstring(fmt.Sprintf("OldPolicy dest nodes: %v", oldPolicyDestNodes)))

	inProgress := func() persistence.AFilter {
		return func(e persistence.Agreement) bool { return e.AgreementCreationTime != 0 && e.AgreementTimedout == 0 }
	}

	notPattern := func() persistence.AFilter {
		return func(e persistence.Agreement) bool { return e.Pattern == "" }
	}

	// Iterate through all agreements, across all protocols.
	for _, agp := range policy.AllAgreementProtocols() {

		// Find all agreements that are in progress.
		agreements, err := w.db.FindAgreements([]persistence.AFilter{inProgress(), notPattern(), persistence.UnarchivedAFilter()}, agp)
		if err != nil {
			glog.Errorf(opLogstring(fmt.Sprintf("unable to read agreements, error %v", err)))
		}

		for _, agreement := range agreements {
			// If an existing policy has changed, and the agreement's node is a destination in the old policy, then check to see if the node
			// is still compatible with the new policy. If not, remove the node from the object's destination list.
			if cmd.Msg.OldPolicy != nil &&  cutil.SliceContains(oldPolicyDestNodes, agreement.DeviceId) {

				// Convert the object's policy into an internal policy so that we can do the compatibility check.
				internalObjPol := policy.Policy_Factory(fmt.Sprintf("object policy for %v type %v", newPolicy.ObjectID, newPolicy.ObjectType))
				internalObjPol.Properties = newPolicy.DestinationPolicy.Properties
				internalObjPol.Constraints = newPolicy.DestinationPolicy.Constraints
				glog.V(5).Infof(opLogstring(fmt.Sprintf("converted new object policy to: %v", internalObjPol)))

				nodePolicy,err := w.GetNodePolicy(agreement.DeviceId)
				if err != nil {
					glog.Errorf(opLogstring(fmt.Sprintf("%v", err)))
				} else if err := policy.Are_Compatible(nodePolicy, internalObjPol); err != nil {
					// This agreement's node is no longer compatible, remove it from the destination list of the object.
					if err := UnassignObjectFromNode(w, &newPolicy, agreement.DeviceId); err != nil {
						glog.Errorf(opLogstring(fmt.Sprintf("%v", err)))
					}
				} else {
					glog.V(5).Infof(opLogstring(fmt.Sprintf("node %v is still compatible with object %v with type %v", agreement.DeviceId, newPolicy.ObjectID, newPolicy.ObjectType)))
				}
			} else {

			// 1. Brand new policy - node might be eligble now
			// 2. Updated policy - node might be eligible now


			// If the agreement has a service id in the list of services for the newpolicy AND the agreement's node is not in the dest list of the old policy,
			// then check to see if the new policy is compatible with node policy. If so, add the object to the node.
				objPolicies := new(exchange.ObjectDestinationPolicies)
				(*objPolicies) = append((*objPolicies), newPolicy)
				for _, serviceId := range agreement.ServiceId {

					serviceNamePieces := strings.SplitN(serviceId, "_", 2)
					if cutil.SliceContains(newPolicyServiceKeys, serviceNamePieces[0]) && !cutil.SliceContains(oldPolicyDestNodes, agreement.DeviceId) {
						// Add the node to the object destination if eligible.
						nodePolicy,err := w.GetNodePolicy(agreement.DeviceId)
						if err != nil {
							glog.Errorf(opLogstring(fmt.Sprintf("%v", err)))
						} else if err := AssignObjectToNode(w, objPolicies, agreement.DeviceId, nodePolicy); err != nil {
							glog.Errorf(opLogstring(fmt.Sprintf("%v", err)))
						}
						break
					}
				}
			}

		}
	}

	return

}

// Get node policy
func (w *AgreementBotWorker) GetNodePolicy(deviceId string) (*policy.Policy, error) {

	nodePolicyHandler := exchange.GetHTTPNodePolicyHandler(w)
	nodePolicy, err := nodePolicyHandler(deviceId)
	if err != nil {
		return nil, fmt.Errorf("error trying to query node policy for %v: %v", deviceId, err)
	}
	if nodePolicy == nil {
		return nil, fmt.Errorf("no node policy found for %v", deviceId)
	}

	extPolicy := nodePolicy.GetExternalPolicy()

	pPolicy, err := policy.GenPolicyFromExternalPolicy(&extPolicy, policy.MakeExternalPolicyHeaderName(deviceId))
	if err != nil {
		return nil, fmt.Errorf("failed to convert node policy to internal policy format for node %v: %v", deviceId, err)
	}
	return pPolicy, nil
}

// =============================================================================================================
var opLogstring = func(v interface{}) string {
	return fmt.Sprintf("Object Policy: %v", v)
}