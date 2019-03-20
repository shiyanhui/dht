// +build amd64 386

package dht

import "unsafe"

const wordSize = int(unsafe.Sizeof(uintptr(0)))

func xor(dst, a, b []byte) {
	n := len(a)

	w := n / wordSize
	if w > 0 {
		dw := *(*[]uintptr)(unsafe.Pointer(&dst))
		aw := *(*[]uintptr)(unsafe.Pointer(&a))
		bw := *(*[]uintptr)(unsafe.Pointer(&b))
		for i := 0; i < w; i++ {
			dw[i] = aw[i] ^ bw[i]
		}
	}

	for i := n - n%wordSize; i < n; i++ {
		dst[i] = a[i] ^ b[i]
	}
}
