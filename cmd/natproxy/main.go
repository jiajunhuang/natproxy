package main

import (
	"flag"

	"go.uber.org/zap"
)

var (
	logger, _ = zap.NewProduction()
)

func main() {
	defer logger.Sync()

	flag.Parse()
}
