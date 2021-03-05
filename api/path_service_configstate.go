package api

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
)

// get the service configuration state for all the registered services.
func FindServiceConfigStateForOutput(errorhandler ErrorHandler, getServicesConfigState exchange.ServicesConfigStateHandler, db *bolt.DB) (bool, map[string][]exchange.ServiceConfigState) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pLocalDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read horizondevice object, error %v", err))), nil
	} else if pLocalDevice == nil {
		return errorhandler(NewAPIUserInputError("Exchange registration not recorded. Complete account and device registration with an exchange and then record device registration using this API's /horizondevice path.", "service/configstate")), nil
	}

	outConfigState, err := getServicesConfigState(pLocalDevice.Id, pLocalDevice.Token)
	if err != nil {
		glog.Errorf(apiLogString(fmt.Sprintf("Unable to retrieve the service configurations for node %v from the exchange, error %v", pLocalDevice.Id, err)))
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to retrieve the service configurations for node %v from the exchange, error %v", pLocalDevice.Id, err))), nil
	}

	out := make(map[string][]exchange.ServiceConfigState)
	out["configstates"] = outConfigState

	return false, out
}

// Change the config state for the given service in the exchange and return the services that are just changed to suspended.
// If the service url and org are both empty string, it applies to all the registered services for the node.
// If the service url is an empty string but org is not, it applies to all the registered the services for the given org.
func ChangeServiceConfigState(service_cs *exchange.ServiceConfigState,
	errorhandler ErrorHandler,
	getDevice exchange.DeviceHandler,
	postDeviceSCS exchange.PostDeviceServicesConfigStateHandler,
	db *bolt.DB) (bool, []events.ServiceConfigState) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pLocalDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read horizondevice object, error %v", err))), nil
	} else if pLocalDevice == nil {
		return errorhandler(NewAPIUserInputError("Exchange registration not recorded. Complete account and device registration with an exchange and then record device registration using this API's /horizondevice path.", "service/configstate")), nil
	}

	// input error checking
	if service_cs.Url != "" && service_cs.Org == "" {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("Please specify organization when the service url is not an empty string: %v", service_cs), "org")), nil
	}
	if service_cs.ConfigState != exchange.SERVICE_CONFIGSTATE_ACTIVE && service_cs.ConfigState != exchange.SERVICE_CONFIGSTATE_SUSPENDED {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("The service configstate '%v' is not supported. The supported states are: %v, %v", service_cs.ConfigState, exchange.SERVICE_CONFIGSTATE_ACTIVE, exchange.SERVICE_CONFIGSTATE_SUSPENDED), "configState")), nil
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Start changing service configuration state for %v for the node.", service_cs)))

	pDevice, err := getDevice(fmt.Sprintf("%v/%v", pLocalDevice.Org, pLocalDevice.Id), pLocalDevice.Token)
	if err != nil {
		glog.Errorf(apiLogString(fmt.Sprintf("Unable to retrieve node resource for %v from the exchange, error %v", pLocalDevice.Id, err)))
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to retrieve node resource for %v from the exchange, error %v", pLocalDevice.Id, err))), nil
	}

	// save the services that are turned into suspended state
	suspended_services := []events.ServiceConfigState{}

	found := false
	for _, svc_exchange := range pDevice.RegisteredServices {

		// svc_exchange.Url is in the form of org/url
		org, url := cutil.SplitOrgSpecUrl(svc_exchange.Url)

		// set to default if empty
		if svc_exchange.ConfigState == "" {
			svc_exchange.ConfigState = exchange.SERVICE_CONFIGSTATE_ACTIVE
		}

		if service_cs.Url != "" {
			// single service case
			if service_cs.Url == url && service_cs.Org == org {
				found = true
				if service_cs.ConfigState != svc_exchange.ConfigState && service_cs.ConfigState == exchange.SERVICE_CONFIGSTATE_SUSPENDED {
					suspended_services = append(suspended_services, *(events.NewServiceConfigState(url, org, service_cs.ConfigState)))
				}
				break
			}
		} else {
			if service_cs.Org == "" {
				// for all the registered services
				found = true
				if service_cs.ConfigState != svc_exchange.ConfigState && service_cs.ConfigState == exchange.SERVICE_CONFIGSTATE_SUSPENDED {
					suspended_services = append(suspended_services, *(events.NewServiceConfigState(url, org, service_cs.ConfigState)))
				}
			} else {
				// for all the registered services in the org
				if service_cs.Org == org {
					found = true
					if service_cs.ConfigState != svc_exchange.ConfigState && service_cs.ConfigState == exchange.SERVICE_CONFIGSTATE_SUSPENDED {
						suspended_services = append(suspended_services, *(events.NewServiceConfigState(url, org, service_cs.ConfigState)))
					}
				}
			}
		}
	}

	//handle not-found error
	if !found {
		if service_cs.Url != "" {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("No changes made. The service %v does not exist or is not a registered service in the exchange for node %v.", cutil.FormOrgSpecUrl(service_cs.Url, service_cs.Org), pDevice.Name), "url, org")), nil
		} else {
			if service_cs.Org == "" {
				return errorhandler(NewAPIUserInputError(fmt.Sprintf("No changes made. No registered services found in the exchange for node %v.", pDevice.Name), "url, org")), nil
			} else {
				return errorhandler(NewAPIUserInputError(fmt.Sprintf("No changes made. No registered services from organization %v found in the exchange for node %v.", service_cs.Org, pDevice.Name), "org")), nil
			}
		}
	}

	// change the exchange only when there are changes needed.
	err = postDeviceSCS(pLocalDevice.Name, pLocalDevice.Token, service_cs)
	if err != nil {
		glog.Errorf(apiLogString(fmt.Sprintf("Failed to change the service configuration state for the node %v in the exchange, error %v", pDevice.Name, err)))
		return errorhandler(NewSystemError(fmt.Sprintf("Failed to change the service configuration state for the node %v in the exchange, error %v", pDevice.Name, err))), nil
	}
	glog.V(5).Infof(apiLogString(fmt.Sprintf("Complete changing service configuration state to %v for the node.", service_cs)))

	return false, suspended_services
}
