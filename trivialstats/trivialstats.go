// Package trivialstats implements some basic statistic functions that I keep reimplementing.
// Notably: counters that track min/max/average, and an implementation of exact percentiles.
// More sophisticated applications should use ddsketch: https://github.com/DataDog/sketches-go
package trivialstats

import (
	"math"
)

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
