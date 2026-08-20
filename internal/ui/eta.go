package ui

import "time"

// ETA estimates remaining time from throughput over a sliding window, so the
// estimate adapts when the run moves between fast and slow parts of a tree.
type ETA struct {
	samples []etaSample
}

type etaSample struct {
	t     time.Time
	bytes int64
}

const (
	etaWindow  = 45 * time.Second
	etaMinData = 5 * time.Second
)

func NewETA() *ETA { return &ETA{} }

// Update records the current progress and returns the estimated remaining
// seconds. ok is false while there is not enough data for a stable estimate.
func (e *ETA) Update(bytesDone, bytesTotal int64) (seconds float64, ok bool) {
	now := time.Now()
	e.samples = append(e.samples, etaSample{now, bytesDone})
	// Drop samples that fell out of the window.
	cut := 0
	for cut < len(e.samples)-1 && now.Sub(e.samples[cut].t) > etaWindow {
		cut++
	}
	e.samples = e.samples[cut:]

	first := e.samples[0]
	elapsed := now.Sub(first.t)
	if elapsed < etaMinData {
		return 0, false
	}
	rate := float64(bytesDone-first.bytes) / elapsed.Seconds()
	if rate <= 0 {
		return 0, false
	}
	remaining := float64(bytesTotal - bytesDone)
	if remaining < 0 {
		remaining = 0
	}
	return remaining / rate, true
}
