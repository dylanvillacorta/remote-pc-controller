package main

import (
	"log"
	"os"

	"remote-pc-controller/sentinel/src/app"
)

func main() {
	logger := log.New(os.Stdout, "remote-pc-controller ", log.LstdFlags|log.LUTC)
	if err := app.Run(logger); err != nil {
		logger.Fatalf("agent stopped: %v", err)
	}
}
