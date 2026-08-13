package command

import (
	"context"
	"fmt"
	"time"
)

// ReplayProtector is the storage port used to reject repeated commands.
type ReplayProtector interface {
	Claim(ctx context.Context, commandID, nonce string, expiresAt, now time.Time) error
}

// ActionExecutor is the side-effect boundary for permitted Windows actions.
type ActionExecutor interface {
	Hibernate(context.Context) error
}

// AuditLogger deliberately exposes only the logging capability Sentinel needs.
type AuditLogger interface {
	Printf(format string, args ...any)
}

// Service is the imperative boundary around the functional command policy.
// Dependencies are supplied with functional options so production and tests can
// use the same service without global state.
type Service struct {
	policy   Policy
	replay   ReplayProtector
	executor ActionExecutor
	now      func() time.Time
	audit    AuditLogger
}

type ServiceOption func(*Service)

func WithReplayProtector(replay ReplayProtector) ServiceOption {
	return func(service *Service) {
		service.replay = replay
	}
}

func WithActionExecutor(executor ActionExecutor) ServiceOption {
	return func(service *Service) {
		service.executor = executor
	}
}

func WithClock(now func() time.Time) ServiceOption {
	return func(service *Service) {
		service.now = now
	}
}

func WithAuditLogger(audit AuditLogger) ServiceOption {
	return func(service *Service) {
		service.audit = audit
	}
}

func NewService(policy Policy, options ...ServiceOption) (*Service, error) {
	service := &Service{
		policy: policy,
		now: func() time.Time {
			return time.Now().UTC()
		},
		audit: discardAuditLogger{},
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	if err := policy.ValidateConfiguration(); err != nil {
		return nil, err
	}
	if service.replay == nil {
		return nil, fmt.Errorf("command service: replay protector is required")
	}
	if service.executor == nil {
		return nil, fmt.Errorf("command service: action executor is required")
	}
	if service.now == nil {
		return nil, fmt.Errorf("command service: clock is required")
	}
	if service.audit == nil {
		service.audit = discardAuditLogger{}
	}
	return service, nil
}

type discardAuditLogger struct{}

func (discardAuditLogger) Printf(string, ...any) {}

// AcceptedCommand can only be created after policy and replay checks succeed.
type AcceptedCommand struct {
	command Command
}

func (c AcceptedCommand) CommandID() string {
	return c.command.CommandID
}

func (c AcceptedCommand) Action() Action {
	return c.command.Action
}

func (s *Service) Accept(ctx context.Context, command Command) (AcceptedCommand, error) {
	if err := ctx.Err(); err != nil {
		return AcceptedCommand{}, err
	}
	now := s.now().UTC()
	if err := s.policy.Validate(command, now); err != nil {
		s.audit.Printf("command rejected id=%q reason=%v", command.CommandID, err)
		return AcceptedCommand{}, err
	}
	if err := s.replay.Claim(ctx, command.CommandID, command.Nonce, s.policy.ReplayUntil(command), now); err != nil {
		s.audit.Printf("command rejected id=%q reason=%v", command.CommandID, err)
		return AcceptedCommand{}, err
	}

	s.audit.Printf("command accepted id=%q action=%s", command.CommandID, command.Action)
	return AcceptedCommand{command: command}, nil
}

func (s *Service) Execute(ctx context.Context, accepted AcceptedCommand) error {
	if accepted.command.Action != ActionHibernate {
		return ErrUnsupportedAction
	}
	if err := s.executor.Hibernate(ctx); err != nil {
		s.audit.Printf("hibernate failed id=%q: %v", accepted.CommandID(), err)
		return err
	}
	s.audit.Printf("hibernate started id=%q", accepted.CommandID())
	return nil
}
