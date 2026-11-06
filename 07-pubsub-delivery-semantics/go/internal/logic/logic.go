// Package logic holds pure, cloud-free helpers used by the demo binary.
package logic

import (
	"fmt"
	"sync"
	"time"
)

// DuplicateTracker records how many times each message ID was seen.
type DuplicateTracker struct {
	mu   sync.Mutex
	seen map[string]int
}

// NewDuplicateTracker returns an empty tracker.
func NewDuplicateTracker() *DuplicateTracker {
	return &DuplicateTracker{seen: map[string]int{}}
}

// Observe returns the delivery count for id after this observation and
// whether this is a duplicate (count > 1).
func (d *DuplicateTracker) Observe(id string) (count int, dup bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen[id]++
	return d.seen[id], d.seen[id] > 1
}

// Duplicates returns ids seen more than once.
func (d *DuplicateTracker) Duplicates() map[string]int {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := map[string]int{}
	for k, v := range d.seen {
		if v > 1 {
			out[k] = v
		}
	}
	return out
}

// OrderChecker verifies per-ordering-key sequence numbers arrive monotonically.
type OrderChecker struct {
	mu   sync.Mutex
	last map[string]int64
}

// NewOrderChecker returns an empty checker.
func NewOrderChecker() *OrderChecker { return &OrderChecker{last: map[string]int64{}} }

// Check returns an error if seq is not greater than the last seq for key.
// An equal seq is reported as a redelivery, a lower one as out-of-order.
func (o *OrderChecker) Check(key string, seq int64) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	prev, ok := o.last[key]
	if ok && seq == prev {
		return fmt.Errorf("key %s: seq %d redelivered", key, seq)
	}
	if ok && seq < prev {
		return fmt.Errorf("key %s: seq %d arrived after %d (out of order)", key, seq, prev)
	}
	o.last[key] = seq
	return nil
}

// Backoff mirrors the shape of a Pub/Sub retry policy: exponential from min,
// doubling per attempt, capped at max. Illustrative only: the real service
// has its own details. attempt starts at 1.
func Backoff(attempt int, min, max time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := min
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= max {
			return max
		}
	}
	if d > max {
		return max
	}
	return d
}

// Outcome describes what a handler should do with a message.
type Outcome int

// Handler outcomes.
const (
	Ack Outcome = iota
	Nack
	Stall // do not ack in time -> ack deadline expires -> redelivery
)

func (o Outcome) String() string {
	switch o {
	case Ack:
		return "ack"
	case Nack:
		return "nack"
	default:
		return "stall"
	}
}

// ChooseOutcome decides deterministic misbehavior from the demo mode and
// delivery attempt (1 for the first delivery).
//
//	"stall":  stall on the first failUntil attempts, then ack
//	"nack":   nack on the first failUntil attempts, then ack
//	"poison": never succeed (ends in the dead-letter topic)
//	other:    always ack
func ChooseOutcome(mode string, attempt, failUntil int) Outcome {
	switch mode {
	case "stall":
		if attempt <= failUntil {
			return Stall
		}
	case "nack":
		if attempt <= failUntil {
			return Nack
		}
	case "poison":
		return Nack
	}
	return Ack
}

// WillDeadLetter reports whether a nack-always message is dead-lettered at
// this attempt given the policy's max_delivery_attempts.
func WillDeadLetter(attempt, maxAttempts int) bool { return attempt >= maxAttempts }

// StallDuration returns how long a stalled handler should sleep so the ack
// deadline (without lease extension) is surely exceeded.
func StallDuration(ackDeadline time.Duration) time.Duration {
	return ackDeadline + ackDeadline/2
}

// OrderingKey builds a stable ordering key for entity i out of n keys.
func OrderingKey(i, n int) string {
	if n < 1 {
		n = 1
	}
	return fmt.Sprintf("entity-%d", i%n)
}
