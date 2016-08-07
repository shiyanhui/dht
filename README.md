## DHT

![](https://raw.githubusercontent.com/shiyanhui/dht/master/doc/screen-shot.png)

DHT implements the bittorrent DHT protocol in Go. Now it includes:

- [BEP-3 (part)](http://www.bittorrent.org/beps/bep_0003.html)
- [BEP-5](http://www.bittorrent.org/beps/bep_0005.html)
- [BEP-9](http://www.bittorrent.org/beps/bep_0009.html)
- [BEP-10](http://www.bittorrent.org/beps/bep_0010.html)

It contains two modes, the standard mode and the crawling mode. The standard
mode follows the BEPs, and you can use it as a standard dht server. The crawling
mode aims to spide as more medata info as possiple. It doesn't follow the
standard BEPs protocol. With the crawling mode, you can build another Pirate Bay.

[bthub.io](http://bthub.io) is a BT search engine based on the crawling mode.

## Install

    go get github.com/shiyanhui/dht

## Example

Below is a simple spider. You can move [here](https://github.com/shiyanhui/dht/blob/master/sample)
to see more samples.

```go
import (
    "fmt"
    "github.com/shiyanhui/dht"
)

func main() {
    downloader := dht.NewWire()
    go func() {
        // once we got the request result
        for resp := range downloader.Response() {
            fmt.Println(resp.InfoHash, resp.MetadataInfo)
        }
    }()
    go downloader.Run()

    config := dht.NewCrawlConfig()
    config.OnAnnouncePeer = func(infoHash, ip string, port int) {
        // request to download the metadata info
        downloader.Request([]byte(infoHash), ip, port)
    }
    d := dht.New(config)

    d.Run()
}
```

## License

MIT.
