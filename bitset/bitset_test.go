package bitset

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/RoaringBitmap/roaring/roaring64"
)

func TestBitSet(t *testing.T) {
	s := newBitSet(5)
	if s.numSet() != 0 {
		t.Errorf("numSet()=%d expected 0", s.numSet())
	}
	if s.contains(4) {
		t.Errorf("s.contains(4) must be false")
	}

	s.add(2)
	s.add(4)
	if !s.contains(2) {
		t.Errorf("s must contain 2")
	}
	if !s.contains(4) {
		t.Errorf("s must contain 4")
	}

	if s.nextSet(0) != 2 {
		t.Errorf("expected s.nextSet(0)=2; was %d", s.nextSet(0))
	}
	if s.nextSet(2) != 2 {
		t.Errorf("expected s.nextSet(2)=4; was %d", s.nextSet(2))
	}
	if s.nextSet(3) != 4 {
		t.Errorf("expected s.nextSet(3)=4; was %d", s.nextSet(3))
	}
	if s.nextSet(4) != 4 {
		t.Errorf("expected s.nextSet(3)=4; was %d", s.nextSet(4))
	}
	if s.nextSet(5) != s.lenBits() {
		t.Errorf("expected s.nextSet(5)=%d; was %d", s.lenBits(), s.nextSet(5))
	}
	if s.nextSet(s.lenBits()-1) != s.lenBits() {
		t.Errorf("expected s.nextSet(%d)=%d; was %d", s.lenBits()-1, s.nextSet(s.lenBits()-1), s.lenBits())
	}

	expected := []int{2, 4}
	indexes := s.toArray()
	if !reflect.DeepEqual(indexes, expected) {
		t.Errorf("s.toArray=%#v; expected=%#v", indexes, expected)
	}

	// test a set that needs multiple integers
	s = newBitSet(128)
	s.add(0)
	s.add(1)
	s.add(61)
	s.add(62)
	s.add(127)
	expected = []int{0, 1, 61, 62, 127}
	indexes = s.toArray()
	if !reflect.DeepEqual(indexes, expected) {
		t.Errorf("s.toArray=%#v; expected=%#v", indexes, expected)
	}
}

func BenchmarkBitSet(b *testing.B) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	const bitSetSize = 1000000

	// do something to make sure a very clever compiler can't optimize the benchmark away
	doNotOptimizeTotal := 0
	for _, percentToSet := range []int{1, 10, 25} {
		numToSet := bitSetSize * percentToSet / 100

		b.Run(fmt.Sprintf("bitset_set_and_iterate_p%02d", percentToSet), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				set := newBitSet(bitSetSize)
				for j := 0; j < numToSet; j++ {
					index := rng.Intn(numToSet)
					set.add(index)
				}

				it := set.iterator()
				count := 0
				for it.hasNext() {
					v := it.next()
					doNotOptimizeTotal += v
					count++
				}
			}
		})

		b.Run(fmt.Sprintf("roaring_set_and_iterate_p%02d", percentToSet), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				set := roaring64.New()
				for j := 0; j < numToSet; j++ {
					index := rng.Intn(numToSet)
					set.Add(uint64(index))
				}

				it := set.Iterator()
				count := 0
				for it.HasNext() {
					v := it.Next()
					doNotOptimizeTotal += int(v)
					count++
				}
			}
		})
	}

	b.Logf("IGNORE: doNotOptimizeTotal=%d", doNotOptimizeTotal)
}

type mapSet struct {
	bits map[int]struct{}
}

func newMapSet() *mapSet {
	return &mapSet{map[int]struct{}{}}
}

func (m *mapSet) add(index int) {
	m.bits[index] = struct{}{}
}

func (m *mapSet) toArray() []int {
	out := make([]int, 0, len(m.bits))
	for bitIndex := range m.bits {
		out = append(out, bitIndex)
	}
	sort.Ints(out)
	return out
}

func FuzzBitSet(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0x01})
	f.Add([]byte{0xff})
	f.Add([]byte{0x00, 0x01})
	f.Add([]byte{0x00, 0x01, 0xfe, 0xff})

	f.Fuzz(func(t *testing.T, bitIndexes []byte) {
		const bitSetSize = 256
		set := newBitSet(bitSetSize)
		mapSet := newMapSet()

		// reinterpret the bytes as bit indexes
		for _, b := range bitIndexes {
			set.add(int(b))
			mapSet.add(int(b))
		}

		setIndexes := set.toArray()
		mapIndexes := mapSet.toArray()
		if !reflect.DeepEqual(setIndexes, mapIndexes) {
			t.Errorf("setIndexes=%#v", setIndexes)
			t.Errorf("mapIndexes=%#v", mapIndexes)
		}
	})
}
