package main

import (
	"flag"

	"github.com/jiajunhuang/natproxy/client"
)

func main() {
	flag.Parse()

	client.Start()
}
