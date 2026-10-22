package linux

import (
	"fmt"
)

type LinuxDriver struct{}

func NewLinuxDriver(_ string) (*LinuxDriver, error) {
	return &LinuxDriver{}, nil
}

func (d *LinuxDriver) IsCS2Foreground() bool {
	// 预留: X11 / Wayland 前台窗口检测
	return true
}

func (d *LinuxDriver) SendChatMessage(chatKey string, message string, delayMs int) error {
	// 预留: /dev/uinput 与剪贴板注入
	return fmt.Errorf("linux input injector is not implemented yet")
}

func (d *LinuxDriver) LocateCS2Path() (string, error) {
	return "", fmt.Errorf("linux cs2 locator is not implemented yet")
}

func (d *LinuxDriver) InstallGSIConfig(listenPort int) error {
	return fmt.Errorf("linux gsi installer is not implemented yet")
}

func (d *LinuxDriver) WriteSelectorCfg(messages []string, bindKeys []string) error {
	return fmt.Errorf("linux cfg writer is not implemented yet")
}

func (d *LinuxDriver) ChooseCS2PathViaDialog() (string, error) {
	return "", fmt.Errorf("linux file dialog is not implemented yet")
}
