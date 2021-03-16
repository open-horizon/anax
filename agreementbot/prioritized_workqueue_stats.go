package agreementbot

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"time"
)

// Statistics gathered by the work queue as it moves work from inbound to consumption by worker threads.
type PrioritizedWorkQueueStats struct {
	numInboundHigh  int       // The number of requests removed from the inbound high channel and moved to the high buffered list.
	numInboundLow   int       // The number of requests removed from the inbound low channel and moved to the low buffered list.
	numHighBuffered int       // The number of requests removed from the high buffer to a worker thread.
	numLowBuffered  int       // The number of requests removed from the low buffer to a worker thread.
	collectTime     time.Time // The time that this record was collected, i.e. added to history
}

func NewPrioritizedWorkQueueStats() *PrioritizedWorkQueueStats {
	return new(PrioritizedWorkQueueStats)
}

func (p *PrioritizedWorkQueueStats) consumedInboundHigh() {
	p.numInboundHigh += 1
}

func (p *PrioritizedWorkQueueStats) consumedInboundLow() {
	p.numInboundLow += 1
}

func (p *PrioritizedWorkQueueStats) consumedHighBuffered() {
	p.numHighBuffered += 1
}

func (p *PrioritizedWorkQueueStats) consumedLowBuffered() {
	p.numLowBuffered += 1
}

func (p *PrioritizedWorkQueueStats) empty() bool {
	return p.numInboundHigh == 0 && p.numInboundLow == 0 && p.numHighBuffered == 0 && p.numLowBuffered == 0
}

func (p *PrioritizedWorkQueueStats) report() string {
	return fmt.Sprintf("%-30s In High: %5d, Out High: %5d, In Low: %5d, Out Low: %5d", p.collectTime.Format(cutil.ExchangeTimeFormat), p.numInboundHigh, p.numHighBuffered, p.numInboundLow, p.numLowBuffered)
}

// This object holds all of the work queue stats generated to date
type PrioritizedWorkQueueHistory struct {
	history       []PrioritizedWorkQueueStats // The list of collected stats
	interval      int                         // How often to collect stats
	intervalStart time.Time                   // When the current stat collection started
	maxRecords    int                         // Maximum number of statistics records kept in memory
}

func NewPrioritizedWorkQueueHistory(interval int, maxRecords int) *PrioritizedWorkQueueHistory {
	return &PrioritizedWorkQueueHistory{
		history:       make([]PrioritizedWorkQueueStats, 0),
		interval:      interval,
		intervalStart: time.Now(),
		maxRecords:    maxRecords,
	}
}

func (p *PrioritizedWorkQueueHistory) Collect(s *PrioritizedWorkQueueStats, force bool) *PrioritizedWorkQueueStats {
	if force || time.Now().Sub(p.intervalStart).Seconds() >= float64(p.interval) {
		s.collectTime = p.intervalStart
		p.intervalStart = time.Now()
		p.addRecord(s)
		return NewPrioritizedWorkQueueStats()
	}
	return s
}

func (p *PrioritizedWorkQueueHistory) addRecord(s *PrioritizedWorkQueueStats) {
	glog.V(5).Infof(pwqString(fmt.Sprintf("Recent stats: %v", s.report())))
	if !s.empty() {
		p.history = append(p.history, *s)
		// If the max number of records is reached, remove the oldest history record.
		if len(p.history) > p.maxRecords {
			p.history = p.history[1:]
		}
	}
}

func (p *PrioritizedWorkQueueHistory) report() string {
	res := "Prioritized Queue History\n"
	for ix, stats := range p.history {
		res += fmt.Sprintf("%3d: ", ix) + stats.report() + "\n"
	}
	return res
}
