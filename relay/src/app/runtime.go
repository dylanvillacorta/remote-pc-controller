package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"remote-pc-controller/relay/src/command"
	"remote-pc-controller/relay/src/config"
	"remote-pc-controller/relay/src/httpapi"
	"remote-pc-controller/relay/src/transport"
	"time"
)

type Runtime struct {
	cfg    config.Config
	server *http.Server
}

func NewRuntime(cfg config.Config, logger *log.Logger) (*Runtime, error) {
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	service, err := command.NewService(command.Policy{DeviceID: cfg.DeviceID, MaxValidity: time.Duration(cfg.ValiditySeconds) * time.Second}, command.WithSigner(command.RSAPSSSigner(cfg.PrivateKey)), command.WithDelivery(transport.NewClient(cfg.SentinelURL)))
	if err != nil {
		return nil, err
	}
	return &Runtime{cfg: cfg, server: &http.Server{Addr: cfg.ListenAddr, Handler: httpapi.NewHandler(service, cfg.APISecret, cfg.MaxBodyBytes), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 30 * time.Second}}, nil
}
func (r *Runtime) Run(ctx context.Context) error {
	result := make(chan error, 1)
	go func() {
		result <- r.server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return r.server.Shutdown(shutdown)
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
