package dht

import (
	"fmt"
	"testing"
)

var blacklist = newBlackList(256)

func TestGenKey(t *testing.T) {
	cases := []struct {
		in struct {
			ip   string
			port int
		}
		out string
	}{
		{struct {
			ip   string
			port int
		}{"0.0.0.0", -1}, "0.0.0.0"},
		{struct {
			ip   string
			port int
		}{"1.1.1.1", 8080}, "1.1.1.1:8080"},
	}

	for _, c := range cases {
		if blacklist.genKey(c.in.ip, c.in.port) != c.out {
			t.Fail()
		}
	}
}

func TestBlackList(t *testing.T) {
	address := []struct {
		ip   string
		port int
	}{
		{"0.0.0.0", -1},
		{"1.1.1.1", 8080},
		{"2.2.2.2", 8081},
	}

	for _, addr := range address {
		blacklist.insert(addr.ip, addr.port)
		if !blacklist.in(addr.ip, addr.port) {
			t.Fail()
		}

		blacklist.delete(addr.ip, addr.port)
		if blacklist.in(addr.ip, addr.port) {
			fmt.Println(addr.ip)
			t.Fail()
		}
	}
}
