package agreementbot

import (
	"sync"
)

type AgreementLockManager struct {
	MapLock           sync.Mutex             // The lock that protects the map of agreement locks
	AgreementMapLocks map[string]*sync.Mutex // A map of locks by agreement id
}

func NewAgreementLockManager() *AgreementLockManager {
	lm := new(AgreementLockManager)
	lm.AgreementMapLocks = make(map[string]*sync.Mutex, 10)
	return lm
}

func (self *AgreementLockManager) getAgreementLock(agid string) *sync.Mutex {
	self.MapLock.Lock()
	defer self.MapLock.Unlock()

	if _, ok := self.AgreementMapLocks[agid]; !ok {
		self.AgreementMapLocks[agid] = new(sync.Mutex)
	}

	return self.AgreementMapLocks[agid]

}

func (self *AgreementLockManager) deleteAgreementLock(agid string) {
	self.MapLock.Lock()
	defer self.MapLock.Unlock()

	if _, ok := self.AgreementMapLocks[agid]; !ok {
		return
	}

	delete(self.AgreementMapLocks, agid)

}
