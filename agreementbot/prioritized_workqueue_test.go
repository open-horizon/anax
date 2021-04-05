// +build unit

package agreementbot

import (
	"flag"
	"testing"
	"time"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_PrioritizedWorkQueue_serial(t *testing.T) {
	nbc := NewPrioritizedWorkQueue(uint64(100), 2, 10)
	if nbc == nil {
		t.Errorf("constructor should return non-nil object")
	}

	wi1 := NewCancelAgreement("1234567890", "Basic", 100, 0)
	wi2 := NewCancelAgreement("1234567890", "Basic", 101, 0)

	nbc.InboundLow() <- &wi1
	nbc.InboundHigh() <- &wi2

	// Pause for a moment for the concurrent routines to catch up
	time.Sleep(100 * time.Millisecond)

	if nbc.HighIsEmpty() {
		t.Errorf("expected inbound high buffer to be non-empty")
	} else if nbc.LowIsEmpty() {
		t.Errorf("expected inbound low buffer to be non-empty")
	} else if nbc.HighAtSize(2) {
		t.Errorf("expected inbound high buffer to have less than 2 elements, has %v", nbc.HighPriorityBufferLen())
	} else if nbc.LowAtSize(2) {
		t.Errorf("expected inbound low buffer to have less than 2 elements, has %v", nbc.LowPriorityBufferLen())
	}

	rwi2 := *(<-nbc.Receive())
	rwi1 := *(<-nbc.Receive())

	// Pause for a moment for the concurrent routines to catch up
	time.Sleep(100 * time.Millisecond)

	// Make sure the work items were processed in the right order
	if rwi1 == nil || rwi1.(CancelAgreement).Reason != 100 {
		t.Errorf("expected %v but got %v", wi1, rwi1)
	} else if rwi2 == nil || rwi2.(CancelAgreement).Reason != 101 {
		t.Errorf("expected %v but got %v", wi2, rwi2)
	} else if !nbc.HighIsEmpty() {
		t.Errorf("expected inbound high buffer to be empty, but was not: %v", nbc.HighPriorityBufferLen())
	} else if !nbc.LowIsEmpty() {
		t.Errorf("expected inbound low buffer to be empty, but was not: %v", nbc.LowPriorityBufferLen())
	} else if nbc.HighAtSize(1) {
		t.Errorf("expected inbound high buffer to be empty, but was not: %v", nbc.HighPriorityBufferLen())
	} else if nbc.LowAtSize(1) {
		t.Errorf("expected inbound low buffer to be empty, but was not: %v", nbc.LowPriorityBufferLen())
	}

	// Close the sending channel
	nbc.Close()

	// Block briefly to give the channel function time to see the close and clean up
	time.Sleep(10 * time.Millisecond)

}

// Test that the non blocking queue will process all the work once on each worker thread.
func Test_PrioritizedWorkQueue_concurrent_priority_mix(t *testing.T) {

	const QSIZE = uint64(100)

	// Make the internal buffer smaller to force the work queue-ing thread to give up control once in a while.
	nbc := NewPrioritizedWorkQueue(10, 2, 10)
	if nbc == nil {
		t.Errorf("constructor should return non-nil object")
	}

	// Set up the initial work item number
	counter := uint(0)

	// Fire off a work receiver that runs concurrently
	worklist1 := make([]uint, 0, QSIZE)
	go workerThread(t, nbc, &worklist1)

	// Fire off another work receiver that runs concurrently
	worklist2 := make([]uint, 0, QSIZE)
	go workerThread(t, nbc, &worklist2)

	// Now load up the work queue as fast as possible.
	// Create randomness with high and low priority work.
	randChan := make(chan bool, QSIZE+1)
	for {

		wi := NewCancelAgreement("1234567890", "Basic", counter, 0)

		select {
		case randChan <- true:
			nbc.InboundHigh() <- &wi
		case randChan <- false:
			nbc.InboundLow() <- &wi
		}

		counter += 1
		if uint64(counter) >= QSIZE {
			break
		}

	}

	// Pause for a moment for the concurrent routines to catch up
	time.Sleep(1000 * time.Millisecond)

	// Ensure the buffer queues are empty
	if !nbc.HighIsEmpty() {
		t.Errorf("expected inbound high buffer to be empty, but was not: %v", nbc.HighPriorityBufferLen())
	} else if !nbc.LowIsEmpty() {
		t.Errorf("expected inbound low buffer to be empty, but was not: %v", nbc.LowPriorityBufferLen())
	}

	// Close the sending channel
	nbc.Close()

	// Block briefly to give the channel function time to see the close and clean up
	time.Sleep(10 * time.Millisecond)

	// Log the queue statistics in case they are needed for debug
	t.Log(nbc.queueHistory.report())

	// Check that there are no intersections in the processed worklists
	for _, v := range worklist1 {
		for _, w := range worklist2 {
			if v == w {
				t.Errorf("processed work lists overlap %v is in 1:%v 2:%v", v, worklist1, worklist2)
			}
		}
	}

	// Make sure the worklists are the right size when combined.
	cl := worklist1
	cl = append(cl, worklist2...)

	if uint64(len(cl)) != QSIZE {
		t.Errorf("combined work list has %v elements, should have %v, elements: %v", len(cl), QSIZE, cl)
	}

}

func workerThread(t *testing.T, nbc *PrioritizedWorkQueue, worklist *[]uint) {
	for {
		// Retrieve a work item from the queue
		rwiPtr := <-nbc.Receive()

		// If the received item is nil, just terminate because that means the channel is closing.
		if rwiPtr == nil {
			return
		}
		rwi := *rwiPtr
		*worklist = append(*worklist, rwi.(CancelAgreement).Reason)

	}
}
