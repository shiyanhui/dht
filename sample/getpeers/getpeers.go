package main

import (
	"fmt"
	"log"
	"time"

	"github.com/shiyanhui/dht"
)

func main() {
	d := dht.New(nil)
	d.OnGetPeersResponse = func(infoHash string, peer *dht.Peer) {
		fmt.Printf("GOT PEER: <%s:%d>\n", peer.IP, peer.Port)
	}

	go func() {
		for {
			// ubuntu-14.04.2-desktop-amd64.iso
			err := d.GetPeers("546cf15f724d19c4319cc17b179d7e035f89c1f4")
			if err != nil && err != dht.ErrNotReady {
				log.Fatal(err)
			}

			if err == dht.ErrNotReady {
				time.Sleep(time.Second * 1)
				continue
			}

			break
		}
	}()

	d.Run()
}
