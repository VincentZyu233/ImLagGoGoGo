package driver

// Driver 是操作系统的顶层统一契约，对上层业务完全隐藏平台细节
type Driver interface {
	WindowTracker
	InputInjector
	CfgLocator
}

// WindowTracker 负责检测 CS2 游戏是否处于当前前台窗口
type WindowTracker interface {
	IsCS2Foreground() bool
}

// InputInjector 负责向游戏窗口模拟按键与注入 Unicode 借口文字
type InputInjector interface {
	SendChatMessage(chatKey string, message string, delayMs int) error
}

// CfgLocator 负责探测 CS2 路径并自动生成部署 GSI 与 CFG 配置文件
type CfgLocator interface {
	LocateCS2Path() (string, error)
	InstallGSIConfig(listenPort int) error
	WriteSelectorCfg(messages []string, bindKeys []string) error
	ChooseCS2PathViaDialog() (string, error)
}
