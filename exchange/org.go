package exchange

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"time"
)

type UserDefinition struct {
	Password    string `json:"password"`
	Admin       bool   `json:"admin"`
	Email       string `json:"email"`
	LastUpdated string `json:"lastUpdated,omitempty"`
}

type GetUsersResponse struct {
	Users     map[string]UserDefinition `json:"users"`
	LastIndex int                       `json:"lastIndex"`
}

// Functions and types for working with organizations in the exchange
type OrgLimits struct {
	MaxNodes int `json:"maxNodes"`
}

func (o OrgLimits) String() string {
	return fmt.Sprintf("MaxNodes: %v", o.MaxNodes)
}

type HeartbeatIntervals struct {
	MinInterval        int `json:"minInterval"`
	MaxInterval        int `json:"maxInterval"`
	IntervalAdjustment int `json:"intervalAdjustment"`
}

type Organization struct {
	Label         string              `json:"label,omitempty"`
	Description   string              `json:"description,omitempty"`
	Tags          map[string]string   `json:"tags,omitempty"`
	HeartbeatIntv *HeartbeatIntervals `json:"heartbeatIntervals,omitempty"`
	Limits        *OrgLimits          `json:"limits,omitempty"`
	LastUpdated   string              `json:"lastUpdated,omitempty"`
}

func (o Organization) String() string {
	return fmt.Sprintf("Label: %v, Description: %v, Tags %v, HeartbeatIntv %v, Limits %v", o.Label, o.Description, o.Tags, o.HeartbeatIntv, o.Limits)
}

type GetOrganizationResponse struct {
	Orgs      map[string]Organization `json:"orgs"`
	LastIndex int                     `json:"lastIndex"`
}

// Get the metadata for a specific organization.
func GetOrganization(httpClientFactory *config.HTTPClientFactory, org string, exURL string, id string, token string) (*Organization, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting organization definition %v", org)))

	if orgDef := GetOrgDefFromCache(org); orgDef != nil {
		return orgDef, nil
	}

	var resp interface{}
	resp = new(GetOrganizationResponse)

	// Search the exchange for the organization definition
	targetURL := fmt.Sprintf("%vorgs/%v", exURL, org)

	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, id, token, nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if httpClientFactory.RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", httpClientFactory.RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			orgs := resp.(*GetOrganizationResponse).Orgs
			if theOrg, ok := orgs[org]; !ok {
				return nil, errors.New(fmt.Sprintf("organization %v not found", org))
			} else {
				glog.V(3).Infof(rpclogString(fmt.Sprintf("found organization %v definition %v", org, theOrg)))
				UpdateCache(org, ORG_DEF_TYPE_CACHE, theOrg)
				return &theOrg, nil
			}
		}
	}

}
