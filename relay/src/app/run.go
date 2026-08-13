package app

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"remote-pc-controller/relay/src/config"
	"syscall"
)

func Run(logger *log.Logger) error {
	if logger == nil {
		return errors.New("logger is required")
	}
	envPath := os.Getenv("ENV_FILE")
	if envPath == "" {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		envPath = filepath.Join(filepath.Dir(executable), ".env")
	}
	cfg, err := config.Load(envPath)
	if err != nil {
		return err
	}
	runtime, err := NewRuntime(cfg, logger)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runtime.Run(ctx)
}
