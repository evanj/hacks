// Package bitset implements a dense bit set with a similar API as roaring bitmaps.
package bitset

import (
	"math/bits"
)

/*
bitSet is a fixed-size set of bits, encoded as an array of uint64. It has a similar interface
as Roaring Bitmaps. In my quick-and-dirty benchmark, bitSet is faster, but may use more
memory, since it uses an uncompressed representation. My quick and dirty benchmark suggests that
when around 25% of the bits are set, you should just use the dense representation.

See: https://pkg.go.dev/github.com/RoaringBitmap/roaring

Benchmark results with Go 1.19 on an Mac M1 Max, showing that this is always faster but uses more
memory.

	BenchmarkBitSet/bitset_set_and_iterate_p01
	BenchmarkBitSet/bitset_set_and_iterate_p01-10         	   11187	    105534 ns/op	  131112 B/op	       3 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p01
	BenchmarkBitSet/roaring_set_and_iterate_p01-10        	    2131	    572778 ns/op	   33576 B/op	      26 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p10
	BenchmarkBitSet/bitset_set_and_iterate_p10-10         	    1308	    911855 ns/op	  131112 B/op	       3 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p10
	BenchmarkBitSet/roaring_set_and_iterate_p10-10        	     433	   2758711 ns/op	   66808 B/op	      43 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p25
	BenchmarkBitSet/bitset_set_and_iterate_p25-10         	     534	   2249230 ns/op	  131112 B/op	       3 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p25
	BenchmarkBitSet/roaring_set_and_iterate_p25-10        	     158	   7536782 ns/op	  133272 B/op	      76 allocs/op
*/
type bitSet struct {
	bits []uint64
}

const bitSetBitSize = 64

// newBitSet returns a new bitSet that stores size bits. The supported indexes are [0, size).
func newBitSet(size int) *bitSet {
	// number of integers is size / bitSetBitSize rounded up
	numUints := (size + bitSetBitSize - 1) / bitSetBitSize
	return &bitSet{make([]uint64, numUints)}
}

// bitSetIndexAndMask returns the index in uint64 and the bit mask for index.
func bitSetIndexes(index int) (uintIndex int, bitIndex int) {
	const bitSetBitMask = bitSetBitSize - 1

	uintIndex = index / bitSetBitSize
	bitIndex = index & bitSetBitMask
	return uintIndex, bitIndex
}

// bitSetIndexAndMask returns the index in uint64 and the bit mask for index.
func bitSetIndexAndMask(index int) (uintIndex int, bitMask uint64) {
	uintIndex, bitIndex := bitSetIndexes(index)
	bitMask = uint64(1) << uint64(bitIndex)
	return uintIndex, bitMask
}

// lenBits returns the number of bits contained in bitSet. It will be >= size that was passed in
// when first created.
func (b *bitSet) lenBits() int {
	return len(b.bits) * bitSetBitSize
}

// add adds index to the set.
func (b *bitSet) add(index int) {
	uintIndex, bitMask := bitSetIndexAndMask(index)
	b.bits[uintIndex] |= bitMask
}

// contains returns true if index has been set.
func (b *bitSet) contains(index int) bool {
	uintIndex, bitMask := bitSetIndexAndMask(index)
	return b.bits[uintIndex]&bitMask != 0
}

// nextSet returns the set bit >= index. It returns lenBits() if there is no set bit >= index.
func (b *bitSet) nextSet(index int) int {
	uintIndex, bitIndex := bitSetIndexes(index)

	for uintIndex < len(b.bits) {
		bitsUint := b.bits[uintIndex]
		bitsUint >>= bitIndex
		zeros := bits.TrailingZeros64(bitsUint)
		if zeros < bitSetBitSize {
			// found a set bit!
			return zeros + index
		}

		// all zeros: nothing is set: check the next uint
		uintIndex++
		bitIndex = 0
		index = uintIndex * bitSetBitSize
	}
	return b.lenBits()
}

// numSet returns the number of set bits.
func (b *bitSet) numSet() int {
	total := 0
	for _, bitsUint := range b.bits {
		total += bits.OnesCount64(bitsUint)
	}
	return total
}

// toArray returns a slice of the indexes in this set. This is slow because it allocates a large
// amount of memory and should only be used when performance is not critical (e.g. tests).
func (b *bitSet) toArray() []int {
	indexes := make([]int, 0, b.numSet())
	it := b.iterator()
	for it.hasNext() {
		indexes = append(indexes, it.next())
	}
	return indexes
}

// bitSetIterators makes it easy to iterator over all set bits using for it.hasNext() { it.next() }.
// This is very similar to the roaring bitmaps interface so it should be possible to switch.
type bitSetIterator struct {
	set     *bitSet
	current int
}

// iterator returns an iterator over the set bits in the bitSet.
func (b *bitSet) iterator() *bitSetIterator {
	return &bitSetIterator{b, b.nextSet(0)}
}

func (b *bitSetIterator) hasNext() bool {
	return b.current < b.set.lenBits()
}

func (b *bitSetIterator) next() int {
	v := b.current
	b.current = b.set.nextSet(v + 1)
	return v
}
