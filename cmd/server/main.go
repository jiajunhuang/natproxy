package main

import (
	"flag"

	"github.com/jiajunhuang/natproxy/server"
)

var (
	addr    = flag.String("addr", "127.0.0.1:10020", "natproxy server listen address")
	wanip   = flag.String("wanip", "127.0.0.1", "natproxy wan IP address")
	bufSize = flag.Int("buf", 1024, "max concurrency")
)

func main() {
	flag.Parse()

	server.Start(*addr, *wanip, *bufSize)
}
