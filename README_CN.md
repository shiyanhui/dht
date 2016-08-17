![](https://raw.githubusercontent.com/shiyanhui/dht/master/doc/screen-shot.png)

在这个视频上你可以看到爬取效果[Youtube](https://www.youtube.com/watch?v=AIpeQtw22kc).

## Introduction

DHT实现了BitTorrent DHT协议，主要包括：

- [BEP-3 (部分)](http://www.bittorrent.org/beps/bep_0003.html)
- [BEP-5](http://www.bittorrent.org/beps/bep_0005.html)
- [BEP-9](http://www.bittorrent.org/beps/bep_0009.html)
- [BEP-10](http://www.bittorrent.org/beps/bep_0010.html)

它包含两种模式，标准模式和爬虫模式。标准模式遵循DHT协议，你可以把它当做一个标准
的DHT组件。爬虫模式是为了嗅探到更多torrent文件信息，它在某些方面不遵循DHT协议。
基于爬虫模式，你可以打造你自己的[BTDigg](http://btdigg.org/)。

[bthub.io](http://bthub.io)是一个基于这个爬虫而建的BT搜索引擎，你可以把他当做
BTDigg的替代品。

## Installation

    go get github.com/shiyanhui/dht

## Example

下面是一个简单的爬虫例子，你可以到[这里](https://github.com/shiyanhui/dht/blob/master/sample)看完整的Demo。

```go
import (
    "fmt"
    "github.com/shiyanhui/dht"
)

func main() {
    downloader := dht.NewWire(65536)
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

## Download

这个是已经编译好的Demo二进制文件，你可以到这里[下载](https://github.com/shiyanhui/dht/files/407021/spider.zip)。

## 注意

- 默认的爬虫配置需要300M左右内存，你可以根据你的服务器内存大小调整MaxNodes和
  BlackListMaxSize
- 目前还不能穿透NAT，因此还不能在局域网运行

## TODO

- [ ] NAT穿透，在局域网内也能够运行
- [ ] 完整地实现BEP-3，这样不但能够下载种子，也能够下载资源
- [ ] 优化

## Blog

你可以在[这里](https://github.com/shiyanhui/dht/wiki)看到DHT Spider教程。

## License

[MIT](https://github.com/shiyanhui/dht/blob/master/LICENSE)
