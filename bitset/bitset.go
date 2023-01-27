// Package bitset implements a dense bit set with a similar API as roaring bitmaps.
// This is written as an experiment and is incomplete. Use bits-and-blooms instead,
// or Roaring Bitmaps if you want compressed sparsed bit sets.
//

package bitset

import (
	"math/bits"
)

/*
bitSet is a fixed-size set of bits, encoded as an array of uint64. It has a similar interface
as Roaring Bitmaps. In my quick-and-dirty benchmark, bitSet is faster, but uses more
memory unless >= ~25% of the bits are set. The bits-and-blooms implementation is more complete,
but has similar performance to this implementation. I had to do some minor tweaks to make the
performance the same on the M1 Max.

Roaring Bitmaps: https://pkg.go.dev/github.com/RoaringBitmap/roaring
bits-and-blooms: https://pkg.go.dev/github.com/bits-and-blooms/bitset

Benchmark results with Go 1.19 on an Mac M1 Max, showing that dense is faster but uses more memory.
This also shows that bits-and-blooms appears to be faster.

	BenchmarkBitSet/bitset_set_and_iterate_no_it_p01-10        81428 ns/op    131073 B/op        1 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p01-10              79303 ns/op    131073 B/op        1 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p01-10       84585 ns/op    131104 B/op        2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p01-10            562687 ns/op     33576 B/op       26 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_no_it_p10-10       675344 ns/op    131072 B/op        1 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p10-10             680148 ns/op    131072 B/op        1 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p10-10      724586 ns/op    131104 B/op        2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p10-10           2772311 ns/op     66808 B/op       43 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_no_it_p25-10      1660769 ns/op    131072 B/op        1 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p25-10            1679911 ns/op    131072 B/op        1 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p25-10     1836730 ns/op    131104 B/op        2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p25-10           7627645 ns/op    133272 B/op       76 allocs/op

Results on an Intel 11th Gen Core i5-1135G7 (TigerLake) shows that this is competitive with
bits-and-blooms.

	BenchmarkBitSet/bitset_set_and_iterate_no_it_p01-8        186328 ns/op    131072 B/op        1 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p01-8              189845 ns/op    131072 B/op        1 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p01-8       150519 ns/op    131104 B/op        2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p01-8             556439 ns/op     33576 B/op       26 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_no_it_p10-8        842341 ns/op    131072 B/op        1 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p10-8              849049 ns/op    131072 B/op        1 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p10-8       912134 ns/op    131104 B/op        2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p10-8            3094966 ns/op     66808 B/op       43 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_no_it_p25-8       2080622 ns/op    131072 B/op        1 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p25-8             2096033 ns/op    131072 B/op        1 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p25-8      2266388 ns/op    131104 B/op        2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p25-8            8376975 ns/op    133272 B/op       76 allocs/op
*/
type bitSet struct {
	bits []uint64
}

const bitSetBitSize = 64
const bitSetBitSizeLog2 = 6

// newBitSet returns a new bitSet that stores size bits. The supported indexes are [0, size).
func newBitSet(size int) *bitSet {
	// number of integers is size / bitSetBitSize rounded up
	numUints := (size + bitSetBitSize - 1) / bitSetBitSize
	return &bitSet{make([]uint64, numUints)}
}

// bitSetIndexAndMask returns the index in uint64 and the bit mask for index.
func bitSetIndexes(index int) (uintIndex int, bitIndex int) {
	const bitSetBitMask = bitSetBitSize - 1

	// index / bitSetBitSize should generate the same code and index >> bitSetBitSizeLog2
	// However, I think I ran into a code generation issue on ARM64/M1 Max, but I can't reproduce
	// it. Use the shift version since it should be better. I think I may have been hitting a
	// code alignment issue.
	uintIndex = index >> bitSetBitSizeLog2
	// uintIndex = index / bitSetBitSize
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
	// special case the first int: removes a few operations from the for loop
	// improves performance on M1 ARM64; no real diff on x86
	uintIndex, bitIndex := bitSetIndexes(index)
	if uintIndex >= len(b.bits) {
		return b.lenBits()
	}
	bitsUint := b.bits[uintIndex]
	bitsUint >>= bitIndex

	// testing if bitsUint != 0 does not seem to change performance
	zeros := bits.TrailingZeros64(bitsUint)
	if zeros < bitSetBitSize {
		// found a set bit!
		return zeros + index
	}

	// check the next integers
	uintIndex++
	for uintIndex < len(b.bits) {
		bitsUint := b.bits[uintIndex]
		zeros := bits.TrailingZeros64(bitsUint)
		if zeros < bitSetBitSize {
			// found a set bit!
			return zeros + uintIndex*bitSetBitSize
		}
		uintIndex++
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
