package agreementbot

import (
	"fmt"
	"github.com/golang/glog"
)

// A work queue that never blocks the sender and blocks the receiver when the internal work queue is empty.
// The high priority inbound channel can inject work into the workers even when the low priority queue is non-empty.
// Essentially, this allows high priority work to skip to the front of the line for the worker threads.
// Each inbound channel also has a buffer which holds inbound work that hasnt yet been dispatched to a worker. This
// ensures that high priority work generators dont block for very long.
type PrioritizedWorkQueue struct {
	inboundHigh         chan *AgreementWork // This is the high priority inbound channel.
	workQueueBufferHigh []*AgreementWork    // The internal work queue buffer for the high inbound channel.

	inboundLow         chan *AgreementWork // This is the low priority inbound channel.
	workQueueBufferLow []*AgreementWork    // The internal work queue buffer for the low inbound channel.

	recv chan *AgreementWork
}

func NewPrioritizedWorkQueue(bufferSize uint64) *PrioritizedWorkQueue {
	n := &PrioritizedWorkQueue{
		inboundHigh:         make(chan *AgreementWork, bufferSize),
		workQueueBufferHigh: make([]*AgreementWork, 0, bufferSize*2),
		inboundLow:          make(chan *AgreementWork, bufferSize),
		workQueueBufferLow:  make([]*AgreementWork, 0, bufferSize*2),
		recv:                make(chan *AgreementWork),
	}

	go n.run()
	return n
}

func (n *PrioritizedWorkQueue) Close() {
	close(n.inboundHigh)
	close(n.inboundLow)
}

func (n *PrioritizedWorkQueue) InboundHigh() chan *AgreementWork {
	return n.inboundHigh
}

func (n *PrioritizedWorkQueue) InboundLow() chan *AgreementWork {
	return n.inboundLow
}

func (n *PrioritizedWorkQueue) Receive() chan *AgreementWork {
	return n.recv
}

const HIGH_PRIORITY = "high"
const LOW_PRIORITY = "low"
const BOTH_PRIORITY = "both"

// This function loops forever buffering items between the Send and Receive channels until the Send
// channels are closed.
func (n *PrioritizedWorkQueue) run() {

	// Create a local receive channel that is guaranteed to block forever when used in the select below.
	// This ensures that this function will block on one of the send channels (and not get into an infinite loop)
	// when the work queue is empty.
	var recvChan chan *AgreementWork
	var recvVal *AgreementWork

	// Also create a local low priority inbound channel for the same reason, so that the high priority inbound channel
	// will be given preference within the select statement.
	var inLowChan chan *AgreementWork

	whichInbound := ""

	for {
		if n.inboundHigh == nil && len(n.workQueueBufferHigh) == 0 {
			glog.V(3).Infof(pwqString("Closing receive channel"))
			close(n.recv)
			break
		}

		// Assume that the select should ONLY block on the inbound channels.
		recvChan = nil

		// However, if there is bufferd work, the select will use the channel that worker threads are blocked on. This
		// will allow work to be passed to a worker.
		whichInbound = BOTH_PRIORITY
		if len(n.workQueueBufferHigh) > 0 {
			recvChan = n.recv
			recvVal = n.workQueueBufferHigh[0]
			whichInbound = HIGH_PRIORITY
		} else if len(n.workQueueBufferLow) > 0 {
			recvChan = n.recv
			recvVal = n.workQueueBufferLow[0]
			whichInbound = LOW_PRIORITY
		}

		// Assume that low priority inbound work is being accepted.
		inLowChan = n.inboundLow

		// However, if there is work on the inbound high priority channel, then dont let the select block on the low priority
		// inbound channel. This ensures the high priority inbound work is processed first.
		if len(n.inboundHigh) != 0 {
			inLowChan = nil
		}

		glog.V(5).Infof(pwqString(fmt.Sprintf("processing %v channels", whichInbound)))

		// When multiple cases of the select are true, one of them will be randomly chosen to execute.
		select {
		case i, ok := <-n.inboundHigh:
			if ok {
				glog.V(3).Infof(pwqString(fmt.Sprintf("queueing inbound high: %v", *i)))
				n.workQueueBufferHigh = append(n.workQueueBufferHigh, i)
			} else {
				// The channel must be closed now.
				glog.V(3).Infof(pwqString("closing inbound high"))
				n.inboundHigh = nil
			}
		case i, ok := <-inLowChan:
			if ok {
				glog.V(3).Infof(pwqString(fmt.Sprintf("queueing inbound low: %v", *i)))
				n.workQueueBufferLow = append(n.workQueueBufferLow, i)
			} else {
				// The channel must be closed now.
				glog.V(3).Infof(pwqString("closing inbound low"))
				n.inboundLow = nil
			}
		case recvChan <- recvVal:
			glog.V(5).Infof(pwqString(fmt.Sprintf("receiving %v", *recvVal)))
			if whichInbound == HIGH_PRIORITY {
				n.workQueueBufferHigh = n.workQueueBufferHigh[1:]
			} else if whichInbound == LOW_PRIORITY {
				n.workQueueBufferLow = n.workQueueBufferLow[1:]
			}
		}
	}
}

// global log record prefix
var pwqString = func(v interface{}) string {
	return fmt.Sprintf("Prioritized Work Queue: %v", v)
}
