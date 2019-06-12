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
	app.Name = "natproxys"
	app.Usage = "natproxy server"
	app.Action = func(c *cli.Context) error {
		logger.Info("natproxy server start!")
		return nil
	}
	err := app.Run(os.Args)
	if err != nil {
		logger.Fatal("failed to run server", zap.Error(err))
	}
}
