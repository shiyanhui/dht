// +build !amd64,!386

package dht

func xor(dst, a, b []byte) {
	n := len(a)
	for i := 0; i < n; i++ {
		dst[i] = a[i] ^ b[i]
	}
}
