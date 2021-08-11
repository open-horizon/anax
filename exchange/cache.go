package exchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cache"
	"golang.org/x/crypto/sha3"
	"reflect"
	"strings"
	"sync"
	"time"
)

// This is the struct type that has the top-level cache of all resource types
type ResourceCache struct {
	allResources map[string]cache.Cache
	Lock         sync.Mutex
}

// The top-level cache
var ExchangeResourceCache *ResourceCache

// These are the resource type keys
const SVC_DEF_TYPE_CACHE = "SVC_DEF_CACHE"
const SVC_POL_TYPE_CACHE = "SVC_POL_CACHE"
const SVC_KEY_TYPE_CACHE = "SVC_KEY_CACHE"
const SVC_DOCKAUTH_TYPE_CACHE = "SVC_DOCKAUTH_CACHE"
const NODE_DEF_TYPE_CACHE = "NODE_DEF_CACHE"
const NODE_POL_TYPE_CACHE = "NODE_POLICY_CACHE"
const EXCH_VERS_TYPE_CACHE = "EXCH_VERS_CACHE"
const ORG_DEF_TYPE_CACHE = "ORG_DEF_CACHE"

// This only applies to the exchange version.
// All others are monitored for changes theough the changes api
const CACHE_TIMEOUT_S = 900

type CacheEntry struct {
	Resource    interface{} `json:"resource"`
	LastUpdated uint64      `json:"lastupdated"`
	Hash        []byte      `json:"hash"`
}

// Allow getresources to return a copy of cached resource so that multiple threads can use the same resource concurrently
func (c *CacheEntry) Copy() interface{} {
	var resourceCopy interface{}
	switch c.Resource.(type) {
	case map[string]ServiceDefinition:
		resourceCopy = ServiceMap(c.Resource.(map[string]ServiceDefinition)).DeepCopy()
	case map[string]string:
		resourceCopy = ServiceKeys(c.Resource.(map[string]string)).DeepCopy()
	case []ImageDockerAuth:
		resourceCopy = ServiceDockerAuth(c.Resource.([]ImageDockerAuth)).DeepCopy()
	case Device:
		resourceCopy = *(c.Resource.(Device)).DeepCopy()
	case ExchangePolicy:
		exchPol := c.Resource.(ExchangePolicy)
		resourceCopy = *(&exchPol).DeepCopy()
	default:
		resourceCopy = c.Resource
	}
	return resourceCopy
}

// GetNodeFromCache returns the node definition from the exchange cache if it is present, or nil if it is not
func GetNodeFromCache(nodeOrg string, nodeId string) *Device {
	node := GetResourceFromCache(NodeCacheMapKey(nodeOrg, nodeId), NODE_DEF_TYPE_CACHE, 0)

	if typedNode, ok := node.(Device); ok {
		return &typedNode
	}
	return nil
}

// GetNodePolicyFromCache returns the node policy from the exchange cache if it is present, or nil if it is not
func GetNodePolicyFromCache(nodeOrg string, nodeId string) *ExchangePolicy {
	nodePol := GetResourceFromCache(NodeCacheMapKey(nodeOrg, nodeId), NODE_POL_TYPE_CACHE, 0)

	if typedNodePol, ok := nodePol.(ExchangePolicy); ok {
		return &typedNodePol
	}

	return nil
}

// GetServiceFromCache returns the service definitions of all service versions from the exchange cache if any are present, or nil if it is not
func GetServiceFromCache(svcOrg string, svcId string, svcArch string) map[string]ServiceDefinition {
	svc := GetResourceFromCache(ServiceCacheMapKey(svcOrg, svcId, svcArch), SVC_DEF_TYPE_CACHE, 0)

	if typedSvc, ok := svc.(map[string]ServiceDefinition); ok {
		return typedSvc
	}
	return nil
}

type ServiceMap map[string]ServiceDefinition

func (s ServiceMap) DeepCopy() map[string]ServiceDefinition {
	svcMapCopy := make(map[string]ServiceDefinition, len(s))
	for key, val := range s {
		svcMapCopy[key] = *val.DeepCopy()
	}
	return svcMapCopy
}

// GetServicePolicyFromCache returns the service policy from the exchange cache if it is present, or nil if it is not
func GetServicePolicyFromCache(sId string) *ExchangePolicy {
	svcPol := GetResourceFromCache(sId, SVC_POL_TYPE_CACHE, 0)

	if typedSvcPol, ok := svcPol.(ExchangePolicy); ok {
		return &typedSvcPol
	}
	return nil
}

// GetServiceDockAuthFromCache returns the service docker auths from the exchange cache if it is present, or nil if it is not
func GetServiceDockAuthFromCache(sId string) *[]ImageDockerAuth {
	svcAuth := GetResourceFromCache(sId, SVC_DOCKAUTH_TYPE_CACHE, 0)

	if typedSvcAuth, ok := svcAuth.([]ImageDockerAuth); ok {
		return &typedSvcAuth
	}
	return nil
}

type ServiceDockerAuth []ImageDockerAuth

func (s ServiceDockerAuth) DeepCopy() []ImageDockerAuth {
	authCopy := []ImageDockerAuth{}
	for _, auth := range s {
		authCopy = append(authCopy, auth)
	}
	return authCopy
}

// GetServiceKeysFromCache returns the service keys from the exchange cache if it is present, or nil if it is not
func GetServiceKeysFromCache(sId string) *map[string]string {
	svcKeys := GetResourceFromCache(sId, SVC_KEY_TYPE_CACHE, 0)

	if typedSvcKeys, ok := svcKeys.(map[string]string); ok {
		return &typedSvcKeys
	}
	return nil
}

type ServiceKeys map[string]string

func (s ServiceKeys) DeepCopy() map[string]string {
	svcKeysCopy := make(map[string]string, len(s))
	for key, val := range s {
		svcKeysCopy[key] = val
	}
	return svcKeysCopy
}

// GetExchangeVersionFromCache returns the version of the exchange from the exchange cache if it is present or an emty string otherwise
func GetExchangeVersionFromCache(exchangeURL string) string {
	exchVers := GetResourceFromCache(exchangeURL, EXCH_VERS_TYPE_CACHE, CACHE_TIMEOUT_S)

	if typedExchVers, ok := exchVers.(string); ok {
		return typedExchVers
	}
	return ""
}

func GetOrgDefFromCache(org string) *Organization {
	orgDef := GetResourceFromCache(org, ORG_DEF_TYPE_CACHE, 0)

	if typedOrgDef, ok := orgDef.(Organization); ok {
		return &typedOrgDef
	}
	return nil
}

// GetResourceFromCache will return the requested resource from the specified type exchange cache or nil if it is not present
func GetResourceFromCache(resourceKey string, resourceType string, expirationS uint64) interface{} {
	glog.V(5).Infof("Get from exchange cache %s/%s", resourceType, resourceKey)

	if ExchangeResourceCache == nil || ExchangeResourceCache.allResources == nil {
		return nil
	}

	ExchangeResourceCache.Lock.Lock()
	defer ExchangeResourceCache.Lock.Unlock()

	resourceCache, ok := ExchangeResourceCache.allResources[resourceType]
	if !ok {
		return nil
	}
	entry := resourceCache.Get(resourceKey)
	if entry == nil {
		return nil
	}
	typedEntry, ok := entry.(CacheEntry)
	if !ok {
		glog.Errorf("Error: object returned from cache not of expected type.")
		return nil
	}
	expired := uint64(time.Now().Unix())-typedEntry.LastUpdated > expirationS
	if expirationS > 0 && expired {
		return nil
	}
	return typedEntry.Copy()
}

// UpdateCache will replace or create the provided resource in the given resource type cache
func UpdateCache(resourceKey string, resourceType string, updatedResource interface{}) {
	glog.V(5).Infof("Update exchange cache %s/%s with %v", resourceType, resourceKey, updatedResource)

	if ExchangeResourceCache == nil {
		newExchangeResourceCache := NewResourceCache()
		ExchangeResourceCache = &newExchangeResourceCache
	}

	ExchangeResourceCache.Lock.Lock()
	defer ExchangeResourceCache.Lock.Unlock()

	resourceCache, ok := ExchangeResourceCache.allResources[resourceType]
	if !ok {
		ExchangeResourceCache.allResources[resourceType] = cache.NewSimpleMapCache()
		resourceCache = ExchangeResourceCache.allResources[resourceType]
	}
	recordHash, err := hashResource(updatedResource)
	if err != nil {
		glog.Errorf("Failed to hash resource for cache. Error was : %v", err)
		recordHash = []byte{}
	}
	existingRecord := resourceCache.Get(resourceKey)
	existingRecordTyped := CacheEntry{}
	if existingRecord != nil {
		var ok bool
		if existingRecordTyped, ok = existingRecord.(CacheEntry); !ok {
			glog.Errorf("Error: object returned from cache not of expected type.")
			existingRecord = nil
		}
	}
	if existingRecord == nil || !bytes.Equal(existingRecordTyped.Hash, recordHash) {
		newRecord := CacheEntry{Resource: updatedResource, LastUpdated: uint64(time.Now().Unix()), Hash: recordHash}
		resourceCache.Put(resourceKey, newRecord)
		return
	}
	existingRecordTyped.LastUpdated = uint64(time.Now().Unix())
	resourceCache.Put(resourceKey, existingRecordTyped)
}

// DeleteCache will delete the entire cache for this type of resource
func DeleteCache(resourceType string) {
	glog.V(5).Infof("Delete exchange cache %s", resourceType)

	if ExchangeResourceCache == nil || ExchangeResourceCache.allResources == nil {
		return
	}

	ExchangeResourceCache.Lock.Lock()
	defer ExchangeResourceCache.Lock.Unlock()

	if _, ok := ExchangeResourceCache.allResources[resourceType]; ok {
		delete(ExchangeResourceCache.allResources, resourceType)
	}
}

// DeleteCacheResource will delete the cached resource specified if it is in the cache
// This will return the existing cached resource
func DeleteCacheResource(resourceType string, resourceKey string) interface{} {
	glog.V(5).Infof("Delete exchange cache resource %s/%s", resourceType, resourceKey)
	if ExchangeResourceCache == nil || ExchangeResourceCache.allResources == nil {
		return nil
	}

	var retResource interface{}
	retResource = nil

	ExchangeResourceCache.Lock.Lock()
	defer ExchangeResourceCache.Lock.Unlock()

	if resourceCache, ok := ExchangeResourceCache.allResources[resourceType]; ok {
		retResource = resourceCache.Get(resourceKey)

		resourceCache.Delete(resourceKey)
	}
	return retResource
}

// DeleteOrgCachedResources will delete all cached resources from the given org
func DeleteOrgCachedResources(org string) {
	glog.V(5).Infof("Delete all resources from org %v", org)
	if ExchangeResourceCache == nil || ExchangeResourceCache.allResources == nil {
		return
	}

	ExchangeResourceCache.Lock.Lock()
	defer ExchangeResourceCache.Lock.Unlock()

	for _, cache := range ExchangeResourceCache.allResources {
		orgResourceKeys := cache.GetKeys()
		for _, orgResourceKey := range orgResourceKeys {
			if strings.Index(orgResourceKey, fmt.Sprintf("%s/", org)) == 0 {
				cache.Delete(orgResourceKey)
			}
		}
	}
}

// DeleteCacheNodeWriteThru will delete the given node and return the typed node resource
func DeleteCacheNodeWriteThru(nodeOrg string, nodeId string) *Device {
	nodeDef := GetNodeFromCache(nodeOrg, nodeId)
	DeleteCacheResource(NODE_DEF_TYPE_CACHE, NodeCacheMapKey(nodeOrg, nodeId))
	return nodeDef
}

// UpdateCacheNodePutWriteThru will update the cached node with the provided changed node def being put to the exchange
// the device request is returned with the fields used to update the device erased. This allows us to ensure all the pdr fields are being used to update the cached device
func UpdateCacheNodePutWriteThru(nodeOrg string, nodeId string, cachedDevice *Device, pdr *PutDeviceRequest) {
	if cachedDevice == nil {
		cachedDevice = &Device{}
	}
	cachedDevice.Token = pdr.Token
	cachedDevice.Name = pdr.Name
	cachedDevice.NodeType = pdr.NodeType
	cachedDevice.Pattern = pdr.Pattern
	cachedDevice.RegisteredServices = pdr.RegisteredServices
	cachedDevice.MsgEndPoint = pdr.MsgEndPoint
	cachedDevice.SoftwareVersions = pdr.SoftwareVersions
	cachedDevice.PublicKey = string(pdr.PublicKey)
	cachedDevice.Arch = pdr.Arch
	cachedDevice.UserInput = pdr.UserInput

	pdr.Token = ""
	pdr.Name = ""
	pdr.NodeType = ""
	pdr.Pattern = ""
	pdr.RegisteredServices = nil
	pdr.MsgEndPoint = ""
	pdr.SoftwareVersions = nil
	pdr.PublicKey = nil
	pdr.Arch = ""
	pdr.UserInput = nil

	if !reflect.DeepEqual(*pdr, PutDeviceRequest{}) {
		// If you see this error, most likely a new field has been added to the PutDeviceRequest struct and this function needs to be updated to accomadate it
		glog.Errorf("Warning: Failed to completely update the cached device %s/%s. Changed fields present in the put request were not applied to the cached device. Dropping cache and continuing.", nodeOrg, nodeId)
		DeleteCacheResource(NODE_DEF_TYPE_CACHE, NodeCacheMapKey(nodeOrg, nodeId))
	} else {
		UpdateCache(NodeCacheMapKey(nodeOrg, nodeId), NODE_DEF_TYPE_CACHE, cachedDevice)
	}
}

// UpdateCacheNodePatchWriteThru will update the cached node with the provided node changes being patched to the exchange
func UpdateCacheNodePatchWriteThru(nodeOrg string, nodeId string, cachedDevice *Device, pdr *PatchDeviceRequest) {
	if cachedDevice == nil {
		return
	}
	if pdr != nil {
		if pdr.UserInput != nil {
			cachedDevice.UserInput = *pdr.UserInput
			pdr.UserInput = nil
		}
		if pdr.Pattern != nil && *pdr.Pattern != "" {
			cachedDevice.Pattern = *pdr.Pattern
			pdr.Pattern = nil
		}
		if pdr.Arch != nil && *pdr.Arch != "" {
			cachedDevice.Arch = *pdr.Arch
			pdr.Arch = nil
		}
		if pdr.RegisteredServices != nil {
			cachedDevice.RegisteredServices = *pdr.RegisteredServices
			pdr.RegisteredServices = nil
		}
	}
	if !reflect.DeepEqual(*pdr, PatchDeviceRequest{}) {
		// If you see this error, most likely a new field has been added to the PatchDeviceRequest struct and this function needs to be updated to accomadate it
		glog.Errorf("Warning: Failed to completely update the cached device %s/%s. Changed fields present in the patch request were not applied to the cached device. Dropping cache and continuing.", nodeOrg, nodeId)
		DeleteCacheResource(NODE_DEF_TYPE_CACHE, NodeCacheMapKey(nodeOrg, nodeId))
	} else {
		UpdateCache(NodeCacheMapKey(nodeOrg, nodeId), NODE_DEF_TYPE_CACHE, cachedDevice)
	}
}

// DeleteCacheResourceFromChange takes an ExchangeChange and attempts to delete the now out-of-date exchange cache resource if it is present
func DeleteCacheResourceFromChange(change ExchangeChange, nodeId string) {
	if change.IsService() {
		id, arch, _ := svcInformationFromSvcId(change.ID)
		DeleteCacheResource(SVC_DEF_TYPE_CACHE, ServiceCacheMapKey(change.OrgID, id, arch))

		sIdWithOrg := fmt.Sprintf("%s/%s", change.OrgID, change.ID)
		DeleteCacheResource(SVC_KEY_TYPE_CACHE, sIdWithOrg)
		DeleteCacheResource(SVC_DOCKAUTH_TYPE_CACHE, sIdWithOrg)
	} else if change.IsNode(nodeId) || change.IsNodeAgreement(nodeId) || change.IsNodeServiceConfigState(nodeId) {
		DeleteCacheResource(NODE_DEF_TYPE_CACHE, NodeCacheMapKey(change.OrgID, change.ID))
	} else if change.IsNodePolicy(nodeId) {
		DeleteCacheResource(NODE_POL_TYPE_CACHE, NodeCacheMapKey(change.OrgID, change.ID))
	} else if change.IsServicePolicy() {
		sIdWithOrg := fmt.Sprintf("%s/%s", change.OrgID, change.ID)
		DeleteCacheResource(SVC_POL_TYPE_CACHE, sIdWithOrg)
	} else if change.IsOrg() && (change.Operation == CHANGE_OPERATION_CREATED || change.Operation == CHANGE_OPERATION_DELETED) {
		DeleteOrgCachedResources(change.OrgID)
	} else if change.IsOrg() {
		DeleteCacheResource(ORG_DEF_TYPE_CACHE, change.OrgID)
	}
}

func svcInformationFromSvcId(svcId string) (svcUrl string, svcArch string, svcVersion string) {
	svcIdPieces := strings.Split(svcId, "_")
	if len(svcIdPieces) < 3 {
		glog.Errorf("Error: could not find service url, arch, and version in service id %s", svcId)
	}
	svcUrl = svcIdPieces[0]
	svcArch = svcIdPieces[len(svcIdPieces)-1]
	svcVersion = svcIdPieces[len(svcIdPieces)-2]
	return
}

// ServiceCacheMapKey returns a string to use for the cache map key for a service with the given org, id, and arch
func ServiceCacheMapKey(svcOrg string, svcId string, svcArch string) string {
	return fmt.Sprintf("%s/%s/%s", svcOrg, svcId, svcArch)
}

// ServicePolicyCacheMapKey returns a string to use for the cache map key for a service policy with the given org, id, arch, and version
func ServicePolicyCacheMapKey(svcOrg string, svcId string, svcArch string, svcVersion string) string {
	return fmt.Sprintf("%s/%s/%s/%s", svcOrg, svcId, svcArch, svcVersion)
}

// NodeCacheMapKey returns a string to use for the cache map key for a node with the given org and id
func NodeCacheMapKey(nodeOrg string, nodeId string) string {
	return fmt.Sprintf("%s/%s", nodeOrg, nodeId)
}

// NewResourceCache will create the top-level cache
func NewResourceCache() ResourceCache {
	return ResourceCache{allResources: map[string]cache.Cache{}, Lock: *new(sync.Mutex)}
}

// Hash the given resource for comparing
func hashResource(resource interface{}) ([]byte, error) {
	jsonResource, err := json.Marshal(resource)
	if err != nil {
		return nil, fmt.Errorf("Unable to marshal resource %v to json. Error was: %v", resource, err)
	}
	hash := sha3.Sum256([]byte(jsonResource))
	return hash[:], nil
}

// clear cache for all resources
func ClearAllResourceCache() {
	if ExchangeResourceCache != nil && ExchangeResourceCache.allResources != nil {
		ExchangeResourceCache.Lock.Lock()
		defer ExchangeResourceCache.Lock.Unlock()

		ExchangeResourceCache.allResources = map[string]cache.Cache{}
	}
}
