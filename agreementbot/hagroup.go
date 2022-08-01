package agreementbot

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
)

// change is already deleted from cache
// save the new hagroup in cache. change contains org, hagroupname
// if change is:
// - create hagroup
//     for each member, agreementbot will need to find the workloadusage, then add hagroup, ha patner
// - update hagroup
//     agreementbot needs to get workloadusage by hagroup name, then update ha patners
// - delete hagroup
//     agreementbot needs to get workloadusage by hagroup name, then delete hagroup and ha patners for the []workloadusage
func (w *AgreementBotWorker) handleHAGroupChange(msg *events.ExchangeChangeMessage) error {
	glog.Info(AWlogString("Lily - AgreementBot start to handle HA group change"))
	change := msg.GetChange()

	glog.Info(AWlogString(fmt.Sprintf("Lily - AgreementBot: msg: %v, change from msg is %v", msg.String(), change)))

	haGroupChange, _ := change.(exchange.ExchangeChange)

	glog.Info(AWlogString(fmt.Sprintf("Lily - AgreementBot: haGroupChange is %v", haGroupChange.String())))
	glog.Info(AWlogString(fmt.Sprintf("Lily - AgreementBot handling HA group change: org: %v, haGroupName: %v, change operation: %v", haGroupChange.OrgID, haGroupChange.ID, haGroupChange.Operation)))

	if haGroupChange.Operation == exchange.CHANGE_OPERATION_CREATED {
		return w.addHAGroupToWorkloadUsage(haGroupChange.OrgID, haGroupChange.ID)
	} else if haGroupChange.Operation == exchange.CHANGE_OPERATION_MODIFIED {
		return w.updateHAGroupInWorkloadUsage(haGroupChange.OrgID, haGroupChange.ID)
	} else if haGroupChange.Operation == exchange.CHANGE_OPERATION_DELETED {
		return w.removeHAGroupFromWorkloadUsage(haGroupChange.OrgID, haGroupChange.ID)
	}

	return nil
}

func (w *AgreementBotWorker) addHAGroupToWorkloadUsage(org string, haGroupName string) error {
	// get ha members for this group
	glog.Info(AWlogString(fmt.Sprintf("Lily - addHAGroupToWorkloadUsage: org: %v, haGroupName: %v", org, haGroupName)))

	// add new group change: need to remove ha group cache
	glog.Info(AWlogString(fmt.Sprintf("Lily - addHAGroupToWorkloadUsage: deleting %v/%v from cache", org, haGroupName)))
	exchange.DeleteCacheResource(exchange.HA_GROUP_TYPE_CACHE, exchange.HAgroupCacheMapKey(org, haGroupName))

	exHAGroup, err := GetHAGroup(org, haGroupName, w.GetHTTPFactory().NewHTTPClient(nil), w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken())
	if err != nil {
		return errors.New(fmt.Sprintf("unable to get ha group %v/%v from exchange , error %v", org, haGroupName, err))
	}

	/*
			"members": [
		      "anaxdevice6",
		      "anaxdevice7"
		    ],
	*/

	glog.Info(AWlogString(fmt.Sprintf("Lily - addHAGroupToWorkloadUsage: get hagroup %v/%v from exchange: %v", org, haGroupName, exHAGroup)))
	devices := exHAGroup.Members

	return w.addOrUpdateHAGroupToWorkloadUsageForAddedMembers(org, haGroupName, exHAGroup.Members, devices)
}

func (w *AgreementBotWorker) updateHAGroupInWorkloadUsage(org string, haGroupName string) error {
	glog.Info(AWlogString(fmt.Sprintf("Lily - updateHAGroupInWorkloadUsage: org: %v, haGroupName: %v", org, haGroupName)))

	glog.Info(AWlogString(fmt.Sprintf("Lily - updateHAGroupInWorkloadUsage: getting %v/%v from cache", org, haGroupName)))
	// 1. get the old ha group memebers from cache
	cachedHAGroup := exchange.GetHAGroupFromCache(org, haGroupName)
	if cachedHAGroup != nil {
		glog.Info(AWlogString(fmt.Sprintf("Lily - updateHAGroupInWorkloadUsage: get %v/%v from cache: %v", org, haGroupName, cachedHAGroup)))
		glog.Info(AWlogString(fmt.Sprintf("Lily - updateHAGroupInWorkloadUsage: deleting %v/%v from cache", org, haGroupName)))
		// delete old cache
		exchange.DeleteCacheResource(exchange.HA_GROUP_TYPE_CACHE, exchange.HAgroupCacheMapKey(org, haGroupName))
	} else {
		errMsg := "cached HAGroup is nil"
		glog.Errorf(errMsg)
		return errors.New(errMsg)

	}
	// get ha members for this group
	exHAGroup, err := GetHAGroup(org, haGroupName, w.GetHTTPFactory().NewHTTPClient(nil), w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken())
	if err != nil {
		return errors.New(fmt.Sprintf("unable to get ha group %v/%v from exchange , error %v", org, haGroupName, err))
	}

	//newHAMembers := exHAGroup.Members
	//oldHAMembers := cachedHAGroup.Members

	newMemberList, deletedMembers, addedMembers, overlappedMembers := compareHAMembers(org, cachedHAGroup.Members, exHAGroup.Members)
	glog.V(5).Infof(AWlogString(fmt.Sprintf("Lily - newMemberList:%v", newMemberList)))
	glog.V(5).Infof(AWlogString(fmt.Sprintf("Lily - deletedMemebers:%v", deletedMembers)))
	glog.V(5).Infof(AWlogString(fmt.Sprintf("Lily - addedMemebers:%v", addedMembers)))
	glog.V(5).Infof(AWlogString(fmt.Sprintf("Lily - overlappedMemebers:%v", overlappedMembers)))

	// remove ha group
	if err = w.removeHAGroupFromWorkloadUsageForDeletedMembers(org, deletedMembers); err != nil {
		return err
	}

	// add ha group
	if err = w.addOrUpdateHAGroupToWorkloadUsageForAddedMembers(org, haGroupName, newMemberList, addedMembers); err != nil {
		return err
	}

	// update ha group
	if err = w.addOrUpdateHAGroupToWorkloadUsageForAddedMembers(org, haGroupName, newMemberList, overlappedMembers); err != nil {
		return err
	}

	return nil
}

func (w *AgreementBotWorker) removeHAGroupFromWorkloadUsage(org string, haGroupName string) error {
	glog.Info(AWlogString(fmt.Sprintf("Lily - removeHAGroupFromWorkloadUsage: org: %v, haGroupName: %v", org, haGroupName)))

	glog.Info(AWlogString(fmt.Sprintf("Lily - removeHAGroupFromWorkloadUsage: getting %v/%v from cache", org, haGroupName)))
	// 1. get the old ha group memebers from cache
	cachedHAGroup := exchange.GetHAGroupFromCache(org, haGroupName)
	if cachedHAGroup != nil {
		glog.Info(AWlogString(fmt.Sprintf("Lily - removeHAGroupFromWorkloadUsage: get %v/%v from cache: %v", org, haGroupName, cachedHAGroup)))
		glog.Info(AWlogString(fmt.Sprintf("Lily - removeHAGroupFromWorkloadUsage: deleting %v/%v from cache", org, haGroupName)))
		// delete old cache
		exchange.DeleteCacheResource(exchange.HA_GROUP_TYPE_CACHE, exchange.HAgroupCacheMapKey(org, haGroupName))
	} else {
		errMsg := "cached HAGroup is nil"
		glog.Errorf(errMsg)
		return errors.New(errMsg)
	}

	// deletedMembers:  [anaxdevice6 anaxdevice7]
	deletedMembers := cachedHAGroup.Members

	// remove ha group
	return w.removeHAGroupFromWorkloadUsageForDeletedMembers(org, deletedMembers)
}

func updateHAGroupAndPartnersInWLUsages(db persistence.AgbotDatabase, org string, haGroupName string, members []string, wlUsages []persistence.WorkloadUsage) error {
	glog.Info(AWlogString(fmt.Sprintf("Lily - updateHAGroupAndPartnersInWLUsages: org: %v, haGroupName: %v, haMembers: %v", org, haGroupName, members))) //haMembers: [userdev/anaxdevice6 userdev/anaxdevice7]
	haPartners := make([]string, 0)
	for _, wlUsage := range wlUsages {

		// TODO: generate partner list or call exchange to get partners???
		if len(members) != 0 {
			// generate HA parterns from ha group members
			haPartners = generateHAPartners(org, members, wlUsage.DeviceId)
		}

		glog.V(3).Infof(AWlogString(fmt.Sprintf("Lily - updateHAGroupAndPartnersInWLUsages: 1. haParnters for device %v are: %v", haPartners, wlUsage.DeviceId)))
		if updatedWorkloadUsage, err := db.UpdateHAGroupNameAndPartners(wlUsage.DeviceId, wlUsage.PolicyName, haGroupName, haPartners); err != nil {
			return err
		} else if updatedWorkloadUsage == nil {
			return errors.New(fmt.Sprintf("updatedWorkloadUsage for %v %v is nil", wlUsage.DeviceId, wlUsage.PolicyName))
		} else {
			glog.V(5).Infof(AWlogString(fmt.Sprintf("Lily - haGroupName and haPartners are updated in workloadUsage. Updated workloadUsage is %v", updatedWorkloadUsage)))
		}
	}
	return nil
}

func (w *AgreementBotWorker) addOrUpdateHAGroupToWorkloadUsageForAddedMembers(org string, haGroupName string, haGroupMemebers []string, devices []string) error {
	glog.Info(AWlogString(fmt.Sprintf("Lily - addOrUpdateHAGroupToWorkloadUsageForAddedMembers: org: %v, haGroupName: %v, haGroupMemebers: %v", org, haGroupName, haGroupMemebers)))
	for _, deviceID := range devices {
		if !deviceIDContainsOrg(deviceID) {
			deviceID = fmt.Sprintf("%v/%v", org, deviceID)
		}

		if wlUsages, err := w.db.FindWorkloadUsages([]persistence.WUFilter{persistence.DWUFilter(deviceID)}); err != nil {
			return err
		} else if len(wlUsages) == 0 {
			glog.V(3).Infof(AWlogString(fmt.Sprintf("Lily - no workloadusage find for device %v", deviceID)))
		} else {
			glog.V(3).Infof(AWlogString(fmt.Sprintf("Lily - find %v workloadusage for device %v", len(wlUsages), deviceID)))
			glog.V(3).Infof(AWlogString(fmt.Sprintf("Lily - adding/updating haGroup %v and partners %v to workloadusages for device %v", haGroupName, haGroupMemebers, deviceID)))

			if err = updateHAGroupAndPartnersInWLUsages(w.db, org, haGroupName, haGroupMemebers, wlUsages); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *AgreementBotWorker) removeHAGroupFromWorkloadUsageForDeletedMembers(org string, deletedMembers []string) error {
	for _, deviceID := range deletedMembers {
		// deviceID is org/device1 or device1
		if !deviceIDContainsOrg(deviceID) {
			deviceID = fmt.Sprintf("%v/%v", org, deviceID)
		}

		if wlUsages, err := w.db.FindWorkloadUsages([]persistence.WUFilter{persistence.DWUFilter(deviceID)}); err != nil {
			return err
		} else if len(wlUsages) != 0 {
			if err = updateHAGroupAndPartnersInWLUsages(w.db, org, "", []string{}, wlUsages); err != nil {
				return err
			}
		}
	}
	return nil
}

// old members are [device1, device2, device3]
// new members are [device2, device3, device4, device5]
// return new members (org/device), deleted members, new added members, overlapped memebers
func compareHAMembers(org string, oldMembers []string, newMembers []string) ([]string, []string, []string, []string) {
	deletedMembers := make([]string, 0)
	addedMembers := make([]string, 0)
	overlappedMembers := make([]string, 0)
	newMembersWithOrg := make([]string, 0) // convert [device2, device3, device4, device5] to [org/device2, org/device3, org/device4, org/device5]

	for _, newMember := range newMembers {
		found := false
		newMemberWithOrg := fmt.Sprintf("%v/%v", org, newMember)
		newMembersWithOrg = append(newMembersWithOrg, newMemberWithOrg)
		for _, oldMember := range oldMembers {
			if oldMember == newMember {
				found = true
				break
			}

		}
		if !found {
			addedMembers = append(addedMembers, newMemberWithOrg)
		}
	}

	for _, oldMember := range oldMembers {
		found := false
		oldMemberWithOrg := fmt.Sprintf("%v/%v", org, oldMember)
		for _, newMemberWithOrg := range newMembersWithOrg {
			if newMemberWithOrg == oldMemberWithOrg {
				found = true
				overlappedMembers = append(overlappedMembers, newMemberWithOrg)
				break
			}
		}
		if !found {
			deletedMembers = append(deletedMembers, oldMemberWithOrg)
		}
	}

	return newMembersWithOrg, deletedMembers, addedMembers, overlappedMembers

}

// haMembers: [userdev/anaxdevice6 userdev/anaxdevice7]
func generateHAPartners(org string, haMembers []string, deviceIDWithOrg string) []string {
	memberList := make([]string, 0)
	for _, haMember := range haMembers {
		glog.V(3).Infof(AWlogString(fmt.Sprintf("Lily - generateHAPartners: 1. haMember: %v", haMember)))
		if !deviceIDContainsOrg(haMember) {
			haMember = fmt.Sprintf("%v/%v", org, haMember)
		}

		glog.V(3).Infof(AWlogString(fmt.Sprintf("Lily - generateHAPartners: 2. haMember: %v", haMember)))
		if deviceIDWithOrg != haMember {
			memberList = append(memberList, haMember)
		}
	}

	return memberList
}

func deviceIDContainsOrg(deviceID string) bool {
	parts := strings.Split(deviceID, "/")
	return len(parts) == 2
}
