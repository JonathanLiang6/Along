package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

// SetAutoStart 设置/取消开机启动
// 通过 Windows 注册表 HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run 实现
func SetAutoStart(enabled bool) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	k, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.SET_VALUE|registry.QUERY_VALUE,
	)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %w", err)
	}
	defer k.Close()

	if enabled {
		// 写入 AppName -> exe 路径
		if err := k.SetStringValue("AICompanion", exe); err != nil {
			return fmt.Errorf("设置开机启动失败: %w", err)
		}
	} else {
		// 删除开机启动项
		if err := k.DeleteValue("AICompanion"); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("取消开机启动失败: %w", err)
		}
	}
	return nil
}

// IsAutoStartEnabled 检查是否已设置开机启动
func IsAutoStartEnabled() (bool, error) {
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	defer k.Close()

	_, _, err = k.GetStringValue("AICompanion")
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
