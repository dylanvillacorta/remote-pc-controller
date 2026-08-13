package main

import (
	"log"
	"os"
	"remote-pc-controller/relay/src/app"
)

func main() {
	logger := log.New(os.Stdout, "remote-pc-controller-relay ", log.LstdFlags|log.LUTC)
	if err := app.Run(logger); err != nil {
		logger.Fatalf("relay stopped: %v", err)
	}
}
