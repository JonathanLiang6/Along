//go:build !windows

package main

import "errors"

// SetAutoStart 在非 Windows 平台为 no-op
func SetAutoStart(enabled bool) error {
	return errors.New("开机启动功能仅在 Windows 上可用")
}

// IsAutoStartEnabled 在非 Windows 平台始终返回 false
func IsAutoStartEnabled() (bool, error) {
	return false, nil
}
