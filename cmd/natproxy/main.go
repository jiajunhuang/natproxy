package main

import (
	"os"

	"github.com/urfave/cli"
	"go.uber.org/zap"
)

var (
	logger, _ = zap.NewProduction()
)

func main() {
	defer logger.Sync()

	app := cli.NewApp()
	app.Name = "natproxy"
	app.Usage = "natproxy client"
	app.Action = func(c *cli.Context) error {
		logger.Info("natproxy client start!")
		return nil
	}
	err := app.Run(os.Args)
	if err != nil {
		logger.Fatal("failed to run natproxy", zap.Error(err))
	}
}
