//go:build !windows

package notify

func NewWindowsNotifier() Notifier {
	return NewNoOp()
}
