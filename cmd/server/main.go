package main

import (
	"flag"

	"github.com/jiajunhuang/natproxy/server"
	"go.uber.org/zap"
)

var (
	logger, _ = zap.NewProduction()
	addr      = flag.String("addr", "127.0.0.1:10020", "natproxy server listen address")
	wanip     = flag.String("wanip", "127.0.0.1", "natproxy wan IP address")
)

func main() {
	defer logger.Sync()

	flag.Parse()

	logger.Info("natproxy server start!")
	server.Start(*addr, *wanip)
}
