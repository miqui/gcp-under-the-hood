// Package cost holds pure BigQuery on-demand billing logic.
package cost

import (
	"fmt"
	"sort"
)

const (
	// MiB is 2^20 bytes.
	MiB int64 = 1 << 20
	// TiB is 2^40 bytes.
	TiB int64 = 1 << 40
	// MinBilledBytes is the on-demand minimum per query (10 MiB).
	MinBilledBytes int64 = 10 * MiB
	// OnDemandUSDPerTiB is the list price at time of writing (verify current pricing).
	OnDemandUSDPerTiB = 6.25
)

// BilledBytes rounds processed bytes up to the next MiB with a 10 MiB floor.
// A query that scans zero bytes (e.g. cache hit) is billed zero; callers
// should pass cacheHit=true for that.
func BilledBytes(processed int64, cacheHit bool) int64 {
	if cacheHit || processed < 0 {
		return 0
	}
	rounded := ((processed + MiB - 1) / MiB) * MiB
	if rounded < MinBilledBytes {
		return MinBilledBytes
	}
	return rounded
}

// OnDemandUSD converts billed bytes to dollars.
func OnDemandUSD(billed int64) float64 {
	return float64(billed) / float64(TiB) * OnDemandUSDPerTiB
}

// BreakEvenTiBPerMonth returns how many TiB/month of on-demand scanning
// equals the cost of a flat monthly capacity commitment.
func BreakEvenTiBPerMonth(monthlyCapacityUSD float64) float64 {
	return monthlyCapacityUSD / OnDemandUSDPerTiB
}

// Human formats bytes with binary units.
func Human(b int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	f := float64(b)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", b)
	}
	return fmt.Sprintf("%.2f %s", f, units[i])
}

// Result is one dry-run measurement.
type Result struct {
	Label     string
	Processed int64
	Billed    int64
}

// Comparison is a Result relative to a baseline.
type Comparison struct {
	Result
	USD       float64
	PctOfBase float64 // billed / baseline billed * 100
}

// Compare computes cost and percent-of-baseline; the first result is the baseline.
func Compare(results []Result) []Comparison {
	out := make([]Comparison, 0, len(results))
	if len(results) == 0 {
		return out
	}
	base := results[0].Billed
	for _, r := range results {
		pct := 0.0
		if base > 0 {
			pct = float64(r.Billed) / float64(base) * 100
		}
		out = append(out, Comparison{Result: r, USD: OnDemandUSD(r.Billed), PctOfBase: pct})
	}
	return out
}

// Cheapest returns the label of the lowest-billed result.
func Cheapest(results []Result) string {
	if len(results) == 0 {
		return ""
	}
	s := append([]Result(nil), results...)
	sort.SliceStable(s, func(i, j int) bool { return s[i].Billed < s[j].Billed })
	return s[0].Label
}

// ColumnBytes estimates bytes scanned for a columnar table: only referenced
// columns are read. widths maps column->bytes per row.
func ColumnBytes(rows int64, widths map[string]int64, selected []string) int64 {
	var total int64
	for _, c := range selected {
		total += widths[c] * rows
	}
	return total
}
