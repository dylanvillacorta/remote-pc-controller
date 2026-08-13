package app

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"remote-pc-controller/sentinel/src/config"
	"remote-pc-controller/sentinel/src/windows"
)

// Run starts Sentinel either as a Windows SCM service or in console mode.
func Run(logger *log.Logger) error {
	if logger == nil {
		return errors.New("logger is required")
	}
	if err := runAsWindowsService(logger); err != nil {
		return err
	}
	if windowsServiceMode() {
		return nil
	}

	runtime, err := runtimeFromExecutable(logger)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runtime.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func runtimeFromExecutable(logger *log.Logger) (*Runtime, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(filepath.Join(filepath.Dir(executable), ".env"))
	if err != nil {
		return nil, err
	}
	return NewRuntime(cfg, logger, windows.NewHibernateExecutor())
}
