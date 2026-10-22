//go:build linux

package driver

import (
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/driver/linux"
)

func New(customCS2Path string) (Driver, error) {
	return linux.NewLinuxDriver(customCS2Path)
}
