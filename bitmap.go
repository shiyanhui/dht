package dht

import (
	"bytes"
	"fmt"
	"strings"
)

// bitmap represents a bit array.
type bitmap struct {
	Size int
	data []byte
}

// newBitmap returns a size-length bitmap pointer.
func newBitmap(size int) *bitmap {
	div, mod := size>>3, size&0x07
	if mod > 0 {
		div++
	}
	return &bitmap{size, make([]byte, div)}
}

// newBitmapFrom returns a new copyed bitmap pointer which
// newBitmap.data = other.data[:size].
func newBitmapFrom(other *bitmap, size int) *bitmap {
	bitmap := newBitmap(size)

	if size > other.Size {
		size = other.Size
	}

	div := size >> 3

	for i := 0; i < div; i++ {
		bitmap.data[i] = other.data[i]
	}

	for i := div << 3; i < size; i++ {
		if other.Bit(i) == 1 {
			bitmap.Set(i)
		}
	}

	return bitmap
}

// newBitmapFromBytes returns a bitmap pointer created from a byte array.
func newBitmapFromBytes(data []byte) *bitmap {
	bitmap := newBitmap(len(data) << 3)
	copy(bitmap.data, data)
	return bitmap
}

// newBitmapFromString returns a bitmap pointer created from a string.
func newBitmapFromString(data string) *bitmap {
	return newBitmapFromBytes([]byte(data))
}

// Bit returns the bit at index.
func (bitmap *bitmap) Bit(index int) int {
	if index >= bitmap.Size {
		panic("index out of range")
	}

	div, mod := index>>3, index&0x07
	return int((uint(bitmap.data[div]) & (1 << uint(7-mod))) >> uint(7-mod))
}

// set sets the bit at index `index`. If bit is true, set 1, otherwise set 0.
func (bitmap *bitmap) set(index int, bit int) {
	if index >= bitmap.Size {
		panic("index out of range")
	}

	div, mod := index>>3, index&0x07
	shift := byte(1 << uint(7-mod))

	bitmap.data[div] &= ^shift
	if bit > 0 {
		bitmap.data[div] |= shift
	}
}

// Set sets the bit at idnex to 1.
func (bitmap *bitmap) Set(index int) {
	bitmap.set(index, 1)
}

// Unset sets the bit at idnex to 0.
func (bitmap *bitmap) Unset(index int) {
	bitmap.set(index, 0)
}

// Compare compares the prefixLen-prefix of two bitmap.
//   - If bitmap.data[:prefixLen] < other.data[:prefixLen], return -1.
//   - If bitmap.data[:prefixLen] > other.data[:prefixLen], return 1.
//   - Otherwise return 0.
func (bitmap *bitmap) Compare(other *bitmap, prefixLen int) int {
	if prefixLen > bitmap.Size || prefixLen > other.Size {
		panic("index out of range")
	}

	div, mod := prefixLen>>3, prefixLen&0x07
	res := bytes.Compare(bitmap.data[:div], other.data[:div])
	if res != 0 {
		return res
	}

	for i := div << 3; i < (div<<3)+mod; i++ {
		bit1, bit2 := bitmap.Bit(i), other.Bit(i)
		if bit1 > bit2 {
			return 1
		} else if bit1 < bit2 {
			return -1
		}
	}

	return 0
}

// Xor returns the xor value of two bitmap.
func (bitmap *bitmap) Xor(other *bitmap) *bitmap {
	if bitmap.Size != other.Size {
		panic("size not the same")
	}

	distance := newBitmap(bitmap.Size)
	xor(distance.data, bitmap.data, other.data)

	return distance
}

// String returns the bit sequence string of the bitmap.
func (bitmap *bitmap) String() string {
	div, mod := bitmap.Size>>3, bitmap.Size&0x07
	buff := make([]string, div+mod)

	for i := 0; i < div; i++ {
		buff[i] = fmt.Sprintf("%08b", bitmap.data[i])
	}

	for i := div; i < div+mod; i++ {
		buff[i] = fmt.Sprintf("%1b", bitmap.Bit(div*8+(i-div)))
	}

	return strings.Join(buff, "")
}

// RawString returns the string value of bitmap.data.
func (bitmap *bitmap) RawString() string {
	return string(bitmap.data)
}
