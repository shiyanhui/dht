// +build !amd64,!386

package dht

func xor(dst, a, b []byte) {
	n := len(a)
	for i := 0; i < n; i++ {
		distance.data[i] = bitmap.data[i] ^ other.data[i]
	}
}
