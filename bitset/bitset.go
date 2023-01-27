// Package bitset implements a dense bit set with a similar API as roaring bitmaps.
// This is written as an experiment and is incomplete. Use bits-and-blooms instead,
// or Roaring Bitmaps if you want compressed sparsed bit sets.
package bitset

import (
	"math/bits"
)

/*
bitSet is a fixed-size set of bits, encoded as an array of uint64. It has a similar interface
as Roaring Bitmaps. In my quick-and-dirty benchmark, bitSet is faster, but uses more
memory unless >= ~25% of the bits are set. The bits-and-blooms implementation is more complete,
but has similar performance to this implementation.

See: https://pkg.go.dev/github.com/RoaringBitmap/roaring
See:

Benchmark results with Go 1.19 on an Mac M1 Max, showing that dense is faster but uses more memory.
This also shows that bits-and-blooms appears to be faster.

	BenchmarkBitSet/bitset_set_and_iterate_p01-10         	   11330	    105355 ns/op	  131112 B/op	       3 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p01-10  	   14216	     84549 ns/op	  131104 B/op	       2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p01-10        	    2143	    560578 ns/op	   33576 B/op	      26 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p10-10         	    1317	    899670 ns/op	  131112 B/op	       3 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p10-10  	    1662	    722709 ns/op	  131104 B/op	       2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p10-10        	     435	   2746712 ns/op	   66808 B/op	      43 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p25-10         	     537	   2222508 ns/op	  131112 B/op	       3 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p25-10  	     669	   1781710 ns/op	  131104 B/op	       2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p25-10        	     159	   7484694 ns/op	  133272 B/op	      76 allocs/op

Results on an Intel 11th Gen Core i5-1135G7 (TigerLake) shows that this is competitive with
bits-and-blooms.

	cpu: 11th Gen Intel(R) Core(TM) i5-1135G7 @ 2.40GHz
	BenchmarkBitSet/bitset_set_and_iterate_no_it_p01-8         	   23568	    148285 ns/op	  131072 B/op	       1 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p01-8               	   25155	    144220 ns/op	  131112 B/op	       3 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p01-8        	   28768	    137650 ns/op	  131104 B/op	       2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p01-8              	    6109	    568225 ns/op	   33576 B/op	      26 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_no_it_p10-8         	    3720	    943776 ns/op	  131072 B/op	       1 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p10-8               	    3747	    958302 ns/op	  131112 B/op	       3 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p10-8        	    3870	    966873 ns/op	  131104 B/op	       2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p10-8              	    1138	   3391472 ns/op	   66808 B/op	      43 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_no_it_p25-8         	    1540	   2386860 ns/op	  131072 B/op	       1 allocs/op
	BenchmarkBitSet/bitset_set_and_iterate_p25-8               	    1428	   2476936 ns/op	  131112 B/op	       3 allocs/op
	BenchmarkBitSet/bitsandblooms_set_and_iterate_p25-8        	    1530	   2440958 ns/op	  131104 B/op	       2 allocs/op
	BenchmarkBitSet/roaring_set_and_iterate_p25-8              	     427	   8397258 ns/op	  133272 B/op	      76 allocs/op
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
