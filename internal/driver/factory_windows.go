//go:build windows

package driver

import (
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/driver/windows"
)

func New(customCS2Path string) (Driver, error) {
	return windows.NewWindowsDriver(customCS2Path)
}
