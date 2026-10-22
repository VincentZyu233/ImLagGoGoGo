package windows

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	runRegistryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
	appName        = "ImLagGoGoGo"
)

// SetAutoStart 配置开机自启动
func SetAutoStart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open run registry key: %w", err)
	}
	defer k.Close()

	if enable {
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("get executable path: %w", err)
		}
		// 开机静默启动参数
		cmd := fmt.Sprintf(`"%s" --silent`, exePath)
		return k.SetStringValue(appName, cmd)
	}

	// 取消自启
	_ = k.DeleteValue(appName)
	return nil
}

// IsAutoStartEnabled 查询当前是否已开启开机自启
func IsAutoStartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(appName)
	return err == nil
}
