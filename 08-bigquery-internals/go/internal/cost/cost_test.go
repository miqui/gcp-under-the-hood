package cost

import (
	"math"
	"testing"
)

func TestBilledBytes(t *testing.T) {
	cases := []struct {
		name      string
		processed int64
		cache     bool
		want      int64
	}{
		{"zero floors to 10MiB", 0, false, 10 * MiB},
		{"tiny floors", 1, false, 10 * MiB},
		{"exactly 10MiB", 10 * MiB, false, 10 * MiB},
		{"rounds up", 10*MiB + 1, false, 11 * MiB},
		{"exact multiple", 50 * MiB, false, 50 * MiB},
		{"cache hit free", 50 * MiB, true, 0},
		{"negative", -5, false, 0},
	}
	for _, c := range cases {
		if got := BilledBytes(c.processed, c.cache); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

func TestOnDemandUSD(t *testing.T) {
	if got := OnDemandUSD(TiB); math.Abs(got-6.25) > 1e-9 {
		t.Errorf("1TiB = %v", got)
	}
	if got := OnDemandUSD(0); got != 0 {
		t.Errorf("0 = %v", got)
	}
}

func TestBreakEven(t *testing.T) {
	if got := BreakEvenTiBPerMonth(625); math.Abs(got-100) > 1e-9 {
		t.Errorf("got %v", got)
	}
}

func TestHuman(t *testing.T) {
	cases := map[int64]string{512: "512 B", 2048: "2.00 KiB", 10 * MiB: "10.00 MiB", TiB: "1.00 TiB"}
	for in, want := range cases {
		if got := Human(in); got != want {
			t.Errorf("Human(%d)=%q want %q", in, got, want)
		}
	}
}

func TestCompare(t *testing.T) {
	rs := []Result{{"flat", 100 * MiB, 100 * MiB}, {"part", 25 * MiB, 25 * MiB}}
	cs := Compare(rs)
	if len(cs) != 2 || cs[0].PctOfBase != 100 || cs[1].PctOfBase != 25 {
		t.Errorf("unexpected %+v", cs)
	}
	if len(Compare(nil)) != 0 {
		t.Error("nil should be empty")
	}
	if Cheapest(rs) != "part" || Cheapest(nil) != "" {
		t.Error("cheapest wrong")
	}
}

func TestColumnBytes(t *testing.T) {
	w := map[string]int64{"a": 8, "b": 100}
	if got := ColumnBytes(1000, w, []string{"a"}); got != 8000 {
		t.Errorf("got %d", got)
	}
	if got := ColumnBytes(1000, w, []string{"a", "b"}); got != 108000 {
		t.Errorf("got %d", got)
	}
	if got := ColumnBytes(1000, w, nil); got != 0 {
		t.Errorf("got %d", got)
	}
}
