package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/sys/windows/svc"

	"remote-pc-controller/sentinel/src/app"
	"remote-pc-controller/sentinel/src/installer"
)

func main() {
	// If running under Windows Service Control Manager, run the background service.
	isService, err := svc.IsWindowsService()
	if err == nil && isService {
		logger := log.New(os.Stdout, "remote-pc-controller ", log.LstdFlags|log.LUTC)
		if err := app.Run(logger); err != nil {
			logger.Fatalf("agent stopped: %v", err)
		}
		return
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "service":
			if len(os.Args) > 2 && strings.ToLower(os.Args[2]) == "gui" {
				if err := installer.LaunchGUI(); err != nil {
					installer.ShowErrorDialog(err.Error())
					os.Exit(1)
				}
				return
			}
			installer.AttachConsoleIfCLI()
			if err := installer.Run(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		case "--gui", "-gui", "gui":
			if err := installer.LaunchGUI(); err != nil {
				installer.ShowErrorDialog(err.Error())
				os.Exit(1)
			}
			return
		}
	}

	// Interactive launch (double-click in Explorer or run without arguments):
	// Open the graphical installer and service control window.
	if err := installer.LaunchGUI(); err != nil {
		installer.ShowErrorDialog(err.Error())
		os.Exit(1)
	}
}
