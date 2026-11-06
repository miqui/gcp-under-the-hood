package logic

import (
	"testing"
	"time"
)

func TestDuplicateTracker(t *testing.T) {
	d := NewDuplicateTracker()
	if c, dup := d.Observe("a"); c != 1 || dup {
		t.Fatalf("first: %d %v", c, dup)
	}
	if c, dup := d.Observe("a"); c != 2 || !dup {
		t.Fatalf("second: %d %v", c, dup)
	}
	d.Observe("b")
	got := d.Duplicates()
	if len(got) != 1 || got["a"] != 2 {
		t.Fatalf("dups: %v", got)
	}
}

func TestOrderChecker(t *testing.T) {
	o := NewOrderChecker()
	if err := o.Check("k", 1); err != nil {
		t.Fatal(err)
	}
	if err := o.Check("k", 2); err != nil {
		t.Fatal(err)
	}
	if err := o.Check("k", 2); err == nil {
		t.Fatal("want redelivery error")
	}
	if err := o.Check("k", 1); err == nil {
		t.Fatal("want out-of-order error")
	}
	if err := o.Check("other", 1); err != nil {
		t.Fatal("keys are independent")
	}
}

func TestBackoff(t *testing.T) {
	lo, hi := 5*time.Second, 60*time.Second
	cases := map[int]time.Duration{0: 5 * time.Second, 1: 5 * time.Second, 2: 10 * time.Second, 3: 20 * time.Second, 4: 40 * time.Second, 5: 60 * time.Second, 50: 60 * time.Second}
	for a, want := range cases {
		if got := Backoff(a, lo, hi); got != want {
			t.Errorf("attempt %d: got %v want %v", a, got, want)
		}
	}
	if got := Backoff(1, 90*time.Second, 60*time.Second); got != 60*time.Second {
		t.Errorf("min>max should cap: %v", got)
	}
}

func TestChooseOutcome(t *testing.T) {
	if ChooseOutcome("stall", 1, 2) != Stall || ChooseOutcome("stall", 3, 2) != Ack {
		t.Error("stall")
	}
	if ChooseOutcome("nack", 2, 2) != Nack || ChooseOutcome("nack", 3, 2) != Ack {
		t.Error("nack")
	}
	if ChooseOutcome("poison", 99, 2) != Nack {
		t.Error("poison")
	}
	if ChooseOutcome("", 1, 2) != Ack {
		t.Error("default")
	}
	if Stall.String() != "stall" || Ack.String() != "ack" || Nack.String() != "nack" {
		t.Error("string")
	}
}

func TestWillDeadLetterAndStall(t *testing.T) {
	if WillDeadLetter(4, 5) || !WillDeadLetter(5, 5) {
		t.Error("dlq")
	}
	if StallDuration(10*time.Second) != 15*time.Second {
		t.Error("stall dur")
	}
}

func TestOrderingKey(t *testing.T) {
	if OrderingKey(5, 3) != "entity-2" || OrderingKey(1, 0) != "entity-0" {
		t.Error("key")
	}
}
