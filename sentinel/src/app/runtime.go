package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"remote-pc-controller/sentinel/src/command"
	"remote-pc-controller/sentinel/src/config"
	"remote-pc-controller/sentinel/src/httpapi"
	"remote-pc-controller/sentinel/src/replay"
)

// Runtime wires the functional core to its HTTP and Windows-facing adapters.
type Runtime struct {
	config config.Config
	logger *log.Logger
	server *http.Server
}

func NewRuntime(cfg config.Config, logger *log.Logger, executor command.ActionExecutor) (*Runtime, error) {
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	replayStore, err := replay.NewStore(cfg.StatePath)
	if err != nil {
		return nil, err
	}
	service, err := command.NewService(
		command.Policy{
			DeviceID:    cfg.DeviceID,
			PublicKey:   cfg.PublicKey,
			ClockSkew:   time.Duration(cfg.ClockSkewSec) * time.Second,
			MaxValidity: command.DefaultMaxValidity,
		},
		command.WithReplayProtector(replayStore),
		command.WithActionExecutor(executor),
		command.WithAuditLogger(logger),
	)
	if err != nil {
		return nil, err
	}
	return &Runtime{
		config: cfg,
		logger: logger,
		server: &http.Server{
			Addr:              cfg.ListenAddr,
			Handler:           httpapi.NewHandler(logger, service, cfg.MaxBodyBytes),
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       30 * time.Second,
		},
	}, nil
}

func (r *Runtime) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if r.config.TLSCertFile != "" && r.config.TLSKeyFile != "" {
			errCh <- r.server.ListenAndServeTLS(r.config.TLSCertFile, r.config.TLSKeyFile)
			return
		}
		r.logger.Printf("WARNING: TLS_CERT_FILE/TLS_KEY_FILE absent; HTTP mode is for local development only")
		errCh <- r.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return r.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
