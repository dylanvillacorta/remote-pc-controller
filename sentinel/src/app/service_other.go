//go:build !windows

package app

import "log"

func runAsWindowsService(*log.Logger) error { return nil }
func windowsServiceMode() bool              { return false }
