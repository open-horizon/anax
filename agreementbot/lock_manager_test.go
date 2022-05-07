//go:build unit
// +build unit

package agreementbot

import (
	"runtime"
	"testing"
	"time"
)

func Test_lock_lifecycle1(t *testing.T) {

	alm := NewAgreementLockManager()

	alock := alm.getAgreementLock("abc")

	if len(alm.AgreementMapLocks) != 1 {
		t.Errorf("There should be 1 lock in the map")
	}

	alm.deleteAgreementLock("abc")

	if len(alm.AgreementMapLocks) != 0 {
		t.Errorf("There should be 0 locks in the map")
	}

	alock.Lock()
	alock.Unlock()

}

func Test_lock_lifecycle2(t *testing.T) {

	s1 := -1
	s2 := -1
	alm := NewAgreementLockManager()

	modifySharedState := func(id string, s *int) {
		for i := 0; i < 100; i++ {
			lock := alm.getAgreementLock(id)
			runtime.Gosched()
			lock.Lock()
			runtime.Gosched()
			*s = i
			runtime.Gosched()
			lock.Unlock()
			runtime.Gosched()
		}
	}

	go modifySharedState("abc", &s1)
	go modifySharedState("abc", &s1)
	go modifySharedState("def", &s2)
	go modifySharedState("def", &s2)

	// Give time to finish
	time.Sleep(3 * time.Second)

	if s1 != 99 && s2 != 99 {
		t.Errorf("Shared state did not get to correct final values, is %v %v", s1, s2)
	}

}

func Test_lock_lifecycle3(t *testing.T) {

	alm := NewAgreementLockManager()

	getDelLock := func(id string) {
		for i := 0; i < 100; i++ {
			runtime.Gosched()
			alock := alm.getAgreementLock(id)
			runtime.Gosched()
			alm.deleteAgreementLock(id)
			runtime.Gosched()
			alock.Lock()
			runtime.Gosched()
			alock.Unlock()
			runtime.Gosched()
		}
	}

	go getDelLock("abc")
	go getDelLock("abc")
	go getDelLock("def")
	go getDelLock("def")
	go getDelLock("ghi")
	go getDelLock("ghi")

	// Give time to finish
	time.Sleep(3 * time.Second)

}
