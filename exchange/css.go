package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/edge-sync-service/common"
	"path"
	"strconv"
	"time"
)

// These structs are mirrors of similar structs in the edge-sync-service library. They are mirrored here
// so that we can use our types when demarhsalling them, which enables us to perform compatibility checks
// using these policies.

const DestinationsSizeLogLimit = 50

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

func (d DestinationPolicy) String() string {
	return fmt.Sprintf("Destination Policy: Props %v, Constraints %v, Services %v, timestamp %v", d.Properties, d.Constraints, d.Services, d.Timestamp)
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
	DestinationPolicy DestinationPolicy `json:"destinationPolicy,omitempty"`

	//Destinations is the list of the object's current destinations
	Destinations []common.DestinationsStatus `json:"destinations"`
}

func (d ObjectDestinationPolicy) String() string {
	if len(d.Destinations) < DestinationsSizeLogLimit {
		return fmt.Sprintf("Object Destination Policy: Org %v, Type %v, ID %v, %v, Destinations %v", d.OrgID, d.ObjectType, d.ObjectID, d.DestinationPolicy, d.Destinations)
	} else {
		return fmt.Sprintf("Object Destination Policy: Org %v, Type %v, ID %v, %v, Destinations size %v", d.OrgID, d.ObjectType, d.ObjectID, d.DestinationPolicy, len(d.Destinations))
	}
}

type PostDestsRequest struct {
	// Action is "add" or "remove"
	Action string `json:"action"`

	// Destinations is an array of destinations, each entry is an string in form of "<destinationType>:<destinationID>"
	Destinations []string `json:"destinations"`
}

type ObjectDestinationPolicies []ObjectDestinationPolicy

type ObjectDestinationStatuses []common.DestinationsStatus

type ObjectDestinationsToAdd []string

type ObjectDestinationsToDelete []string

// Query the CSS to retrieve object policy for a given service id.
func GetObjectsByService(ec ExchangeContext, org string, serviceId string) (*ObjectDestinationPolicies, error) {

	var resp interface{}
	resp = new(ObjectDestinationPolicies)

	url := path.Join("/api/v1/objects", org)
	url = ec.GetCSSURL() + url + fmt.Sprintf("?destination_policy=true&service=%v", serviceId)

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", url, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			objPolicies := resp.(*ObjectDestinationPolicies)
			if glog.V(5) {
				glog.Infof(rpclogString(fmt.Sprintf("found object policies for objects in %v, with service %v, %v", org, serviceId, objPolicies)))
			}
			return objPolicies, nil
		}
	}
}

// Query the CSS to retrieve object policy updates that haven't been seen before.
func GetUpdatedObjects(ec ExchangeContext, org string, since int64) (*ObjectDestinationPolicies, error) {

	var resp interface{}
	resp = new(ObjectDestinationPolicies)

	url := path.Join("/api/v1/objects", org)
	url = ec.GetCSSURL() + url + "?destination_policy=true"

	if since == 0 {
		url = url + "&received=true"
	} else {
		url = url + "&since=" + strconv.FormatInt(since, 10)
	}

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", url, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			objPolicies := resp.(*ObjectDestinationPolicies)
			if glog.V(5) {
				glog.Infof(rpclogString(fmt.Sprintf("found object policies for org %v, objpolicies %v", org, objPolicies)))
			}
			return objPolicies, nil
		}
	}
}

// Add or Remove the destinations of the object when that object's policy enables it to be placed on the node.
func AddOrRemoveDestinations(ec ExchangeContext, org string, objType string, objID string, postDestsRequest *PostDestsRequest) error {
	// There is no response to CSS API.
	var resp interface{}

	url := path.Join("/api/v1/objects", org, objType, objID, "destinations")
	url = ec.GetCSSURL() + url

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()

	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "POST", url, ec.GetExchangeId(), ec.GetExchangeToken(), postDestsRequest, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			if glog.V(5) {
				if len(postDestsRequest.Destinations) > DestinationsSizeLogLimit {
					glog.Infof(rpclogString(fmt.Sprintf("%s destinations for object %v of type %v with length of destinations %v", postDestsRequest.Action, objID, objType, len(postDestsRequest.Destinations))))
				} else {
					glog.Infof(rpclogString(fmt.Sprintf("%s destinations for object %v of type %v with %v", postDestsRequest.Action, objID, objType, postDestsRequest.Destinations)))
				}
			}
			return nil
		}
	}
}

// Get the object's metadata.
func GetObject(ec ExchangeContext, org string, objID string, objType string) (*common.MetaData, error) {

	var resp interface{}
	resp = new(common.MetaData)

	url := path.Join("/api/v1/objects", org, objType, objID)
	url = ec.GetCSSURL() + url

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", url, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			objMeta := resp.(*common.MetaData)
			if objMeta.ObjectID != "" {
				glog.V(5).Infof(rpclogString(fmt.Sprintf("found object %v %v for org %v: %v", objID, objType, org, objMeta)))
				return objMeta, nil
			} else {
				glog.V(5).Infof(rpclogString(fmt.Sprintf("object %v %v for org %v not found", objID, objType, org)))
				return nil, nil
			}
		}
	}
}

// Get the object's list of destinations.
func GetObjectDestinations(ec ExchangeContext, org string, objID string, objType string) (*ObjectDestinationStatuses, error) {

	var resp interface{}
	resp = new(ObjectDestinationStatuses)

	url := path.Join("/api/v1/objects", org, objType, objID, "destinations")
	url = ec.GetCSSURL() + url

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", url, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			dests := resp.(*ObjectDestinationStatuses)
			if len(*dests) != 0 {
				if glog.V(5) {
					if len(*(dests)) > DestinationsSizeLogLimit {
						glog.Infof(rpclogString(fmt.Sprintf("found destinations for %v %v %v: length of %v", org, objID, objType, len(*(dests)))))
					} else {
						glog.Infof(rpclogString(fmt.Sprintf("found destinations for %v %v %v: %v", org, objID, objType, dests)))
					}
				}
				return dests, nil
			} else {
				if glog.V(5) {
					glog.Infof(rpclogString(fmt.Sprintf("no destinations found for %v %v %v", org, objID, objType)))
				}
				return nil, nil
			}
		}
	}

}

// Tell the MMS that a policy update has been received.
func SetPolicyReceived(ec ExchangeContext, objPol *ObjectDestinationPolicy) error {
	// There is no response to CSS API.
	var resp interface{}

	url := path.Join("/api/v1/objects", objPol.OrgID, objPol.ObjectType, objPol.ObjectID, "policyreceived")
	url = ec.GetCSSURL() + url

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "PUT", url, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			if glog.V(5) {
				glog.Infof(rpclogString(fmt.Sprintf("set policy received for object %v %v of type %v", objPol.OrgID, objPol.ObjectID, objPol.ObjectType)))
			}
			return nil
		}
	}
}
