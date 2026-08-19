package windows

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// HibernateExecutor is the Windows adapter that performs the only allowed action.
type HibernateExecutor struct{}

func NewHibernateExecutor() HibernateExecutor {
	return HibernateExecutor{}
}

func (HibernateExecutor) Hibernate(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "shutdown.exe", "/h")
	if out, err := cmd.CombinedOutput(); err != nil {
		outStr := strings.TrimSpace(string(out))
		if outStr != "" {
			return fmt.Errorf("shutdown /h failed (%w): %s", err, outStr)
		}
		return fmt.Errorf("shutdown /h failed: %w", err)
	}
	return nil
}
