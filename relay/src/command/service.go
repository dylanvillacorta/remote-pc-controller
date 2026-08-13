package command

import (
	"context"
	"fmt"
	"time"
)

type Delivery interface { Deliver(context.Context, Command) error }
type Service struct { policy Policy; id IDGenerator; nonce NonceGenerator; sign Signer; deliver Delivery; now func() time.Time }

type ServiceOption func(*Service)
func WithIDGenerator(value IDGenerator) ServiceOption { return func(s *Service) { s.id = value } }
func WithNonceGenerator(value NonceGenerator) ServiceOption { return func(s *Service) { s.nonce = value } }
func WithSigner(value Signer) ServiceOption { return func(s *Service) { s.sign = value } }
func WithDelivery(value Delivery) ServiceOption { return func(s *Service) { s.deliver = value } }
func WithClock(value func() time.Time) ServiceOption { return func(s *Service) { s.now = value } }

func NewService(policy Policy, options ...ServiceOption) (*Service, error) {
	s := &Service{policy: policy, id: RandomID, nonce: RandomNonce, now: func() time.Time { return time.Now().UTC() }}
	for _, option := range options { if option != nil { option(s) } }
	if policy.DeviceID == "" || policy.MaxValidity <= 0 || s.id == nil || s.nonce == nil || s.sign == nil || s.deliver == nil || s.now == nil { return nil, fmt.Errorf("command service: invalid configuration") }
	return s, nil
}

func (s *Service) Create(ctx context.Context, request Request) (Command, error) {
	if err := ctx.Err(); err != nil { return Command{}, err }
	return Build(s.policy, request, s.now(), s.id, s.nonce, s.sign)
}

func (s *Service) Deliver(ctx context.Context, request Request) (Command, error) {
	c, err := s.Create(ctx, request); if err != nil { return Command{}, err }
	if err := s.deliver.Deliver(ctx, c); err != nil { return Command{}, fmt.Errorf("%w: %v", ErrDeliveryFailed, err) }
	return c, nil
}
