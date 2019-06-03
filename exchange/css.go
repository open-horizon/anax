package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/edge-sync-service/common"
	"path"
	"time"
	)

// These structs are mirrors of similar structs in the edge-sync-service library. They are mirrored here
// so that we can use our types when demarhsalling them, which enables us to perform compatibility checks
// using these policies.

type DestinationPolicy struct {
	// Properties is the set of properties for a particular policy
	Properties externalpolicy.PropertyList `json:"properties" bson:"properties"`

	// Constraints is a set of expressions that form the constraints for the policy
	Constraints externalpolicy.ConstraintExpression `json:"constraints" bson:"constraints"`

	// Services is the list of services this object has affinity for
	Services []common.ServiceID `json:"services" bson:"services"`

	// Timestamp indicates when the policy was last updated (result of time.Now().UnixNano())
	Timestamp int64 `json:"timestamp" bson:"timestamp"`
}

type ObjectDestinationPolicy struct {
	// OrgID is the organization ID of the object (an object belongs to exactly one organization).
	//   required: true
	OrgID string `json:"orgID"`

	// ObjectType is the type of the object.
	// The type is used to group multiple objects, for example when checking for object updates.
	//   required: true
	ObjectType string `json:"objectType"`

	// ObjectID is a unique identifier of the object
	//   required: true
	ObjectID string `json:"objectID"`

	// DestinationPolicy is the policy specification that should be used to distribute this object
	// to the appropriate set of destinations.
	DestinationPolicy DestinationPolicy `json:"destinationPolicy"`

	//Destinations is the list of the object's current destinations
	Destinations []common.DestinationsStatus `json:"destinations"`
}

type ObjectDestinationPolicies []ObjectDestinationPolicy

type PutDestinationListRequest []string

// Query the CSS to retrieve object policy for a given service id.
func GetObjectsByService(ec ExchangeContext, org string, serviceId string) (*ObjectDestinationPolicies, error) {

	var resp interface{}
	resp = new(ObjectDestinationPolicies)

	url := path.Join("/api/v1/objects", org)
	url = ec.GetCSSURL() + url + fmt.Sprintf("?destination_policy=true&service=%v", serviceId)
		
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", url, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			objPolicies := resp.(*ObjectDestinationPolicies)
			glog.V(5).Infof(rpclogString(fmt.Sprintf("found object policies for objects in %v, with service %v, %v", org, serviceId, objPolicies)))
			return objPolicies, nil
		}
	}
}

// Query the CSS to retrieve object policy that hasn't been seen before.
func GetUpdatedObjects(ec ExchangeContext, org string, firstTime bool) (*ObjectDestinationPolicies, error) {

	var resp interface{}
	resp = new(ObjectDestinationPolicies)

	url := path.Join("/api/v1/objects", org)
	url = ec.GetCSSURL() + url + "?destination_policy=true"

	if firstTime {
		url = url + "&received=true"
	}
		
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", url, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			objPolicies := resp.(*ObjectDestinationPolicies)
			glog.V(5).Infof(rpclogString(fmt.Sprintf("found object policies for org %v", org)))
			return objPolicies, nil
		}
	}
}

// Update the destination list of the object when that object's policy enables it to be placed on the node.
func UpdateObjectDestinationList(ec ExchangeContext, org string, objPol *ObjectDestinationPolicy, dests *PutDestinationListRequest) error {

	// There is no response to CSS API.
	var resp interface{}

	url := path.Join("/api/v1/objects", org, objPol.ObjectType, objPol.ObjectID, "destinations")
	url = ec.GetCSSURL() + url

	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "PUT", url, ec.GetExchangeId(), ec.GetExchangeToken(), dests, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(rpclogString(fmt.Sprintf("updated destination list for object %v of type %v with %v", objPol.ObjectID, objPol.ObjectType, dests)))
			return nil
		}
	}

}

// Tell the MMS that a policy update has been received.
func SetPolicyReceived(ec ExchangeContext, objPol *ObjectDestinationPolicy) error {
	// There is no response to CSS API.
	var resp interface{}

	url := path.Join("/api/v1/objects", objPol.OrgID, objPol.ObjectType, objPol.ObjectID, "policyreceived")
	url = ec.GetCSSURL() + url

	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "PUT", url, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(rpclogString(fmt.Sprintf("set policy received for object %v %v of type %v", objPol.OrgID, objPol.ObjectID, objPol.ObjectType)))
			return nil
		}
	}
}