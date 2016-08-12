package dht

import (
	"testing"
)

func TestBitmap(t *testing.T) {
	a := newBitmap(10)
	b := newBitmapFrom(a, 10)
	c := newBitmapFromBytes([]byte{48, 49, 50, 51, 52, 53, 54, 55, 56, 57})
	d := newBitmapFromString("0123456789")
	e := newBitmap(10)

	// Bit
	for i := 0; i < a.Size; i++ {
		if a.Bit(i) != 0 {
			t.Fail()
		}
	}

	// Compare
	if c.Compare(d, d.Size) != 0 {
		t.Fail()
	}

	// RawString
	if c.RawString() != d.RawString() || c.RawString() != "0123456789" {
		t.Fail()
	}

	// Set
	b.Set(5)
	if b.Bit(5) != 1 {
		t.Fail()
	}

	// Unset
	b.Unset(5)
	if b.Bit(5) == 1 {
		t.Fail()
	}

	// String
	if e.String() != "0000000000" {
		t.Fail()
	}
	e.Set(9)
	if e.String() != "0000000001" {
		t.Fail()
	}
	e.Set(2)
	if e.String() != "0010000001" {
		t.Fail()
	}

	a.Set(0)
	a.Set(5)
	a.Set(8)
	if a.String() != "1000010010" {
		t.Fail()
	}

	// Xor
	b.Set(5)
	b.Set(9)
	if a.Xor(b).String() != "1000000011" {
		t.Fail()
	}
}
