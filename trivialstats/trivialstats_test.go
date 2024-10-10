package trivialstats

import (
	"math"
	"strings"
	"testing"
)

func TestAverageMinMax(t *testing.T) {
	s := NewAverageMinMax()
	if s.Count() != 0 {
		t.Error(s.Count())
	}
	mustPanic(t, "count=0;", func() { s.Min() })
	mustPanic(t, "count=0;", func() { s.Max() })
	mustPanic(t, "count=0;", func() { s.Average() })

	// recording a maximal value should work
	s.Record(math.MaxInt64)
	if s.Count() != 1 {
		t.Error(s.Count())
	}
	if s.Min() != math.MaxInt64 {
		t.Error(s.Min())
	}
	if s.Max() != math.MaxInt64 {
		t.Error(s.Max())
	}
	if s.Average() != math.MaxInt64 {
		t.Error(s.Average())
	}
	if s.Sum() != math.MaxInt64 {
		t.Error(s.Sum())
	}
}

func TestAverageMinMaxValues(t *testing.T) {
	type testCase struct {
		inputs          []int64
		expectedMin     int64
		expectedMax     int64
		expectedAverage float64
		expectedSum     int64
	}
	testCases := []testCase{
		{[]int64{0}, 0, 0, 0, 0},
		{[]int64{1}, 1, 1, 1, 1},
		{[]int64{1, 2}, 1, 2, 1.5, 3},
		{[]int64{-1}, -1, -1, -1, -1},
		{[]int64{-1, math.MaxInt64}, -1, math.MaxInt64, 4611686018427387904.0, math.MaxInt64 - 1},
	}

	for i, testCase := range testCases {
		s := NewAverageMinMax()
		for _, input := range testCase.inputs {
			s.Record(input)
		}
		if s.Min() != testCase.expectedMin {
			t.Errorf("%d: s.Min()=%d expected=%d", i, s.Min(), testCase.expectedMin)
		}
		if s.Max() != testCase.expectedMax {
			t.Errorf("%d: s.Max()=%d expected=%d", i, s.Max(), testCase.expectedMax)
		}
		if s.Average() != testCase.expectedAverage {
			t.Errorf("%d: s.Average()=%f expected=%f", i, s.Average(), testCase.expectedAverage)
		}
		if s.Sum() != testCase.expectedSum {
			t.Errorf("%d: s.Sum()=%d expected=%d", i, s.Sum(), testCase.expectedSum)
		}

	}
}

func TestAverageMinMaxSumOverflow(t *testing.T) {
	// add MaxInt64 3 times: this will wrap around to a positive value but will have overflowed
	s := NewAverageMinMax()
	for i := 0; i < 3; i++ {
		s.Record(math.MaxInt64)
	}
	mustPanic(t, "sum overflowed;", func() { s.Sum() })
	mustPanic(t, "sum overflowed;", func() { s.Average() })

	// a successful addition must not reset the "overflowed" state
	s.Record(0)
	mustPanic(t, "sum overflowed;", func() { s.Sum() })
	mustPanic(t, "sum overflowed;", func() { s.Average() })

	// negative version: add minimum 2; will wrap to 0
	s = NewAverageMinMax()
	for i := 0; i < 2; i++ {
		s.Record(math.MinInt64)
	}
	mustPanic(t, "sum overflowed;", func() { s.Sum() })
	mustPanic(t, "sum overflowed;", func() { s.Average() })

	s.Record(0)
	mustPanic(t, "sum overflowed;", func() { s.Sum() })
	mustPanic(t, "sum overflowed;", func() { s.Average() })
}

func mustPanic(t *testing.T, panicSubstring string, f func()) {
	t.Helper()

	defer func() {
		t.Helper()
		r := recover()
		if r != nil {
			if s, ok := r.(string); ok {
				if !strings.Contains(s, panicSubstring) {
					t.Errorf("expected panic string %#v to contain %#v", panicSubstring, s)
				}
			} else {
				t.Errorf("expected panic with string; panic type=%T; value=%v", r, r)
			}
		}
	}()
	f()
	t.Errorf("did not panic; expected panic with substring=%#v", panicSubstring)
}

func TestMustPanic(t *testing.T) {
	// these all fail; to test uncomment
	// mustPanic(t, "must fail", func() {})
	// mustPanic(t, "must fail", func() { panic(42) })
	// mustPanic(t, "must fail", func() { panic("xxx") })

	mustPanic(t, "xxx", func() { panic("abc xxx xyz") })
}

func TestAddOverflow(t *testing.T) {
	negatives := []int64{math.MinInt64, math.MinInt64 + 1, -1}
	positives := []int64{1, math.MaxInt64 - 1, math.MaxInt64}

	all := append(append(negatives, 0), positives...)

	testCommutative := func(a int64, b int64) (int64, bool) {
		t.Helper()
		sum, ok := addOverflow(a, b)
		sum2, ok2 := addOverflow(b, a)
		if (sum != sum2) || (ok != ok2) {
			t.Errorf("not commutative: addOverflow(%d, %d)=%d,%t != addOverflow(%d, %d)=%d,%t",
				a, b, sum, ok,
				b, a, sum2, ok2,
			)
		}
		return sum, ok
	}

	// zero plus anything: okay same value
	for _, anyInt := range all {
		sum, ok := testCommutative(0, anyInt)
		if (sum != anyInt) || !ok {
			t.Errorf("addOverflow(0, %d)=%d,%t expected same, ok", anyInt, sum, ok)
		}
	}

	// negative plus min: not okay
	for _, negInt := range negatives {
		sum, ok := testCommutative(math.MinInt64, negInt)
		if ok {
			t.Errorf("addOverflow(%d, %d)=%d,%t expected not ok", math.MinInt64, negInt, sum, ok)
		}
	}
	// positive plus max: not okay
	for _, posInt := range positives {
		sum, ok := testCommutative(math.MaxInt64, posInt)
		if ok {
			t.Errorf("addOverflow(%d, %d)=%d,%t expected not ok", math.MaxInt64, posInt, sum, ok)
		}
	}
}

func TestDistribution(t *testing.T) {
	d := NewDistribution()
	for i := int64(0); i < 10; i++ {
		d.Add(i)
	}
	stats := d.Stats()
	expected := DistributionStats{
		0,
		9,
		4.5,
		10,

		0,
		2,
		5,
		7,
		9,
		9,
		9,
	}
	if expected != stats {
		t.Errorf("expected=%#v != stats=%#v", expected, stats)
	}

	d.Add(10)
	stats = d.Stats()
	expected = DistributionStats{
		0,
		10,
		5.0,
		11,

		0,
		2,
		5,
		8,
		9,
		10,
		10,
	}
	if expected != stats {
		t.Errorf("expected=%#v != stats=%#v", expected, stats)
	}

	empty := NewDistribution()
	d.Merge(empty)
	if d.Stats().Avg != 5.0 {
		t.Errorf("d.Stats().Avg=%f expected 5.0: Merge(empty) should not change anything",
			d.Stats().Avg)
	}

	other := NewDistribution()
	other.Add(20)
	d.Merge(other)
	if d.Stats().Avg != 6.25 {
		t.Errorf("d.Stats().Avg=%f expected 6.25: Merge(empty) should not change anything",
			d.Stats().Avg)
	}

	if other.Stats().Count != 1 {
		t.Errorf("other.Stats().Count=%d expected 1; other should not be modified by Merge",
			other.Stats().Count)
	}
	other.Add(50)
	if d.Stats().Count != 12 {
		t.Errorf("d.Stats().Count=%d expected 12; d should not be modified by Merge",
			d.Stats().Count)
	}

	for i := 0; i < distributionChunkSize; i++ {
		other.Add(int64(i))
	}
	dBefore := d.Stats().Count
	otherBefore := other.Stats().Count
	d.Merge(other)
	if d.Stats().Count != dBefore+otherBefore {
		t.Errorf("d.Stats().Count=%d expected %d; Merge should merge all records",
			d.Stats().Count, dBefore+otherBefore)
	}
}
