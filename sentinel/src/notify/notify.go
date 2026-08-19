package notify

// Notifier defines the interface for emitting desktop notifications.
type Notifier interface {
	NotifyValidationFailure(commandID, reason string)
	NotifyActionExecuted(action, deviceID string)
}

type noOpNotifier struct{}

func (n *noOpNotifier) NotifyValidationFailure(commandID, reason string) {}
func (n *noOpNotifier) NotifyActionExecuted(action, deviceID string)      {}

// NewNoOp returns a notifier that discards all notifications.
func NewNoOp() Notifier {
	return &noOpNotifier{}
}
