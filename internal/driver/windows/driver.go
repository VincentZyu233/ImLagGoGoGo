package windows

type WindowsDriver struct {
	*CS2Tracker
	*WindowsInjector
	*GsiConfigWriter
	locator *CS2Locator
}

func NewWindowsDriver(customCS2Path string) (*WindowsDriver, error) {
	locator := NewCS2Locator(customCS2Path)
	tracker := NewCS2Tracker()
	injector := NewWindowsInjector()
	gsiWriter := NewGsiConfigWriter(locator)

	return &WindowsDriver{
		CS2Tracker:      tracker,
		WindowsInjector: injector,
		GsiConfigWriter: gsiWriter,
		locator:         locator,
	}, nil
}

func (d *WindowsDriver) LocateCS2Path() (string, error) {
	return d.locator.LocateCS2Path()
}

func (d *WindowsDriver) ChooseCS2PathViaDialog() (string, error) {
	return d.locator.ChooseCS2PathViaDialog()
}
