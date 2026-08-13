//go:build windows

package app

import (
	"context"
	"log"

	"golang.org/x/sys/windows/svc"
)

var serviceMode bool

func windowsServiceMode() bool { return serviceMode }

func runAsWindowsService(logger *log.Logger) error {
	interactive, err := svc.IsAnInteractiveSession()
	if err != nil {
		return err
	}
	if interactive {
		return nil
	}
	serviceMode = true
	return svc.Run("RemotePcController", windowsService{logger: logger})
}

type windowsService struct {
	logger *log.Logger
}

func (s windowsService) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
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
			if request.Cmd == svc.Stop || request.Cmd == svc.Shutdown {
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			}
		case err := <-done:
			if err != nil {
				s.logger.Printf("agent stopped: %v", err)
			}
			return false, 1
		}
	}
}
