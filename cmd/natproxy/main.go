package main

import (
	"flag"

	"github.com/jiajunhuang/natproxy/client"
	"go.uber.org/zap"
)

var (
	logger, _ = zap.NewProduction()
)

func main() {
	defer logger.Sync()

	flag.Parse()

	client.Start()
}
