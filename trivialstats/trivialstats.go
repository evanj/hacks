// Package trivialstats implements some basic statistic functions that I keep reimplementing.
// Notably: counters that track min/max/average, and an implementation of exact percentiles.
// More sophisticated applications should use ddsketch: https://github.com/DataDog/sketches-go
package trivialstats

import (
	"fmt"
	"math"

	"golang.org/x/exp/slices"
)

// a 4 kiB page in case that helps memory allocation somehow
const distributionChunkSize = 4096 / 8

// AverageMinMax computes the average, minimum and maximum of a set of values.
type AverageMinMax struct {
	count      int64
	sum        int64
	min        int64
	max        int64
	overflowed bool
}

// Count returns the count of records.
func (a *AverageMinMax) Count() int64 {
	return a.count
}

func (a *AverageMinMax) panicIfEmpty() {
	if a.count == 0 {
		panic("count=0; result undefined")
	}
}

func (a *AverageMinMax) panicIfOverflow() {
	if a.overflowed {
		panic("sum overflowed; result undefined")
	}
}

// Sum returns the sum of all records.
func (a *AverageMinMax) Sum() int64 {
	a.panicIfEmpty()
	a.panicIfOverflow()
	return a.sum
}

// Min returns the minimum of all records.
func (a *AverageMinMax) Min() int64 {
	a.panicIfEmpty()
	return a.min
}

// Max returns the maximum of all records.
func (a *AverageMinMax) Max() int64 {
	a.panicIfEmpty()
	return a.max
}

// Average returns the average of all records.
func (a *AverageMinMax) Average() float64 {
	a.panicIfEmpty()
	a.panicIfOverflow()
	return float64(a.sum) / float64(a.count)
}

// NewAverageMinMax returns a new empty AverageMinMax.
func NewAverageMinMax() *AverageMinMax {
	return &AverageMinMax{max: math.MinInt64, min: math.MaxInt64}
}

// addOverflow returns a + b and true if the result did not cause underflow/overflow.
func addOverflow(a int64, b int64) (int64, bool) {
	out := a + b
	overflowed := (b > 0 && out < a) || (b < 0 && out > a)
	return out, !overflowed
}

// Record adds value to the set.
func (a *AverageMinMax) Record(value int64) {
	// count in theory could overflow, but overflowing int64 seems unlikely and paranoid
	// check for sum overflow though
	a.count++
	sum, ok := addOverflow(a.sum, value)
	a.sum = sum
	a.overflowed = a.overflowed || !ok

	if value < a.min {
		a.min = value
	}
	if value > a.max {
		a.max = value
	}
}

// Distribution records all samples to provide exact percentiles.
// More serious applications should use https://github.com/DataDog/sketches-go.
// Not thread-safe.
type Distribution struct {
	// records samples in separate chunks to limit the "worst case" delay in Add()
	sampleChunks [][]int64
}

func NewDistribution() *Distribution {
	return &Distribution{[][]int64{make([]int64, 0, distributionChunkSize)}}
}

func (d *Distribution) Add(sample int64) {
	last := d.sampleChunks[len(d.sampleChunks)-1]
	if len(last) >= distributionChunkSize {
		last = make([]int64, 0, distributionChunkSize)
		d.sampleChunks = append(d.sampleChunks, last)
	}
	d.sampleChunks[len(d.sampleChunks)-1] = append(last, sample)
}

// Merge combines values from other into this Distribution. Other is unmodified.
func (d *Distribution) Merge(other *Distribution) {
	// we share "full" chunks, but need to merge the "last" chunk
	merged := make([][]int64, 0, len(d.sampleChunks)+len(other.sampleChunks))
	merged = append(merged, other.sampleChunks[:len(other.sampleChunks)-1]...)
	merged = append(merged, d.sampleChunks...)
	d.sampleChunks = merged

	lastOtherChunk := other.sampleChunks[len(other.sampleChunks)-1]
	for _, value := range lastOtherChunk {
		d.Add(value)
	}
}

func (d *Distribution) Stats() DistributionStats {
	allValues := d.sampleChunks[0]
	for _, chunks := range d.sampleChunks[1:] {
		allValues = append(allValues, chunks...)
	}

	slices.Sort(allValues)

	total := int64(0)
	for _, v := range allValues {
		total += v
	}

	return DistributionStats{
		allValues[0],
		allValues[len(allValues)-1],
		float64(total) / float64(len(allValues)),
		int64(len(allValues)),

		allValues[int(float64(len(allValues))*0.01)],
		allValues[int(float64(len(allValues))*0.25)],
		allValues[int(float64(len(allValues))*0.5)],
		allValues[int(float64(len(allValues))*0.75)],
		allValues[int(float64(len(allValues))*0.9)],
		allValues[int(float64(len(allValues))*0.95)],
		allValues[int(float64(len(allValues))*0.99)],
	}
}

type DistributionStats struct {
	Min   int64
	Max   int64
	Avg   float64
	Count int64

	P1  int64
	P25 int64
	P50 int64
	P75 int64
	P90 int64
	P95 int64
	P99 int64
}

func (d DistributionStats) String() string {
	return fmt.Sprintf("count=%d avg=%.1f min=%d p1=%d p25=%d p50=%d p75=%d p90=%d p95=%d p99=%d max=%d",
		d.Count, d.Avg, d.Min, d.P1, d.P25, d.P50, d.P75, d.P90, d.P95, d.P99, d.Max,
	)
}
