//go:build windows

package app

import (
	"context"
	"log"

	"golang.org/x/sys/windows/svc"

	"remote-pc-controller/sentinel/src/installer"
)

var serviceMode bool

func windowsServiceMode() bool { return serviceMode }

func runAsWindowsService(logger *log.Logger) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isService {
		return nil
	}
	serviceMode = true

	// When running as a Windows service, redirect logs to the Windows Event Log
	// so they appear in Event Viewer instead of being lost to a non-existent console.
	eventLogWriter, err := installer.OpenEventLog()
	if err != nil {
		// Fall back to the original logger if event log is unavailable.
		logger.Printf("warning: could not open event log, using default logger: %v", err)
	} else {
		logger = log.New(eventLogWriter, "remote-pc-controller ", log.LstdFlags|log.LUTC)
	}

	return svc.Run(installer.ServiceName, windowsService{logger: logger, eventLog: eventLogWriter})
}

type windowsService struct {
	logger   *log.Logger
	eventLog *installer.EventLogWriter
}

func (s windowsService) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	defer func() {
		if s.eventLog != nil {
			s.eventLog.Close()
		}
	}()

	status <- svc.Status{State: svc.StartPending}
	runtime, err := runtimeFromExecutable(s.logger)
	if err != nil {
		s.logger.Printf("initialize runtime: %v", err)
		return false, 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runtime.Run(ctx)
	}()

	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				status <- request.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			default:
				s.logger.Printf("unexpected service control request: #%d", request.Cmd)
			}
		case err := <-done:
			if err != nil {
				s.logger.Printf("agent stopped: %v", err)
			}
			return false, 1
		}
	}
}
