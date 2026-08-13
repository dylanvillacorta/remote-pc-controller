package windows

import (
	"context"
	"os/exec"
)

// HibernateExecutor is the Windows adapter that performs the only allowed action.
type HibernateExecutor struct{}

func NewHibernateExecutor() HibernateExecutor {
	return HibernateExecutor{}
}

func (HibernateExecutor) Hibernate(ctx context.Context) error {
	return exec.CommandContext(ctx, "shutdown.exe", "/h").Run()
}
